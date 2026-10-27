package orchestrator_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/policy"
	"github.com/orieken/loom/internal/provider/mock"
)

// preShipPlan is a small plan with an analyst, the pre-ship-checks stage,
// and a gated devops-engineer after it — the same shape the built-in plan
// has around pre-ship-checks, so the stage's own behavior can be tested
// without scripting the whole built-in sequence.
func preShipPlan() orchestrator.Plan {
	return orchestrator.Plan{
		Name: "preship-plan",
		Stages: []orchestrator.Stage{
			{ID: "analyst", Agent: "analyst", Timeout: 5 * time.Second},
			{ID: orchestrator.PreShipChecksStageID, Agent: orchestrator.PreShipChecksStageID,
				Internal: true, Timeout: 5 * time.Second},
			{ID: "devops-engineer", Agent: "devops-engineer", Gate: "confirm-ship", Timeout: 5 * time.Second},
		},
	}
}

func preShipScripts() map[string]mock.Script {
	return map[string]mock.Script{
		"analyst":         {ArtifactContent: "# analysis"},
		"devops-engineer": {ArtifactContent: "# devops"},
	}
}

// newPreShipHarness is newHarness plus the one setting every pre-ship-check
// test needs: ProjectRoot pointed at the workspace, so bash -c has a
// directory that actually exists to run in.
func newPreShipHarness(t *testing.T, checks []policy.PreShipCheck) (*orchestrator.Executor, *orchestrator.StateStore, orchestrator.StageInput) {
	t.Helper()
	executor, _, store, input := newHarness(t, preShipScripts())
	input.ProjectRoot = input.WorkspaceDir
	executor.WithPreShipChecks(checks)
	return executor, store, input
}

func readPreShipReport(t *testing.T, input orchestrator.StageInput) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(input.WorkspaceDir, "pre-ship-checks.md"))
	if err != nil {
		t.Fatalf("read pre-ship-checks.md: %v", err)
	}
	return string(raw)
}

// extractCheckOutput pulls one check's fenced output block out of the
// rendered report, as a slice of lines, so a test can assert exactly what
// survived the last-50-lines trim.
func extractCheckOutput(t *testing.T, report, name string) []string {
	t.Helper()
	section := report[strings.Index(report, "## "+name):]
	fenceStart := strings.Index(section, "```\n")
	if fenceStart < 0 {
		t.Fatalf("no fenced output for check %q:\n%s", name, report)
	}
	body := section[fenceStart+len("```\n"):]
	body = body[:strings.Index(body, "\n```")]
	if body == "" {
		return nil
	}
	return strings.Split(body, "\n")
}

func TestPreShipChecksSkipsWithNoChecksConfigured(t *testing.T) {
	executor, store, input := newPreShipHarness(t, nil)

	err := executor.Run(context.Background(), preShipPlan(), input)

	if !errors.Is(err, orchestrator.ErrWaitingApproval) {
		t.Fatalf("Run error = %v, want a halt at confirm-ship", err)
	}
	record := mustLoad(t, store).Stages[orchestrator.PreShipChecksStageID]
	if record.Status != orchestrator.StageStatusSkipped {
		t.Errorf("pre-ship-checks status = %q, want SKIPPED", record.Status)
	}
	if record.SkipReason != "no preShipChecks configured" {
		t.Errorf("SkipReason = %q, want the exact self-skip reason", record.SkipReason)
	}
}

func TestPreShipChecksCompletesAndRecordsEveryCheckOnAllPass(t *testing.T) {
	checks := []policy.PreShipCheck{
		{Name: "first", Command: "true", Timeout: 5 * time.Second},
		{Name: "second", Command: "true", Timeout: 5 * time.Second},
	}
	executor, store, input := newPreShipHarness(t, checks)

	err := executor.Run(context.Background(), preShipPlan(), input)

	if !errors.Is(err, orchestrator.ErrWaitingApproval) {
		t.Fatalf("Run error = %v, want a halt at confirm-ship", err)
	}
	record := mustLoad(t, store).Stages[orchestrator.PreShipChecksStageID]
	if record.Status != orchestrator.StageStatusCompleted {
		t.Fatalf("pre-ship-checks status = %q, want COMPLETED", record.Status)
	}
	if record.ArtifactPath == "" || record.ArtifactSHA256 == "" {
		t.Error("a completed pre-ship-checks stage recorded no artifact")
	}
	report := readPreShipReport(t, input)
	for _, name := range []string{"first", "second"} {
		if !strings.Contains(report, "## "+name) {
			t.Errorf("report does not record check %q:\n%s", name, report)
		}
	}
}

