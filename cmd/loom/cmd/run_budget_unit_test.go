package cmd

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/spf13/cobra"
)

// In-process tests for the budget plumbing, which the binary tests cover
// end to end but cannot measure.

func onePriceyStagePlan() orchestrator.Plan {
	return orchestrator.Plan{Name: "budget-test", Stages: []orchestrator.Stage{{ID: "analyst", Agent: "analyst"}, {ID: "developer", Agent: "developer"}}}
}

func runWithBudgetFlags(t *testing.T, flags map[string]string) *orchestrator.RunState {
	t.Helper()
	for name, value := range flags {
		if err := runCmd.Flags().Set(name, value); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, name := range budgetFlags {
			runCmd.Flags().Lookup(name).Changed = false
		}
		runArgs.maxUSD, runArgs.maxTokens, runArgs.maxHours = 0, 0, 0
	})
	plan := onePriceyStagePlan()
	store := orchestrator.NewStateStore(filepath.Join(t.TempDir(), orchestrator.RunStateFileName))
	executor := orchestrator.NewExecutor(newMockProvider(plan, mockOptions{stageCost: 1}), store)
	applyBudgetFlags(runCmd, executor)
	_ = executor.Run(context.Background(), plan, orchestrator.StageInput{WorkspaceDir: t.TempDir()})
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestBudgetFlagsSetTheRunsCeiling(t *testing.T) {
	state := runWithBudgetFlags(t, map[string]string{"max-usd": "0.5", "max-tokens": "100", "max-hours": "2"})
	if state.Budget == nil || state.Budget.MaxUSD != 0.5 || state.Budget.MaxTokens != 100 || state.Budget.MaxHours != 2 {
		t.Fatalf("budget = %+v, want every flag recorded", state.Budget)
	}
	if state.BudgetHalt == nil || state.BudgetHalt.BeforeStage != "developer" {
		t.Errorf("halt = %+v, want the run stopped before developer", state.BudgetHalt)
	}
}

func TestNoBudgetFlagLeavesTheRunUnbounded(t *testing.T) {
	if state := runWithBudgetFlags(t, nil); state.Budget != nil {
		t.Errorf("budget = %+v, want none", state.Budget)
	}
}

func TestReportBudgetHaltListsSpendByStage(t *testing.T) {
	var out bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(&out)
	reportBudgetHalt(command, &orchestrator.BudgetExhaustedError{PerStage: []orchestrator.StageSpend{
		{Stage: "analyst", CostUSD: 0.25, Tokens: 10, Measured: true}, {Stage: "developer"},
	}})
	for _, want := range []string{"Budget halt", "analyst", "$0.2500", "10 tokens", "developer", "unmeasured", "--max-usd"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report lacks %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "--accept-unmeasured") {
		t.Errorf("a halt with every stage measured offered to accept unmeasured spend:\n%s", out.String())
	}
	out.Reset()
	reportBudgetHalt(command, &orchestrator.BudgetExhaustedError{Halt: orchestrator.BudgetHalt{Unmeasured: []string{"developer"}}})
	if !strings.Contains(out.String(), "--accept-unmeasured <your name>") {
		t.Errorf("an unverifiable halt does not say how to accept it:\n%s", out.String())
	}
	out.Reset()
	reportBudgetHalt(command, errors.New("some other failure"))
	if out.Len() != 0 {
		t.Errorf("a non-budget error printed a budget report:\n%s", out.String())
	}
}

func TestReportBudgetWarningSaysHowClose(t *testing.T) {
	var out bytes.Buffer
	command := &cobra.Command{}
	command.SetErr(&out)
	reportBudgetWarning(command, orchestrator.BudgetWarning{Limit: "usd", Allowed: 10, Consumed: 9})
	if !strings.Contains(out.String(), "budget usd at 90%") {
		t.Errorf("warning = %q", out.String())
	}
}

func TestTheMockReportsNoUsageUnlessGivenACost(t *testing.T) {
	plan := onePriceyStagePlan()
	stage := plan.Stages[0]
	input := orchestrator.StageInput{WorkspaceDir: t.TempDir()}
	free, _ := selectProvider(plan, "mock", mockOptions{})
	if output, _ := free.Invoke(context.Background(), stage, input); output.Usage != nil {
		t.Errorf("a costless mock reported usage %+v", output.Usage)
	}
	priced, _ := selectProvider(plan, "mock", mockOptions{stageCost: 0.5})
	if output, _ := priced.Invoke(context.Background(), stage, input); output.Usage == nil || output.Usage.CostUSD != 0.5 {
		t.Errorf("a priced mock reported %+v, want $0.50", output.Usage)
	}
	if _, err := selectProvider(plan, "nonsense", mockOptions{}); err == nil {
		t.Error("an unknown provider was accepted")
	}
}
