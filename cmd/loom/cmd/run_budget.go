package cmd

import (
	"errors"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/spf13/cobra"
)

// ExitCodeBudgetExhausted is the exit code for a run its budget halted
// (roadmap L4.3). Distinct from a failure, because nothing broke: the run
// checkpointed before the next stage and resumes with a larger budget.
const ExitCodeBudgetExhausted = 4

// ExitCodeInterrupted is the exit code for a run stopped by a signal after
// it checkpointed. 130 is the shell's convention for SIGINT, and a
// supervisor reads it as "stopped, resumable" rather than as a failure.
const ExitCodeInterrupted = 130

// budgetFlags are the flags that set a run's ceiling.
var budgetFlags = []string{"max-usd", "max-tokens", "max-hours"}

// applyBudgetFlags hands the executor a budget only when one was given on
// this command line. Otherwise a resume keeps the budget the run recorded —
// a ceiling belongs to the run, and dropping it because the resume command
// did not repeat it is how a stuck run would escape it.
func applyBudgetFlags(cmd *cobra.Command, executor *orchestrator.Executor) {
	executor.OnBudgetWarning(func(warning orchestrator.BudgetWarning) { reportBudgetWarning(cmd, warning) })
	if anyFlagChanged(cmd, budgetFlags) {
		executor.WithBudget(orchestrator.Budget{MaxUSD: runArgs.maxUSD, MaxTokens: runArgs.maxTokens, MaxHours: runArgs.maxHours})
	}
}

func anyFlagChanged(cmd *cobra.Command, names []string) bool {
	for _, name := range names {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return false
}

func reportBudgetWarning(cmd *cobra.Command, warning orchestrator.BudgetWarning) {
	cmd.PrintErrf("warning: budget %s at %.0f%% (%g of %g)\n",
		warning.Limit, 100*warning.Consumed/warning.Allowed, warning.Consumed, warning.Allowed)
}

// reportBudgetHalt prints where the money went when a budget stopped the
// run, so the decision to raise it is made against the numbers.
func reportBudgetHalt(cmd *cobra.Command, err error) {
	var exhausted *orchestrator.BudgetExhaustedError
	if !errors.As(err, &exhausted) {
		return
	}
	cmd.Println("Budget halt — spend by stage:")
	for _, stage := range exhausted.PerStage {
		if !stage.Measured {
			cmd.Printf("  %-24s unmeasured\n", stage.Stage)
			continue
		}
		cmd.Printf("  %-24s $%.4f  %d tokens\n", stage.Stage, stage.CostUSD, stage.Tokens)
	}
	cmd.Printf("Raise the ceiling to continue: loom run --spec %s --resume --max-usd <dollars>\n", runArgs.spec)
}
