package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
)

func writeSpec(t *testing.T, dir string) string {
	t.Helper()
	spec := filepath.Join(dir, "user-auth.md")
	if err := os.WriteFile(spec, []byte("# Feature: user auth\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	return spec
}

func loadRunState(t *testing.T, projectDir string) *orchestrator.RunState {
	t.Helper()
	path := filepath.Join(projectDir, ".claude", "feature-workspace", "user-auth", orchestrator.RunStateFileName)
	state, err := orchestrator.NewStateStore(path).Load()
	if err != nil {
		t.Fatalf("load run state: %v", err)
	}
	return state
}

// stagesBeforeFirstGate are the built-in plan's stages ahead of the
// confirm-design gate that always run. The conditional ones between the
// router and the gate are routed out for the mock provider's analysis,
// which declares no crossing, migration, threshold, or UI surface.
var stagesBeforeFirstGate = []string{"context-engineer", "analyst", "router"}

// stagesRoutedOut are skipped for the mock provider's scripted analysis.
var stagesRoutedOut = []string{"architect", "performance-engineer", "data-engineer", "accessibility-engineer", "devops-engineer"}

// TestRunMockProviderHaltsAtFirstGate runs the real built binary against the
// mock provider: the run executes every ungated stage, halts at the
// confirm-design gate, and refuses to restart over the resulting state.
// Same build-the-binary pattern as mcp_serve_test.go.
func TestRunMockProviderHaltsAtFirstGate(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)

	run := exec.Command(binary, "run", "--spec", spec, "--provider", "mock")
	run.Dir = projectDir
	output, err := run.CombinedOutput()
	if err == nil {
		t.Fatalf("run completed without approval; it must halt at confirm-design\n%s", output)
	}

	assertStagesCompleted(t, projectDir, stagesBeforeFirstGate, true)
	assertStagesSkipped(t, projectDir, stagesRoutedOut)
	assertWaitingOnGate(t, projectDir, "developer", "confirm-design")
	assertNoApprovals(t, projectDir)
	assertFreshRunOverStateRefused(t, binary, projectDir, spec)
}

func assertStagesCompleted(t *testing.T, projectDir string, stageIDs []string, wantArtifacts bool) {
	t.Helper()
	state := loadRunState(t, projectDir)
	if state == nil {
		t.Fatal("no run state persisted")
	}
	for _, stageID := range stageIDs {
		if !state.IsStageCompleted(stageID) {
			t.Errorf("stage %q not COMPLETED", stageID)
		}
		if wantArtifacts {
			assertArtifactExists(t, projectDir, stageID)
		}
	}
}

func assertWaitingOnGate(t *testing.T, projectDir, stageID, gate string) {
	t.Helper()
	state := loadRunState(t, projectDir)
	if got := state.Stages[stageID].Status; got != orchestrator.StageStatusWaitingApproval {
		t.Fatalf("stage %q status = %q, want WAITING_APPROVAL", stageID, got)
	}
	if got := state.WaitingGate(); got != gate {
		t.Errorf("run waiting on gate %q, want %q", got, gate)
	}
}

// assertNoApprovals is the L2.13 check at the CLI boundary: reaching a gate
// must never record an approval on its own.
func assertNoApprovals(t *testing.T, projectDir string) {
	t.Helper()
	if approvals := loadRunState(t, projectDir).Approvals; len(approvals) != 0 {
		t.Errorf("run recorded approvals %v without any human approving", approvals)
	}
}

// assertArtifactExists checks the artifact a stage actually produces: a
// typed stage's state document (roadmap L2.9), or a markdown file for the
// stages still on markdown.
func assertArtifactExists(t *testing.T, projectDir, stageID string) {
	t.Helper()
	if _, err := os.Stat(stageArtifactPath(projectDir, stageID)); err != nil {
		t.Errorf("stage %q artifact missing: %v", stageID, err)
	}
}

func assertStagesSkipped(t *testing.T, projectDir string, stageIDs []string) {
	t.Helper()
	state := loadRunState(t, projectDir)
	for _, stageID := range stageIDs {
		record := state.Stages[stageID]
		if record.Status != orchestrator.StageStatusSkipped {
			t.Errorf("stage %q status = %q, want SKIPPED", stageID, record.Status)
		}
		if record.SkipReason == "" {
			t.Errorf("stage %q was skipped with no recorded reason", stageID)
		}
	}
}

func stageArtifactPath(projectDir, stageID string) string {
	workspace := filepath.Join(projectDir, ".claude", "feature-workspace", "user-auth")
	for _, stage := range orchestrator.DefaultDeliverFeaturePlan().Stages {
		if stage.ID == stageID && stage.StateKind != "" {
			return filepath.Join(workspace, "state", stageID+".json")
		}
	}
	return filepath.Join(workspace, stageID+".md")
}

func assertFreshRunOverStateRefused(t *testing.T, binary, projectDir, spec string) {
	t.Helper()
	rerun := exec.Command(binary, "run", "--spec", spec, "--provider", "mock")
	rerun.Dir = projectDir
	output, err := rerun.CombinedOutput()
	if err == nil {
		t.Fatal("second run without --resume succeeded; must refuse to start over existing state")
	}
	if !strings.Contains(string(output), "--resume") {
		t.Errorf("refusal message should point at --resume, got:\n%s", output)
	}
}

// TestRunInterruptCheckpointAndResume kills a run mid-stage with SIGINT
// through the real CLI, asserts a resumable checkpoint was persisted, then
// resumes to completion — the M0.4 done-when criterion.
func TestRunInterruptCheckpointAndResume(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGINT delivery is POSIX-only in this test")
	}
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)

	interruptRunMidStage(t, binary, projectDir, spec)
	assertInterruptCheckpoint(t, projectDir)

	resume := exec.Command(binary, "run", "--spec", spec, "--provider", "mock", "--resume")
	resume.Dir = projectDir
	if output, err := resume.CombinedOutput(); err == nil {
		t.Fatalf("resumed run completed without approval; it must halt at confirm-design\n%s", output)
	}
	assertStagesCompleted(t, projectDir, stagesBeforeFirstGate, false)
	assertWaitingOnGate(t, projectDir, "developer", "confirm-design")
}

