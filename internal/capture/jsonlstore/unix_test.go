//go:build unix

package jsonlstore

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
)

// withOpenUmask lets the test see the mode the code asked for, not the mode
// the umask of the shell left.
func withOpenUmask(t *testing.T) {
	t.Helper()
	previous := syscall.Umask(0)
	t.Cleanup(func() { syscall.Umask(previous) })
}

func TestStoreFilesAreCreatedPrivateEvenWithAnOpenUmask(t *testing.T) {
	withOpenUmask(t)
	layout := newLayout(t)
	at := time.Now().UTC()

	_ = NewDaySink(layout).Append(t.Context(), []capture.SessionEvent{event("a", at, capture.EventStop, 0)})
	NewFailureLog(layout, fixedClock{}).Record(capture.CauseDecode)
	_ = NewPurger(layout).PurgeIfDue(t.Context(), time.Now(), capture.DefaultConfig().Retention)

	assertMode(t, layout.SessionsDir(), 0o700)
	assertMode(t, layout.DayFile(at), 0o600)
	assertMode(t, layout.FailureLogPath(), 0o600)
	assertMode(t, layout.PurgeStampPath(), 0o600)
}

// makePipe creates a named pipe. Opening one for writing blocks until a
// reader opens it, which is how a test models a stalled disk.
func makePipe(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTailSkipsADayFileThatIsAPipeWithoutBlocking(t *testing.T) {
	layout := newLayout(t)
	now := time.Now().UTC()
	makePipe(t, layout.DayFile(now))
	done := make(chan error, 1)

	go func() {
		_, err := NewTail(layout).Recent(t.Context(), now, capture.DefaultCorrectionWindow)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("err = %v, want nil: a pipe holds nothing to read", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the tail read blocked on a pipe")
	}
}

func TestFailureLogIgnoresAPipeInsteadOfBlocking(t *testing.T) {
	layout := newLayout(t)
	makePipe(t, layout.FailureLogPath())
	done := make(chan bool, 1)

	go func() { done <- NewFailureLog(layout, fixedClock{}).IsStalled(time.Now()) }()

	select {
	case isStalled := <-done:
		if isStalled {
			t.Fatal("a pipe must not count as a stall marker")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the breaker check blocked on a pipe")
	}
}

func TestTrimIgnoresALogThatIsAPipe(t *testing.T) {
	layout := newLayout(t)
	makePipe(t, layout.FailureLogPath())
	done := make(chan error, 1)

	go func() { done <- trimFailureLog(layout, logNow, trimRetention) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the trim blocked on a pipe")
	}
}
