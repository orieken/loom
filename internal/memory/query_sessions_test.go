package memory_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
	"github.com/orieken/loom/internal/memory"
)

func kindCounts(report memory.SessionReport) map[capture.CorrectionKind]int {
	counts := map[capture.CorrectionKind]int{}
	for _, row := range report.Kinds {
		counts[row.Kind] = row.Count
	}
	return counts
}

func ingestedStore(t *testing.T) (*memory.Store, *fakeSessionSource) {
	t.Helper()
	store := newSessionStore(t)
	source := mixedSource()
	ingestSessions(t, store, source)
	return store, source
}

func TestReportCountsKindsAndEstimatesMinutesAtDefaultWeights(t *testing.T) {
	store, source := ingestedStore(t)

	report := sessionReport(t, store, source, capture.DefaultConfig(), memory.SessionFilter{})

	if report.State != memory.SessionStateReady || report.Method != "fixed weight per kind (estimate)" {
		t.Errorf("state %q method %q", report.State, report.Method)
	}
	// hunk-edit 1 x 2, corrective-prompt 1 x 1, revert 1 x 3.
	if report.TotalEstimatedMinutes != 6 {
		t.Errorf("total minutes = %d, want 6", report.TotalEstimatedMinutes)
	}
	want := []memory.SessionKindRow{
		{Kind: capture.KindCorrectivePrompt, Count: 1, EstimatedMinutes: 1},
		{Kind: capture.KindHunkEdit, Count: 1, EstimatedMinutes: 2},
		{Kind: capture.KindRevert, Count: 1, EstimatedMinutes: 3},
	}
	if len(report.Kinds) != 3 {
		t.Fatalf("kinds = %+v", report.Kinds)
	}
	for i := range want {
		if report.Kinds[i] != want[i] {
			t.Errorf("kind %d = %+v, want %+v", i, report.Kinds[i], want[i])
		}
	}
}

func TestReportUsesConfiguredWeights(t *testing.T) {
	store, source := ingestedStore(t)
	cfg := capture.DefaultConfig()
	cfg.Weights.HunkEdit = 10

	report := sessionReport(t, store, source, cfg, memory.SessionFilter{})

	if report.TotalEstimatedMinutes != 14 {
		t.Errorf("total = %d, want 14 with a hunk-edit weight of 10", report.TotalEstimatedMinutes)
	}
}

func TestUnattributedCorrectionsAreShownButNeverCounted(t *testing.T) {
	store := newSessionStore(t)
	source := &fakeSessionSource{events: joinEvents(unknownEvents("src/u.go", 0), hunkEditEvents("src/a.go", time.Hour))}
	ingestSessions(t, store, source)

	report := sessionReport(t, store, source, capture.DefaultConfig(), memory.SessionFilter{})

	if report.Unattributed != 1 || report.TotalEstimatedMinutes != 2 {
		t.Errorf("unattributed %d total %d, want 1 and 2", report.Unattributed, report.TotalEstimatedMinutes)
	}
	if _, isListed := kindCounts(report)[capture.KindUnknown]; isListed {
		t.Error("unknown appears among the counted kinds")
	}
}

func TestSinceKeepsCorrectionsAtOrAfterTheBound(t *testing.T) {
	store, source := ingestedStore(t)
	// The hunk-edit is at 12:00:30; the prompt is at 13:00:00.
	atEdit := sessionBase.Add(30 * time.Second)

	inclusive := sessionReport(t, store, source, capture.DefaultConfig(), memory.SessionFilter{Since: atEdit})
	after := sessionReport(t, store, source, capture.DefaultConfig(), memory.SessionFilter{Since: atEdit.Add(time.Millisecond)})

	if inclusive.Kinds[0].Count+inclusive.Kinds[1].Count+inclusive.Kinds[2].Count != 3 {
		t.Errorf("a bound equal to a row time must keep that row: %+v", inclusive.Kinds)
	}
	if _, hasEdit := kindCounts(after)[capture.KindHunkEdit]; hasEdit {
		t.Errorf("a bound after the row kept it: %+v", after.Kinds)
	}
}

