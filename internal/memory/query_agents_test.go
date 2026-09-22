package memory_test

// Roadmap L3.13 — metrics counted from execution, not derived from prose.
//
// Most of these are about a number that was never measured. A scorecard
// averaging an unfinished stage's duration as 0, or reading an unreported
// cost as free, produces a confident wrong figure — which is the failure
// mode this whole workstream keeps meeting.

import (
	"testing"
	"time"

	"github.com/orieken/loom/internal/memory"
	"github.com/orieken/loom/internal/orchestrator"
)

// agentRun builds a run where the analyst finished twice-attempted, the
// developer never finished, and the qa-engineer failed — one of each shape
// the metrics have to tell apart.
func agentRun() (*orchestrator.RunState, []orchestrator.Event) {
	started := time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC)
	finished := started.Add(4 * time.Second)
	state := orchestrator.NewRunState("deliver-feature", orchestrator.CreatedByExecutor)
	state.FeatureName = "billing"
	state.StartedAt = started
	state.Stages["analyst"] = orchestrator.StageRecord{
		Status: orchestrator.StageStatusCompleted, Sequence: 1, Agent: "analyst",
		StartedAt: started, FinishedAt: &finished, Iteration: 2,
	}
	state.Stages["developer"] = orchestrator.StageRecord{
		Status: orchestrator.StageStatusWaitingApproval, Sequence: 2, Agent: "developer",
		StartedAt: started,
	}
	state.Stages["qa-engineer"] = orchestrator.StageRecord{
		Status: orchestrator.StageStatusFailed, Sequence: 3, Agent: "qa-engineer",
		StartedAt: started, FinishedAt: &finished,
	}
	return state, nil
}

func agentsOfDefaultRun(t *testing.T) map[string]memory.AgentMetrics {
	t.Helper()
	state, events := agentRun()
	return agentsFrom(t, state, events)
}

func agentsFrom(t *testing.T, state *orchestrator.RunState, events []orchestrator.Event) map[string]memory.AgentMetrics {
	t.Helper()
	store := openStore(t)
	if _, err := store.Ingest(state, events); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	metrics, err := store.Agents()
	if err != nil {
		t.Fatalf("Agents: %v", err)
	}
	byName := map[string]memory.AgentMetrics{}
	for _, agent := range metrics {
		byName[agent.Agent] = agent
	}
	return byName
}

func TestRetriesAndFailuresAreCountedPerAgent(t *testing.T) {
	agents := agentsOfDefaultRun(t)

	if got := agents["analyst"].Retried; got != 1 {
		t.Errorf("analyst retried = %d, want 1 — it took two attempts", got)
	}
	if got := agents["qa-engineer"].Failed; got != 1 {
		t.Errorf("qa-engineer failed = %d, want 1", got)
	}
	if got := agents["developer"].Retried; got != 0 {
		t.Errorf("developer retried = %d, want 0 — one attempt, still in flight", got)
	}
}

// A stage that never finished has no duration. Reading it as 0 would make
// the agent that got furthest from finishing look like the fastest.
func TestAnUnfinishedStageHasNoDurationRatherThanZero(t *testing.T) {
	agents := agentsOfDefaultRun(t)

	if agents["developer"].DurationP50Ms != nil {
		t.Errorf("developer p50 = %v, want absent — no stage of its finished",
			*agents["developer"].DurationP50Ms)
	}
	analyst := agents["analyst"].DurationP50Ms
	if analyst == nil {
		t.Fatal("analyst p50 is absent, but its stage finished")
	}
	if *analyst != 4000 {
		t.Errorf("analyst p50 = %dms, want 4000", *analyst)
	}
}

// Zero cost reported is not zero cost. The mock provider reports nothing,
// so a corpus of mock runs must not read as a corpus of free ones.
func TestUnreportedCostIsNotReportedAsFree(t *testing.T) {
	agents := agentsOfDefaultRun(t)

	if agents["analyst"].CostReported {
		t.Error("an agent that reported no usage is marked as having reported cost")
	}
	if agents["analyst"].CostUSD != 0 {
		t.Errorf("cost = %v, want 0 alongside costReported=false", agents["analyst"].CostUSD)
	}
}

// A rate needs something to be a rate of. An agent with no stages must
// report absent, not 0% — "never failed" and "never ran" are different.
func TestARateOfNothingIsAbsentNotZero(t *testing.T) {
	none := memory.AgentMetrics{Agent: "never-ran"}

	if none.RetryRate() != nil || none.FailureRate() != nil || none.CorrectionRate() != nil {
		t.Error("an agent with no stages reported a rate")
	}
	ran := memory.AgentMetrics{Agent: "ran", Stages: 4, Retried: 1}
	if got := ran.RetryRate(); got == nil || *got != 0.25 {
		t.Errorf("retry rate = %v, want 0.25", got)
	}
}

// Corrections carry L4.5's meaning: recorded, not adopted. The count has to
// reach the agent row, or the scorecard falls back to reading markdown.
func TestHumanCorrectionsReachTheAgentRow(t *testing.T) {
	state, events := agentRun()
	state.Corrections = []orchestrator.Correction{
		{Stage: "analyst", Agent: "analyst", Gate: "confirm-design", At: state.StartedAt},
		{Stage: "analyst", Agent: "analyst", Gate: "confirm-design", At: state.StartedAt},
	}

	agents := agentsFrom(t, state, events)

	if got := agents["analyst"].Corrections; got != 2 {
		t.Errorf("analyst corrections = %d, want 2", got)
	}
	if got := agents["analyst"].CorrectionRate(); got == nil || *got != 2 {
		t.Errorf("correction rate = %v, want 2 — one stage corrected twice", got)
	}
}

// Nearest-rank, so a reported p95 is a duration some stage really took.
// Interpolating over the handful of samples a real corpus holds invents one
// that nothing ever did.
func TestPercentilesAreDurationsThatActuallyHappened(t *testing.T) {
	state, events := agentRun()
	base := state.StartedAt
	for index, seconds := range []int{1, 2, 3, 10} {
		finished := base.Add(time.Duration(seconds) * time.Second)
		state.Stages[stageName(index)] = orchestrator.StageRecord{
			Status: orchestrator.StageStatusCompleted, Sequence: index + 10, Agent: "architect",
			StartedAt: base, FinishedAt: &finished,
		}
	}

	architect := agentsFrom(t, state, events)["architect"]

	if architect.DurationP95Ms == nil || *architect.DurationP95Ms != 10000 {
		t.Errorf("p95 = %v, want 10000 — the slowest stage that really ran", architect.DurationP95Ms)
	}
	if architect.DurationP50Ms == nil || *architect.DurationP50Ms != 2000 {
		t.Errorf("p50 = %v, want 2000", architect.DurationP50Ms)
	}
}

func stageName(index int) string {
	return "architect-" + string(rune('a'+index))
}
