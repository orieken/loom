package memory_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/orieken/loom/internal/memory"
	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/state"
)

// fixtureRun builds a halted run with two stages, a correction, a policy
// decision, and reported usage — the shape epics 84 through 87 produce.
func fixtureRun() (*orchestrator.RunState, []orchestrator.Event) {
	started := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	finished := started.Add(90 * time.Second)
	state := orchestrator.NewRunState("deliver-feature", orchestrator.CreatedByExecutor)
	state.FeatureName = "user-auth"
	state.SpecPath = "/specs/user-auth.md"
	state.StartedAt = started
	state.Stages["analyst"] = orchestrator.StageRecord{
		Status: orchestrator.StageStatusCompleted, Sequence: 1, Agent: "analyst",
		StartedAt: started, FinishedAt: &finished,
		Usage: &orchestrator.Usage{InputTokens: 100, OutputTokens: 20, CostUSD: 0.5},
	}
	state.Stages["developer"] = orchestrator.StageRecord{
		Status: orchestrator.StageStatusWaitingApproval, Sequence: 2, Agent: "developer",
		Gate: "confirm-design", Iteration: 3,
	}
	state.Corrections = []orchestrator.Correction{{
		Stage: "analyst", Agent: "analyst", Gate: "confirm-design", At: finished,
		Stat: orchestrator.DiffStat{Added: 4, Removed: 1}, DiffPath: "/tmp/a.diff",
	}}
	state.PolicyDecisions = []orchestrator.PolicyRecord{{
		Gate: "confirm-design", Effect: "require-human", At: finished,
	}}
	events := []orchestrator.Event{
		{At: started, Kind: orchestrator.EventRunStarted},
		{At: started, Kind: orchestrator.EventStageStarted, Stage: "analyst", Sequence: 1},
		{At: finished, Kind: orchestrator.EventStageCompleted, Stage: "analyst", Sequence: 1},
		{At: finished, Kind: orchestrator.EventGateWaiting, Stage: "developer", Gate: "confirm-design"},
	}
	return state, events
}

func openStore(t *testing.T) *memory.Store {
	t.Helper()
	store, err := memory.Open(memory.DefaultPath(t.TempDir()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestIngestStoresARun(t *testing.T) {
	store := openStore(t)
	state, events := fixtureRun()

	run, err := store.Ingest(state, events)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if run.Feature != "user-auth" || run.Stages != 2 || run.Events != 4 {
		t.Errorf("run = %+v, want the fixture's two stages and four events", run)
	}
	// A run halted at a gate is not a completed run, and the distinction is
	// what anyone analysing history cares about.
	if run.Complete {
		t.Error("a run halted at a gate was recorded as complete")
	}
}

// Re-ingest must replace, not duplicate. A retrospective that double-counts
// a review loop's rounds is worse than one with no data, because it looks
// like an answer.
func TestReIngestIsANoOp(t *testing.T) {
	store := openStore(t)
	state, events := fixtureRun()

	first, err := store.Ingest(state, events)
	if err != nil {
		t.Fatalf("first Ingest: %v", err)
	}
	second, err := store.Ingest(state, events)
	if err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("run id changed between ingests: %q then %q", first.ID, second.ID)
	}
	assertCounts(t, store, map[string]int{"runs": 1, "stages": 2, "events": 4, "corrections": 1, "policy_decisions": 1})
}

// A resumed run keeps its identity: StartedAt is set once and survives
// every checkpoint, so ingesting before and after a resume updates one row.
func TestAResumedRunKeepsItsIdentity(t *testing.T) {
	store := openStore(t)
	state, events := fixtureRun()
	if _, err := store.Ingest(state, events); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	// The run continues: another stage completes and the timeline grows.
	state.Stages["qa-engineer"] = orchestrator.StageRecord{
		Status: orchestrator.StageStatusCompleted, Sequence: 3, Agent: "qa-engineer",
	}
	events = append(events, orchestrator.Event{Kind: orchestrator.EventRunCompleted, At: state.StartedAt})
	run, err := store.Ingest(state, events)
	if err != nil {
		t.Fatalf("second Ingest: %v", err)
	}

	if !run.Complete {
		t.Error("the completed run was not recorded as complete")
	}
	assertCounts(t, store, map[string]int{"runs": 1, "stages": 3, "events": 5})
}

// Most runs a human looks at are halted ones, so a run with no timeline at
// all must still ingest rather than being refused.
func TestARunWithNoEventsStillIngests(t *testing.T) {
	store := openStore(t)
	state, _ := fixtureRun()

	if _, err := store.Ingest(state, nil); err != nil {
		t.Fatalf("Ingest with no events: %v", err)
	}
	assertCounts(t, store, map[string]int{"runs": 1, "events": 0})
}

func TestIngestRefusesNilState(t *testing.T) {
	if _, err := openStore(t).Ingest(nil, nil); err == nil {
		t.Error("Ingest accepted nil state")
	}
}

// Two features must not collide, and neither must two runs of one feature
// started at different times.
func TestRunIdentityDoesNotCollide(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name            string
		featureA, featB string
		timeA, timeB    time.Time
	}{
		{"different features", "a", "b", now, now},
		{"same feature, different starts", "a", "a", now, now.Add(time.Second)},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if memory.RunID(testCase.featureA, testCase.timeA) == memory.RunID(testCase.featB, testCase.timeB) {
				t.Error("run identities collided")
			}
		})
	}
	// Stability across calls is what makes re-ingest an update rather than
	// an insert, so it is asserted rather than assumed. The time is
	// round-tripped through a format to prove the derivation does not depend
	// on the value's monotonic clock reading.
	first := memory.RunID("a", now)
	second := memory.RunID("a", now.Round(0))
	if first != second {
		t.Errorf("run identity is not stable: %q then %q", first, second)
	}
}

