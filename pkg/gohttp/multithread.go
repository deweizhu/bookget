package gohttp

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func (r *Request) FastGet(uri string, opts ...Options) (resp *Response, err error) {
	if len(opts) > 0 {
		r.opts = opts[0]
		if !opts[0].Overwrite {
			fi, err := os.Stat(opts[0].DestFile)
			if err == nil && fi.Size() > 0 {
				return nil, nil
			}
		}
		if opts[0].Concurrency == 1 {
			return Get(r.ctx, uri, opts...)
		}
	}
	d := &Download{
		ctx:         r.ctx,
		URL:         uri,
		Dest:        r.opts.DestFile,
		opts:        r.opts,
		Concurrency: r.opts.Concurrency,
	}
	d.mutex = new(sync.RWMutex)
	//多线程下载
	if err = d.ChunkInit(); err != nil {
		return nil, err
	}
	err = d.ChunkStart()
	resp = &Response{
		resp: nil,
		req:  r.req,
		err:  err,
	}
	return resp, err
}

// Init set defaults and split file into chunks and gets Info,
// you should call Init before Start
func (d *Download) ChunkInit() (err error) {
	// Set start time.
	d.startedAt = time.Now()

	// Set default context.
	if d.ctx == nil {
		d.ctx = context.Background()
	}

	// Get URL info and partial content support state
	if d.info, err = d.GetInfoOrDownload(); err != nil {
		return err
	}

	// Partial content not supported, and the file downladed.
	if d.info.Rangeable == false {
		return nil
	}

	// Set concurrency default.
	if d.Concurrency == 0 {
		d.Concurrency = getDefaultConcurrency()
	}

	// Set default chunk size
	if d.ChunkSize == 0 {
		d.ChunkSize = getDefaultChunkSize(d.info.Size, d.MinChunkSize, d.MaxChunkSize, uint64(d.Concurrency))
	}

	chunksLen := d.info.Size / d.ChunkSize
	d.chunks = make([]*Chunk, 0, chunksLen)

	// Set chunk ranges.
	for i := uint64(0); i < chunksLen; i++ {

		chunk := new(Chunk)
		d.chunks = append(d.chunks, chunk)

		chunk.Start = (d.ChunkSize * i) + i
		chunk.End = chunk.Start + d.ChunkSize
		if chunk.End >= d.info.Size || i == chunksLen-1 {
			chunk.End = d.info.Size - 1
			// Break on last chunk if i < chunksLen
			break
		}
	}

	return nil
}

// Try downloading the first byte of the file using a range request.
// If the server supports range requests, then we'll extract the length info from content-range,
// Otherwise this just downloads the whole file in one go
func (d *Download) GetInfoOrDownload() (*Info, error) {
	if d.opts.Headers == nil {
		d.opts.Headers = make(map[string]interface{})
	}
	d.opts.Headers["Range"] = "bytes=0-0"
	r := NewClient(d.ctx)
	r.Request("GET", d.URL, d.opts)
	_resp, err := r.cli.Do(r.req)
	defer _resp.Body.Close()

	info := &Info{}
	if _resp.ContentLength > 0 {
		atomic.StoreUint64(&info.Size, uint64(_resp.ContentLength))
	}

	if _resp.StatusCode >= 300 {
		return info, fmt.Errorf("Response status code is not ok: %d", _resp.StatusCode)
	}

	// Set content disposition non trusted name
	d.unsafeName = _resp.Header.Get("content-disposition")
	//只使用临时文件
	var destTemp = fmt.Sprintf("%s.downloading", d.Path())
	dest, err := os.Create(destTemp)
	if err != nil {
		return info, err
	}
	defer dest.Close()

	if _, err = io.Copy(dest, io.TeeReader(_resp.Body, d)); err != nil {
		return info, err
	}

	// Get content length from content-range response header,
	// if content-range exists, that means partial content is supported.
	if cr := _resp.Header.Get("content-range"); cr != "" && _resp.ContentLength == 1 {
		l := strings.Split(cr, "/")
		if len(l) == 2 {
			if length, err := strconv.ParseUint(l[1], 10, 64); err == nil {

				return &Info{
					Size:      length,
					Rangeable: true,
				}, nil
			}
		}
		// Make sure the caller knows about the problem and we don't just silently fail
		return info, fmt.Errorf("Response includes content-range header which is invalid: %s", cr)
	}

	return info, nil
}