func TestPreShipChecksRunsEachCommandWithTheBaseCommitEnvVar(t *testing.T) {
	checks := []policy.PreShipCheck{{Name: "base", Command: `echo "BASE=$LOOM_BASE_COMMIT"`, Timeout: 5 * time.Second}}
	executor, _, input := newPreShipHarness(t, checks)
	executor.WithWorkTree(&changingTree{head: "feedface", digests: []string{"d"}})

	if err := executor.Run(context.Background(), preShipPlan(), input); !errors.Is(err, orchestrator.ErrWaitingApproval) {
		t.Fatalf("Run error = %v, want a halt at confirm-ship", err)
	}

	report := readPreShipReport(t, input)
	if !strings.Contains(report, "BASE=feedface") {
		t.Errorf("report does not show LOOM_BASE_COMMIT reaching the check:\n%s", report)
	}
}

func TestPreShipChecksFailsOnANonZeroExitAndStopsBeforeLaterChecks(t *testing.T) {
	checks := []policy.PreShipCheck{
		{Name: "first", Command: "true", Timeout: 5 * time.Second},
		{Name: "second", Command: "false", Timeout: 5 * time.Second},
		{Name: "third", Command: "true", Timeout: 5 * time.Second},
	}
	executor, store, input := newPreShipHarness(t, checks)

	err := executor.Run(context.Background(), preShipPlan(), input)

	if err == nil || errors.Is(err, orchestrator.ErrWaitingApproval) {
		t.Fatalf("Run error = %v, want a terminal failure naming the failing check", err)
	}
	if !strings.Contains(err.Error(), "second") {
		t.Errorf("error %q does not name the failing check", err)
	}
	state := mustLoad(t, store)
	record := state.Stages[orchestrator.PreShipChecksStageID]
	if record.Status != orchestrator.StageStatusFailed {
		t.Fatalf("pre-ship-checks status = %q, want FAILED", record.Status)
	}
	if record.FailureClass != orchestrator.FailureTerminal {
		t.Errorf("FailureClass = %q, want terminal (no supervisor retry)", record.FailureClass)
	}
	if record.ArtifactPath == "" || record.ArtifactSHA256 == "" {
		t.Error("a failed pre-ship-checks stage must still keep its artifact")
	}
	if devops := state.Stages["devops-engineer"]; devops.Status != "" {
		t.Errorf("devops-engineer status = %q, want never reached", devops.Status)
	}
	report := readPreShipReport(t, input)
	if !strings.Contains(report, "## second") {
		t.Errorf("report does not keep the failing check:\n%s", report)
	}
	if strings.Contains(report, "## third") {
		t.Error("a check after the failing one ran — it must not")
	}
}

func TestPreShipChecksKeepsFullOutputOnlyForTheFailingCheck(t *testing.T) {
	checks := []policy.PreShipCheck{
		{Name: "noisy-pass", Command: "seq 1 55", Timeout: 5 * time.Second},
		{Name: "noisy-fail", Command: "seq 1 55; exit 1", Timeout: 5 * time.Second},
	}
	executor, _, input := newPreShipHarness(t, checks)

	_ = executor.Run(context.Background(), preShipPlan(), input)

	report := readPreShipReport(t, input)
	passLines := extractCheckOutput(t, report, "noisy-pass")
	if len(passLines) != 50 || passLines[0] != "6" || passLines[49] != "55" {
		t.Errorf("passing check kept lines %v, want the last 50 (6..55)", passLines)
	}
	failLines := extractCheckOutput(t, report, "noisy-fail")
	if len(failLines) != 55 || failLines[0] != "1" || failLines[54] != "55" {
		t.Errorf("failing check kept lines %v, want the full 55", failLines)
	}
}

