package jsonlstore

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
)

var tailNow = time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)

func tailEvent(id string, age time.Duration) capture.SessionEvent {
	return hunkEvent(id, tailNow.Add(-age), capture.EventToolResult, "ref", "hash-"+id)
}

func writeTail(t *testing.T, layout capture.Layout, at time.Time, body string) {
	t.Helper()
	writeFile(t, layout.DayFile(at), body)
}

func linesOf(t *testing.T, events ...capture.SessionEvent) string {
	t.Helper()
	var body strings.Builder
	for _, e := range events {
		body.WriteString(lineOf(t, e))
	}
	return body.String()
}

func recent(t *testing.T, layout capture.Layout, now time.Time) []capture.SessionEvent {
	t.Helper()
	events, err := NewTail(layout).Recent(t.Context(), now, capture.DefaultCorrectionWindow)
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func TestTailReturnsTheWindowOldestFirstAndStopsAtTheFirstOlderEvent(t *testing.T) {
	tests := []struct {
		name   string
		events []capture.SessionEvent
		want   []string
	}{
		{name: "events inside the window",
			events: []capture.SessionEvent{tailEvent("old", 12*time.Minute), tailEvent("a", 9*time.Minute), tailEvent("b", 5*time.Minute), tailEvent("c", time.Minute)},
			want:   []string{"a", "b", "c"}},
		{name: "an event exactly at the cutoff is inside",
			events: []capture.SessionEvent{tailEvent("edge", capture.DefaultCorrectionWindow), tailEvent("c", time.Minute)},
			want:   []string{"edge", "c"}},
		{name: "an event a millisecond too old is outside",
			events: []capture.SessionEvent{tailEvent("late", capture.DefaultCorrectionWindow+time.Millisecond), tailEvent("c", time.Minute)},
			want:   []string{"c"}},
		{name: "the search stops at the first older event",
			events: []capture.SessionEvent{tailEvent("hidden", 5*time.Minute), tailEvent("older", 20*time.Minute), tailEvent("c", time.Minute)},
			want:   []string{"c"}},
		{name: "no events", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := newLayout(t)
			writeTail(t, layout, tailNow, linesOf(t, tt.events...))

			got := idsOf(recent(t, layout, tailNow))

			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("ids = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTailKeepsOnlyTheFieldsTheHookNeeds(t *testing.T) {
	layout := newLayout(t)
	stored := hunkEvent("e1", tailNow.Add(-time.Minute), capture.EventToolResult, "ref", "post").
		WithSessionRef("session-secret-ref").WithArgShape([]string{"k"}, []string{"string"})
	stored.PreHash = "pre"
	writeTail(t, layout, tailNow, linesOf(t, stored))

	got := recent(t, layout, tailNow)[0]

	equal(t, "kind", got.Kind, capture.EventToolResult)
	equal(t, "tool", got.Tool, "Edit")
	equal(t, "path ref", got.PathRef, "ref")
	equal(t, "path", *got.Path, "a.go")
	equal(t, "range", [2]int{got.RangeStart, got.RangeEnd}, [2]int{1, 2})
	equal(t, "pre hash", got.PreHash, "pre")
	equal(t, "post hash", got.PostHash, "post")
	equal(t, "at", got.At, stored.At)
	equal(t, "session ref left out", got.SessionRef, "")
	equal(t, "arg keys left out", len(got.ArgKeys), 0)
}

func TestTailSkipsBrokenLines(t *testing.T) {
	good1, good2 := tailEvent("g1", 3*time.Minute), tailEvent("g2", time.Minute)
	glued := `{"v":1,"id":"torn","at":"` + strings.TrimSpace(lineOf(t, good2))
	body := lineOf(t, good1) + glued + "\n" + "\n" + `{"v":2,"id":"future"}` + "\n" + "not json\n" + lineOf(t, good2)
	layout := newLayout(t)
	writeTail(t, layout, tailNow, body)

	got := idsOf(recent(t, layout, tailNow))

	if strings.Join(got, ",") != "g1,g2" {
		t.Fatalf("ids = %v, want g1,g2 (torn, glued, empty, unknown-version lines skipped)", got)
	}
}

func TestTailDropsAFinalLineWithoutANewline(t *testing.T) {
	complete := linesOf(t, tailEvent("a", 3*time.Minute), tailEvent("b", time.Minute))
	unterminated := strings.TrimSuffix(lineOf(t, tailEvent("c", 30*time.Second)), "\n")
	tests := []struct {
		name string
		body string
	}{
		{name: "short unterminated line", body: complete + unterminated},
		{name: "unterminated line longer than a chunk", body: complete + strings.Repeat("x", 3*capture.TailChunkBytes)},
		{name: "partial garbage", body: complete + `{"v":1,"id":"c","at":"20`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := newLayout(t)
			writeTail(t, layout, tailNow, tt.body)

			got := idsOf(recent(t, layout, tailNow))

			if strings.Join(got, ",") != "a,b" {
				t.Fatalf("ids = %v, want a,b", got)
			}
		})
	}
}

func TestTailReadsLinesThatCrossChunkBoundaries(t *testing.T) {
	var body strings.Builder
	const count = 600
	for i := range count {
		e := event(fmt.Sprintf("e%04d", i), tailNow.Add(-time.Minute), capture.EventToolUse, 20+i%200)
		body.WriteString(lineOf(t, e))
	}
	layout := newLayout(t)
	writeTail(t, layout, tailNow, body.String())

	got := idsOf(recent(t, layout, tailNow))

	if len(body.String()) < 3*capture.TailChunkBytes {
		t.Fatal("the fixture must span several chunks")
	}
	if strings.Join(got, ",") != strings.Join(sequence("e", 0, count), ",") {
		t.Fatalf("got %d ids, want all %d in order", len(got), count)
	}
}

func TestTailReadsAtMostTheTailCapAndDropsThePartialLine(t *testing.T) {
	const eventCount = 3000
	var body strings.Builder
	lineBytes := 0
	for i := range eventCount {
		line := lineOf(t, event(fmt.Sprintf("e%04d", i), tailNow.Add(-time.Minute), capture.EventToolUse, 100))
		lineBytes = len(line)
		body.WriteString(line)
	}
	layout := newLayout(t)
	writeTail(t, layout, tailNow, body.String())

	got := idsOf(recent(t, layout, tailNow))

	wantCount := capture.MaxTailBytes / lineBytes
	if len(got) != wantCount {
		t.Fatalf("got %d events, want %d (the cap holds %d whole lines)", len(got), wantCount, wantCount)
	}
	if got[0] != fmt.Sprintf("e%04d", eventCount-wantCount) || got[len(got)-1] != fmt.Sprintf("e%04d", eventCount-1) {
		t.Fatalf("ids run %s..%s, want the newest %d events", got[0], got[len(got)-1], wantCount)
	}
}

func TestTailReadsYesterdayOnlyWhenTheWindowCrossesMidnight(t *testing.T) {
	midnightNow := time.Date(2026, 3, 5, 0, 3, 0, 0, time.UTC)
	yesterday := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		now   time.Time
		setup func(*testing.T, capture.Layout)
		want  string
	}{
		{name: "window crosses midnight: both files, oldest first", now: midnightNow,
			setup: func(t *testing.T, l capture.Layout) {
				writeTail(t, l, yesterday, linesOf(t, hunkEvent("y-old", midnightNow.Add(-13*time.Minute), capture.EventToolResult, "r", "h"),
					hunkEvent("y1", midnightNow.Add(-8*time.Minute), capture.EventToolResult, "r", "h")))
				writeTail(t, l, midnightNow, linesOf(t, hunkEvent("t1", midnightNow.Add(-time.Minute), capture.EventToolResult, "r", "h")))
			}, want: "y1,t1"},
		{name: "today's file is missing", now: midnightNow,
			setup: func(t *testing.T, l capture.Layout) {
				writeTail(t, l, yesterday, linesOf(t, hunkEvent("y1", midnightNow.Add(-5*time.Minute), capture.EventToolResult, "r", "h")))
			}, want: "y1"},
		{name: "window inside the day: yesterday is not read", now: tailNow,
			setup: func(t *testing.T, l capture.Layout) {
				// A planted event inside the window proves that the file is not read.
				writeTail(t, l, yesterday, linesOf(t, tailEvent("planted", time.Minute)))
				writeTail(t, l, tailNow, linesOf(t, tailEvent("t1", time.Minute)))
			}, want: "t1"},
		{name: "an older event in today's file ends the search", now: midnightNow,
			setup: func(t *testing.T, l capture.Layout) {
				writeTail(t, l, yesterday, linesOf(t, hunkEvent("planted", midnightNow.Add(-5*time.Minute), capture.EventToolResult, "r", "h")))
				writeTail(t, l, midnightNow, linesOf(t, hunkEvent("old", midnightNow.Add(-20*time.Minute), capture.EventToolResult, "r", "h"),
					hunkEvent("t1", midnightNow.Add(-time.Minute), capture.EventToolResult, "r", "h")))
			}, want: "t1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := newLayout(t)
			tt.setup(t, layout)

			got := strings.Join(idsOf(recent(t, layout, tt.now)), ",")

			if got != tt.want {
				t.Fatalf("ids = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTailSharesOneByteBudgetAcrossBothFiles(t *testing.T) {
	midnightNow := time.Date(2026, 3, 5, 0, 3, 0, 0, time.UTC)
	layout := newLayout(t)
	var today strings.Builder
	for i := range 2500 {
		today.WriteString(lineOf(t, event(fmt.Sprintf("t%04d", i), midnightNow.Add(-time.Minute), capture.EventToolUse, 100)))
	}
	writeTail(t, layout, midnightNow, today.String())
	writeTail(t, layout, midnightNow.AddDate(0, 0, -1), linesOf(t, tailEvent("planted", time.Minute)))

	got := idsOf(recent(t, layout, midnightNow))

	for _, id := range got {
		if id == "planted" {
			t.Fatal("yesterday was read although today's file used the whole budget")
		}
	}
}

func TestTailGivesNothingForMissingEmptyOrDirectoryDayFiles(t *testing.T) {
	layout := newLayout(t)
	if events := recent(t, layout, tailNow); len(events) != 0 {
		t.Fatalf("missing file gave %d events", len(events))
	}
	writeTail(t, layout, tailNow, "")
	if events := recent(t, layout, tailNow); len(events) != 0 {
		t.Fatalf("empty file gave %d events", len(events))
	}
	if err := os.Remove(layout.DayFile(tailNow)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(layout.DayFile(tailNow), 0o700); err != nil {
		t.Fatal(err)
	}
	if events := recent(t, layout, tailNow); len(events) != 0 {
		t.Fatalf("directory gave %d events", len(events))
	}
}

func TestTailStopsOnACancelledContext(t *testing.T) {
	layout := newLayout(t)
	writeTail(t, layout, tailNow, linesOf(t, tailEvent("a", time.Minute)))
	cancelled, cancel := cancelledContext(t)
	defer cancel()

	events, err := NewTail(layout).Recent(cancelled, tailNow, capture.DefaultCorrectionWindow)

	if err != nil || len(events) != 0 {
		t.Fatalf("events = %d, err = %v, want nothing read", len(events), err)
	}
}
