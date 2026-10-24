package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
)

// writeStageArtifact creates a workspace artifact the way a markdown-pipeline
// agent would, and returns its path.
func writeStageArtifact(t *testing.T, projectDir, stageID, content string) string {
	t.Helper()
	workspace := filepath.Join(projectDir, ".claude", "feature-workspace", "user-auth")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	path := filepath.Join(workspace, stageID+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	return path
}

func recordStage(t *testing.T, binary, projectDir, spec, stageID, artifact string) {
	t.Helper()
	output, code := runLoom(t, binary, projectDir, "state", "record", "--spec", spec, "--stage", stageID, "--artifact", artifact)
	if code != 0 {
		t.Fatalf("state record %s: exit %d\n%s", stageID, code, output)
	}
}

// TestStateRecordVerifyCatchesHandEdit is Phase D's done-when: the markdown
// pipeline records checkpoints through this binary, and a hand-edited
// artifact is caught — with no model computing a hash anywhere in the path.
func TestStateRecordVerifyCatchesHandEdit(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)

	analysis := writeStageArtifact(t, projectDir, "analyst", "# analysis\n")
	implementation := writeStageArtifact(t, projectDir, "developer", "# implementation\n")
	recordStage(t, binary, projectDir, spec, "analyst", analysis)
	recordStage(t, binary, projectDir, spec, "developer", implementation)

	assertCleanVerify(t, binary, projectDir, spec)

	if err := os.WriteFile(analysis, []byte("# analysis, hand-edited\n"), 0o644); err != nil {
		t.Fatalf("edit artifact: %v", err)
	}
	assertVerifyCatchesEdit(t, binary, projectDir, spec)
}

// TestStateVerifyCatchesAnEditedSpec is review item I-1 for the markdown
// pipeline: `loom state record` binds the run to the spec's digest, so an
// edited spec demotes every recorded stage, just as it does under the executor.
func TestStateVerifyCatchesAnEditedSpec(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)
	recordStage(t, binary, projectDir, spec, "analyst", writeStageArtifact(t, projectDir, "analyst", "# analysis\n"))
	assertCleanVerify(t, binary, projectDir, spec)

	if err := os.WriteFile(spec, []byte("# Feature: user auth, with lockout added\n"), 0o644); err != nil {
		t.Fatalf("edit spec: %v", err)
	}
	output, code := runLoom(t, binary, projectDir, "state", "verify", "--spec", spec)
	if code == 0 || !strings.Contains(output, string(orchestrator.StaleReasonSpecChanged)) {
		t.Errorf("verify over an edited spec: exit %d, want a failure naming %s\n%s",
			code, orchestrator.StaleReasonSpecChanged, output)
	}
}

func assertCleanVerify(t *testing.T, binary, projectDir, spec string) {
	t.Helper()
	output, code := runLoom(t, binary, projectDir, "state", "verify", "--spec", spec)
	if code != 0 || !strings.Contains(output, "analyst") || !strings.Contains(output, "OK") {
		t.Fatalf("clean verify: exit %d\n%s", code, output)
	}
}

func assertVerifyCatchesEdit(t *testing.T, binary, projectDir, spec string) {
	t.Helper()
	output, code := runLoom(t, binary, projectDir, "state", "verify", "--spec", spec)
	if code == 0 {
		t.Fatalf("verify passed over a hand-edited artifact:\n%s", output)
	}
	if !strings.Contains(output, string(orchestrator.StaleReasonEdited)) {
		t.Errorf("verify output should name the edited stage:\n%s", output)
	}
	// The cascade follows recording order, which is the only order the
	// markdown pipeline has.
	if !strings.Contains(output, string(orchestrator.StaleReasonUpstream)) {
		t.Errorf("verify should demote the stage recorded after the edited one:\n%s", output)
	}
}

