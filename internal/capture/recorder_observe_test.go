package capture

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	hunkStaleHash = "stale-hash"
	oneMegabyte   = 1 << 20
)

// seedHunk puts an agent-written hunk into the tail, as an earlier call would.
func (r *rig) seedHunk(relative string, start, end int, age time.Duration) {
	path := relative
	r.sink.events = append(r.sink.events, NewSessionEvent("seed", r.clock.now.Add(-age), EventToolResult).
		WithTool(ToolEdit).
		WithPath(r.pathRef(relative), &path).
		WithHunk(HunkFacts{RangeStart: start, RangeEnd: end, PostHash: hunkStaleHash}))
}

func (r *rig) observations() []SessionEvent {
	var found []SessionEvent
	for _, event := range r.sink.events {
		if event.Kind == EventHunkObserved {
			found = append(found, event)
		}
	}
	return found
}

func readOf(relative string) Observation {
	return Observation{Kind: EventToolUse, Tool: "Read", SessionID: "s1", FilePath: filepath.Join(testRoot, relative)}
}

func TestRecorderOneHumanEditGivesOneObservationAndOneCorrection(t *testing.T) {
	r := newRig(t, edit("a.go"), EditText{}.With("x", "b"))
	r.files.put("a.go", "a\nb\nc\n")
	r.mustCall(t, edit("a.go"))
	r.files.put("a.go", "a\nHUMAN\nc\n")

	for range 4 {
		r.clock.advance(time.Minute)
		r.mustCall(t, readOf("a.go"))
	}

	observed := r.observations()
	equal(t, "observation count", len(observed), 1)
	equal(t, "observed hash", observed[0].PostHash, r.lineHash("a\nHUMAN\nc\n", 2, 2))
	detector := NewDetector(DefaultConfig())
	var corrections []Correction
	for _, event := range r.sink.events {
		corrections = append(corrections, detector.Observe(event)...)
	}
	if len(corrections) != 1 || corrections[0].Kind != KindHunkEdit {
		t.Fatalf("corrections = %+v, want exactly one hunk edit", corrections)
	}
}

func TestRecorderRevertIsSeenAsARevert(t *testing.T) {
	r := newRig(t, edit("a.go"), EditText{}.With("orig", "b"))
	r.files.put("a.go", "a\nb\nc\n")
	_ = r.call(t, edit("a.go"))
	r.files.put("a.go", "a\norig\nc\n")
	r.clock.advance(time.Minute)

	_ = r.call(t, readOf("a.go"))

	detector := NewDetector(DefaultConfig())
	var corrections []Correction
	for _, event := range r.sink.events {
		corrections = append(corrections, detector.Observe(event)...)
	}
	if len(corrections) != 1 || corrections[0].Kind != KindRevert {
		t.Fatalf("corrections = %+v, want one revert", corrections)
	}
}

func TestRecorderUnchangedHunkWritesNothing(t *testing.T) {
	r := newRig(t, edit("a.go"), EditText{}.With("x", "b"))
	r.files.put("a.go", "a\nb\nc\n")
	_ = r.call(t, edit("a.go"))

	r.clock.advance(time.Minute)
	_ = r.call(t, readOf("a.go"))

	if len(r.observations()) != 0 || len(r.sink.events) != 2 {
		t.Fatalf("events = %+v, want the two main events only", r.sink.events)
	}
}

func TestRecorderObservationFacts(t *testing.T) {
	r := newRig(t, readOf("a.go"), EditText{})
	r.files.put("a.go", "one\ntwo\n")
	r.seedHunk("a.go", 1, 2, time.Minute)

	_ = r.call(t, readOf("a.go"))

	observed := r.observations()[0]
	equal(t, "tool", observed.Tool, ObserverTool)
	equal(t, "session ref kept", observed.SessionRef == "", false)
	equal(t, "range start", observed.RangeStart, 1)
	equal(t, "range end", observed.RangeEnd, 2)
	equal(t, "path ref", observed.PathRef, r.pathRef("a.go"))
	equal(t, "path", *observed.Path, "a.go")
	equal(t, "hash", observed.PostHash, r.lineHash("one\ntwo\n", 1, 2))
	equal(t, "instant of the main event", observed.At, r.sink.events[len(r.sink.events)-2].At)
}

func TestRecorderShrunkFileHashesAsEmptyText(t *testing.T) {
	r := newRig(t, readOf("a.go"), EditText{})
	r.files.put("a.go", "only\n")
	r.seedHunk("a.go", 3, 5, time.Minute)

	_ = r.call(t, readOf("a.go"))

	observed := r.observations()
	if len(observed) != 1 || observed[0].PostHash != r.hasher.Hash(HashLines, "") {
		t.Fatalf("observations = %+v, want the hash of an empty text", observed)
	}
}

