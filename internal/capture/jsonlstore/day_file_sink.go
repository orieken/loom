// Package jsonlstore stores Session Events in UTC day files. It is an
// adapter: it implements the ports of internal/capture and holds no domain
// rule.
package jsonlstore

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/orieken/loom/internal/capture"
)

const (
	// dayFileMode keeps a day file private to the user.
	dayFileMode os.FileMode = 0o600
	// sessionsDirMode keeps the sessions directory private to the user.
	sessionsDirMode os.FileMode = 0o700
	appendFlags                 = os.O_APPEND | os.O_CREATE | os.O_WRONLY
)

var _ capture.EventSink = DaySink{}

// DaySink appends events to the day file of their UTC date. It takes no
// lock: one O_APPEND write per call keeps the lines whole
// (architecture-notes.md decision 2).
type DaySink struct {
	layout capture.Layout
}

// NewDaySink builds a sink for one project layout.
func NewDaySink(layout capture.Layout) DaySink {
	return DaySink{layout: layout}
}

// Append writes all events of one call with one Write. The first event is
// the main event of the call and must fit. A later event that would push
// the batch over MaxBatchBytes is dropped, with every event after it. That
// loses recall only. The file is chosen by the UTC date of the first event.
func (s DaySink) Append(ctx context.Context, events []capture.SessionEvent) error {
	if len(events) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	batch, err := encodeBatch(events)
	if err != nil {
		return err
	}
	return s.writeBatch(s.layout.DayFile(events[0].At), batch)
}

func (s DaySink) writeBatch(path string, batch []byte) error {
	file, err := s.openForAppend(path)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(batch)
	return errors.Join(writeErr, file.Close())
}

// openForAppend creates the sessions directory when it is the missing part.
// The parent directories are never created: a hook for a root without the
// capture directories must fail, not build them.
func (s DaySink) openForAppend(path string) (*os.File, error) {
	file, err := os.OpenFile(path, appendFlags, dayFileMode)
	if !errors.Is(err, fs.ErrNotExist) {
		return file, err
	}
	if mkdirErr := os.Mkdir(s.layout.SessionsDir(), sessionsDirMode); mkdirErr != nil && !errors.Is(mkdirErr, fs.ErrExist) {
		return nil, mkdirErr
	}
	return os.OpenFile(path, appendFlags, dayFileMode)
}

func encodeBatch(events []capture.SessionEvent) ([]byte, error) {
	batch := make([]byte, 0, capture.MaxBatchBytes)
	for index, event := range events {
		line, err := capture.MarshalLine(event)
		if err != nil && index == 0 {
			return nil, err
		}
		if err != nil || len(batch)+len(line) > capture.MaxBatchBytes {
			break
		}
		batch = append(batch, line...)
	}
	return batch, nil
}
