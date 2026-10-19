//go:build unix

package localfs

import (
	"errors"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
)

// TestCappedReaderRefusesAPipeWithoutOpeningIt: opening a pipe for reading
// blocks until a writer appears, so the type check must come first.
func TestCappedReaderRefusesAPipeWithoutOpeningIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pipe")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)

	go func() {
		_, err := CappedReader{}.ReadCapped(t.Context(), path, 10)
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, capture.ErrNotRegularFile) {
			t.Fatalf("err = %v, want ErrNotRegularFile", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the reader blocked on a pipe")
	}
}

func TestSaltFileRefusesAPipeWithoutOpeningIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pipe")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)

	go func() {
		_, err := NewSaltFile(path).Salt()
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, ErrSaltCorrupt) {
			t.Fatalf("err = %v, want ErrSaltCorrupt", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the salt read blocked on a pipe")
	}
}