func assertInterruptCheckpoint(t *testing.T, projectDir string) {
	t.Helper()
	state := loadRunState(t, projectDir)
	if got := state.Stages["analyst"].Status; got != orchestrator.StageStatusInterrupted {
		t.Fatalf("hung stage status = %q, want INTERRUPTED", got)
	}
	for _, done := range []string{"context-engineer"} {
		if !state.IsStageCompleted(done) {
			t.Errorf("stage %q should be COMPLETED before the interrupt", done)
		}
	}
}

func interruptRunMidStage(t *testing.T, binary, projectDir, spec string) {
	t.Helper()
	run := exec.Command(binary, "run", "--spec", spec, "--provider", "mock", "--mock-hang-stage", "analyst")
	run.Dir = projectDir
	if err := run.Start(); err != nil {
		t.Fatalf("start hung run: %v", err)
	}
	waitForStageStatus(t, projectDir, "analyst", orchestrator.StageStatusRunning)
	if err := run.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}
	if err := run.Wait(); err == nil {
		t.Fatal("interrupted run exited 0; a canceled run must report the interruption")
	}
	// A supervisor reads 130 as "stopped and resumable", not as a failure.
	if code := run.ProcessState.ExitCode(); code != ExitCodeInterrupted {
		t.Errorf("interrupted run exit code = %d, want %d", code, ExitCodeInterrupted)
	}
}

func waitForStageStatus(t *testing.T, projectDir, stageID string, want orchestrator.StageStatus) {
	t.Helper()
	path := filepath.Join(projectDir, ".claude", "feature-workspace", "user-auth", orchestrator.RunStateFileName)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if stageStatusIs(path, stageID, want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("stage %q never reached %q", stageID, want)
}

func stageStatusIs(path, stageID string, want orchestrator.StageStatus) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var state orchestrator.RunState
	if json.Unmarshal(raw, &state) != nil {
		return false
	}
	return state.Stages[stageID].Status == want
}

