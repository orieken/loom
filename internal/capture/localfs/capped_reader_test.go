package localfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCappedReaderReadsAFileWithinTheCap(t *testing.T) {
	tests := []struct {
		name    string
		content string
		cap     int64
	}{
		{name: "small file", content: "hello\n", cap: 100},
		{name: "exactly the cap", content: strings.Repeat("x", 64), cap: 64},
		{name: "empty file", content: "", cap: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTemp(t, "f.txt", tt.content)

			got, err := CappedReader{}.ReadCapped(t.Context(), path, tt.cap)

			if err != nil || string(got) != tt.content {
				t.Fatalf("got %q, err %v; want %q", got, err, tt.content)
			}
		})
	}
}

func TestCappedReaderRefusesAFileOverTheCapAndReturnsNoContent(t *testing.T) {
	path := writeTemp(t, "big.txt", strings.Repeat("x", 65))

	got, err := CappedReader{}.ReadCapped(t.Context(), path, 64)

	if !errors.Is(err, capture.ErrFileTooLarge) || got != nil {
		t.Fatalf("got %d bytes, err %v; want ErrFileTooLarge and no content", len(got), err)
	}
}

// countingHandle reports a size and fails the test when anything is read.
type countingHandle struct {
	t    *testing.T
	size int64
	mode fs.FileMode
	read int
}

func (h *countingHandle) Read([]byte) (int, error) {
	h.read++
	h.t.Error("a file over the cap must not be read at all")
	return 0, errors.New("read")
}
func (h *countingHandle) Close() error { return nil }
func (h *countingHandle) Stat() (fs.FileInfo, error) {
	return fakeInfo{size: h.size, mode: h.mode}, nil
}

type fakeInfo struct {
	size int64
	mode fs.FileMode
}

func (i fakeInfo) Name() string       { return "f" }
func (i fakeInfo) Size() int64        { return i.size }
func (i fakeInfo) Mode() fs.FileMode  { return i.mode }
func (i fakeInfo) ModTime() time.Time { return time.Time{} }
func (i fakeInfo) IsDir() bool        { return false }
func (i fakeInfo) Sys() any           { return nil }

func TestCappedReaderChecksTheSizeOnTheOpenHandleBeforeAnyRead(t *testing.T) {
	path := writeTemp(t, "f.txt", "x")
	fake := &countingHandle{t: t, size: capture.MaxTargetFileBytes + 1}
	reader := CappedReader{open: func(string) (handle, error) { return fake, nil }}

	got, err := reader.ReadCapped(t.Context(), path, capture.MaxTargetFileBytes)

	if !errors.Is(err, capture.ErrFileTooLarge) || got != nil || fake.read != 0 {
		t.Fatalf("got %d bytes, err %v, reads %d; want ErrFileTooLarge and zero reads", len(got), err, fake.read)
	}
}

func TestCappedReaderRefusesAHandleThatIsNotARegularFile(t *testing.T) {
	path := writeTemp(t, "f.txt", "x")
	fake := &countingHandle{t: t, size: 1, mode: fs.ModeNamedPipe}
	reader := CappedReader{open: func(string) (handle, error) { return fake, nil }}

	_, err := reader.ReadCapped(t.Context(), path, 10)

	if !errors.Is(err, capture.ErrNotRegularFile) {
		t.Fatalf("err = %v, want ErrNotRegularFile", err)
	}
}

func TestCappedReaderRefusesAFileThatGrewPastTheCapWhileReading(t *testing.T) {
	path := writeTemp(t, "f.txt", strings.Repeat("x", 50))
	reader := CappedReader{open: func(p string) (handle, error) {
		file, err := os.Open(p)
		return &liarHandle{File: file}, err
	}}

	_, err := reader.ReadCapped(t.Context(), path, 10)

	if !errors.Is(err, capture.ErrFileTooLarge) {
		t.Fatalf("err = %v, want ErrFileTooLarge: the stat said 1 byte, the file had more", err)
	}
}

// liarHandle reports a tiny size, as a file that grows after the stat does.
type liarHandle struct{ *os.File }

func (h *liarHandle) Stat() (fs.FileInfo, error) {
	return fakeInfo{size: 1, mode: 0}, nil
}

func TestCappedReaderFileErrors(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name string
		path string
		want error
	}{
		{name: "missing file", path: filepath.Join(dir, "none"), want: fs.ErrNotExist},
		{name: "directory", path: dir, want: capture.ErrNotRegularFile},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CappedReader{}.ReadCapped(t.Context(), tt.path, 100)

			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestCappedReaderStopsOnACancelledContext(t *testing.T) {
	path := writeTemp(t, "f.txt", "x")
	cancelled, cancel := cancelContext(t)
	defer cancel()

	if _, err := (CappedReader{}).ReadCapped(cancelled, path, 10); err == nil {
		t.Fatal("a cancelled context must stop the read")
	}
}

func TestCappedReaderOpenErrorIsReturned(t *testing.T) {
	path := writeTemp(t, "f.txt", "x")
	boom := errors.New("open failed")
	reader := CappedReader{open: func(string) (handle, error) { return nil, boom }}

	if _, err := reader.ReadCapped(t.Context(), path, 10); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the open error", err)
	}
}

func TestSystemClockGivesUTCNow(t *testing.T) {
	before := time.Now()
	got := SystemClock{}.Now()

	if got.Location() != time.UTC || got.Before(before.Add(-time.Second)) || got.After(time.Now().Add(time.Second)) {
		t.Fatalf("Now = %v, want the current time in UTC", got)
	}
}
