package orchestrator_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
)

// Roadmap L4.3: a run halts cleanly at its budget and reports per-stage
// spend; usage the governor cannot see is unknown, not zero.

func costing(usd float64) *orchestrator.Usage {
	return &orchestrator.Usage{Model: "mock", CostUSD: usd, InputTokens: 100, OutputTokens: 50}
}

func pricedScripts(usd float64) map[string]mock.Script {
	return map[string]mock.Script{
		"analyst":     {ArtifactContent: "# analysis", Usage: costing(usd)},
		"developer":   {ArtifactContent: "# implementation", Usage: costing(usd)},
		"qa-engineer": {ArtifactContent: "# qa report", Usage: costing(usd)},
	}
}

func TestABudgetHaltsTheRunBeforeTheStageThatWouldExceedIt(t *testing.T) {
	executor, provider, store, input := newHarness(t, pricedScripts(0.6))
	executor.WithBudget(orchestrator.Budget{MaxUSD: 1})

	err := executor.Run(context.Background(), threeStagePlan(), input)

	var exhausted *orchestrator.BudgetExhaustedError
	if !errors.As(err, &exhausted) || !errors.Is(err, orchestrator.ErrBudgetExhausted) {
		t.Fatalf("err = %v, want a budget halt", err)
	}
	assertInvocations(t, provider, []string{"analyst", "developer"})
	state := mustLoad(t, store)
	halt := state.BudgetHalt
	if halt == nil || halt.BeforeStage != "qa-engineer" || halt.Limit != orchestrator.BudgetLimitUSD || halt.Consumed != 1.2 {
		t.Fatalf("halt = %+v, want the usd ceiling recorded before qa-engineer at $1.20", halt)
	}
	if len(exhausted.PerStage) != 2 || exhausted.PerStage[0].Stage != "analyst" || exhausted.PerStage[1].CostUSD != 0.6 {
		t.Errorf("per-stage spend = %+v, want analyst and developer at $0.60 each", exhausted.PerStage)
	}
	if !strings.Contains(err.Error(), "spent $1.2000 of the $1.0000 usd ceiling") || !strings.Contains(err.Error(), "qa-engineer") {
		t.Errorf("error %q does not say what was spent against what", err)
	}
}

func TestARunUnderBudgetCompletesAndWarnsNearTheCeiling(t *testing.T) {
	executor, _, store, input := newHarness(t, pricedScripts(0.3))
	executor.WithBudget(orchestrator.Budget{MaxUSD: 1})
	var warnings []orchestrator.BudgetWarning
	executor.OnBudgetWarning(func(warning orchestrator.BudgetWarning) { warnings = append(warnings, warning) })

	if err := executor.Run(context.Background(), threeStagePlan(), input); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if state := mustLoad(t, store); state.BudgetHalt != nil || state.Budget.MaxUSD != 1 {
		t.Errorf("state budget = %+v, halt = %+v, want the budget recorded and no halt", state.Budget, state.BudgetHalt)
	}
	if len(warnings) != 0 {
		t.Errorf("warned at %+v; $0.60 of $1 is under the 80%% threshold", warnings)
	}
}

func TestABudgetWarnsOnceSpendPassesEightyPercent(t *testing.T) {
	executor, _, _, input := newHarness(t, pricedScripts(0.45))
	executor.WithBudget(orchestrator.Budget{MaxUSD: 1})
	var warnings []orchestrator.BudgetWarning
	executor.OnBudgetWarning(func(warning orchestrator.BudgetWarning) { warnings = append(warnings, warning) })

	_ = executor.Run(context.Background(), threeStagePlan(), input)

	if len(warnings) != 1 || warnings[0].Consumed != 0.9 {
		t.Errorf("warnings = %+v, want one at $0.90 before qa-engineer", warnings)
	}
}

// A provider that reports nothing cannot be held to a dollar ceiling.
func TestAnUnmeasuredStageMakesTheCeilingUnverifiable(t *testing.T) {
	scripts := pricedScripts(0.1)
	scripts["analyst"] = mock.Script{ArtifactContent: "# analysis"}
	executor, provider, store, input := newHarness(t, scripts)
	executor.WithBudget(orchestrator.Budget{MaxUSD: 100})

	err := executor.Run(context.Background(), threeStagePlan(), input)

	if !errors.Is(err, orchestrator.ErrBudgetExhausted) || !strings.Contains(err.Error(), "cannot be verified") {
		t.Fatalf("err = %v, want an unverifiable-ceiling halt", err)
	}
	assertInvocations(t, provider, []string{"analyst"})
	if halt := mustLoad(t, store).BudgetHalt; halt == nil || len(halt.Unmeasured) != 1 || halt.Unmeasured[0] != "analyst" {
		t.Errorf("halt = %+v, want analyst named as unmeasured", halt)
	}
}