func TestRunResumeWithoutStateFails(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)

	run := exec.Command(binary, "run", "--spec", spec, "--provider", "mock", "--resume")
	run.Dir = projectDir
	output, err := run.CombinedOutput()
	if err == nil {
		t.Fatal("--resume with no existing state succeeded; it must fail")
	}
	if !strings.Contains(string(output), "no run state") {
		t.Errorf("error should explain state is missing, got:\n%s", output)
	}
}

func runLoom(t *testing.T, binary, projectDir string, args ...string) (string, int) {
	t.Helper()
	run := exec.Command(binary, args...)
	run.Dir = projectDir
	output, err := run.CombinedOutput()
	if err == nil {
		return string(output), 0
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("loom %v: %v\n%s", args, err, output)
	}
	return string(output), exit.ExitCode()
}

// TestRunHaltExitCodeAndResumeCommand proves the non-interactive channel:
// the halted run exits 3 (not the generic failure code) and prints the
// exact command that continues it.
func TestRunHaltExitCodeAndResumeCommand(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)

	output, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock")
	if code != ExitCodeWaitingApproval {
		t.Fatalf("halted run exit code = %d, want %d\n%s", code, ExitCodeWaitingApproval, output)
	}
	for _, want := range []string{"confirm-design", "developer", "--resume --approve confirm-design"} {
		if !strings.Contains(output, want) {
			t.Errorf("halt output missing %q:\n%s", want, output)
		}
	}
	assertWaitingOnGate(t, projectDir, "developer", "confirm-design")
	assertNoApprovals(t, projectDir)
	if strings.Contains(output, "approve gate") {
		t.Errorf("non-interactive run printed an interactive prompt:\n%s", output)
	}
}

// TestRunApproveEachGateToCompletion is the end-to-end L2.13 demonstration:
// halt, approve, continue — three times — to all 14 stages completed.
func TestRunApproveEachGateToCompletion(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)

	if _, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock"); code != ExitCodeWaitingApproval {
		t.Fatalf("first leg exit code = %d, want %d", code, ExitCodeWaitingApproval)
	}
	for _, gate := range []string{"confirm-design", "confirm-security"} {
		output, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock", "--resume", "--approve", gate)
		if code != ExitCodeWaitingApproval {
			t.Fatalf("leg after approving %q exit code = %d, want %d\n%s", gate, code, ExitCodeWaitingApproval, output)
		}
	}
	output, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock", "--resume", "--approve", "confirm-ship")
	if code != 0 {
		t.Fatalf("final leg exit code = %d, want 0\n%s", code, output)
	}

	assertEveryStageSettled(t, projectDir)
	assertThreeFlagApprovals(t, projectDir)
}

// assertEveryStageSettled checks the run finished: every stage either ran
// or was routed around with a reason. Nothing is left pending.
func assertEveryStageSettled(t *testing.T, projectDir string) {
	t.Helper()
	state := loadRunState(t, projectDir)
	for _, stage := range orchestrator.DefaultDeliverFeaturePlan().Stages {
		if !state.IsStageSettled(stage.ID) {
			t.Errorf("stage %q is neither completed nor skipped after approving every gate: %q",
				stage.ID, state.Stages[stage.ID].Status)
		}
	}
	assertStagesSkipped(t, projectDir, stagesRoutedOut)
}

func assertThreeFlagApprovals(t *testing.T, projectDir string) {
	t.Helper()
	approvals := loadRunState(t, projectDir).Approvals
	if len(approvals) != 3 {
		t.Errorf("approvals = %v, want exactly the three plan gates", approvals)
	}
	for gate, approval := range approvals {
		if approval.Method != orchestrator.ApprovalMethodFlag || approval.Approver == "" {
			t.Errorf("gate %q approval incomplete: %+v", gate, approval)
		}
	}
}

