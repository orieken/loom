package jsonlstore

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
)

var readerBase = time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)

func readerEvent(id string, offset time.Duration) capture.SessionEvent {
	return event(id, readerBase.Add(offset), capture.EventToolUse, 0)
}

func readAll(t *testing.T, body string) ([]string, ReadStats, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "2026-03-04.jsonl")
	writeFile(t, path, body)
	var ids []string
	stats, err := ReadDayFile(t.Context(), path, func(e capture.SessionEvent) error {
		ids = append(ids, e.ID)
		return nil
	})
	return ids, stats, err
}

func TestReadDayFileCountsWhatItSkipsAndNeverFails(t *testing.T) {
	good := func(id string, offset time.Duration) string { return lineOf(t, readerEvent(id, offset)) }
	overCap := rawLine(t, event("long", readerBase, capture.EventToolUse, 4500))
	hugeLine := `{"v":1,"id":"huge","path":"` + strings.Repeat("h", 3*readerBufferBytes) + `"}` + "\n"
	tests := []struct {
		name    string
		body    string
		wantIDs string
		want    ReadStats
	}{
		{name: "clean file", body: good("a", 0) + good("b", time.Second), wantIDs: "a,b", want: ReadStats{Events: 2}},
		{name: "torn line", body: good("a", 0) + "{\"v\":1,\"id\":\"torn\n" + good("b", time.Second), wantIDs: "a,b", want: ReadStats{Events: 2, TornLines: 1}},
		{name: "torn fragment glued to a good line", body: good("a", 0) + `{"v":1,"id":"x` + good("b", time.Second) + good("c", 2*time.Second),
			wantIDs: "a,c", want: ReadStats{Events: 2, TornLines: 1}},
		{name: "blank line", body: good("a", 0) + "\n" + good("b", time.Second), wantIDs: "a,b", want: ReadStats{Events: 2, TornLines: 1}},
		{name: "final line without a newline", body: good("a", 0) + strings.TrimSuffix(good("b", time.Second), "\n"), wantIDs: "a", want: ReadStats{Events: 1, TornLines: 1}},
		{name: "unknown version", body: good("a", 0) + `{"v":2,"id":"future"}` + "\n" + good("b", time.Second), wantIDs: "a,b", want: ReadStats{Events: 2, UnknownVersionLines: 1}},
		{name: "line over the cap but inside the buffer", body: good("a", 0) + overCap + good("b", time.Second), wantIDs: "a,b", want: ReadStats{Events: 2, TornLines: 1}},
		{name: "line over the buffer", body: good("a", 0) + hugeLine + good("b", time.Second), wantIDs: "a,b", want: ReadStats{Events: 2, TornLines: 1}},
		{name: "huge line at the end without a newline", body: good("a", 0) + strings.TrimSuffix(hugeLine, "\n"), wantIDs: "a", want: ReadStats{Events: 1, TornLines: 1}},
		{name: "empty file", body: "", wantIDs: "", want: ReadStats{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids, stats, err := readAll(t, tt.body)

			if err != nil {
				t.Fatalf("err = %v, want none: bad lines never fail the read", err)
			}
			if strings.Join(ids, ",") != tt.wantIDs || stats != tt.want {
				t.Fatalf("ids = %v, stats = %+v; want %q, %+v", ids, stats, tt.wantIDs, tt.want)
			}
		})
	}
}

func TestReadDayFileClockAnomalyThreshold(t *testing.T) {
	tests := []struct {
		name    string
		offsets []time.Duration
		want    int
	}{
		{name: "forward time", offsets: []time.Duration{0, time.Second, time.Hour}, want: 0},
		{name: "step back of exactly 5 s", offsets: []time.Duration{10 * time.Second, 5 * time.Second}, want: 0},
		{name: "step back of 6 s", offsets: []time.Duration{10 * time.Second, 4 * time.Second}, want: 1},
		{name: "each step is compared with the event before it", offsets: []time.Duration{60 * time.Second, 40 * time.Second, 41 * time.Second, 20 * time.Second}, want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body strings.Builder
			for i, offset := range tt.offsets {
				body.WriteString(lineOf(t, readerEvent(string(rune('a'+i)), offset)))
			}

			_, stats, err := readAll(t, body.String())

			if err != nil || stats.ClockAnomalies != tt.want || stats.Events != len(tt.offsets) {
				t.Fatalf("err = %v, stats = %+v, want %d anomalies and every event kept", err, stats, tt.want)
			}
		})
	}
}

func TestReadDayFileStopsWhenTheVisitorFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.jsonl")
	writeFile(t, path, lineOf(t, readerEvent("a", 0))+lineOf(t, readerEvent("b", time.Second)))
	stop := errors.New("stop")
	var seen int

	_, err := ReadDayFile(t.Context(), path, func(capture.SessionEvent) error {
		seen++
		return stop
	})

	if !errors.Is(err, stop) || seen != 1 {
		t.Fatalf("err = %v, seen = %d, want the visitor error after one event", err, seen)
	}
}

func TestReadDayFileReportsAnUnreadableFileAndACancelledContext(t *testing.T) {
	if _, err := ReadDayFile(t.Context(), filepath.Join(t.TempDir(), "none.jsonl"), nil); err == nil {
		t.Fatal("a missing file must be an error")
	}
	path := filepath.Join(t.TempDir(), "d.jsonl")
	writeFile(t, path, lineOf(t, readerEvent("a", 0)))
	cancelled, cancel := cancelledContext(t)
	defer cancel()

	_, err := ReadDayFile(cancelled, path, func(capture.SessionEvent) error { return nil })

	if err == nil {
		t.Fatal("a cancelled context must stop the read")
	}
}

func TestReadStatsAddSumsEveryCount(t *testing.T) {
	a := ReadStats{Events: 1, TornLines: 2, UnknownVersionLines: 3, ClockAnomalies: 4}
	b := ReadStats{Events: 10, TornLines: 20, UnknownVersionLines: 30, ClockAnomalies: 40}

	if got := a.Add(b); got != (ReadStats{Events: 11, TornLines: 22, UnknownVersionLines: 33, ClockAnomalies: 44}) {
		t.Fatalf("sum = %+v", got)
	}
}

func TestDayFilePathsListsOnlyDayFilesOldestFirst(t *testing.T) {
	layout := newLayout(t)
	for _, name := range []string{"2026-03-05.jsonl", "2026-03-03.jsonl", "notes.txt", "2026-03-04.jsonl"} {
		writeFile(t, filepath.Join(layout.SessionsDir(), name), "x")
	}

	paths, err := DayFilePaths(layout)

	var names []string
	for _, path := range paths {
		names = append(names, filepath.Base(path))
	}
	if err != nil || strings.Join(names, ",") != "2026-03-03.jsonl,2026-03-04.jsonl,2026-03-05.jsonl" {
		t.Fatalf("paths = %v, err = %v", names, err)
	}
	if paths, err := DayFilePaths(capture.NewLayout(t.TempDir())); err != nil || len(paths) != 0 {
		t.Fatalf("missing directory: paths = %v, err = %v, want an empty list", paths, err)
	}
}

// rawLine encodes an event without the line cap, to build an over-long line.
func rawLine(t *testing.T, e capture.SessionEvent) string {
	t.Helper()
	body, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return string(body) + "\n"
}