func TestRecorderRehashScope(t *testing.T) {
	tests := []struct {
		name          string
		obs           Observation
		wantReadPaths []string
	}{
		{name: "a file call sees its own path only", obs: readOf("p.go"), wantReadPaths: []string{"p.go"}},
		{name: "a prompt sees every path", obs: prompt(), wantReadPaths: []string{"p.go", "q.go"}},
		{name: "a call with no path sees none", obs: Observation{Kind: EventToolUse, Tool: ToolBash}},
		{name: "a stop sees none", obs: Observation{Kind: EventStop}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRig(t, tt.obs, EditText{})
			r.files.put("p.go", "p\n")
			r.files.put("q.go", "q\n")
			r.seedHunk("p.go", 1, 1, time.Minute)
			r.seedHunk("q.go", 1, 1, time.Minute)

			_ = r.call(t, tt.obs)

			if len(r.files.reads) != len(tt.wantReadPaths) {
				t.Fatalf("read %v, want %v", r.files.reads, tt.wantReadPaths)
			}
			for _, path := range tt.wantReadPaths {
				if r.files.reads[filepath.Join(testRoot, path)] != 1 {
					t.Fatalf("read %v, want %s read once", r.files.reads, path)
				}
			}
		})
	}
}

func TestRecorderAgentWriteSkipsTheHunksOfItsOwnPath(t *testing.T) {
	r := newRig(t, edit("p.go"), EditText{}.With("x", "p"))
	r.files.put("p.go", "p\n")
	r.seedHunk("p.go", 1, 1, time.Minute)

	_ = r.call(t, edit("p.go"))

	if len(r.observations()) != 0 {
		t.Fatalf("observations = %+v, want none: the agent write explains the change", r.observations())
	}
}

func TestRecorderForgetsHunksAfterTheWindow(t *testing.T) {
	r := newRig(t, readOf("p.go"), EditText{})
	r.files.put("p.go", "p\n")
	r.seedHunk("p.go", 1, 1, DefaultCorrectionWindow+time.Second)

	_ = r.call(t, readOf("p.go"))

	if r.files.totalReads() != 0 {
		t.Fatalf("reads = %d, want 0 for a hunk past the window", r.files.totalReads())
	}
}

func TestRecorderReadsEachPathOnce(t *testing.T) {
	r := newRig(t, readOf("p.go"), EditText{})
	r.files.put("p.go", "1\n2\n3\n4\n5\n")
	for line := 1; line <= 5; line++ {
		r.seedHunk("p.go", line, line, time.Minute)
	}

	_ = r.call(t, readOf("p.go"))

	if got := r.files.totalReads(); got != 1 {
		t.Fatalf("reads = %d, want 1 for five hunks on one path", got)
	}
	if len(r.observations()) != 5 {
		t.Fatalf("observations = %d, want 5", len(r.observations()))
	}
}

func TestRecorderRehashesAtMostTwentyHunksAndTheNewestFirst(t *testing.T) {
	r := newRig(t, prompt(), EditText{})
	for i := 0; i <= MaxRehashedHunks; i++ {
		name := fmt.Sprintf("f%02d.go", i)
		r.files.put(name, "x\n")
		r.seedHunk(name, 1, 1, time.Duration(100-i)*time.Second)
	}

	_ = r.call(t, prompt())

	if got := len(r.observations()); got != MaxRehashedHunks {
		t.Fatalf("observations = %d, want %d", got, MaxRehashedHunks)
	}
	if r.files.reads[filepath.Join(testRoot, "f00.go")] != 0 {
		t.Fatal("the oldest hunk must be the one that is left out")
	}
}

func TestRecorderReadBudgetStopsTheWork(t *testing.T) {
	r := newRig(t, prompt(), EditText{})
	body := strings.Repeat("0123456789abcde\n", oneMegabyte/16)
	for i := range MaxRehashedHunks {
		name := fmt.Sprintf("big%02d.go", i)
		r.files.put(name, body)
		r.seedHunk(name, 1, 1, time.Duration(100-i)*time.Second)
	}

	_ = r.call(t, prompt())

	wantReads := MaxReadBudgetBytes / oneMegabyte
	if got := r.files.totalReads(); got != wantReads {
		t.Fatalf("reads = %d, want %d: the 4 MB budget must stop the work", got, wantReads)
	}
	if got := len(r.observations()); got != wantReads {
		t.Fatalf("observations = %d, want %d", got, wantReads)
	}
}

func TestRecorderReadsTheTargetAndItsHunksFromOneRead(t *testing.T) {
	pre := Observation{Kind: EventToolUse, Tool: ToolWrite, SessionID: "s1", FilePath: filepath.Join(testRoot, "p.go")}
	r := newRig(t, pre, EditText{})
	r.files.put("p.go", "one\ntwo\n")
	r.seedHunk("p.go", 1, 1, time.Minute)

	_ = r.call(t, pre)

	if got := r.files.totalReads(); got != 1 {
		t.Fatalf("reads = %d, want 1: the target and the hunk share a path", got)
	}
	if len(r.observations()) != 1 {
		t.Fatalf("observations = %d, want 1", len(r.observations()))
	}
}

func TestRecorderTailFailureLosesRecallOnly(t *testing.T) {
	r := newRig(t, readOf("p.go"), EditText{})
	r.deps.Tail = failingTail{}

	err := r.call(t, readOf("p.go"))

	if err != nil || len(r.sink.events) != 1 {
		t.Fatalf("err = %v, events = %d, want the main event written", err, len(r.sink.events))
	}
}

