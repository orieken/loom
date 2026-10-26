package factory

import (
	"fmt"

	"github.com/orieken/loom/internal/orchestrator"
)

// The exit codes `loom run` uses as the supervisor's protocol.
const (
	exitCompleted       = 0
	exitWaitingApproval = 3
	exitBudget          = 4
	exitInterrupted     = 130
)

type outcomeKind string

const (
	outcomeCompleted   outcomeKind = "completed"
	outcomeWaiting     outcomeKind = "waiting"
	outcomeBudget      outcomeKind = "budget"
	outcomeInterrupted outcomeKind = "interrupted"
	outcomeFailed      outcomeKind = "failed"
)

// outcome is how one `loom run` invocation ended, judged from the run's
// own state. The exit code says which way it went; the state says why.
type outcome struct {
	kind   outcomeKind
	gate   string
	class  orchestrator.FailureClass
	reason string
	spend  *Spend
	// unmeasured names the stages a budget could not verify.
	unmeasured []string
}

// judge reads an invocation's outcome. exitCode is nil when the process was
// not the supervisor's child — one a previous supervisor started — so only
// the state is there to read.
func judge(exitCode *int, state *orchestrator.RunState) outcome {
	if state == nil {
		return outcome{kind: outcomeFailed, class: orchestrator.FailureTerminal,
			reason: fmt.Sprintf("loom run %s before recording any run state — see the order's log", exitText(exitCode))}
	}
	if judged, ok := judgeFromState(state); ok {
		return judged
	}
	return judgeFromExit(exitCode)
}

// judgeFromState covers every outcome the state records unambiguously.
func judgeFromState(state *orchestrator.RunState) (outcome, bool) {
	if state.BudgetHalt != nil {
		return budgetOutcome(state), true
	}
	for _, stageID := range state.StagesInSequence() {
		record := state.Stages[stageID]
		if judged, ok := judgeStage(stageID, record); ok {
			return judged, true
		}
	}
	return outcome{}, false
}

func judgeStage(stageID string, record orchestrator.StageRecord) (outcome, bool) {
	switch record.Status {
	case orchestrator.StageStatusWaitingApproval:
		return outcome{kind: outcomeWaiting, gate: record.Gate}, true
	case orchestrator.StageStatusFailed:
		return outcome{kind: outcomeFailed, class: classOf(record),
			reason: fmt.Sprintf("stage %s failed: %s", stageID, record.Error)}, true
	case orchestrator.StageStatusInterrupted, orchestrator.StageStatusRunning:
		return outcome{kind: outcomeInterrupted, class: orchestrator.FailureTransient,
			reason: fmt.Sprintf("stage %s was interrupted", stageID)}, true
	}
	return outcome{}, false
}

// classOf treats an unclassified failure — state written before classes
// existed — as terminal: retrying it would spend money on a guess.
func classOf(record orchestrator.StageRecord) orchestrator.FailureClass {
	if record.FailureClass == "" {
		return orchestrator.FailureTerminal
	}
	return record.FailureClass
}

// judgeFromExit decides what the state leaves open. A run can fail without
// failing a stage — a spec edited mid-run, a stale approval — and only the
// exit code says so.
func judgeFromExit(exitCode *int) outcome {
	if exitCode == nil || *exitCode == exitCompleted {
		return outcome{kind: outcomeCompleted}
	}
	if *exitCode == exitInterrupted {
		return outcome{kind: outcomeInterrupted, class: orchestrator.FailureTransient, reason: "the run was interrupted"}
	}
	return outcome{kind: outcomeFailed, class: orchestrator.FailureTerminal,
		reason: fmt.Sprintf("loom run %s without failing a stage — see the order's log", exitText(exitCode))}
}

func budgetOutcome(state *orchestrator.RunState) outcome {
	halt := state.BudgetHalt
	reason := fmt.Sprintf("budget: %s ceiling reached before stage %s (%g of %g)",
		halt.Limit, halt.BeforeStage, halt.Consumed, halt.Allowed)
	if len(halt.Unmeasured) > 0 {
		reason = fmt.Sprintf("budget: %s ceiling cannot be verified — %v reported no usage", halt.Limit, halt.Unmeasured)
	}
	return outcome{kind: outcomeBudget, reason: reason, spend: spendOf(state), unmeasured: halt.Unmeasured}
}

// spendOf is the run's spend, stage by stage in the order they ran.
func spendOf(state *orchestrator.RunState) *Spend {
	spend := &Spend{TotalUSD: state.TotalUsage().CostUSD}
	for _, stageID := range state.StagesInSequence() {
		record := state.Stages[stageID]
		entry := orchestrator.StageSpend{Stage: stageID, Measured: record.Usage != nil}
		if record.Usage != nil {
			entry.CostUSD = record.Usage.CostUSD
			entry.Tokens = record.Usage.InputTokens + record.Usage.OutputTokens
		}
		spend.PerStage = append(spend.PerStage, entry)
	}
	return spend
}

func exitText(exitCode *int) string {
	if exitCode == nil {
		return "ended"
	}
	return fmt.Sprintf("exited %d", *exitCode)
}

// loadRunState reads an order's run state; nil when there is none yet.
func loadRunState(worktree, id string) (*orchestrator.RunState, error) {
	return orchestrator.NewStateStore(RunStatePath(worktree, id)).Load()
}