func TestRunApproveRejectsWrongGateAndMissingResume(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)
	if _, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock"); code != ExitCodeWaitingApproval {
		t.Fatal("setup run did not halt at the first gate")
	}

	output, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock", "--resume", "--approve", "confirm-ship")
	if code == 0 || !strings.Contains(output, "confirm-design") {
		t.Errorf("approving a gate the run is not waiting on must fail and name the real gate, got exit %d:\n%s", code, output)
	}
	output, code = runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock", "--approve", "confirm-design")
	if code == 0 || !strings.Contains(output, "--resume") {
		t.Errorf("--approve without --resume must fail, got exit %d:\n%s", code, output)
	}
	assertWaitingOnGate(t, projectDir, "developer", "confirm-design")
}

// TestRunInterruptAfterApprovalKeepsRunResumable covers the SIGINT/gate
// interplay: an approved gate stays approved across an interrupt, and the
// resumed run is not asked to approve it again.
func TestRunInterruptAfterApprovalKeepsRunResumable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGINT delivery is POSIX-only in this test")
	}
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)
	if _, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock"); code != ExitCodeWaitingApproval {
		t.Fatal("setup run did not halt at the first gate")
	}

	interruptApprovedRun(t, binary, projectDir, spec)
	state := loadRunState(t, projectDir)
	if state.Stages["developer"].Status != orchestrator.StageStatusInterrupted {
		t.Fatalf("interrupted stage status = %q, want INTERRUPTED", state.Stages["developer"].Status)
	}
	if !state.IsGateApproved("confirm-design") {
		t.Fatal("interrupt discarded the recorded approval")
	}

	output, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock", "--resume")
	if code != ExitCodeWaitingApproval || !strings.Contains(output, "confirm-security") {
		t.Fatalf("resume should re-run developer and halt at the next gate, got exit %d:\n%s", code, output)
	}
}

func interruptApprovedRun(t *testing.T, binary, projectDir, spec string) {
	t.Helper()
	run := exec.Command(binary, "run", "--spec", spec, "--provider", "mock",
		"--resume", "--approve", "confirm-design", "--mock-hang-stage", "developer")
	run.Dir = projectDir
	if err := run.Start(); err != nil {
		t.Fatalf("start approved run: %v", err)
	}
	waitForStageStatus(t, projectDir, "developer", orchestrator.StageStatusRunning)
	if err := run.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}
	if err := run.Wait(); err == nil {
		t.Fatal("interrupted run exited 0; a canceled run must report the interruption")
	}
}

// TestAskApprovalReadsTheHumanAnswer unit-tests the TTY channel with a
// scripted reader rather than a real PTY.
func TestAskApprovalReadsTheHumanAnswer(t *testing.T) {
	cases := []struct {
		answer string
		want   bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{"n\n", false},
		{"\n", false},
		{"approved by the agent\n", false},
		{"", false},
	}
	waiting := &orchestrator.WaitingApprovalError{Gate: "confirm-ship", Stage: "devops-engineer"}
	for _, tc := range cases {
		t.Run(strings.TrimSpace(tc.answer), func(t *testing.T) {
			var prompt bytes.Buffer
			got, err := askApproval(&prompt, strings.NewReader(tc.answer), waiting)
			if err != nil {
				t.Fatalf("askApproval: %v", err)
			}
			if got != tc.want {
				t.Errorf("answer %q approved = %v, want %v", tc.answer, got, tc.want)
			}
			if !strings.Contains(prompt.String(), `approve gate "confirm-ship" for stage "devops-engineer"?`) {
				t.Errorf("prompt did not name the gate and stage: %q", prompt.String())
			}
		})
	}
}

func workspaceArtifact(projectDir, stageID string) string {
	return stageArtifactPath(projectDir, stageID)
}

