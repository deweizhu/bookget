package gohttp

import (
	"io"
	"path/filepath"
	"runtime"
)

type OffsetWriter struct {
	io.WriterAt
	offset int64
}

func (dst *OffsetWriter) Write(b []byte) (n int, err error) {
	n, err = dst.WriteAt(b, dst.offset)
	dst.offset += int64(n)
	return
}

// Chunk represents the partial content range
type Chunk struct {
	Start, End uint64
}

// Return constant path which will not change once the download starts
func (d *Download) Path() string {

	// Set the default path
	if d.path == "" {

		if d.Dest != "" {
			d.path = d.Dest
		} else if d.unsafeName != "" {
			d.path = getNameFromHeader(d.unsafeName)
		} else {
			d.path = getFilename(d.URL)
		}
		d.path = filepath.Join(d.Dir, d.path)
	}

	return d.path
}

func getDefaultConcurrency() int {
	c := runtime.NumCPU() * 2
	return c
}

func getDefaultChunkSize(totalSize, min, max, concurrency uint64) uint64 {

	cs := totalSize / concurrency

	// if chunk size >= 102400000 bytes set default to (ChunkSize / 2)
	if cs >= 102400000 {
		cs = cs / 2
	}

	// Set default min chunk size to 2m, or file size / 2
	if min == 0 {

		min = 2097152

		if min >= totalSize {
			min = totalSize / 2
		}
	}

	// if Chunk size < Min size set chunk size to min.
	if cs < min {
		cs = min
	}

	// Change ChunkSize if MaxChunkSize are set and ChunkSize > Max size
	if max > 0 && cs > max {
		cs = max
	}

	// When chunk size > total file size, divide chunk / 2
	if cs >= totalSize {
		cs = totalSize / 2
	}

	return cs
}
