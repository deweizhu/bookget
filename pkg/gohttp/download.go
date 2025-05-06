package gohttp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var byteUnits = []string{"B", "KB", "MB", "GB", "TB", "PB"}

func ByteUnitString(n int64) string {
	var unit string
	size := float64(n)
	for i := 1; i < len(byteUnits); i++ {
		if size < 1000 {
			unit = byteUnits[i-1]
			break
		}

		size = size / 1000
	}

	return fmt.Sprintf("%.3g %s", size, unit)
}

// Info holds downloadable file info.
type Info struct {
	Size      uint64
	Rangeable bool
}

// Download holds downloadable file config and infos.
type Download struct {
	ctx            context.Context
	chunks         []*Chunk
	info           *Info
	startedAt      time.Time
	size, lastSize uint64

	Cookie                                          []http.Cookie
	Concurrency                                     int
	StopProgress                                    bool
	URL, Dir, Dest, path, unsafeName                string
	Interval, ChunkSize, MinChunkSize, MaxChunkSize uint64
	opts                                            Options
	mutex                                           *sync.RWMutex
}

// TotalSize returns file total size (0 if unknown).
func (d *Download) TotalSize() uint64 {
	return atomic.LoadUint64(&d.info.Size)
}

// Size returns downloaded size.
func (d *Download) Size() uint64 {
	return atomic.LoadUint64(&d.size)
}

// Speed returns download speed.
func (d *Download) Speed() uint64 {
	return (atomic.LoadUint64(&d.size) - atomic.LoadUint64(&d.lastSize)) / d.Interval * 1000
}

// AvgSpeed returns average download speed.
func (d *Download) AvgSpeed() uint64 {

	if totalMills := d.TotalCost().Milliseconds(); totalMills > 0 {
		return uint64(atomic.LoadUint64(&d.size) / uint64(totalMills) * 1000)
	}

	return 0
}

// TotalCost returns download duration.
func (d *Download) TotalCost() time.Duration {
	return time.Now().Sub(d.startedAt)
}

// Write updates progress size.
func (d *Download) Write(b []byte) (int, error) {
	n := len(b)
	atomic.AddUint64(&d.size, uint64(n))
	return n, nil
}

// IsRangeable returns file server partial content support state.
func (d *Download) IsRangeable() bool {
	return d.info.Rangeable
}

func (r *Response) dlFile(d *Download) (size int64, err error) {
	defer func(d *Download) {
		d.StopProgress = true
	}(d)
	//if r.resp.StatusCode != 200 || r.resp.ContentLength == -1 {
	//	return 0, errors.New(r.resp.Status)
	//}
	var destTemp = fmt.Sprintf("%s.downloading", d.Dest)
	file, err := os.Create(destTemp)
	defer os.Rename(destTemp, d.Dest)
	defer file.Close()
	if err != nil {
		return
	}
	// Allocate the file completely so that we can write concurrently
	file.Truncate(r.resp.ContentLength)
	size, err = io.Copy(file, io.TeeReader(r.resp.Body, d))
	return
}
func dlProgressBar(wg *sync.WaitGroup, d *Download) {
	defer wg.Done()
	// Set default interval.
	if d.Interval == 0 {
		d.Interval = uint64(400 / runtime.NumCPU())
	}
	sleepd := time.Duration(d.Interval) * time.Millisecond
	for {
		// Run progress func.
		if d.TotalSize() <= 0 {
			return
		}
		pd := d.Size() * 100 / d.TotalSize()
		speed := "="
		max := int(pd)
		for k := 0; k < max; k += 2 {
			speed += "="
		}
		speed += ">"
		after := 50 - len(speed)
		for k := 0; k < after; k++ {
			speed += " "
		}
		fmt.Fprintf(os.Stdout, "\r%d%%[%s]  %s/%s  %s/s    in %s", pd, speed, ByteUnitString(int64(d.Size())),
			ByteUnitString(int64(d.TotalSize())), ByteUnitString(int64(d.AvgSpeed())), d.TotalCost())

		// Update last size
		atomic.StoreUint64(&d.lastSize, atomic.LoadUint64(&d.size))
		//stop
		if pd == 100 || d.StopProgress {
			d.StopProgress = true
			break
		}
		// Context check.
		select {
		case <-d.ctx.Done():
			return
		default:
		}
		// Interval.
		time.Sleep(sleepd)
	}
}