// TestRunDetectsOutOfBandArtifactEdit is the L2.12 done-when through the
// real binary: a workspace file edited between runs invalidates that stage
// and everything recorded after it, the run says so, and it re-runs them.
func TestRunDetectsOutOfBandArtifactEdit(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)
	if _, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock"); code != ExitCodeWaitingApproval {
		t.Fatal("setup run did not halt at the first gate")
	}

	// The analyst's artifact is now its typed state document — L2.12's
	// integrity machinery covers typed state with no new mechanism.
	if err := os.WriteFile(workspaceArtifact(projectDir, "analyst"), []byte(`{"hand":"edited"}`), 0o644); err != nil {
		t.Fatalf("edit analyst state: %v", err)
	}
	output, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock", "--resume", "--approve", "confirm-design")
	if code != ExitCodeWaitingApproval {
		t.Fatalf("resume exit code = %d, want %d (halt at the next gate)\n%s", code, ExitCodeWaitingApproval, output)
	}
	for _, want := range []string{`stage "analyst" was COMPLETED but its artifact changed on disk`, `stage "router"`} {
		if !strings.Contains(output, want) {
			t.Errorf("resume output missing %q:\n%s", want, output)
		}
	}

	// Re-running the analyst rewrites the canned artifact, so the edit is
	// gone and every stage up to the next gate is COMPLETED again.
	assertStagesCompleted(t, projectDir, stagesBeforeFirstGate, true)

	// Since L2.14 the approval given on that same command line is bound to
	// the digests recorded before verification ran, so the edit resets it
	// and the run halts at confirm-design rather than continuing. A second
	// approval — of the state as it now stands — proceeds.
	assertWaitingOnGate(t, projectDir, "developer", "confirm-design")
	output, code = runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock", "--resume", "--approve", "confirm-design")
	if code != ExitCodeWaitingApproval {
		t.Fatalf("re-approval exit code = %d, want %d\n%s", code, ExitCodeWaitingApproval, output)
	}
	assertWaitingOnGate(t, projectDir, "qa-engineer", "confirm-security")
}

// TestRunEditAfterAnApprovalLeavesThatGateAlone is the other half of
// L2.14's binding rule: an approval binds only what was complete when the
// human gave it. The developer had not run when confirm-design was
// approved, so editing its artifact demotes the stage without resetting a
// gate that was never about that work.
func TestRunEditAfterAnApprovalLeavesThatGateAlone(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)
	if _, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock"); code != ExitCodeWaitingApproval {
		t.Fatal("setup run did not halt at the first gate")
	}
	if _, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock", "--resume", "--approve", "confirm-design"); code != ExitCodeWaitingApproval {
		t.Fatal("approved run did not reach the second gate")
	}

	if err := os.WriteFile(workspaceArtifact(projectDir, "developer"), []byte("# implementation, hand-edited\n"), 0o644); err != nil {
		t.Fatalf("edit developer artifact: %v", err)
	}
	output, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock", "--resume")
	if code != ExitCodeWaitingApproval {
		t.Fatalf("resume exit code = %d, want %d\n%s", code, ExitCodeWaitingApproval, output)
	}
	state := loadRunState(t, projectDir)
	if !state.IsGateApproved("confirm-design") {
		t.Error("an edit to work completed after the approval reset the gate; the binding is not that wide")
	}
	if strings.Contains(output, "approve gate") {
		t.Errorf("stale stage re-prompted for a gate whose binding still holds:\n%s", output)
	}
	// The demoted developer re-runs and the run halts at the next gate.
	assertWaitingOnGate(t, projectDir, "qa-engineer", "confirm-security")
}

func TestRunRecordsRunIdentityAndRefusesAnotherSpec(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)
	if _, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock"); code != ExitCodeWaitingApproval {
		t.Fatal("setup run did not halt at the first gate")
	}

	state := loadRunState(t, projectDir)
	if state.FeatureName != "user-auth" || state.SpecPath != spec || state.StartedAt.IsZero() {
		t.Errorf("run identity not recorded: feature %q, spec %q, startedAt %v", state.FeatureName, state.SpecPath, state.StartedAt)
	}

	otherSpec := filepath.Join(projectDir, "user-auth.md.copy.md")
	if err := os.WriteFile(otherSpec, []byte("# Feature: something else\n"), 0o644); err != nil {
		t.Fatalf("write second spec: %v", err)
	}
	output, code := runLoom(t, binary, projectDir, "run", "--spec", otherSpec, "--provider", "mock", "--resume")
	if code == 0 {
		t.Errorf("resuming against a different spec succeeded:\n%s", output)
	}
}

