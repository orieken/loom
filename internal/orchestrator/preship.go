package orchestrator

// The second executor-internal stage (after the router): running the
// project's declared preShipChecks — commands from .claude/delivery-policy.yaml
// that must exit 0 before a human is asked to approve shipping, closing the
// gap PR #18 exposed. Every agent's tests passed and the PR was approved,
// and CI's own changed-line coverage gate still rejected the change,
// because nothing in the pipeline had run the check CI runs.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/orieken/loom/internal/policy"
)

// noPreShipChecksConfigured is the exact self-skip reason a test asserts
// verbatim.
const noPreShipChecksConfigured = "no preShipChecks configured"

// preShipCheckReportName is where the stage's markdown report lives in the
// run's workspace. It is a plain artifact, not typed state: nothing
// downstream reads a pre-ship check's output, the same precedent the
// router's own route.md already set for an internal stage with no consumer.
const preShipCheckReportName = "pre-ship-checks.md"

// preShipCheckOutcome is one check's result, kept so the report can be
// rendered whichever way the stage ends.
type preShipCheckOutcome struct {
	Name     string
	Command  string
	ExitCode int
	Duration time.Duration
	Output   string
	TimedOut bool
}

// err reports this outcome as a failure, or nil when the check passed.
func (o preShipCheckOutcome) err() error {
	if o.TimedOut {
		return fmt.Errorf("timed out after %s", o.Duration)
	}
	if o.ExitCode != 0 {
		return fmt.Errorf("exited %d", o.ExitCode)
	}
	return nil
}

// runPreShipChecks runs every configured check in order at the project
// root, under its own timeout, with LOOM_BASE_COMMIT set to the run's
// StartCommit. The first failing or timed-out check stops the loop:
// continuing past a known failure would spend time, and for a check with
// shell side effects risk, on a result the run has already decided to fail.
func (e *Executor) runPreShipChecks(ctx context.Context, input StageInput, state *RunState) (StageOutput, error) {
	if len(e.preShipChecks) == 0 {
		return StageOutput{Skipped: true, SkipReason: noPreShipChecksConfigured}, nil
	}
	root := projectRootFor(input)
	outcomes := make([]preShipCheckOutcome, 0, len(e.preShipChecks))
	for _, check := range e.preShipChecks {
		outcome := runOnePreShipCheck(ctx, check, root, state.StartCommit)
		outcomes = append(outcomes, outcome)
		if err := outcome.err(); err != nil {
			return failPreShipChecks(input, outcomes, outcome.Name, err)
		}
	}
	path, err := writePreShipCheckReport(input, outcomes)
	if err != nil {
		return StageOutput{}, fmt.Errorf("pre-ship-checks: write report: %w", err)
	}
	return StageOutput{ArtifactPath: path}, nil
}

// failPreShipChecks writes the report so far — the failing check's full
// output included — and returns it alongside the error that failed the
// stage, so the AC's "full output kept in the stage's artifact" holds even
// though the stage ends FAILED rather than COMPLETED.
func failPreShipChecks(input StageInput, outcomes []preShipCheckOutcome, name string, cause error) (StageOutput, error) {
	path, writeErr := writePreShipCheckReport(input, outcomes)
	if writeErr != nil {
		return StageOutput{}, fmt.Errorf("pre-ship check %q: %w (its report could not be written: %v)", name, cause, writeErr)
	}
	return StageOutput{ArtifactPath: path}, fmt.Errorf("pre-ship check %q: %w", name, cause)
}

// runOnePreShipCheck runs one command through bash -c at root, with
// LOOM_BASE_COMMIT in its environment and ctx bounded by the check's own
// timeout (architecture-guardrails.md #5 — every subprocess call must have
// an explicit timeout). CombinedOutput, not separated streams: it is what a
// human running the command locally would see, and the AC asks only for
// "output".
func runOnePreShipCheck(ctx context.Context, check policy.PreShipCheck, root, baseCommit string) preShipCheckOutcome {
	checkCtx, cancel := context.WithTimeout(ctx, check.Timeout)
	defer cancel()
	cmd := exec.CommandContext(checkCtx, "bash", "-c", check.Command)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "LOOM_BASE_COMMIT="+baseCommit)
	start := time.Now()
	output, err := cmd.CombinedOutput()
	outcome := preShipCheckOutcome{
		Name: check.Name, Command: check.Command, Duration: time.Since(start), Output: string(output),
	}
	if checkCtx.Err() == context.DeadlineExceeded {
		outcome.TimedOut = true
		return outcome
	}
	outcome.ExitCode = exitCodeOf(err)
	return outcome
}

// exitCodeOf reads a subprocess's exit code, or -1 for a failure that never
// produced one (e.g. the command could not be started at all).
func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

// writePreShipCheckReport renders every check's outcome to the workspace.
func writePreShipCheckReport(input StageInput, outcomes []preShipCheckOutcome) (string, error) {
	var body strings.Builder
	body.WriteString("# Pre-Ship Checks\n\n")
	for _, outcome := range outcomes {
		renderPreShipCheckOutcome(&body, outcome)
	}
	path := filepath.Join(input.WorkspaceDir, preShipCheckReportName)
	if err := os.WriteFile(path, []byte(body.String()), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func renderPreShipCheckOutcome(body *strings.Builder, outcome preShipCheckOutcome) {
	fmt.Fprintf(body, "## %s\n\n- Command: `%s`\n- Duration: %s\n- Status: %s\n\n```\n%s\n```\n\n",
		outcome.Name, outcome.Command, outcome.Duration, outcome.status(), outcome.reportedOutput())
}

func (o preShipCheckOutcome) status() string {
	if o.TimedOut {
		return fmt.Sprintf("TIMED OUT after %s", o.Duration)
	}
	return fmt.Sprintf("exit %d", o.ExitCode)
}

// reportedOutput is the AC's own split: the last 50 lines for a check that
// passed, the full output for one that failed or timed out — a human
// diagnosing a failure needs everything, and 50 lines of a clean run is
// already more than anyone reads. Never sent to telemetry either way
// (architecture-guardrails.md #9) — this is the artifact file only.
func (o preShipCheckOutcome) reportedOutput() string {
	if o.err() != nil {
		return strings.TrimRight(o.Output, "\n")
	}
	return lastLines(o.Output, 50)
}

func lastLines(text string, n int) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) <= n {
		return text
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
