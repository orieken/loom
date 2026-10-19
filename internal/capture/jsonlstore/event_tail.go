package jsonlstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"slices"
	"time"

	"github.com/orieken/loom/internal/capture"
)

var _ capture.EventTail = Tail{}

// Tail reads the end of the day files, backward. It answers the question
// "which events are inside the Correction window" without reading the whole
// file (performance-report.md section 3, finding 2).
type Tail struct {
	layout capture.Layout
}

// NewTail builds a tail reader for one project layout.
func NewTail(layout capture.Layout) Tail {
	return Tail{layout: layout}
}

// tailRecord lists the only fields the hook needs from a stored line.
type tailRecord struct {
	ID         string            `json:"id"`
	V          int               `json:"v"`
	At         time.Time         `json:"at"`
	Kind       capture.EventKind `json:"kind"`
	Tool       string            `json:"tool"`
	PathRef    string            `json:"pathRef"`
	Path       *string           `json:"path"`
	RangeStart int               `json:"rangeStart"`
	RangeEnd   int               `json:"rangeEnd"`
	PreHash    string            `json:"preHash"`
	PostHash   string            `json:"postHash"`
}

// Recent returns the events newer than now minus the window, oldest first.
// It reads at most MaxTailBytes over all files. When the window reaches back
// over midnight UTC, it reads the file of the day before as well.
func (t Tail) Recent(ctx context.Context, now time.Time, window time.Duration) ([]capture.SessionEvent, error) {
	cutoff := now.Add(-window)
	budget := capture.MaxTailBytes
	today, isDone, used, err := readTailFile(ctx, t.layout.DayFile(now), cutoff, budget)
	if err != nil || isDone || !reachesPreviousDay(now, cutoff) {
		return today, err
	}
	yesterday, _, _, err := readTailFile(ctx, t.layout.DayFile(now.AddDate(0, 0, -1)), cutoff, budget-used)
	return append(yesterday, today...), err
}

func reachesPreviousDay(now, cutoff time.Time) bool {
	startOfDay := now.UTC().Truncate(24 * time.Hour)
	return cutoff.Before(startOfDay)
}

// readTailFile reads a file backward in chunks. It reports whether it met an
// event older than the cutoff, which ends the whole search.
func readTailFile(ctx context.Context, path string, cutoff time.Time, limit int) (events []capture.SessionEvent, isDone bool, used int, err error) {
	file, size, err := openRegular(path)
	if err != nil || file == nil {
		return nil, false, 0, err
	}
	defer file.Close()
	scan := &backwardScan{file: file, position: size, cutoff: cutoff, limit: limit}
	for scan.hasMore(ctx) {
		if err := scan.readChunk(); err != nil {
			return nil, false, scan.used, err
		}
	}
	slices.Reverse(scan.found)
	return scan.found, scan.isDone, scan.used, nil
}

// openRegular opens a regular file. A missing file and a non-regular file
// give no handle and no error: there is nothing to read. The check comes
// first because opening a pipe can block.
func openRegular(path string) (*os.File, int64, error) {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.Mode().IsRegular()) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	file, err := os.Open(path)
	return file, info.Size(), err
}

// backwardScan holds the state of one backward read.
type backwardScan struct {
	file     *os.File
	position int64
	carry    []byte
	used     int
	limit    int
	cutoff   time.Time
	isDone   bool
	isEnd    bool
	found    []capture.SessionEvent
}

func (s *backwardScan) hasMore(ctx context.Context) bool {
	return s.position > 0 && s.used < s.limit && !s.isDone && ctx.Err() == nil
}

// readChunk reads one chunk before the current position and reads the whole
// lines in it, newest first. The first piece of a chunk may be the end of a
// line that began earlier, so it waits in carry for the next chunk. The
// piece after the last newline of the file is a line without a newline: it
// is never complete, so it is dropped.
func (s *backwardScan) readChunk() error {
	size := min(int64(capture.TailChunkBytes), s.position, int64(s.limit-s.used))
	chunk := make([]byte, size)
	if _, err := s.file.ReadAt(chunk, s.position-size); err != nil {
		return err
	}
	s.position -= size
	s.used += int(size)
	pieces := bytes.Split(append(chunk, s.carry...), []byte{'\n'})
	if !s.isEnd && len(pieces) == 1 && s.position > 0 {
		s.carry = pieces[0]
		return nil
	}
	if !s.isEnd {
		pieces, s.isEnd = pieces[:len(pieces)-1], true
	}
	if s.position > 0 {
		s.carry, pieces = pieces[0], pieces[1:]
	}
	s.takeLines(pieces)
	return nil
}

func (s *backwardScan) takeLines(pieces [][]byte) {
	for i := len(pieces) - 1; i >= 0 && !s.isDone; i-- {
		record, isValid := decodeTailLine(pieces[i])
		if !isValid {
			continue
		}
		if record.At.Before(s.cutoff) {
			s.isDone = true
			return
		}
		s.found = append(s.found, record)
	}
}

func decodeTailLine(line []byte) (capture.SessionEvent, bool) {
	var record tailRecord
	if len(line) == 0 || json.Unmarshal(line, &record) != nil || record.V != capture.EventVersion {
		return capture.SessionEvent{}, false
	}
	return capture.SessionEvent{
		V: record.V, ID: record.ID, At: record.At.UTC(), Kind: record.Kind, Tool: record.Tool,
		PathRef: record.PathRef, Path: record.Path,
		RangeStart: record.RangeStart, RangeEnd: record.RangeEnd,
		PreHash: record.PreHash, PostHash: record.PostHash,
	}, true
}
