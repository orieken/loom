package capture

import (
	"context"
	"errors"
	"time"
)

// Ports of the hook use case. They live here, on the consumer side, and the
// adapters implement them (go-conventions.md).

// EventTail answers the query "which events are recent". It returns the
// events of the Correction window in file order, oldest first. An
// implementation reads the end of the day file only.
type EventTail interface {
	Recent(ctx context.Context, now time.Time, window time.Duration) ([]SessionEvent, error)
}

// StallMarker tells the hook that a recent call stalled, so the hook can
// stop at once (architecture-notes.md decision 15).
type StallMarker interface {
	IsStalled(now time.Time) bool
}

// FailureCounter logs one failure. An implementation never returns an
// error: a log that fails must not fail the hook.
type FailureCounter interface {
	Record(cause FailureCause)
}

// RetentionPurger deletes day files that are too old or too large. It does
// nothing when the last purge is younger than PurgeInterval.
type RetentionPurger interface {
	PurgeIfDue(ctx context.Context, now time.Time, retention Retention) error
}

// ConfigSource gives the capture settings. A missing file gives the
// defaults. An invalid file is an error.
type ConfigSource interface {
	Load() (Config, error)
}

// EditTextSource hands the text of an Edit or MultiEdit to the hook for one
// call. The text exists in memory only (see EditText).
type EditTextSource interface {
	EditTextOf(payload []byte) EditText
}

// Errors an adapter returns for the file rules of the hook.
var (
	// ErrFileTooLarge marks a file above the size cap. The file was not read.
	ErrFileTooLarge = errors.New("capture: file exceeds the size cap")
	// ErrNotRegularFile marks a target that is a directory, a pipe or a device.
	ErrNotRegularFile = errors.New("capture: target is not a regular file")
)

// RecordError tells the caller which failure cause to count. It wraps the
// reason and never holds payload text.
type RecordError struct {
	Cause FailureCause
	Err   error
}

// Error gives a stable text without the details of the payload.
func (e *RecordError) Error() string {
	return "capture: record failed: " + e.Err.Error()
}

// Unwrap lets errors.Is see the reason.
func (e *RecordError) Unwrap() error { return e.Err }

func failed(cause FailureCause, err error) error {
	return &RecordError{Cause: cause, Err: err}
}

// CauseOf returns the failure cause of an error. An error without a cause
// counts as a write failure.
func CauseOf(err error) FailureCause {
	var recordErr *RecordError
	if errors.As(err, &recordErr) {
		return recordErr.Cause
	}
	return CauseWrite
}
