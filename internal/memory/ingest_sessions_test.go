package memory_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
	"github.com/orieken/loom/internal/memory"
)

func mixedSource() *fakeSessionSource {
	return &fakeSessionSource{
		hasFiles: true, newest: sessionBase.Add(2 * time.Hour), hasNewest: true,
		stats: memory.SessionReadStats{TornLines: 2, UnknownVersionLines: 1, ClockAnomalies: 3},
		events: joinEvents(
			hunkEditEvents("src/a.go", 0),
			[]capture.SessionEvent{promptEvent("p", time.Hour)},
			revertEvents("docs/b.md", 90*time.Minute),
		),
	}
}

func reportBytes(t *testing.T, store *memory.Store, source memory.SessionSource) string {
	t.Helper()
	report := sessionReport(t, store, source, capture.DefaultConfig(), memory.SessionFilter{})
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestIngestTwiceGivesIdenticalRowsAndReportBytes(t *testing.T) {
	store := newSessionStore(t)
	source := mixedSource()
	ingestSessions(t, store, source)
	firstRows, firstReport := storedRows(t, store), reportBytes(t, store, source)

	ingestSessions(t, store, source)

	if !reflect.DeepEqual(storedRows(t, store), firstRows) {
		t.Errorf("rows changed on the second ingest")
	}
	if reportBytes(t, store, source) != firstReport {
		t.Errorf("report bytes changed on the second ingest")
	}
}

// seq follows the detector output order: 1, 2, 3 in the order the events
// complete a correction.
func TestIngestNumbersCorrectionsInDetectorOrder(t *testing.T) {
	store := newSessionStore(t)

	result := ingestSessions(t, store, mixedSource())

	rows := storedRows(t, store)
	wantKinds := []string{"hunk-edit", "corrective-prompt", "revert"}
	if result.Corrections != 3 || len(rows) != 3 {
		t.Fatalf("stored %d rows (result %d), want 3", len(rows), result.Corrections)
	}
	for i, row := range rows {
		if row.Seq != i+1 || row.Kind != wantKinds[i] {
			t.Errorf("row %d = seq %d kind %s, want seq %d kind %s", i, row.Seq, row.Kind, i+1, wantKinds[i])
		}
	}
}

func TestIngestReportsTheSkipCountsOfTheSource(t *testing.T) {
	store := newSessionStore(t)

	result := ingestSessions(t, store, mixedSource())

	want := memory.SessionReadStats{TornLines: 2, UnknownVersionLines: 1, ClockAnomalies: 3}
	if result.Stats != want {
		t.Errorf("stats = %+v, want %+v", result.Stats, want)
	}
}

func TestIngestCountsUnattributedCorrections(t *testing.T) {
	store := newSessionStore(t)
	source := &fakeSessionSource{events: joinEvents(unknownEvents("src/u.go", 0), hunkEditEvents("src/a.go", time.Hour))}

	result := ingestSessions(t, store, source)

	if result.Unattributed != 1 || result.Corrections != 2 {
		t.Errorf("unattributed %d of %d, want 1 of 2", result.Unattributed, result.Corrections)
	}
}

// Switches apply at report time, so a denied-call is stored even though its
// switch is off by default, and a switch change needs no re-ingest.
func TestSignalSwitchesApplyAtReportTimeNotIngestTime(t *testing.T) {
	store := newSessionStore(t)
	source := &fakeSessionSource{events: []capture.SessionEvent{deniedEvent("d", 0), promptEvent("p", time.Minute)}}
	ingestSessions(t, store, source)
	if rows := storedRows(t, store); len(rows) != 2 {
		t.Fatalf("stored %d rows, want both signals", len(rows))
	}

	defaults := sessionReport(t, store, source, capture.DefaultConfig(), memory.SessionFilter{})
	enabled := capture.DefaultConfig()
	enabled.Signals[capture.SignalDeniedCall] = true
	withDenied := sessionReport(t, store, source, enabled, memory.SessionFilter{})
	disabled := capture.DefaultConfig()
	disabled.Signals = capture.SignalSwitches{}
	withNone := sessionReport(t, store, source, disabled, memory.SessionFilter{})

	if len(defaults.Kinds) != 1 || defaults.Kinds[0].Kind != capture.KindCorrectivePrompt {
		t.Errorf("defaults = %+v, want the prompt only", defaults.Kinds)
	}
	if len(withDenied.Kinds) != 2 {
		t.Errorf("denied-call on = %+v, want two kinds", withDenied.Kinds)
	}
	if len(withNone.Kinds) != 0 {
		t.Errorf("all signals off = %+v, want none", withNone.Kinds)
	}
}

func TestPurgedDayFileLosesItsRowsOnReingest(t *testing.T) {
	store := newSessionStore(t)
	ingestSessions(t, store, mixedSource())

	ingestSessions(t, store, &fakeSessionSource{events: hunkEditEvents("src/a.go", 0)})

	if rows := storedRows(t, store); len(rows) != 1 {
		t.Errorf("after the purge %d rows remain, want 1", len(rows))
	}
}

func TestIngestEmptySourceClearsTheTable(t *testing.T) {
	store := newSessionStore(t)
	ingestSessions(t, store, mixedSource())

	ingestSessions(t, store, &fakeSessionSource{})

	if rows := storedRows(t, store); len(rows) != 0 {
		t.Errorf("%d rows remain after an empty ingest", len(rows))
	}
}

// A failure in the middle of the stream rolls back, so the old rows stay.
func TestFailedIngestKeepsThePreviousRows(t *testing.T) {
	store := newSessionStore(t)
	ingestSessions(t, store, mixedSource())
	before := storedRows(t, store)
	failing := &fakeSessionSource{events: manyHunkEdits(3), streamErr: errors.New("disk fell over")}

	_, err := store.IngestSessions(context.Background(), failing, capture.DefaultConfig())

	if err == nil || !strings.Contains(err.Error(), "disk fell over") {
		t.Fatalf("error = %v, want the stream error", err)
	}
	if !reflect.DeepEqual(storedRows(t, store), before) {
		t.Error("a failed ingest changed the stored rows")
	}
}

func TestIngestStoresNoRedactedPathInPlainForm(t *testing.T) {
	store := newSessionStore(t)
	const redactedName = "canary-secret.pem"
	source := &fakeSessionSource{events: joinEvents(redactedEditEvents(0), hunkEditEvents("src/visible.go", time.Hour))}
	ingestSessions(t, store, source)

	rows := storedRows(t, store)

	if len(rows) != 2 {
		t.Fatalf("stored %d rows", len(rows))
	}
	if rows[0].Path.Valid {
		t.Errorf("a redacted path was stored: %q", rows[0].Path.String)
	}
	if !rows[1].Path.Valid || rows[1].Path.String != "src/visible.go" {
		t.Errorf("visible path = %+v", rows[1].Path)
	}
	assertAbsentFromFile(t, store.Path(), redactedName, "secret.pem")
}

func assertAbsentFromFile(t *testing.T, path string, canaries ...string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, canary := range canaries {
		if bytes.Contains(raw, []byte(canary)) {
			t.Errorf("%q appears in %s", canary, path)
		}
	}
}

// The deadline is applied inside ingest: the context the source receives
// ends within IngestDeadline.
func TestIngestRunsUnderTheIngestDeadline(t *testing.T) {
	store := newSessionStore(t)
	var seen time.Time
	source := &fakeSessionSource{streamHook: func(ctx context.Context) { seen, _ = ctx.Deadline() }}

	ingestSessions(t, store, source)

	remaining := time.Until(seen)
	if remaining <= memory.IngestDeadline-10*time.Second || remaining > memory.IngestDeadline {
		t.Errorf("deadline in %s, want about %s", remaining, memory.IngestDeadline)
	}
}

func TestIngestStopsAtTheDeadlineAndKeepsPreviousRows(t *testing.T) {
	store := newSessionStore(t)
	ingestSessions(t, store, mixedSource())
	before := storedRows(t, store)
	parent, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	slow := &fakeSessionSource{events: manyHunkEdits(3), streamHook: func(ctx context.Context) { <-ctx.Done() }}

	_, err := store.IngestSessions(parent, slow, capture.DefaultConfig())

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want a deadline error", err)
	}
	if !reflect.DeepEqual(storedRows(t, store), before) {
		t.Error("a timed-out ingest changed the stored rows")
	}
}

func TestReportRunsUnderTheReportDeadline(t *testing.T) {
	store := newSessionStore(t)
	ingestSessions(t, store, mixedSource())
	parent, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.SessionReport(parent, mixedSource(), capture.DefaultConfig(), memory.SessionFilter{})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want a cancelled-context error", err)
	}
}
