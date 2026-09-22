package orchestrator_test

// Roadmap L3.16 — one run.started per run, one run.resumed per continuation.
//
// The TTY approval path re-enters Run after every gate, and Run emitted
// run.started unconditionally. The first real end-to-end run's timeline
// therefore showed run.started at 0s and again at 7m38s, immediately after
// gate.approved, leaving "how long did this run take" with no unambiguous
// answer and every consumer of the event log needing to know to ignore all
// but the first.
//
// Found by running the thing: every executor test drove Run once per
// assertion or resumed through --approve, and neither shape surfaced it. It
// took a human answering y at three consecutive gates — which is what these
// tests now do without one.

import (
	"context"
	"errors"
	"testing"

	"github.com/orieken/loom/internal/memory"
	"github.com/orieken/loom/internal/orchestrator"
)

// thriceGatedPlan halts before every stage, so driving it to completion
// takes four invocations of Run: one start and three resumes.
func thriceGatedPlan() orchestrator.Plan {
	plan := threeStagePlan()
	plan.Stages[0].Gate = "confirm-design"
	plan.Stages[1].Gate = "confirm-security"
	plan.Stages[2].Gate = "confirm-ship"
	return plan
}

// driveThroughGates answers every gate in turn and runs the plan to
// completion — the loop a human performs at a TTY, which is the shape that
// surfaced this defect and that no test had.
func driveThroughGates(t *testing.T, executor *orchestrator.Executor, plan orchestrator.Plan, input orchestrator.StageInput, gates ...string) {
	t.Helper()
	for _, gate := range gates {
		if err := executor.Run(context.Background(), plan, input); !errors.Is(err, orchestrator.ErrWaitingApproval) {
			t.Fatalf("run before %s = %v, want a halt at that gate", gate, err)
		}
		if err := executor.Approve(gate, orchestrator.ApprovalMethodTTY); err != nil {
			t.Fatalf("Approve %s: %v", gate, err)
		}
	}
	if err := executor.Run(context.Background(), plan, input); err != nil {
		t.Fatalf("final run: %v", err)
	}
}

func assertKindCount(t *testing.T, events []orchestrator.Event, kind orchestrator.EventKind, want int, why string) {
	t.Helper()
	if got := countKind(events, kind); got != want {
		t.Errorf("%s fired %d times, want %d — %s", kind, got, want, why)
	}
}

func countKind(events []orchestrator.Event, kind orchestrator.EventKind) int {
	count := 0
	for _, event := range events {
		if event.Kind == kind {
			count++
		}
	}
	return count
}

// TestARunResumedThreeTimesRecordsOneStart is the done-when, minus the
// episodic half, which the test below covers.
func TestARunResumedThreeTimesRecordsOneStart(t *testing.T) {
	executor, _, store, input := newHarness(t, completedScripts())

	driveThroughGates(t, executor, thriceGatedPlan(), input, "confirm-design", "confirm-security", "confirm-ship")

	events := readTimeline(t, store)
	assertKindCount(t, events, orchestrator.EventRunStarted, 1, "a run starts once however often it is re-entered")
	assertKindCount(t, events, orchestrator.EventRunResumed, 3, "one per continuation")
	assertKindCount(t, events, orchestrator.EventRunCompleted, 1, "the run settled once")
}

// TestTheResumeIsRecordedAfterTheApprovalThatCausedIt pins the ordering the
// ambiguity lived in. The first real run's timeline showed run.started
// immediately after gate.approved at 7m38s; what belongs there is
// run.resumed, and run.started belongs only at the top.
func TestTheResumeIsRecordedAfterTheApprovalThatCausedIt(t *testing.T) {
	executor, _, store, input := newHarness(t, completedScripts())
	plan := gatedPlan()

	_ = runUntilGate(t, executor, plan, input)
	if err := executor.Approve("confirm-design", orchestrator.ApprovalMethodTTY); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := executor.Run(context.Background(), plan, input); err != nil {
		t.Fatalf("resume: %v", err)
	}

	events := readTimeline(t, store)
	if events[0].Kind != orchestrator.EventRunStarted {
		t.Errorf("first event = %q, want run.started", events[0].Kind)
	}
	approved := indexOfKind(t, events, orchestrator.EventGateApproved)
	if resumed := indexOfKind(t, events, orchestrator.EventRunResumed); resumed < approved {
		t.Errorf("run.resumed at %d precedes the gate.approved at %d that caused it", resumed, approved)
	}
	assertNoSecondStart(t, events)
	assertTimelineIsOrdered(t, events)
}

func assertNoSecondStart(t *testing.T, events []orchestrator.Event) {
	t.Helper()
	for i, event := range events[1:] {
		if event.Kind == orchestrator.EventRunStarted {
			t.Errorf("a second run.started at index %d — this is the L3.16 defect", i+1)
		}
	}
}

func indexOfKind(t *testing.T, events []orchestrator.Event, kind orchestrator.EventKind) int {
	t.Helper()
	for i, event := range events {
		if event.Kind == kind {
			return i
		}
	}
	t.Fatalf("no %s event in %v", kind, eventKinds(events))
	return -1
}

// TestTheEpisodicStoreStillSeesOneRunAcrossResumes is the done-when's other
// half. Run identity is feature plus StartedAt, which is set once when the
// run is created — so what this actually guards is that a resume does not
// reset it, which is the property that would turn one run into four rows.
func TestTheEpisodicStoreStillSeesOneRunAcrossResumes(t *testing.T) {
	executor, _, store, input := newHarness(t, completedScripts())
	plan := thriceGatedPlan()

	identities := map[string]struct{}{}
	record := func() {
		state := mustLoad(t, store)
		identities[memory.RunID(state.FeatureName, state.StartedAt)] = struct{}{}
	}
	for _, gate := range []string{"confirm-design", "confirm-security", "confirm-ship"} {
		if err := executor.Run(context.Background(), plan, input); !errors.Is(err, orchestrator.ErrWaitingApproval) {
			t.Fatalf("run before %s = %v, want a halt", gate, err)
		}
		record()
		if err := executor.Approve(gate, orchestrator.ApprovalMethodTTY); err != nil {
			t.Fatalf("Approve %s: %v", gate, err)
		}
	}
	if err := executor.Run(context.Background(), plan, input); err != nil {
		t.Fatalf("final run: %v", err)
	}
	record()

	if len(identities) != 1 {
		t.Errorf("four invocations produced %d run identities, want 1", len(identities))
	}
}