type failingTail struct{}

func (failingTail) Recent(context.Context, time.Time, time.Duration) ([]SessionEvent, error) {
	return nil, errors.New("tail unreadable")
}

func TestRecorderUnreadableHunkFileWritesNoObservation(t *testing.T) {
	r := newRig(t, readOf("gone.go"), EditText{})
	r.seedHunk("gone.go", 1, 1, time.Minute)

	err := r.call(t, readOf("gone.go"))

	if err != nil || len(r.observations()) != 0 {
		t.Fatalf("err = %v, observations = %d, want none", err, len(r.observations()))
	}
}

func TestLedgerFromTailFollowsTheDetectorRules(t *testing.T) {
	path := "a.go"
	write := func(id string, when time.Time, start int) SessionEvent {
		return NewSessionEvent(id, when, EventToolResult).WithTool(ToolEdit).WithPath("ref", &path).
			WithHunk(HunkFacts{RangeStart: start, RangeEnd: start, PostHash: "h"})
	}
	t0 := baseTime
	tests := []struct {
		name   string
		events []SessionEvent
		want   int
	}{
		{name: "two ranges of one call stay", events: []SessionEvent{write("1", t0, 1), write("2", t0, 5)}, want: 2},
		{name: "a later write replaces the older hunks", events: []SessionEvent{write("1", t0, 1), write("2", t0.Add(time.Second), 5)}, want: 1},
		{name: "a hunk past the window leaves", events: []SessionEvent{write("1", t0.Add(-time.Hour), 1)}, want: 0},
		{name: "an event without a range opens nothing", events: []SessionEvent{NewSessionEvent("1", t0, EventToolResult).WithTool(ToolEdit).WithPath("ref", &path)}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ledger := ledgerFromTail(tt.events, t0.Add(time.Second), DefaultCorrectionWindow)

			if ledger.Len() != tt.want {
				t.Fatalf("open hunks = %d, want %d", ledger.Len(), tt.want)
			}
		})
	}
}

func TestLedgerObservationUpdatesTheLastHash(t *testing.T) {
	path := "a.go"
	events := []SessionEvent{
		NewSessionEvent("1", baseTime, EventToolResult).WithTool(ToolEdit).WithPath("ref", &path).
			WithHunk(HunkFacts{RangeStart: 2, RangeEnd: 3, PostHash: "first"}),
		NewSessionEvent("2", baseTime.Add(time.Second), EventHunkObserved).WithPath("ref", &path).
			WithHunk(HunkFacts{RangeStart: 2, RangeEnd: 3, PostHash: "second"}),
		NewSessionEvent("3", baseTime.Add(time.Second), EventHunkObserved).WithPath("ref", &path).
			WithHunk(HunkFacts{RangeStart: 9, RangeEnd: 9, PostHash: "other range"}),
	}

	ledger := ledgerFromTail(events, baseTime.Add(2*time.Second), DefaultCorrectionWindow)

	hunk, isOpen := ledger.find("ref", 2, 3)
	if !isOpen || hunk.facts.PostHash != "second" || ledger.Len() != 1 {
		t.Fatalf("hunk = %+v (open %v), len = %d, want the newest hash on the one open hunk", hunk, isOpen, ledger.Len())
	}
}

func TestGroupByPathKeepsOrderAndSkipsHunksWithoutAPath(t *testing.T) {
	p, q := "p.go", "q.go"
	hunks := []openHunk{
		{pathRef: "p", path: &p, writePosition: 3},
		{pathRef: "q", path: &q, writePosition: 2},
		{pathRef: "p", path: &p, writePosition: 1},
		{pathRef: "x", path: nil},
	}

	groups := groupByPath(hunks)

	if len(groups) != 2 || groups[0].path != "p.go" || len(groups[0].hunks) != 2 || groups[1].path != "q.go" {
		t.Fatalf("groups = %+v, want p.go (2 hunks) then q.go", groups)
	}
}

func TestDropPathKeepsHunksOfTheSameInstant(t *testing.T) {
	ledger := NewHunkLedger()
	ledger.open(openHunk{pathRef: "p", writtenAt: baseTime.Add(-time.Second), facts: HunkFacts{RangeStart: 1, RangeEnd: 1}})
	ledger.open(openHunk{pathRef: "p", writtenAt: baseTime, facts: HunkFacts{RangeStart: 2, RangeEnd: 2}})
	ledger.open(openHunk{pathRef: "p", writtenAt: baseTime.Add(time.Second), facts: HunkFacts{RangeStart: 3, RangeEnd: 3}})

	ledger.dropPath("p", baseTime)

	if _, isOpen := ledger.find("p", 1, 1); isOpen {
		t.Fatal("a hunk from an earlier call must be dropped")
	}
	if ledger.Len() != 2 {
		t.Fatalf("open hunks = %d, want 2 (same instant and later stay)", ledger.Len())
	}
}