func TestStateRecordAssignsAndPreservesSequence(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)

	for _, stageID := range []string{"analyst", "developer", "code-reviewer"} {
		recordStage(t, binary, projectDir, spec, stageID, writeStageArtifact(t, projectDir, stageID, "# "+stageID+"\n"))
	}
	// A CHANGES REQUESTED loop re-runs the developer: same step of the run,
	// so the sequence must not move.
	revised := writeStageArtifact(t, projectDir, "developer", "# implementation, revised\n")
	recordStage(t, binary, projectDir, spec, "developer", revised)

	state := loadRunState(t, projectDir)
	if got := state.Stages["developer"].Sequence; got != 2 {
		t.Errorf("developer sequence = %d, want 2 (unchanged by the re-run)", got)
	}
	if got := state.StagesInSequence(); strings.Join(got, ",") != "analyst,developer,code-reviewer" {
		t.Errorf("sequence order = %v", got)
	}
	if state.CreatedBy != orchestrator.CreatedByMarkdown {
		t.Errorf("state createdBy = %q, want markdown", state.CreatedBy)
	}
}

func TestStateApproveRecordsAnAuditableApproval(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)
	recordStage(t, binary, projectDir, spec, "analyst", writeStageArtifact(t, projectDir, "analyst", "# analysis\n"))

	output, code := runLoom(t, binary, projectDir, "state", "approve", "--spec", spec, "--gate", "confirm-design")
	if code != 0 {
		t.Fatalf("state approve: exit %d\n%s", code, output)
	}
	if !strings.Contains(output, "audit record") {
		t.Errorf("approve must say plainly that it records rather than enforces:\n%s", output)
	}

	approval := loadRunState(t, projectDir).Approvals["confirm-design"]
	if approval.Method != orchestrator.ApprovalMethodCLI || approval.Approver == "" || approval.ApprovedAt.IsZero() {
		t.Errorf("approval record incomplete: %+v", approval)
	}
}

func TestStateShowReportsWhereTheRunStands(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)
	recordStage(t, binary, projectDir, spec, "analyst", writeStageArtifact(t, projectDir, "analyst", "# analysis\n"))
	recordStage(t, binary, projectDir, spec, "developer", writeStageArtifact(t, projectDir, "developer", "# implementation\n"))

	output, code := runLoom(t, binary, projectDir, "state", "show", "--spec", spec)
	if code != 0 || !strings.Contains(output, "user-auth") || !strings.Contains(output, "developer") {
		t.Fatalf("state show: exit %d\n%s", code, output)
	}

	output, code = runLoom(t, binary, projectDir, "state", "show", "--spec", spec, "--json")
	if code != 0 {
		t.Fatalf("state show --json: exit %d\n%s", code, output)
	}
	var decoded orchestrator.RunState
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("--json output does not parse: %v\n%s", err, output)
	}
	if decoded.Stages["developer"].Sequence != 2 {
		t.Errorf("decoded state missing sequence: %+v", decoded.Stages["developer"])
	}
}

func TestStateAndRunRefuseEachOthersRuns(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)
	recordStage(t, binary, projectDir, spec, "analyst", writeStageArtifact(t, projectDir, "analyst", "# analysis\n"))

	output, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock", "--resume")
	if code == 0 || !strings.Contains(output, "markdown") {
		t.Errorf("loom run accepted markdown-pipeline state: exit %d\n%s", code, output)
	}

	executorDir := t.TempDir()
	executorSpec := writeSpec(t, executorDir)
	if _, code := runLoom(t, binary, executorDir, "run", "--spec", executorSpec, "--provider", "mock"); code != ExitCodeWaitingApproval {
		t.Fatal("setup executor run did not halt at the first gate")
	}
	artifact := filepath.Join(executorDir, ".claude", "feature-workspace", "user-auth", "analyst.md")
	output, code = runLoom(t, binary, executorDir, "state", "record", "--spec", executorSpec, "--stage", "analyst", "--artifact", artifact)
	if code == 0 || !strings.Contains(output, "executor") {
		t.Errorf("loom state accepted executor state: exit %d\n%s", code, output)
	}
}

func TestStateVerifyWithoutCheckpointsFails(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	spec := writeSpec(t, projectDir)

	output, code := runLoom(t, binary, projectDir, "state", "verify", "--spec", spec)
	if code == 0 || !strings.Contains(output, "no checkpoints") {
		t.Errorf("verify with no state should fail clearly: exit %d\n%s", code, output)
	}
}
