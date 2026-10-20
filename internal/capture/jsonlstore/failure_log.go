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

// FailureLog is the append-only failure log. One line is one failure: the UTC
// time and a cause from the closed set (capture.FailureCause). It keeps the
// history that `loom health` summarises, and it holds the stall marker of the
// fast-fail breaker (architecture-notes.md decision 15).
//
// The append path never trims the file. The Stop-time purge does (see
// trimFailureLog), so the hook stays one short write.
type FailureLog struct {
	path  string
	clock capture.Clock
}

// NewFailureLog builds the failure log of one project layout.
func NewFailureLog(layout capture.Layout, clock capture.Clock) FailureLog {
	return FailureLog{path: layout.FailureLogPath(), clock: clock}
}

// Record appends one line with one Write. A log that cannot write stays
// silent: the hook must not fail because it cannot log.
func (l FailureLog) Record(cause capture.FailureCause) {
	if !isUsableLogPath(l.path) {
		return
	}
	file, err := os.OpenFile(l.path, appendFlags, dayFileMode)
	if err != nil {
		return
	}
	line := capture.NewFailureEntry(l.clock.Now(), cause).Line()
	_, _ = file.WriteString(line)
	_ = file.Close()
}

// IsStalled reports whether the newest line is a stall younger than
// StallWindow. It reads the end of the file only (capture.FailureTailBytes).
// Only a regular file counts, because opening a pipe can block. A call that
// finds the breaker open writes nothing, so the line keeps its age and the
// breaker closes by itself.
func (l FailureLog) IsStalled(now time.Time) bool {
	tail, err := readTailOf(l.path, capture.FailureTailBytes)
	if err != nil {
		return false
	}
	entry, isParsed := lastEntryOf(tail)
	return isParsed && entry.Cause == capture.CauseStall && isFresh(now.Sub(entry.At))
}

// isFresh says whether a stall line of this age still holds the breaker open.
// A negative age means the clock moved back or the line is dated in the
// future. Such a line must not hold the breaker, or one bad clock could stop
// the recording for days.
func isFresh(age time.Duration) bool { return age >= 0 && age < capture.StallWindow }

// isUsableLogPath says whether the log may be opened for append. A path that
// does not exist yet is usable. A path that exists must be a regular file,
// because opening a pipe with no reader blocks, and the flag-error path runs
// before the watchdog.
func isUsableLogPath(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return os.IsNotExist(err)
	}
	return info.Mode().IsRegular()
}
