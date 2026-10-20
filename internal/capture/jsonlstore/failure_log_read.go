package jsonlstore

import (
	"bytes"
	"errors"
	"io"
	"os"
	"time"

	"github.com/orieken/loom/internal/capture"
)

var errNotRegularFile = errors.New("jsonlstore: not a regular file")

// FailureSummary is what `loom health` shows of the failure log.
type FailureSummary struct {
	// Count is the number of failures inside the window.
	Count int
	// MostCommon is the cause with the most failures inside the window. A tie
	// goes to the cause that comes first in capture.FailureCauses.
	MostCommon capture.FailureCause
	// Last is the time of the newest failure inside the window.
	Last time.Time
}

// Summarize counts the failures younger than the window. It reads at most
// capture.MaxFailureLogBytes from the end of the log, so a log that the
// purge has not yet trimmed cannot make health slow. A line it cannot parse,
// such as a byte of the old counter format, counts as nothing.
func (l FailureLog) Summarize(now time.Time, window time.Duration) FailureSummary {
	data, err := readTailOf(l.path, capture.MaxFailureLogBytes)
	if err != nil {
		return FailureSummary{}
	}
	perCause := map[capture.FailureCause]int{}
	summary := FailureSummary{}
	for _, entry := range parseEntries(data) {
		if now.Sub(entry.At) >= window {
			continue
		}
		perCause[entry.Cause]++
		summary.Count++
		summary.Last = latest(summary.Last, entry.At)
	}
	summary.MostCommon = mostCommon(perCause)
	return summary
}

func latest(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func mostCommon(perCause map[capture.FailureCause]int) capture.FailureCause {
	var winner capture.FailureCause
	best := 0
	for _, cause := range capture.FailureCauses() {
		if perCause[cause] > best {
			winner, best = cause, perCause[cause]
		}
	}
	return winner
}

// readTailOf reads the last maxBytes of a regular file. A path that is not a
// regular file gives an error and is never opened, because opening a pipe
// can block.
func readTailOf(path string, maxBytes int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errNotRegularFile
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readTail(file, info.Size(), maxBytes)
}

// readTail reads the last maxBytes of size bytes. It never asks the reader
// for more than maxBytes.
func readTail(reader io.ReaderAt, size, maxBytes int64) ([]byte, error) {
	start := max(0, size-maxBytes)
	tail := make([]byte, size-start)
	read, err := reader.ReadAt(tail, start)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return tail[:read], nil
}

// parseEntries returns the parsable lines of a log text, oldest first. The
// first line of a tail read may be cut short; it does not parse and is
// dropped with every other line that Line could not have written.
func parseEntries(data []byte) []capture.FailureEntry {
	var entries []capture.FailureEntry
	for _, line := range bytes.Split(data, []byte("\n")) {
		if entry, isParsed := capture.ParseFailureLine(string(line)); isParsed {
			entries = append(entries, entry)
		}
	}
	return entries
}

// lastEntryOf returns the last complete line of a tail. A tail that does not
// end with a newline ends in a torn line, which is not an entry.
func lastEntryOf(tail []byte) (capture.FailureEntry, bool) {
	text, hasNewline := bytes.CutSuffix(tail, []byte("\n"))
	if !hasNewline {
		return capture.FailureEntry{}, false
	}
	start := bytes.LastIndexByte(text, '\n') + 1
	return capture.ParseFailureLine(string(text[start:]))
}
