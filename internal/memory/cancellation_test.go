package memory

import (
	"context"
	"errors"
	"testing"
	"time"
)

// An ended context wins over the error a closed statement reports, so the
// caller sees the deadline and plainSessionError can say what happened.
func TestAnEndedContextReplacesTheStatementError(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	closed := errors.New("insert session correction: sql: statement is closed")

	err := cancellationOr(ctx, closed)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want the deadline", err)
	}
}

func TestALiveContextKeepsTheOriginalError(t *testing.T) {
	failure := errors.New("clear session corrections: disk I/O error")

	err := cancellationOr(context.Background(), failure)

	if !errors.Is(err, failure) {
		t.Errorf("err = %v, want the original error", err)
	}
}