func TestPreShipCheckTimesOutAndIsClassifiedTerminal(t *testing.T) {
	checks := []policy.PreShipCheck{{Name: "slow", Command: "sleep 5", Timeout: 50 * time.Millisecond}}
	executor, store, input := newPreShipHarness(t, checks)

	err := executor.Run(context.Background(), preShipPlan(), input)

	if err == nil {
		t.Fatal("Run succeeded; a check past its timeout must fail the stage")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error %q does not say the check timed out", err)
	}
	record := mustLoad(t, store).Stages[orchestrator.PreShipChecksStageID]
	if record.Status != orchestrator.StageStatusFailed {
		t.Errorf("pre-ship-checks status = %q, want FAILED (not interrupted — the run itself was not canceled)", record.Status)
	}
	if record.FailureClass != orchestrator.FailureTerminal {
		t.Errorf("FailureClass = %q, want terminal", record.FailureClass)
	}
}

// TestPreShipCheckFailsWhenItsWorkingDirectoryDoesNotExist covers the
// branch where the subprocess never starts at all — a failure exec reports
// as something other than *exec.ExitError, so it must still fail the check
// rather than being read as a zero exit code.
func TestPreShipCheckFailsWhenItsWorkingDirectoryDoesNotExist(t *testing.T) {
	checks := []policy.PreShipCheck{{Name: "unreachable", Command: "true", Timeout: 5 * time.Second}}
	executor, store, input := newPreShipHarness(t, checks)
	input.ProjectRoot = filepath.Join(input.WorkspaceDir, "does-not-exist")

	err := executor.Run(context.Background(), preShipPlan(), input)

	if err == nil {
		t.Fatal("Run succeeded; a check whose working directory does not exist must fail")
	}
	record := mustLoad(t, store).Stages[orchestrator.PreShipChecksStageID]
	if record.Status != orchestrator.StageStatusFailed {
		t.Errorf("status = %q, want FAILED", record.Status)
	}
}

// TestPreShipCheckOutputNeverReachesTheStageErrorOrStateRecord guards the
// security-reviewer's flagged boundary: a check's stdout/stderr (which may
// contain secret-shaped project output) must stay inside the workspace
// report file. record.Error is what a telemetry sink or run-events.jsonl
// would carry outward (architecture-guardrails.md #9 — telemetry records
// properties, not payloads), so proving it never contains the check's
// output is the fact that matters, not any one sink's plumbing.
func TestPreShipCheckOutputNeverReachesTheStageErrorOrStateRecord(t *testing.T) {
	const secretShaped = "sk-live-TOTALLY-FAKE-TEST-SECRET-00000000000000000000"
	checks := []policy.PreShipCheck{{Name: "leaky", Command: "echo " + secretShaped + "; exit 1", Timeout: 5 * time.Second}}
	executor, store, input := newPreShipHarness(t, checks)

	err := executor.Run(context.Background(), preShipPlan(), input)

	if err == nil {
		t.Fatal("Run succeeded; the failing check should have failed the stage")
	}
	if strings.Contains(err.Error(), secretShaped) {
		t.Errorf("Run error contains the check's raw output: %v", err)
	}
	record := mustLoad(t, store).Stages[orchestrator.PreShipChecksStageID]
	if strings.Contains(record.Error, secretShaped) {
		t.Errorf("stage record.Error contains the check's raw output: %q", record.Error)
	}
	report := readPreShipReport(t, input)
	if !strings.Contains(report, secretShaped) {
		t.Error("the workspace report must still keep the failing check's full output")
	}
}

func TestPreShipChecksStageIsPositionedBetweenTechWriterAndDevOps(t *testing.T) {
	plan := orchestrator.DefaultDeliverFeaturePlan()
	positions := map[string]int{}
	stages := map[string]orchestrator.Stage{}
	for index, stage := range plan.Stages {
		positions[stage.ID] = index
		stages[stage.ID] = stage
	}

	if !(positions["tech-writer"] < positions[orchestrator.PreShipChecksStageID] &&
		positions[orchestrator.PreShipChecksStageID] < positions["devops-engineer"]) {
		t.Fatalf("pre-ship-checks is not positioned between tech-writer and devops-engineer: %+v", positions)
	}
	preShip := stages[orchestrator.PreShipChecksStageID]
	if !preShip.Internal {
		t.Error("pre-ship-checks must be Internal")
	}
	if preShip.Gate != "" {
		t.Errorf("pre-ship-checks carries gate %q, want none", preShip.Gate)
	}
	if preShip.Skippable {
		t.Error("pre-ship-checks must be absent from the router's skippable-stage map")
	}
	if preShip.StateKind != "" {
		t.Errorf("pre-ship-checks StateKind = %q, want empty — it writes a markdown artifact", preShip.StateKind)
	}
}
