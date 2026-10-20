package jsonlstore

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
)

var logNow = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// lineAt is the exact text the hook must append for one failure.
func lineAt(at time.Time, cause capture.FailureCause) string {
	return capture.NewFailureEntry(at, cause).Line()
}

func logAt(layout capture.Layout, now time.Time) FailureLog {
	return NewFailureLog(layout, fixedClock{now: now})
}

func TestFailureLogAppendsOneFixedFormatLineForEachFailure(t *testing.T) {
	layout := newLayout(t)
	log := logAt(layout, logNow)

	log.Record(capture.CauseSalt)
	log.Record(capture.CauseDecode)
	log.Record(capture.CauseStall)

	want := "2026-10-19T12:00:00.000Z salt\n2026-10-19T12:00:00.000Z decode\n2026-10-19T12:00:00.000Z stall\n"
	if got := readFile(t, layout.FailureLogPath()); got != want {
		t.Fatalf("failure log = %q, want %q", got, want)
	}
}

func TestFailureLogWritesTheTimeInUTC(t *testing.T) {
	layout := newLayout(t)
	plusFive := time.FixedZone("UTC+5", 5*3600)

	logAt(layout, time.Date(2026, 10, 19, 17, 0, 0, 0, plusFive)).Record(capture.CausePanic)

	equal(t, "failure log", readFile(t, layout.FailureLogPath()), "2026-10-19T12:00:00.000Z panic\n")
}

// The code is the only variable part of a line (guardrail #9). A cause that
// is not in the closed set must never reach the file, whatever it holds.
func TestFailureLogWritesACodeOfTheClosedSetAndNothingElse(t *testing.T) {
	layout := newLayout(t)
	canary := capture.FailureCause("CANARY-SECRET /home/user/project/.env: permission denied")

	logAt(layout, logNow).Record(canary)

	got := readFile(t, layout.FailureLogPath())
	if strings.Contains(got, "CANARY") || strings.Contains(got, "/home") {
		t.Fatalf("free text reached the failure log: %q", got)
	}
	equal(t, "failure log", got, lineAt(logNow, capture.CauseWrite))
}

func TestFailureLogStaysSilentWhenItCannotWrite(t *testing.T) {
	log := logAt(capture.NewLayout(t.TempDir()), logNow)

	log.Record(capture.CauseSalt)

	if log.IsStalled(logNow) {
		t.Fatal("a log that never wrote cannot be stalled")
	}
}

func TestFailureLogStallMarker(t *testing.T) {
	stallAt := func(age time.Duration) string { return lineAt(logNow.Add(-age), capture.CauseStall) }
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "stall line, 30 s old", content: stallAt(30 * time.Second), want: true},
		{name: "stall line last of several", content: lineAt(logNow, capture.CauseSalt) + stallAt(time.Second), want: true},
		{name: "stall line, 59 s old", content: stallAt(59 * time.Second), want: true},
		{name: "stall line, exactly 60 s old", content: stallAt(capture.StallWindow), want: false},
		{name: "stall line, 61 s old", content: stallAt(61 * time.Second), want: false},
		{name: "another cause is last", content: stallAt(time.Second) + lineAt(logNow, capture.CauseDecode), want: false},
		{name: "stall line in the future", content: stallAt(-time.Hour), want: true},
		{name: "empty file", content: "", want: false},
		{name: "old one-byte-per-failure format", content: "sdt", want: false},
		{name: "old format, a lone stall byte", content: "t", want: false},
		{name: "torn last line", content: strings.TrimSuffix(stallAt(time.Second), "\n"), want: false},
		{name: "free text", content: "disk exploded\n", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := newLayout(t)
			writeFile(t, layout.FailureLogPath(), tt.content)

			got := logAt(layout, logNow).IsStalled(logNow)

			equal(t, "IsStalled", got, tt.want)
		})
	}
}

func TestFailureLogMissingFileIsNotStalled(t *testing.T) {
	if logAt(newLayout(t), logNow).IsStalled(logNow) {
		t.Fatal("a missing file must not trip the breaker")
	}
}

func TestFailureLogCheckWritesNothing(t *testing.T) {
	layout := newLayout(t)
	writeFile(t, layout.FailureLogPath(), lineAt(logNow.Add(-30*time.Second), capture.CauseStall))
	before, _ := os.Stat(layout.FailureLogPath())

	_ = logAt(layout, logNow).IsStalled(logNow)

	after, _ := os.Stat(layout.FailureLogPath())
	if after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("the breaker check must leave the file as it was, or the breaker never closes")
	}
}

// countingReader records every byte range that a reader is asked for.
type countingReader struct {
	data      []byte
	requested int
}

func (c *countingReader) ReadAt(p []byte, offset int64) (int, error) {
	c.requested += len(p)
	return copy(p, c.data[offset:]), nil
}

func TestReadTailAsksForNoMoreThanTheCap(t *testing.T) {
	reader := &countingReader{data: []byte(strings.Repeat("0123456789", 1000))}

	tail, err := readTail(reader, int64(len(reader.data)), capture.FailureTailBytes)

	if err != nil || reader.requested != capture.FailureTailBytes || len(tail) != capture.FailureTailBytes {
		t.Fatalf("requested %d bytes, got %d, err = %v; want %d", reader.requested, len(tail), err, capture.FailureTailBytes)
	}
	equal(t, "tail", string(tail), string(reader.data[len(reader.data)-capture.FailureTailBytes:]))
}