func TestATokenCeilingHalts(t *testing.T) {
	executor, provider, _, input := newHarness(t, pricedScripts(0.01))
	executor.WithBudget(orchestrator.Budget{MaxTokens: 150})

	err := executor.Run(context.Background(), threeStagePlan(), input)

	if !errors.Is(err, orchestrator.ErrBudgetExhausted) || !strings.Contains(err.Error(), "spent 150 of the 150 tokens ceiling") {
		t.Fatalf("err = %v, want a token halt at 150", err)
	}
	assertInvocations(t, provider, []string{"analyst"})
}

// Hours count time spent running, so a ceiling of zero-point-nothing halts
// after the first stage that took any time at all.
func TestAnHoursCeilingCountsActiveTime(t *testing.T) {
	executor, provider, _, input := newHarness(t, pricedScripts(0))
	executor.WithBudget(orchestrator.Budget{MaxHours: 1e-12})

	err := executor.Run(context.Background(), threeStagePlan(), input)

	if !errors.Is(err, orchestrator.ErrBudgetExhausted) || !strings.Contains(err.Error(), "hours ceiling") {
		t.Fatalf("err = %v, want an hours halt", err)
	}
	assertInvocations(t, provider, []string{"analyst"})
}

// A budget belongs to the run: a resume without one enforces what was
// recorded, and a resume with a larger one continues past the halt.
func TestAResumeKeepsTheRecordedBudgetUnlessRaised(t *testing.T) {
	executor, _, store, input := newHarness(t, pricedScripts(0.6))
	executor.WithBudget(orchestrator.Budget{MaxUSD: 1})
	_ = executor.Run(context.Background(), threeStagePlan(), input)

	again := orchestrator.NewExecutor(mock.New(pricedScripts(0.6)), store)
	if err := again.Run(context.Background(), threeStagePlan(), input); !errors.Is(err, orchestrator.ErrBudgetExhausted) {
		t.Fatalf("resume without a budget = %v, want the recorded $1 enforced", err)
	}
	raised := orchestrator.NewExecutor(mock.New(pricedScripts(0.6)), store).WithBudget(orchestrator.Budget{MaxUSD: 5})
	if err := raised.Run(context.Background(), threeStagePlan(), input); err != nil {
		t.Fatalf("resume with a raised budget: %v", err)
	}
	if state := mustLoad(t, store); state.BudgetHalt != nil || !state.IsStageCompleted("qa-engineer") {
		t.Errorf("halt = %+v, want the run completed and the halt cleared", state.BudgetHalt)
	}
}

func TestAZeroBudgetOnResumeClearsTheCeiling(t *testing.T) {
	executor, _, store, input := newHarness(t, pricedScripts(0.6))
	executor.WithBudget(orchestrator.Budget{MaxUSD: 1})
	_ = executor.Run(context.Background(), threeStagePlan(), input)

	cleared := orchestrator.NewExecutor(mock.New(pricedScripts(0.6)), store).WithBudget(orchestrator.Budget{})
	if err := cleared.Run(context.Background(), threeStagePlan(), input); err != nil {
		t.Fatalf("resume with no ceiling: %v", err)
	}
	if mustLoad(t, store).Budget != nil {
		t.Error("a zero budget on resume left the old ceiling recorded")
	}
}

func TestARunWithoutABudgetIsNeverHalted(t *testing.T) {
	executor, _, store, input := newHarness(t, map[string]mock.Script{})
	if err := executor.Run(context.Background(), threeStagePlan(), input); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if mustLoad(t, store).Budget != nil {
		t.Error("a run started without a budget recorded one")
	}
}

func TestUnmeasuredStagesIgnoresInternalAndInterruptedStages(t *testing.T) {
	plan := orchestrator.Plan{Stages: []orchestrator.Stage{
		{ID: "router", Internal: true}, {ID: "analyst"}, {ID: "developer"}, {ID: "qa"},
	}}
	state := &orchestrator.RunState{Stages: map[string]orchestrator.StageRecord{
		"router":    {Status: orchestrator.StageStatusCompleted},
		"analyst":   {Status: orchestrator.StageStatusInterrupted},
		"developer": {Status: orchestrator.StageStatusFailed},
		"qa":        {Status: orchestrator.StageStatusCompleted, Usage: costing(1)},
	}}
	if got := orchestrator.UnmeasuredStages(plan, state); len(got) != 1 || got[0] != "developer" {
		t.Errorf("UnmeasuredStages = %v, want only the failed developer", got)
	}
}

func TestBudgetIsZero(t *testing.T) {
	for _, tc := range []struct {
		budget orchestrator.Budget
		want   bool
	}{
		{orchestrator.Budget{}, true},
		{orchestrator.Budget{MaxUSD: 1}, false},
		{orchestrator.Budget{MaxTokens: 1}, false},
		{orchestrator.Budget{MaxHours: 1}, false},
	} {
		if got := tc.budget.IsZero(); got != tc.want {
			t.Errorf("%+v.IsZero() = %v, want %v", tc.budget, got, tc.want)
		}
	}
}
