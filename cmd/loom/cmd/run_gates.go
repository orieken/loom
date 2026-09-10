package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/orieken/loom/internal/orchestrator"
	"github.com/spf13/cobra"
)

// ExitCodeWaitingApproval is the process exit code for a run halted at an
// approval gate. It is distinct from the generic failure code so CI and
// scripts can tell "waiting on a human" apart from "the pipeline broke".
const ExitCodeWaitingApproval = 3

// applyApproveFlag records the approval requested by --approve before the
// run starts. The flag is only meaningful while resuming a run that is
// actually halted on that gate — the executor enforces the second half, and
// requiring --resume stops a caller from pre-approving every gate on one
// command line and hollowing out the interrupt.
func applyApproveFlag(executor *orchestrator.Executor, gate string, resume bool) error {
	if gate == "" {
		return nil
	}
	if !resume {
		return fmt.Errorf("--approve %q is only valid with --resume — gates are approved as a run reaches them", gate)
	}
	if err := refuseStaleApproval(executor); err != nil {
		return err
	}
	return executor.Approve(gate, orchestrator.ApprovalMethodFlag)
}

// refuseStaleApproval stops a human from approving a run whose own
// verification is about to reset that approval. Recording it and then
// invalidating it in the same command wastes the decision and reads like a
// bug; the fix is to re-run the changed stage first, then approve what it
// produced (roadmap L2.14).
func refuseStaleApproval(executor *orchestrator.Executor) error {
	stale, err := executor.WouldInvalidateApprovals()
	if err != nil || stale == nil {
		return err
	}
	return fmt.Errorf("cannot approve %q: stage %q's artifact changed since it completed, which resets that approval — resume without --approve first to re-run it, then approve what it produces",
		stale.Gate, stale.ChangedStage)
}

// runWithGates drives the executor across gate halts: on a TTY it asks the
// human at the barrier and continues on yes; otherwise it halts so the
// approval arrives through --resume --approve. No other path writes an
// approval — provider output never does (roadmap L2.13).
func runWithGates(ctx context.Context, cmd *cobra.Command, executor *orchestrator.Executor, plan orchestrator.Plan, input orchestrator.StageInput) error {
	for {
		err := executor.Run(ctx, plan, input)
		var waiting *orchestrator.WaitingApprovalError
		if !errors.As(err, &waiting) {
			return err
		}
		approved, askErr := askAtBarrier(cmd, waiting)
		if askErr != nil {
			return askErr
		}
		if !approved {
			return haltForApproval(cmd, waiting, err)
		}
		if approveErr := executor.Approve(waiting.Gate, orchestrator.ApprovalMethodTTY); approveErr != nil {
			return approveErr
		}
	}
}

func askAtBarrier(cmd *cobra.Command, waiting *orchestrator.WaitingApprovalError) (bool, error) {
	if !stdinIsInteractive() {
		return false, nil
	}
	return askApproval(cmd.ErrOrStderr(), cmd.InOrStdin(), waiting)
}

// askApproval prompts for one gate and reports whether the human said yes.
// Anything other than y/yes is a no — the gate stays shut.
func askApproval(out io.Writer, in io.Reader, waiting *orchestrator.WaitingApprovalError) (bool, error) {
	if _, err := fmt.Fprintf(out, "approve gate %q for stage %q? [y/N] ", waiting.Gate, waiting.Stage); err != nil {
		return false, fmt.Errorf("write approval prompt: %w", err)
	}
	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read approval answer: %w", err)
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

// haltForApproval prints the exact command that resumes the run and returns
// the gate error, which Execute turns into exit code 3.
func haltForApproval(cmd *cobra.Command, waiting *orchestrator.WaitingApprovalError, err error) error {
	cmd.PrintErrf("Halted at gate %q before stage %q — approval required.%s\n",
		waiting.Gate, waiting.Stage, routedOutNote(waiting))
	cmd.PrintErrf("Approve and continue with: loom run --spec %s --resume --approve %s\n", runArgs.spec, waiting.Gate)
	return err
}

// routedOutNote says so when the gate guards a stage this run already
// routed out. The gate still halts — a human checkpoint must not vanish
// because routing removed the stage behind it — but asking someone to
// approve work that will not happen, without saying so, is what made this
// look like a routing bypass in run 4 (roadmap L3.32). Approving it
// completes the run; the stage stays skipped.
func routedOutNote(waiting *orchestrator.WaitingApprovalError) string {
	if waiting.SkipReason == "" {
		return ""
	}
	return fmt.Sprintf("\n  Note: %q was routed out of this run (%s), so approving this gate "+
		"completes the run rather than starting that stage.", waiting.Stage, waiting.SkipReason)
}

// reportApprovalReset explains a reset approval as it happens, so a halt
// caused by the human's own edit does not look like an unexplained stop.
func reportApprovalReset(cmd *cobra.Command, reset *orchestrator.StaleApprovalError) {
	cmd.PrintErrf("approval for gate %q was reset: stage %q's artifact changed after it was approved — re-approve to continue\n",
		reset.Gate, reset.ChangedStage)
}

// reportRoute says what the run is about to do, in one line, before the
// design gate asks a human to approve it. The reasons are in route.md.
func reportRoute(cmd *cobra.Command, summary orchestrator.RouteSummary, workspaceDir string) {
	if len(summary.Skipped) == 0 {
		cmd.Printf("routed %d of %d stages — nothing skipped\n", summary.Included, summary.Total)
		return
	}
	cmd.Printf("routed %d of %d stages — skipped %s (see %s)\n",
		summary.Included, summary.Total, strings.Join(summary.Skipped, ", "),
		filepath.Join(workspaceDir, "route.md"))
}

// reportLoopRound says which round is starting, so a run taking four
// attempts reads as progress rather than as a stuck process.
func reportLoopRound(cmd *cobra.Command, round orchestrator.LoopRound) {
	cmd.Printf("%s loop: changes requested — round %d of %d, re-running from %s\n",
		round.Loop, round.Iteration, round.Max, round.From)
}

// reportStaleStages tells the human which completed work is being redone
// and why, before any of it re-runs. Verification is not optional and has
// no flag to skip it — a way to opt out would reopen the hole it closes.
func reportStaleStages(cmd *cobra.Command, stale []orchestrator.StaleStage) {
	for _, stage := range stale {
		cmd.PrintErrln(stage.Description())
	}
}

// stdinIsInteractive reports whether stdin is a terminal, which is what
// separates "ask the human now" from "halt and print the resume command".
// A ModeCharDevice check is not enough: /dev/null is a character device, so
// it would make every CI run and every `exec.Command` child look like a
// human sitting at a keyboard.
func stdinIsInteractive() bool {
	fd := os.Stdin.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}
