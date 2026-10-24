package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
)

// TestRunDeliversAnIntakeOrderToCompletion is roadmap L3.63's Done-when and
// review item I-11: an intake file — work-order frontmatter and all — is a
// spec `loom run` accepts as it stands, from the relative path a supervisor
// would pass, through every gate to completion.
//
// What this does NOT prove: that a real analyst reads the frontmatter
// correctly. The executor handles only the spec's path; the claude provider
// passes that path to each agent, and the mock reads nothing. Agent behavior
// on an intake file is roadmap L3.74's to prove against the real provider.
func TestRunDeliversAnIntakeOrderToCompletion(t *testing.T) {
	binary := buildLoomBinary(t)
	projectDir := t.TempDir()
	order := filepath.Join("intake", "lockout.md")
	writeTestFile(t, filepath.Join(projectDir, order), validIntakeOrder)
	before := readForTest(t, filepath.Join(projectDir, order))

	if output, code := runLoom(t, binary, projectDir, "intake", "check", order); code != 0 {
		t.Fatalf("the order must be valid before it is run (exit %d)\n%s", code, output)
	}
	approveEveryGate(t, binary, projectDir, order)

	assertIntakeRunSettled(t, projectDir, "lockout")
	if after := readForTest(t, filepath.Join(projectDir, order)); !bytes.Equal(before, after) {
		t.Errorf("the run changed its own input; an intake file is human-owned\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// approveEveryGate drives the mock run through the built-in plan's three
// gates, approving each explicitly — the path a factory harness must take,
// since nothing an agent returns can approve a gate.
func approveEveryGate(t *testing.T, binary, projectDir, spec string) {
	t.Helper()
	if output, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock"); code != ExitCodeWaitingApproval {
		t.Fatalf("first leg exit code = %d, want %d\n%s", code, ExitCodeWaitingApproval, output)
	}
	gates := []string{"confirm-design", "confirm-security", "confirm-ship"}
	for index, gate := range gates {
		want := ExitCodeWaitingApproval
		if index == len(gates)-1 {
			want = 0
		}
		output, code := runLoom(t, binary, projectDir, "run", "--spec", spec, "--provider", "mock", "--resume", "--approve", gate)
		if code != want {
			t.Fatalf("leg after approving %q exit code = %d, want %d\n%s", gate, code, want, output)
		}
	}
}

// assertIntakeRunSettled checks the run is named for the order's id and that
// every stage of the plan either ran or was routed around.
func assertIntakeRunSettled(t *testing.T, projectDir, id string) {
	t.Helper()
	path := filepath.Join(projectDir, ".claude", "feature-workspace", id, orchestrator.RunStateFileName)
	state, err := orchestrator.NewStateStore(path).Load()
	if err != nil {
		t.Fatalf("no run state under the order's id %q: %v", id, err)
	}
	if state.FeatureName != id {
		t.Errorf("run feature = %q, want the order id %q", state.FeatureName, id)
	}
	for _, stage := range orchestrator.DefaultDeliverFeaturePlan().Stages {
		if !state.IsStageSettled(stage.ID) {
			t.Errorf("stage %q is neither completed nor skipped: %q", stage.ID, state.Stages[stage.ID].Status)
		}
	}
}

func readForTest(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return content
}
