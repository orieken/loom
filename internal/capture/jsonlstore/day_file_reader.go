package jsonlstore

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"time"

	"github.com/orieken/loom/internal/capture"
)

// readerBufferBytes is the explicit buffer of the ingest reader. It is
// larger than MaxLineBytes, so a line over the cap shows up as a full
// buffer and not as a hidden scanner limit (performance-report.md mandate G).
const readerBufferBytes = 2 * capture.MaxLineBytes

// ReadStats counts what the reader skipped. The reader never fails on these.
type ReadStats struct {
	// Events is the number of lines that decoded and were passed on.
	Events int
	// TornLines counts lines that do not parse, lines over MaxLineBytes and
	// a final line with no newline.
	TornLines int
	// UnknownVersionLines counts lines whose "v" this reader does not know.
	UnknownVersionLines int
	// ClockAnomalies counts events earlier than the event before them by more
	// than ClockAnomalyThreshold.
	ClockAnomalies int
}

// Add sums the counts of two files.
func (s ReadStats) Add(other ReadStats) ReadStats {
	return ReadStats{
		Events:              s.Events + other.Events,
		TornLines:           s.TornLines + other.TornLines,
		UnknownVersionLines: s.UnknownVersionLines + other.UnknownVersionLines,
		ClockAnomalies:      s.ClockAnomalies + other.ClockAnomalies,
	}
}

// ReadDayFile streams one day file in file order and calls visit for each
// valid event. It holds one line in memory at a time. A visit error stops
// the read and is returned.
func ReadDayFile(ctx context.Context, path string, visit func(capture.SessionEvent) error) (ReadStats, error) {
	file, err := os.Open(path)
	if err != nil {
		return ReadStats{}, err
	}
	defer file.Close()
	return streamLines(ctx, bufio.NewReaderSize(file, readerBufferBytes), visit)
}

func streamLines(ctx context.Context, reader *bufio.Reader, visit func(capture.SessionEvent) error) (ReadStats, error) {
	var stats ReadStats
	var previous time.Time
	for {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		line, state, err := nextLine(reader)
		if err != nil {
			return stats, err
		}
		if state == lineEnd {
			return stats, nil
		}
		event, isEvent := stats.classify(line, state)
		if !isEvent {
			continue
		}
		stats.noteClock(event, previous)
		previous = event.At
		if err := visit(event); err != nil {
			return stats, err
		}
	}
}

type lineState int

const (
	lineComplete lineState = iota
	lineTooLong
	lineUnterminated
	lineEnd
)

// nextLine reads one line. A line over the cap is skipped to its newline and
// reported as too long. A final line without a newline is reported as
// unterminated.
func nextLine(reader *bufio.Reader) ([]byte, lineState, error) {
	line, err := reader.ReadSlice('\n')
	switch {
	case errors.Is(err, bufio.ErrBufferFull):
		return nil, lineTooLong, skipRestOfLine(reader)
	case errors.Is(err, io.EOF) && len(line) > 0:
		return nil, lineUnterminated, nil
	case errors.Is(err, io.EOF):
		return nil, lineEnd, nil
	case err != nil:
		return nil, lineEnd, err
	case len(line) > capture.MaxLineBytes:
		return nil, lineTooLong, nil
	}
	return line, lineComplete, nil
}

func skipRestOfLine(reader *bufio.Reader) error {
	for {
		_, err := reader.ReadSlice('\n')
		if !errors.Is(err, bufio.ErrBufferFull) {
			return skipError(err)
		}
	}
}

// skipError ends the skip at the end of the file, which is not a failure.
func skipError(err error) error {
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

// classify counts a line that is not an event and returns the event of one
// that is.
func (s *ReadStats) classify(line []byte, state lineState) (capture.SessionEvent, bool) {
	if state != lineComplete {
		s.TornLines++
		return capture.SessionEvent{}, false
	}
	event, err := capture.DecodeLine(line)
	switch {
	case errors.Is(err, capture.ErrUnknownVersion):
		s.UnknownVersionLines++
	case err != nil:
		s.TornLines++
	default:
		s.Events++
		return event, true
	}
	return capture.SessionEvent{}, false
}

func (s *ReadStats) noteClock(event capture.SessionEvent, previous time.Time) {
	if !previous.IsZero() && previous.Sub(event.At) > capture.ClockAnomalyThreshold {
		s.ClockAnomalies++
	}
}

// DayFilePaths lists the day files of a layout in date order, oldest first.
// A missing sessions directory is an empty list.
func DayFilePaths(layout capture.Layout) ([]string, error) {
	files, err := NewPurger(layout).dayFiles()
	paths := make([]string, len(files))
	for i, file := range files {
		paths[i] = file.path
	}
	return paths, err
}
