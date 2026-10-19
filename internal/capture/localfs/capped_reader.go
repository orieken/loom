// Package localfs reads local files for the capture hook: capped file reads,
// the Capture salt, the settings file and the system clock. It is an
// adapter and holds no domain rule.
package localfs

import (
	"context"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/orieken/loom/internal/capture"
)

var (
	_ capture.FileReader = CappedReader{}
	_ capture.Clock      = SystemClock{}
)

// handle is the part of an open file that the reader uses. A test replaces
// it to prove that no byte is read from a file over the cap.
type handle interface {
	io.ReadCloser
	Stat() (fs.FileInfo, error)
}

// CappedReader reads a file only when its size is within the cap. The zero
// value reads real files.
type CappedReader struct {
	open func(path string) (handle, error)
}

func (r CappedReader) opener() func(string) (handle, error) {
	if r.open != nil {
		return r.open
	}
	return func(path string) (handle, error) { return os.Open(path) }
}

// ReadCapped returns the content of a regular file of at most maxBytes. It
// checks the size on the open handle, before any read. A larger file gives
// ErrFileTooLarge and no content: reading the first maxBytes of it would
// hash the wrong lines. A pipe, a device or a directory gives
// ErrNotRegularFile, and is checked by path first because opening a pipe
// can block.
func (r CappedReader) ReadCapped(ctx context.Context, path string, maxBytes int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := requireRegular(path); err != nil {
		return nil, err
	}
	file, err := r.opener()(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readWithinCap(file, maxBytes)
}

func requireRegular(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return capture.ErrNotRegularFile
	}
	return nil
}

func readWithinCap(file handle, maxBytes int64) ([]byte, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, capture.ErrNotRegularFile
	}
	if info.Size() > maxBytes {
		return nil, capture.ErrFileTooLarge
	}
	content, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err == nil && int64(len(content)) > maxBytes {
		return nil, capture.ErrFileTooLarge
	}
	return content, err
}

// SystemClock gives the real time in UTC.
type SystemClock struct{}

// Now returns the current instant in UTC.
func (SystemClock) Now() time.Time { return time.Now().UTC() }