// A stage that never finished has no duration. Storing zero would make it
// the fastest stage in every report.
func TestUnfinishedStagesHaveNoDuration(t *testing.T) {
	store := openStore(t)
	state, events := fixtureRun()
	if _, err := store.Ingest(state, events); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	if got := nullableInt(t, store, `SELECT duration_ms FROM stages WHERE stage_id = 'developer'`); got != nil {
		t.Errorf("duration_ms = %v for a stage that never finished, want NULL", *got)
	}
	if got := nullableInt(t, store, `SELECT duration_ms FROM stages WHERE stage_id = 'analyst'`); got == nil || *got != 90000 {
		t.Errorf("duration_ms = %v for the finished stage, want 90000", got)
	}
}

func TestReadRecordsFromAWorkspace(t *testing.T) {
	dir := t.TempDir()
	state, events := fixtureRun()
	writeWorkspace(t, dir, state, events)

	records, err := memory.ReadRecords(dir)
	if err != nil {
		t.Fatalf("ReadRecords: %v", err)
	}
	if records.State.FeatureName != "user-auth" || len(records.Events) != 4 {
		t.Errorf("records = %+v, want the written run", records.State)
	}
}

func TestReadRecordsWithoutStateFails(t *testing.T) {
	if _, err := memory.ReadRecords(t.TempDir()); err == nil {
		t.Error("ReadRecords succeeded on a directory with no run state")
	}
}

// The archived copy is the durable record — the workspace is temporary.
func TestArchiveRecordsCopiesBothFiles(t *testing.T) {
	workspace, archive := t.TempDir(), t.TempDir()
	state, events := fixtureRun()
	writeWorkspace(t, workspace, state, events)

	if err := memory.ArchiveRecords(workspace, filepath.Join(archive, "user-auth")); err != nil {
		t.Fatalf("ArchiveRecords: %v", err)
	}
	for _, name := range []string{orchestrator.RunStateFileName, orchestrator.RunEventsFileName} {
		if _, err := os.Stat(filepath.Join(archive, "user-auth", name)); err != nil {
			t.Errorf("%s was not archived: %v", name, err)
		}
	}
}

// The typed stage documents are where the facts live. run-state.json says a
// stage completed and which KIND it produced; the document holds the review
// verdict, security findings, test results and changed paths — everything a
// policy reads. Archiving without them made every finished run unanalysable
// for those questions, which is what the L2.19 experiment found: four
// sourceable facts, all UNKNOWN, because the documents were gone.
func TestArchiveKeepsTheTypedStageDocuments(t *testing.T) {
	workspace, archive := t.TempDir(), t.TempDir()
	runState, events := fixtureRun()
	writeWorkspace(t, workspace, runState, events)
	writeTypedState(t, workspace)

	if err := memory.ArchiveRecords(workspace, filepath.Join(archive, "user-auth")); err != nil {
		t.Fatalf("ArchiveRecords: %v", err)
	}

	archived := filepath.Join(archive, "user-auth", state.TypedStateDir)
	for _, name := range []string{"code-reviewer.json", "qa-engineer.json"} {
		if _, err := os.Stat(filepath.Join(archived, name)); err != nil {
			t.Errorf("typed state %s was not archived: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(archived, "notes.txt")); err == nil {
		t.Error("notes.txt was archived; only .json stage documents should be")
	}
}

// writeTypedState lays down two stage documents and one file that is not
// one, so the test covers both what must be copied and what must not.
func writeTypedState(t *testing.T, workspace string) {
	t.Helper()
	dir := filepath.Join(workspace, state.TypedStateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create typed state dir: %v", err)
	}
	files := map[string]string{
		"code-reviewer.json": `{"schemaVersion":10}`,
		"qa-engineer.json":   `{"schemaVersion":10}`,
		"notes.txt":          "scratch",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

// Most runs predate typed state, and a run whose stages all produced
// markdown has none. That is ordinary, not a failure.
func TestArchiveToleratesNoTypedState(t *testing.T) {
	workspace, archive := t.TempDir(), t.TempDir()
	runState, events := fixtureRun()
	writeWorkspace(t, workspace, runState, events)

	if err := memory.ArchiveRecords(workspace, archive); err != nil {
		t.Errorf("ArchiveRecords failed with no typed state present: %v", err)
	}
}

// A run with no timeline yet is a normal state, not a failure.
func TestArchiveToleratesAMissingTimeline(t *testing.T) {
	workspace, archive := t.TempDir(), t.TempDir()
	state, _ := fixtureRun()
	writeWorkspace(t, workspace, state, nil)

	if err := memory.ArchiveRecords(workspace, archive); err != nil {
		t.Errorf("ArchiveRecords failed with no timeline present: %v", err)
	}
}