// TestRunRefusesApprovingIntoAStaleRun covers the trap L2.14 would
// otherwise create: approving a run whose own verification is about to
// reset that approval. The refusal names the artifact and says what to do.
func TestRunRefusesApprovingIntoAStaleRun(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)
	if _, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock"); code != ExitCodeWaitingApproval {
		t.Fatal("setup run did not halt at the first gate")
	}
	if _, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock", "--resume", "--approve", "confirm-design"); code != ExitCodeWaitingApproval {
		t.Fatal("approved run did not reach the second gate")
	}

	// Edit something bound by the approval the next command would give.
	if err := os.WriteFile(workspaceArtifact(projectDir, "analyst"), []byte(`{"hand":"edited"}`), 0o644); err != nil {
		t.Fatalf("edit analyst state: %v", err)
	}
	output, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock", "--resume", "--approve", "confirm-security")
	if code == 0 {
		t.Fatalf("approving into a stale run succeeded:\n%s", output)
	}
	assertOutputContains(t, output, "cannot approve", "analyst", "resume without --approve first")
	// Nothing was recorded: the human's decision was not spent.
	if _, recorded := loadRunState(t, projectDir).Approvals["confirm-security"]; recorded {
		t.Error("a refused approval was still written to state")
	}
}

func assertOutputContains(t *testing.T, output string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q:\n%s", want, output)
		}
	}
}

// TestRunExplainsWhyAnApprovalWasReset checks the halt is legible: an edit
// made on purpose should not read as an unexplained stop.
func TestRunExplainsWhyAnApprovalWasReset(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)
	if _, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock"); code != ExitCodeWaitingApproval {
		t.Fatal("setup run did not halt at the first gate")
	}
	if _, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock", "--resume", "--approve", "confirm-design"); code != ExitCodeWaitingApproval {
		t.Fatal("approved run did not reach the second gate")
	}
	if err := os.WriteFile(workspaceArtifact(projectDir, "analyst"), []byte(`{"hand":"edited"}`), 0o644); err != nil {
		t.Fatalf("edit analyst state: %v", err)
	}

	output, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock", "--resume")
	if code != ExitCodeWaitingApproval {
		t.Fatalf("resume exit code = %d, want %d\n%s", code, ExitCodeWaitingApproval, output)
	}
	assertOutputContains(t, output, `approval for gate "confirm-design" was reset`, `stage "analyst"`, "re-approve to continue")
	assertWaitingOnGate(t, projectDir, "developer", "confirm-design")
}

// TestRunRefusesARemovedDeliveryPolicyKey is ADR-010's fitness function for
// 4B: a delivery-policy.yaml still configuring auto-proceed stops the run
// before any stage starts, and names what to delete.
func TestRunRefusesARemovedDeliveryPolicyKey(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)
	writeTestFile(t, filepath.Join(projectDir, ".claude", "delivery-policy.yaml"), "mode: policy-driven\n")

	output, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock")

	if code == 0 || code == ExitCodeWaitingApproval {
		t.Fatalf("run with a removed key exit code = %d, want a failure\n%s", code, output)
	}
	for _, want := range []string{"`mode`", "ADR-010"} {
		if !strings.Contains(output, want) {
			t.Errorf("output does not name %s:\n%s", want, output)
		}
	}
	statePath := filepath.Join(projectDir, ".claude", "feature-workspace", "user-auth", orchestrator.RunStateFileName)
	if _, err := os.Stat(statePath); err == nil {
		t.Error("a stage ran: run state was written although the config was invalid")
	}
}
