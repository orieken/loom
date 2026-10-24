package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/factory"
	"github.com/orieken/loom/internal/orchestrator"
)

// The factory through the real binary and the mock provider. Every gate is
// approved explicitly through the inbox — nothing auto-approves.

func factoryOrder(id string) string {
	text := strings.Replace(validIntakeOrder, "id: lockout", "id: "+id, 1)
	return strings.Replace(text, "finish: pr", "finish: none", 1)
}

// newFactoryProject is a git repository with the given orders committed on main.
func newFactoryProject(t *testing.T, ids ...string) string {
	t.Helper()
	root := t.TempDir()
	gitForTest(t, root, "init", "-q", "-b", "main")
	gitForTest(t, root, "config", "user.email", "test@example.com")
	gitForTest(t, root, "config", "user.name", "test")
	writeTestFile(t, filepath.Join(root, "README.md"), "fixture\n")
	for _, id := range ids {
		writeTestFile(t, filepath.Join(root, "intake", id+".md"), factoryOrder(id))
	}
	gitForTest(t, root, "add", "-A")
	gitForTest(t, root, "commit", "-qm", "orders")
	return root
}

// factoryPass is one `loom factory run --once` against the mock. The mock
// reports a cost per stage, or the order's dollar ceiling could not be verified.
func factoryPass(t *testing.T, binary, root string, extra ...string) string {
	t.Helper()
	args := append([]string{"factory", "run", "--once", "--provider", "mock",
		"--run-arg=--mock-stage-cost=0.01", "--retry-base", "0s"}, extra...)
	output, code := runLoom(t, binary, root, args...)
	if code != 0 {
		t.Fatalf("factory pass exit %d\n%s", code, output)
	}
	return output
}

// Roadmap L3.67's Done-when: approving through the inbox produces the run
// state approving with `loom run --resume --approve` does — approvals made by
// the flag, bound to digests, every stage settled.
func TestFactoryDeliversAnOrderThroughEveryGateViaTheInbox(t *testing.T) {
	binary := buildLoomBinary(t)
	root := newFactoryProject(t, "alpha")

	for _, gate := range []string{"confirm-design", "confirm-security", "confirm-ship"} {
		factoryPass(t, binary, root)
		inbox, _ := runLoom(t, binary, root, "factory", "inbox")
		assertOutputContains(t, inbox, "alpha", "loom factory approve alpha "+gate)
		if output, code := runLoom(t, binary, root, "factory", "approve", "alpha", gate); code != 0 {
			t.Fatalf("approve %s exit %d\n%s", gate, code, output)
		}
	}
	factoryPass(t, binary, root)

	status, _ := runLoom(t, binary, root, "factory", "status")
	assertOutputContains(t, status, "alpha", string(factory.PhaseShipped))
	assertApprovedByFlag(t, root, "alpha", "confirm-design", "confirm-security", "confirm-ship")
}

func assertApprovedByFlag(t *testing.T, root, id string, gates ...string) {
	t.Helper()
	layout := factory.Layout{Root: root}
	state, err := orchestrator.NewStateStore(factory.RunStatePath(layout.WorktreePath(id), id)).Load()
	if err != nil || state == nil {
		t.Fatalf("run state: %v", err)
	}
	for _, gate := range gates {
		approval, ok := state.Approvals[gate]
		if !ok || approval.Method != orchestrator.ApprovalMethodFlag || len(approval.ArtifactDigests) == 0 {
			t.Errorf("approval for %s = %+v, want one made by --approve and bound to digests", gate, approval)
		}
	}
	for _, stage := range orchestrator.DefaultDeliverFeaturePlan().Stages {
		if !state.IsStageSettled(stage.ID) {
			t.Errorf("stage %s is %s, want settled", stage.ID, state.Stages[stage.ID].Status)
		}
	}
}

// Roadmap L3.65's Done-when, through the binary: three ready orders and two
// slots — two run, and the third runs once a slot is free.
func TestFactoryRunsTwoOrdersAtATime(t *testing.T) {
	binary := buildLoomBinary(t)
	root := newFactoryProject(t, "alpha", "bravo", "charlie")

	factoryPass(t, binary, root, "--max-concurrent", "2")
	status, _ := runLoom(t, binary, root, "factory", "status")
	assertOutputContains(t, status, "alpha", "bravo")
	if strings.Contains(status, "charlie") {
		t.Errorf("three orders started with two slots:\n%s", status)
	}

	factoryPass(t, binary, root, "--max-concurrent", "2")
	status, _ = runLoom(t, binary, root, "factory", "status")
	assertOutputContains(t, status, "charlie")
}

func TestFactoryRefusesToRunOutsideARepository(t *testing.T) {
	binary := buildLoomBinary(t)
	output, code := runLoom(t, binary, t.TempDir(), "factory", "status")
	if code == 0 || !strings.Contains(output, "git repository") {
		t.Errorf("exit %d, output %q; want a refusal outside git", code, output)
	}
}

func TestFactoryControlsRefuseTheWrongOrder(t *testing.T) {
	binary := buildLoomBinary(t)
	root := newFactoryProject(t, "alpha")
	factoryPass(t, binary, root)
	for _, args := range [][]string{
		{"factory", "stop", "alpha"}, {"factory", "resume", "alpha"}, {"factory", "rerun", "alpha"},
		{"factory", "approve", "alpha", "confirm-ship"}, {"factory", "stop", "nobody"},
	} {
		if output, code := runLoom(t, binary, root, args...); code == 0 {
			t.Errorf("loom %v succeeded; the order is waiting at confirm-design\n%s", args, output)
		}
	}
}

// Nothing is waiting before anything has run.
func TestFactoryInboxAndStatusStartEmpty(t *testing.T) {
	binary := buildLoomBinary(t)
	root := newFactoryProject(t)
	status, _ := runLoom(t, binary, root, "factory", "status")
	inbox, _ := runLoom(t, binary, root, "factory", "inbox")
	assertOutputContains(t, status, "No orders claimed yet")
	assertOutputContains(t, inbox, "Nothing is waiting on you")
}
