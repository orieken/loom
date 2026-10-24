package cmd

import (
	"strings"
	"testing"
)

// Roadmap L4.3's Done-when, through the real binary: a budgeted run halts
// cleanly at its ceiling with exit 4, reports per-stage spend, and resumes
// once the ceiling is raised.
func TestRunHaltsAtItsBudgetAndResumesWhenRaised(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)

	output, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock",
		"--mock-stage-cost", "0.6", "--max-usd", "0.5")
	if code != ExitCodeBudgetExhausted {
		t.Fatalf("budgeted run exit code = %d, want %d\n%s", code, ExitCodeBudgetExhausted, output)
	}
	for _, want := range []string{"Budget halt", "context-engineer", "$0.6000", "--max-usd"} {
		if !strings.Contains(output, want) {
			t.Errorf("halt output missing %q:\n%s", want, output)
		}
	}
	state := loadRunState(t, projectDir)
	if state.BudgetHalt == nil || state.BudgetHalt.BeforeStage != "analyst" || state.IsStageCompleted("analyst") {
		t.Fatalf("budget halt = %+v, want the run stopped before analyst", state.BudgetHalt)
	}

	output, code = runLoom(t, binary, projectDir, "run", "--spec", spec, "--resume", "--mock-stage-cost", "0.6", "--max-usd", "100")
	if code != ExitCodeWaitingApproval {
		t.Fatalf("resume with a raised ceiling exit code = %d, want it to reach the first gate (%d)\n%s",
			code, ExitCodeWaitingApproval, output)
	}
	if state := loadRunState(t, projectDir); state.BudgetHalt != nil || state.Budget.MaxUSD != 100 {
		t.Errorf("after resume budget = %+v, halt = %+v, want $100 recorded and the halt cleared", state.Budget, state.BudgetHalt)
	}
}

// A mock that reports nothing cannot be held to a dollar ceiling.
func TestRunRefusesToSpendAgainstAnUnmeasurableBudget(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)

	output, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock", "--max-usd", "10")
	if code != ExitCodeBudgetExhausted || !strings.Contains(output, "cannot be verified") {
		t.Fatalf("exit %d, want %d and the ceiling called unverifiable\n%s", code, ExitCodeBudgetExhausted, output)
	}
}
