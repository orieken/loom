package orchestrator

import (
	"strings"
	"testing"
	"time"
)

func unmeasuredRun(finished time.Time) (Plan, *RunState) {
	plan := Plan{Stages: []Stage{{ID: "analyst"}, {ID: "router", Internal: true}, {ID: "developer"}}}
	state := NewRunState("deliver-feature", CreatedByExecutor)
	state.Stages["analyst"] = StageRecord{Status: StageStatusCompleted, Sequence: 1, Usage: &Usage{CostUSD: 1}}
	state.Stages["router"] = StageRecord{Status: StageStatusCompleted, Sequence: 2}
	state.Stages["developer"] = StageRecord{Status: StageStatusFailed, Sequence: 3, FinishedAt: &finished}
	state.Spend = &Usage{CostUSD: 1}
	return plan, state
}

func TestAnUnmeasuredStageHaltsUntilAHumanAcceptsIt(t *testing.T) {
	plan, state := unmeasuredRun(time.Date(2026, 10, 26, 5, 40, 0, 0, time.UTC))
	budget := Budget{MaxUSD: 40}
	if halt, _ := evaluateBudget(budget, plan, state); halt == nil || strings.Join(halt.Unmeasured, ",") != "developer" {
		t.Fatalf("halt = %+v, want developer's unknown spend to halt the run", halt)
	}

	(&Executor{acceptUnmeasuredBy: "oscar"}).acceptUnmeasured(plan, state)

	if halt, _ := evaluateBudget(budget, plan, state); halt != nil {
		t.Errorf("halt = %+v after the human accepted the unknown spend", halt)
	}
	accepted := state.UnmeasuredAccepted["developer"]
	if accepted.AcceptedBy != "oscar" || !accepted.FinishedAt.Equal(*state.Stages["developer"].FinishedAt) || accepted.AcceptedAt.IsZero() {
		t.Errorf("acceptance = %+v, want oscar bound to developer's failed attempt", accepted)
	}
	if got := UnmeasuredStages(plan, state); strings.Join(got, ",") != "developer" {
		t.Errorf("unmeasured = %v; an accepted stage is still unmeasured, never zero", got)
	}
}

func TestAnAcceptanceDoesNotCoverALaterAttempt(t *testing.T) {
	plan, state := unmeasuredRun(time.Date(2026, 10, 26, 5, 40, 0, 0, time.UTC))
	(&Executor{acceptUnmeasuredBy: "oscar"}).acceptUnmeasured(plan, state)

	later := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	record := state.Stages["developer"]
	record.FinishedAt = &later
	state.Stages["developer"] = record

	if halt, _ := evaluateBudget(Budget{MaxUSD: 40}, plan, state); halt == nil {
		t.Error("a later attempt that also reported nothing was covered by the earlier acceptance")
	}
}

func TestNoAcceptanceWithoutAName(t *testing.T) {
	plan, state := unmeasuredRun(time.Date(2026, 10, 26, 5, 40, 0, 0, time.UTC))
	(&Executor{}).acceptUnmeasured(plan, state)
	if len(state.UnmeasuredAccepted) != 0 {
		t.Errorf("accepted = %+v with nobody named", state.UnmeasuredAccepted)
	}
}

// Accepting the unknown does not lift the ceiling on what was measured.
func TestAMeasuredCeilingStillHaltsAfterAnAcceptance(t *testing.T) {
	plan, state := unmeasuredRun(time.Date(2026, 10, 26, 5, 40, 0, 0, time.UTC))
	(&Executor{acceptUnmeasuredBy: "oscar"}).acceptUnmeasured(plan, state)
	halt, _ := evaluateBudget(Budget{MaxUSD: 1}, plan, state)
	if halt == nil || len(halt.Unmeasured) != 0 || halt.Consumed != 1 {
		t.Errorf("halt = %+v, want the $1 ceiling reached on measured spend alone", halt)
	}
}

func TestFinishedAtOfARecordThatNeverFinished(t *testing.T) {
	if got := finishedAt(StageRecord{}); !got.IsZero() {
		t.Errorf("finishedAt = %v", got)
	}
}