// Start downloads the file chunks, and merges them.
// Must be called only after init
func (d *Download) ChunkStart() (err error) {
	// If the file was already downloaded during GetInfoOrDownload, then there will be no chunks
	if d.info.Rangeable == false {
		select {
		case <-d.ctx.Done():
			return d.ctx.Err()
		default:
			return nil
		}
	}

	// Otherwise there are always at least 2 chunks
	var destTemp = fmt.Sprintf("%s.downloading", d.Path())
	file, err := os.Create(destTemp)
	if err != nil {
		return err
	}
	defer func() {
		err = file.Close()
		if err == nil {
			os.Rename(destTemp, d.Path())
		}
	}()
	size := d.TotalSize()
	// Allocate the file completely so that we can write concurrently
	file.Truncate(int64(size))

	// Download chunks.
	errs := make(chan error, 1)
	go d.dl(file, errs)

	select {
	case err = <-errs:
	case <-d.ctx.Done():
		err = d.ctx.Err()
	}
	return
}

// Download chunks
func (d *Download) dl(dest io.WriterAt, errC chan error) {

	var (
		// Wait group.
		wg sync.WaitGroup

		// Concurrency limit.
		max = make(chan int, d.Concurrency)
	)
	if d.opts.Headers == nil {
		d.opts.Headers = make(map[string]interface{})
	}
	wg.Add(1)
	go dlProgressBar(&wg, d)

	for i := 0; i < len(d.chunks); i++ {

		max <- 1
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			// Concurrently download and write chunk
			if err := d.DownloadChunk(d.chunks[i], &OffsetWriter{dest, int64(d.chunks[i].Start)}); err != nil {
				errC <- err
				return
			}

			<-max
		}(i)
	}

	wg.Wait()
	errC <- nil
}

// DownloadChunk downloads a file chunk.
func (d *Download) DownloadChunk(c *Chunk, dest io.Writer) error {
	contentRange := fmt.Sprintf("bytes=%d-%d", c.Start, c.End)
	d.mutex.Lock()
	d.opts.Headers["Range"] = contentRange
	r := NewClient(d.ctx)
	r.Request("GET", d.URL, d.opts)
	d.mutex.Unlock()
	resp, err := r.cli.Do(r.req)
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	// Verify the length
	if resp.ContentLength != int64(c.End-c.Start+1) {
		return fmt.Errorf(
			"Range request returned invalid Content-Length: %d however the range was: %s",
			resp.ContentLength, contentRange,
		)
	}
	_, err = io.CopyN(dest, io.TeeReader(resp.Body, d), resp.ContentLength)
	return err
}

// RunProgress runs ProgressFunc based on Interval and updates lastSize.
func (d *Download) RunProgress() {

	//// Set default interval.
	//if d.Interval == 0 {
	//	d.Interval = uint64(400 / runtime.NumCPU())
	//}
	//
	//sleepd := time.Duration(d.Interval) * time.Millisecond

	//for {
	//
	//	if d.StopProgress {
	//		break
	//	}
	//
	//	// Context check.
	//	select {
	//	case <-d.ctx.Done():
	//		return
	//	default:
	//	}
	//
	//	// Run progress func.
	//	dlProgressBar(&wg, d)
	//
	//	// Update last size
	//	atomic.StoreUint64(&d.lastSize, atomic.LoadUint64(&d.size))
	//
	//	// Interval.
	//	time.Sleep(sleepd)
	//}
}