func TestReadTailOfASmallFileReadsItAll(t *testing.T) {
	reader := &countingReader{data: []byte("abc\n")}

	tail, err := readTail(reader, 4, capture.FailureTailBytes)

	if err != nil || string(tail) != "abc\n" || reader.requested != 4 {
		t.Fatalf("tail = %q, requested = %d, err = %v", tail, reader.requested, err)
	}
}

// A log of megabytes must not hide its last line from the breaker check.
func TestIsStalledFindsTheLastLineOfALogLargerThanTheTailCap(t *testing.T) {
	layout := newLayout(t)
	filler := strings.Repeat(lineAt(logNow.Add(-time.Hour), capture.CauseDecode), 100_000)
	writeFile(t, layout.FailureLogPath(), filler+lineAt(logNow.Add(-time.Second), capture.CauseStall))

	if !logAt(layout, logNow).IsStalled(logNow) {
		t.Fatal("the last line of a long log was not read")
	}
}

func TestFailureLogSummarizeCountsOnlyTheLast24Hours(t *testing.T) {
	layout := newLayout(t)
	content := lineAt(logNow.Add(-24*time.Hour-time.Minute), capture.CauseSalt) +
		lineAt(logNow.Add(-24*time.Hour), capture.CauseSalt) +
		lineAt(logNow.Add(-23*time.Hour-59*time.Minute), capture.CauseDecode) +
		lineAt(logNow.Add(-time.Minute), capture.CauseDecode) +
		lineAt(logNow.Add(-time.Second), capture.CausePanic)
	writeFile(t, layout.FailureLogPath(), content)

	summary := logAt(layout, logNow).Summarize(logNow, capture.FailureWindow)

	equal(t, "count", summary.Count, 3)
	equal(t, "most common", summary.MostCommon, capture.CauseDecode)
	equal(t, "last", summary.Last, logNow.Add(-time.Second))
}

func TestFailureLogSummarizeBreaksATieInCauseOrder(t *testing.T) {
	layout := newLayout(t)
	writeFile(t, layout.FailureLogPath(), lineAt(logNow, capture.CausePanic)+lineAt(logNow, capture.CauseSalt))

	summary := logAt(layout, logNow).Summarize(logNow, capture.FailureWindow)

	equal(t, "most common", summary.MostCommon, capture.CauseSalt)
}

func TestFailureLogSummarizeOfNothingIsZero(t *testing.T) {
	layout := newLayout(t)

	missing := logAt(layout, logNow).Summarize(logNow, capture.FailureWindow)
	writeFile(t, layout.FailureLogPath(), "")
	empty := logAt(layout, logNow).Summarize(logNow, capture.FailureWindow)

	equal(t, "missing count", missing.Count, 0)
	equal(t, "empty count", empty.Count, 0)
	equal(t, "empty most common", empty.MostCommon, capture.FailureCause(""))
}

func TestFailureLogSummarizeIgnoresTheOldFormatAndGarbage(t *testing.T) {
	layout := newLayout(t)
	writeFile(t, layout.FailureLogPath(), "dda\n\x00\xff junk\n"+lineAt(logNow, capture.CauseFlag)+"2026-10-19T12:00:00.000Z made-up\n")

	summary := logAt(layout, logNow).Summarize(logNow, capture.FailureWindow)

	equal(t, "count", summary.Count, 1)
	equal(t, "most common", summary.MostCommon, capture.CauseFlag)
}

// The read is capped, so a log that grew past the cap between two purges
// cannot make health read megabytes. The newest lines are the ones read.
func TestFailureLogSummarizeReadsTheNewestCapOnly(t *testing.T) {
	layout := newLayout(t)
	oldLine := lineAt(logNow.Add(-time.Hour), capture.CauseSalt)
	fitting := capture.MaxFailureLogBytes / len(oldLine)
	old := strings.Repeat(oldLine, fitting+10)
	writeFile(t, layout.FailureLogPath(), old+lineAt(logNow, capture.CausePanic))

	summary := logAt(layout, logNow).Summarize(logNow, capture.FailureWindow)

	if summary.Count > fitting+1 || summary.MostCommon != capture.CauseSalt {
		t.Fatalf("count = %d, most common = %q: more than the cap was read", summary.Count, summary.MostCommon)
	}
	equal(t, "last", summary.Last, logNow)
}

// allocatedBy measures the bytes that work allocates. A read of the whole
// file allocates at least the size of the file; a bounded read does not.
func allocatedBy(work func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	work()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func hugeLog(t *testing.T, layout capture.Layout) {
	t.Helper()
	line := lineAt(logNow.Add(-time.Hour), capture.CauseDecode)
	writeFile(t, layout.FailureLogPath(), strings.Repeat(line, (16<<20)/len(line))+lineAt(logNow.Add(-time.Second), capture.CauseStall))
}

func TestIsStalledAllocatesFarLessThanAHugeLog(t *testing.T) {
	layout := newLayout(t)
	hugeLog(t, layout)
	log := logAt(layout, logNow)
	var isStalled bool

	allocated := allocatedBy(func() { isStalled = log.IsStalled(logNow) })

	if !isStalled || allocated > 64<<10 {
		t.Fatalf("stalled = %v, allocated %d bytes for a 16 MB log: the check must read a small tail", isStalled, allocated)
	}
}

func TestSummarizeAllocatesFarLessThanAHugeLog(t *testing.T) {
	layout := newLayout(t)
	hugeLog(t, layout)
	log := logAt(layout, logNow)
	var summary FailureSummary

	allocated := allocatedBy(func() { summary = log.Summarize(logNow, capture.FailureWindow) })

	if summary.Count == 0 || allocated > 8<<20 {
		t.Fatalf("count = %d, allocated %d bytes for a 16 MB log: the read must be capped", summary.Count, allocated)
	}
}