func TestToolFilterKeepsOneTool(t *testing.T) {
	store := newSessionStore(t)
	source := &fakeSessionSource{events: joinEvents(hunkEditEvents("src/a.go", 0), []capture.SessionEvent{
		capture.NewSessionEvent("d", sessionBase.Add(time.Hour), capture.EventToolDenied).WithTool(capture.ToolWrite)})}
	ingestSessions(t, store, source)
	cfg := capture.DefaultConfig()
	cfg.Signals[capture.SignalDeniedCall] = true

	edit := sessionReport(t, store, source, cfg, memory.SessionFilter{Tool: capture.ToolEdit})
	write := sessionReport(t, store, source, cfg, memory.SessionFilter{Tool: capture.ToolWrite})

	if got := kindCounts(edit); len(got) != 1 || got[capture.KindHunkEdit] != 1 {
		t.Errorf("Edit filter = %+v", got)
	}
	if got := kindCounts(write); len(got) != 1 || got[capture.KindDeniedCall] != 1 {
		t.Errorf("Write filter = %+v", got)
	}
}

// --glob matches the stored path. A redacted path is stored as NULL and
// never matches, not even "**".
func TestGlobMatchesStoredPathAndNullNeverMatches(t *testing.T) {
	store := newSessionStore(t)
	source := &fakeSessionSource{events: joinEvents(
		hunkEditEvents("src/deep/a.go", 0), revertEvents("docs/b.md", time.Hour),
		redactedEditEvents(2*time.Hour), []capture.SessionEvent{promptEvent("p", 3*time.Hour)})}
	ingestSessions(t, store, source)
	cfg := capture.DefaultConfig()

	for glob, want := range map[string]map[capture.CorrectionKind]int{
		"src/**":   {capture.KindHunkEdit: 1},
		"*.md":     {capture.KindRevert: 1},
		"**":       {capture.KindHunkEdit: 1, capture.KindRevert: 1},
		"nomatch*": {},
	} {
		got := kindCounts(sessionReport(t, store, source, cfg, memory.SessionFilter{Glob: glob}))
		if len(got) != len(want) {
			t.Errorf("glob %q = %+v, want %+v", glob, got, want)
		}
		for kind, count := range want {
			if got[kind] != count {
				t.Errorf("glob %q kind %s = %d, want %d", glob, kind, got[kind], count)
			}
		}
	}
}

func TestGlobRespectsSinceToolAndSwitches(t *testing.T) {
	store, source := ingestedStore(t)
	cfg := capture.DefaultConfig()
	cfg.Signals[capture.SignalRevert] = false

	report := sessionReport(t, store, source, cfg, memory.SessionFilter{Glob: "**", Since: sessionBase.Add(time.Hour)})

	if len(report.Kinds) != 0 {
		t.Errorf("revert is off and the hunk-edit is before --since, got %+v", report.Kinds)
	}
}

func TestMalformedGlobIsAnError(t *testing.T) {
	store, source := ingestedStore(t)

	_, err := store.SessionReport(context.Background(), source, capture.DefaultConfig(), memory.SessionFilter{Glob: "[a"})

	if err == nil {
		t.Error("a malformed glob must be rejected")
	}
}

func TestHeaderShowsTheNewestEventTimeOrUnknown(t *testing.T) {
	store, source := ingestedStore(t)
	cases := map[string]*fakeSessionSource{
		"2026-10-18T14:00:00.000Z": {newest: sessionBase.Add(2 * time.Hour), hasNewest: true},
		"unknown":                  {},
	}
	for want, header := range cases {
		report := sessionReport(t, store, header, capture.DefaultConfig(), memory.SessionFilter{})
		if report.NewestEventAt != want {
			t.Errorf("NewestEventAt = %q, want %q", report.NewestEventAt, want)
		}
	}
	_ = source
}

func TestAbsentTableReportsNoDataOrNeedsIngest(t *testing.T) {
	store := newSessionStore(t)

	none := sessionReport(t, store, &fakeSessionSource{}, capture.DefaultConfig(), memory.SessionFilter{})
	files := sessionReport(t, store, &fakeSessionSource{hasFiles: true}, capture.DefaultConfig(), memory.SessionFilter{})

	if none.State != memory.SessionStateNoData || files.State != memory.SessionStateNeedsIngest {
		t.Errorf("states = %q and %q", none.State, files.State)
	}
	if !strings.HasPrefix(memory.SessionNeedsIngestMessage, memory.SessionNoDataMessage) ||
		!strings.Contains(memory.SessionNeedsIngestMessage, "`loom memory ingest`") {
		t.Errorf("messages = %q / %q", memory.SessionNoDataMessage, memory.SessionNeedsIngestMessage)
	}
}

// The report is read-only: it must not create the table it found missing.
func TestReportDoesNotCreateTheTable(t *testing.T) {
	store := newSessionStore(t)

	sessionReport(t, store, &fakeSessionSource{hasFiles: true}, capture.DefaultConfig(), memory.SessionFilter{})

	if hasTable(t, store, "session_corrections") {
		t.Error("the report created session_corrections")
	}
}
