package jsonlstore

import (
	"os"
	"time"

	"github.com/orieken/loom/internal/capture"
)

var (
	_ capture.FailureCounter = FailureLog{}
	_ capture.StallMarker    = FailureLog{}
)

// FailureLog is the append-only failure counter. One byte is one failure and
// the byte says why. The file size is the count and the mtime is the time of
// the last failure. It also holds the stall marker of the fast-fail breaker
// (architecture-notes.md decision 15).
type FailureLog struct {
	path string
}

// NewFailureLog builds the counter of one project layout.
func NewFailureLog(layout capture.Layout) FailureLog {
	return FailureLog{path: layout.FailuresPath()}
}

// Record appends one cause byte. A counter that cannot write stays silent:
// the hook must not fail because it cannot count.
func (l FailureLog) Record(cause capture.FailureCause) {
	file, err := os.OpenFile(l.path, appendFlags, dayFileMode)
	if err != nil {
		return
	}
	_, _ = file.Write([]byte{byte(cause)})
	_ = file.Close()
}

// IsStalled reports whether the newest failure is a stall younger than
// StallWindow. Only a regular file counts, because opening a pipe can block.
// A call that finds the breaker open writes nothing, so the mtime keeps its
// age and the breaker closes by itself.
func (l FailureLog) IsStalled(now time.Time) bool {
	info, err := os.Stat(l.path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return false
	}
	if now.Sub(info.ModTime()) >= capture.StallWindow {
		return false
	}
	return l.lastByte(info.Size()) == capture.StallCause
}

func (l FailureLog) lastByte(size int64) byte {
	file, err := os.Open(l.path)
	if err != nil {
		return 0
	}
	defer file.Close()
	last := make([]byte, 1)
	if _, err := file.ReadAt(last, size-1); err != nil {
		return 0
	}
	return last[0]
}
