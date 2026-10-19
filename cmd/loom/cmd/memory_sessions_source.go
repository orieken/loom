package cmd

// The day-file adapter of memory.SessionSource. It lives in the command
// layer so internal/memory never imports the file adapter.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"time"

	"github.com/orieken/loom/internal/capture"
	"github.com/orieken/loom/internal/capture/jsonlstore"
	"github.com/orieken/loom/internal/memory"
)

var _ memory.SessionSource = daySessionSource{}

// daySessionSource reads the day files of one project layout.
type daySessionSource struct {
	layout capture.Layout
}

func newDaySessionSource(layout capture.Layout) daySessionSource {
	return daySessionSource{layout: layout}
}

// Stream reads every day file in date order and sums the skip counts.
func (d daySessionSource) Stream(ctx context.Context, visit func(capture.SessionEvent) error) (memory.SessionReadStats, error) {
	paths, err := jsonlstore.DayFilePaths(d.layout)
	if err != nil {
		return memory.SessionReadStats{}, err
	}
	var total jsonlstore.ReadStats
	for _, path := range paths {
		stats, err := jsonlstore.ReadDayFile(ctx, path, visit)
		total = total.Add(stats)
		if err != nil {
			return skipCounts(total), err
		}
	}
	return skipCounts(total), nil
}

func skipCounts(stats jsonlstore.ReadStats) memory.SessionReadStats {
	return memory.SessionReadStats{
		TornLines:           stats.TornLines,
		UnknownVersionLines: stats.UnknownVersionLines,
		ClockAnomalies:      stats.ClockAnomalies,
	}
}

// HasDayFiles reports whether at least one day file exists.
func (d daySessionSource) HasDayFiles() (bool, error) {
	paths, err := jsonlstore.DayFilePaths(d.layout)
	return len(paths) > 0, err
}

// NewestEventAt reads the end of the newest day file.
func (d daySessionSource) NewestEventAt(ctx context.Context) (time.Time, bool, error) {
	paths, err := jsonlstore.DayFilePaths(d.layout)
	if err != nil || len(paths) == 0 {
		return time.Time{}, false, err
	}
	tail, err := readFileTail(paths[len(paths)-1], memory.HealthTailBytes)
	if err != nil {
		return time.Time{}, false, err
	}
	at, isFound := lastCompleteEventTime(tail)
	return at, isFound, ctx.Err()
}

// readFileTail returns the last limit bytes of a file, or all of it when it
// is smaller. The result says whether the start of the file was cut off.
func readFileTail(path string, limit int64) (fileTail, error) {
	file, err := os.Open(path)
	if err != nil {
		return fileTail{}, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return fileTail{}, err
	}
	start := max(info.Size()-limit, 0)
	content := make([]byte, info.Size()-start)
	if _, err := file.ReadAt(content, start); err != nil && !errors.Is(err, io.EOF) {
		return fileTail{}, err
	}
	return fileTail{content: content, isStartCut: start > 0}, nil
}

type fileTail struct {
	content    []byte
	isStartCut bool
}

// lastCompleteEventTime finds the time of the last complete valid line. It
// drops the final piece after the last newline (a torn write) and, when the
// read began inside the file, the first piece (a partial line).
func lastCompleteEventTime(tail fileTail) (time.Time, bool) {
	pieces := bytes.Split(tail.content, []byte{'\n'})
	pieces = pieces[:len(pieces)-1]
	if tail.isStartCut && len(pieces) > 0 {
		pieces = pieces[1:]
	}
	for i := len(pieces) - 1; i >= 0; i-- {
		if event, err := capture.DecodeLine(pieces[i]); err == nil {
			return event.At, true
		}
	}
	return time.Time{}, false
}
