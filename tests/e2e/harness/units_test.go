package harness

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/factory"
	"github.com/orieken/loom/internal/intake"
	"github.com/orieken/loom/internal/orchestrator"
)

// The harness's own judgments, each shown able to fail.

func artifactState(t *testing.T, content string) (*orchestrator.RunState, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "analysis.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sum, err := orchestrator.ArtifactSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	state := orchestrator.NewRunState("p", orchestrator.CreatedByExecutor)
	state.Stages["analyst"] = orchestrator.StageRecord{Status: orchestrator.StageStatusCompleted, Sequence: 1,
		ArtifactPath: path, ArtifactSHA256: sum}
	return state, path
}

func TestDigestsMatchCatchesAnEditedArtifact(t *testing.T) {
	state, path := artifactState(t, "# analysis\n")
	if err := digestsMatch(state); err != nil {
		t.Fatalf("an untouched artifact failed: %v", err)
	}
	if err := tamperIsCaught(state); err != nil {
		t.Errorf("the control failed: %v", err)
	}
	if err := os.WriteFile(path, []byte("# edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := digestsMatch(state); err == nil || !strings.Contains(err.Error(), "stage analyst") {
		t.Errorf("err = %v, want the edited artifact named", err)
	}
}

func TestDigestChecksNeedSomethingToCheck(t *testing.T) {
	empty := orchestrator.NewRunState("p", orchestrator.CreatedByExecutor)
	if err := digestsMatch(empty); err == nil {
		t.Error("a run with no artifacts passed the digest check")
	}
	if err := tamperIsCaught(empty); err == nil {
		t.Error("the control passed with nothing to tamper with")
	}
	state, path := artifactState(t, "x")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := digestsMatch(state); err == nil {
		t.Error("a missing artifact passed")
	}
	if err := tamperIsCaught(state); err == nil {
		t.Error("the control passed with an unreadable artifact")
	}
}

func TestCheckDigestsReportsAMissingRunState(t *testing.T) {
	var report Report
	checkDigests(&report, &factory.OrderState{ID: "x", Claim: factory.Claim{Worktree: t.TempDir()}})
	if report.Passed() || len(report.Checks) != 1 || !strings.Contains(report.Checks[0].Detail, "no run state") {
		t.Errorf("report = %+v, want one failed check for the missing state", report)
	}
}

func TestEveryStageSettledFailsAnUnfinishedOrHaltedRun(t *testing.T) {
	state := orchestrator.NewRunState("p", orchestrator.CreatedByExecutor)
	if err := everyStageSettled(state); err == nil {
		t.Error("a run with no stages passed")
	}
	state.Stages["analyst"] = orchestrator.StageRecord{Status: orchestrator.StageStatusFailed, Sequence: 1}
	if err := everyStageSettled(state); err == nil || !strings.Contains(err.Error(), "FAILED") {
		t.Errorf("err = %v, want the failed stage named", err)
	}
	state.Stages["analyst"] = orchestrator.StageRecord{Status: orchestrator.StageStatusCompleted, Sequence: 1}
	state.BudgetHalt = &orchestrator.BudgetHalt{BeforeStage: "developer"}
	if err := everyStageSettled(state); err == nil {
		t.Error("a budget-halted run passed")
	}
}

func TestGatesApprovedByHandRefusesAnythingElse(t *testing.T) {
	state := orchestrator.NewRunState("p", orchestrator.CreatedByExecutor)
	if err := gatesApprovedByHand(state, []string{factory.GateCommit}); err == nil {
		t.Error("a run that never stopped at a gate passed")
	}
	state.Approvals["confirm-design"] = orchestrator.Approval{Method: orchestrator.ApprovalMethodTTY,
		ArtifactDigests: map[string]string{"analyst": "x"}}
	if err := gatesApprovedByHand(state, []string{"confirm-design"}); err == nil {
		t.Error("an approval made another way passed")
	}
	state.Approvals["confirm-design"] = orchestrator.Approval{Method: orchestrator.ApprovalMethodFlag,
		ArtifactDigests: map[string]string{"analyst": "x"}}
	if err := gatesApprovedByHand(state, []string{"confirm-design", factory.GateCommit}); err != nil {
		t.Errorf("a --approve approval failed: %v", err)
	}
}

func TestWithinBudget(t *testing.T) {
	state := orchestrator.NewRunState("p", orchestrator.CreatedByExecutor)
	state.Spend = &orchestrator.Usage{CostUSD: 6}
	if err := withinBudget(state, intake.Budget{MaxUSD: 5}); err == nil {
		t.Error("a run over budget passed")
	}
	if err := withinBudget(state, intake.Budget{MaxUSD: 10}); err != nil {
		t.Errorf("a run under budget failed: %v", err)
	}
}

func TestSettledJudgesTheOrdersPhase(t *testing.T) {
	waiting := []factory.InboxItem{{Gate: "confirm-design"}}
	for _, tc := range []struct {
		phase   factory.Phase
		items   []factory.InboxItem
		done    bool
		failing bool
	}{
		{factory.PhaseShipped, nil, true, false},
		{factory.PhaseParked, nil, false, true},
		{factory.PhaseStopped, waiting, false, true},
		{factory.PhaseRunning, nil, false, true},
		{factory.PhaseWaiting, waiting, false, false},
	} {
		done, err := settled(&factory.OrderState{Phase: tc.phase}, tc.items)
		if done != tc.done || (err != nil) != tc.failing {
			t.Errorf("settled(%s) = %v, %v", tc.phase, done, err)
		}
	}
}

func TestReportPassesOnlyWhenEveryCheckDoes(t *testing.T) {
	var report Report
	if report.Passed() {
		t.Error("an empty report passed")
	}
	report.record("a", nil)
	report.record("b", errors.New("broken"))
	if report.Passed() || len(report.Failed()) != 1 {
		t.Errorf("report = %+v", report)
	}
	var out bytes.Buffer
	Print(&out, report)
	if !strings.Contains(out.String(), "PASS  a") || !strings.Contains(out.String(), "FAIL  b — broken") {
		t.Errorf("printed:\n%s", out.String())
	}
}

func TestVerifyNeedsADeliveryFirst(t *testing.T) {
	if _, err := Verify(Config{Workdir: t.TempDir()}); !errors.Is(err, ErrNoDelivery) {
		t.Errorf("err = %v, want ErrNoDelivery", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, reportFile), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(Config{Workdir: dir}); err == nil {
		t.Error("a corrupt report was read")
	}
	if err := os.WriteFile(filepath.Join(dir, reportFile), []byte(`{"order":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(Config{Workdir: dir, Fixture: t.TempDir()}); err == nil {
		t.Error("a fixture with no order was verified")
	}
}

func TestDeliverRefusesABrokenFixture(t *testing.T) {
	if _, err := Deliver(Config{Fixture: t.TempDir(), Workdir: t.TempDir()}); err == nil {
		t.Error("a fixture with no order was delivered")
	}
	broken := t.TempDir()
	if err := os.WriteFile(filepath.Join(broken, "order.md"), []byte("no frontmatter"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Deliver(Config{Fixture: broken, Workdir: t.TempDir()}); err == nil {
		t.Error("an unparseable order was delivered")
	}
	if _, err := Deliver(Config{Fixture: fixtureWithoutRepo(t), Workdir: t.TempDir()}); err == nil {
		t.Error("a fixture with no repository was delivered")
	}
}

func fixtureWithoutRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	order, err := os.ReadFile(filepath.Join(fixture(t, ""), "order.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "order.md"), order, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A loom that cannot run fails the delivery check rather than the harness.
func TestABrokenLoomFailsTheDelivery(t *testing.T) {
	report, err := Deliver(Config{Loom: "/bin/false", Fixture: fixture(t, ""), Workdir: t.TempDir(), Provider: "mock"})
	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if report.Passed() || report.Checks[0].Name != "the factory shipped the order" {
		t.Errorf("report = %+v, want the delivery failed", report)
	}
}

// The fixture's own order must be valid, or every harness run fails at claim.
func TestTheFixtureOrderIsValid(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(fixture(t, ""), "order.md"))
	if err != nil {
		t.Fatal(err)
	}
	if violations := intake.Check(source, intake.Context{FileStem: "divide", Plans: []string{"deliver-feature"}}); len(violations) != 0 {
		t.Errorf("the fixture order is invalid: %v", violations)
	}
}
