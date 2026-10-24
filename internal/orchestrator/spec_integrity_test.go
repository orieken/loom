package orchestrator_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
)

// Review item I-1: the executor recorded the spec's path and never its
// content, so acceptance criteria could change mid-run while every approval
// derived from the old ones stood. Every stage derives from the spec, so the
// spec is verified like an artifact upstream of all of them.

func writeSpecFile(t *testing.T, input orchestrator.StageInput, content string) {
	t.Helper()
	if err := os.WriteFile(input.SpecPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
}

// specHarness is newHarness with a real spec on disk.
func specHarness(t *testing.T, scripts map[string]mock.Script) (*orchestrator.Executor, *mock.Provider, *orchestrator.StateStore, orchestrator.StageInput) {
	t.Helper()
	executor, provider, store, input := newHarness(t, scripts)
	writeSpecFile(t, input, "# Feature\n- [ ] Given a user, when they log in, then they see the dashboard\n")
	return executor, provider, store, input
}

func completedSpecRun(t *testing.T) (*orchestrator.Executor, *mock.Provider, *orchestrator.StateStore, orchestrator.StageInput) {
	t.Helper()
	executor, provider, store, input := specHarness(t, completedScripts())
	if err := executor.Run(context.Background(), threeStagePlan(), input); err != nil {
		t.Fatalf("first run: %v", err)
	}
	return executor, provider, store, input
}

func specDigestOf(t *testing.T, input orchestrator.StageInput) string {
	t.Helper()
	digest, err := orchestrator.ArtifactSHA256(input.SpecPath)
	if err != nil {
		t.Fatalf("hash spec: %v", err)
	}
	return digest
}

func TestFreshRunRecordsTheSpecDigest(t *testing.T) {
	_, _, store, input := completedSpecRun(t)

	if got, want := mustLoad(t, store).SpecSHA256, specDigestOf(t, input); got != want {
		t.Errorf("recorded spec digest = %q, want %q", got, want)
	}
}

// TestChangedSpecMakesEveryCompletedStageStale is I-1's done-when: an edited
// spec demotes every completed stage, each re-runs against the new spec, and
// the run is re-bound to the spec as it now stands.
func TestChangedSpecMakesEveryCompletedStageStale(t *testing.T) {
	executor, provider, store, input := completedSpecRun(t)
	writeSpecFile(t, input, "# Feature\n- [ ] Given a user, when they log in twice, then they are locked out\n")

	stale := resume(t, executor, input)

	if len(stale) != 3 {
		t.Fatalf("stale = %+v, want all three completed stages", stale)
	}
	for _, item := range stale {
		if item.Reason != orchestrator.StaleReasonSpecChanged {
			t.Errorf("stage %q demoted for %q, want %q", item.StageID, item.Reason, orchestrator.StaleReasonSpecChanged)
		}
	}
	assertInvocations(t, provider, []string{"analyst", "developer", "qa-engineer", "analyst", "developer", "qa-engineer"})
	if got, want := mustLoad(t, store).SpecSHA256, specDigestOf(t, input); got != want {
		t.Errorf("after re-running, spec digest = %q, want the new spec's %q", got, want)
	}
}

func TestUnchangedSpecDemotesNothing(t *testing.T) {
	executor, provider, _, input := completedSpecRun(t)

	if stale := resume(t, executor, input); len(stale) != 0 {
		t.Errorf("stale = %+v, want nothing for an untouched spec", stale)
	}
	assertInvocations(t, provider, []string{"analyst", "developer", "qa-engineer"})
}

// A deleted spec is not the spec the run was started from.
func TestDeletedSpecCountsAsChanged(t *testing.T) {
	executor, _, _, input := completedSpecRun(t)
	if err := os.Remove(input.SpecPath); err != nil {
		t.Fatalf("remove spec: %v", err)
	}
	var reported []orchestrator.StaleStage
	executor.OnStale(func(stale []orchestrator.StaleStage) { reported = stale })
	_ = executor.Run(context.Background(), threeStagePlan(), input)

	if len(reported) == 0 || reported[0].Reason != orchestrator.StaleReasonSpecChanged {
		t.Errorf("stale = %+v, want every stage demoted for the missing spec", reported)
	}
}

// The approval a human gave was a decision about outputs derived from the old
// spec; it must not carry over to the new one.
func TestChangedSpecResetsTheApproval(t *testing.T) {
	executor, _, store, input := specHarness(t, completedScripts())
	_ = runUntilGate(t, executor, gatedPlan(), input)
	if err := executor.Approve("confirm-design", orchestrator.ApprovalMethodTTY); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	writeSpecFile(t, input, "# Feature, rewritten after the human approved the design\n")
	executor.OnStale(func([]orchestrator.StaleStage) {})

	err := executor.Run(context.Background(), gatedPlan(), input)

	var waiting *orchestrator.WaitingApprovalError
	if !errors.As(err, &waiting) || waiting.Gate != "confirm-design" {
		t.Fatalf("Run error = %v, want a halt at the reset gate", err)
	}
	if mustLoad(t, store).IsGateApproved("confirm-design") {
		t.Error("the approval survived a change to the spec it was derived from")
	}
}

// The CLI asks before recording an approval whether the same command's
// verification would destroy it; a changed spec must answer yes.
func TestWouldInvalidateApprovalsSeesASpecChange(t *testing.T) {
	executor, _, store, input := specHarness(t, completedScripts())
	_ = runUntilGate(t, executor, gatedPlan(), input)
	if err := executor.Approve("confirm-design", orchestrator.ApprovalMethodTTY); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	writeSpecFile(t, input, "# Feature, rewritten\n")

	stale, err := executor.WouldInvalidateApprovals()
	if err != nil || stale == nil || stale.Gate != "confirm-design" {
		t.Fatalf("WouldInvalidateApprovals = %+v, %v; want confirm-design", stale, err)
	}
	if !mustLoad(t, store).IsGateApproved("confirm-design") {
		t.Error("the dry-run check mutated the real state")
	}
}

// TestSpecEditedMidRunStopsBeforeTheNextStage: a running process checks the
// spec before starting each stage, so an edit made while one stage runs
// stops the run before anything is started against content nobody reviewed.
func TestSpecEditedMidRunStopsBeforeTheNextStage(t *testing.T) {
	executor, provider, _, input := specHarness(t, completedScripts())
	provider.SetHook(func(stageID string, _ int) *mock.Script {
		if stageID == "developer" {
			writeSpecFile(t, input, "# Feature, edited while the developer stage was running\n")
		}
		return nil
	})

	err := executor.Run(context.Background(), threeStagePlan(), input)

	var changed *orchestrator.SpecChangedError
	if !errors.As(err, &changed) || changed.Path != input.SpecPath {
		t.Fatalf("Run error = %v, want a SpecChangedError naming the spec", err)
	}
	assertInvocations(t, provider, []string{"analyst", "developer"})
	if !strings.Contains(err.Error(), "resume") {
		t.Errorf("error %q does not say how to continue", err)
	}
}

// State written before specs were digested adopts the spec as it is now:
// there is nothing older to compare against, and later edits are caught.
func TestStateWithoutASpecDigestAdoptsOneOnResume(t *testing.T) {
	executor, _, store, input := completedRun(t) // no spec on disk: no digest recorded
	if mustLoad(t, store).SpecSHA256 != "" {
		t.Fatal("a run with no readable spec recorded a digest")
	}
	writeSpecFile(t, input, "# Feature\n")

	if stale := resume(t, executor, input); len(stale) != 0 {
		t.Fatalf("stale = %+v, want nothing: an adopted digest has no history to contradict", stale)
	}
	if mustLoad(t, store).SpecSHA256 != specDigestOf(t, input) {
		t.Error("resume did not adopt the spec's digest")
	}
	writeSpecFile(t, input, "# Feature, now edited\n")
	if stale := resume(t, executor, input); len(stale) != 3 {
		t.Errorf("stale = %+v, want every stage once an adopted spec changes", stale)
	}
}

func TestSpecChangedDescriptionNamesTheSpec(t *testing.T) {
	item := orchestrator.StaleStage{StageID: "analyst", Reason: orchestrator.StaleReasonSpecChanged}
	if !strings.Contains(item.Description(), "spec") {
		t.Errorf("description %q does not mention the spec", item.Description())
	}
}

// Only completed stages derive from the spec's content. A stage halted at its
// gate has produced nothing yet, so a spec change has nothing of it to demote.
func TestSpecChangeDemotesOnlyCompletedStages(t *testing.T) {
	executor, _, _, input := specHarness(t, completedScripts())
	_ = runUntilGate(t, executor, gatedPlan(), input) // analyst completed; developer waits at the gate
	writeSpecFile(t, input, "# Feature, rewritten while the run waited at its gate\n")
	var reported []orchestrator.StaleStage
	executor.OnStale(func(stale []orchestrator.StaleStage) { reported = stale })

	_ = executor.Run(context.Background(), gatedPlan(), input)

	if len(reported) != 1 || reported[0].StageID != "analyst" {
		t.Errorf("stale = %+v, want only the completed analyst", reported)
	}
}

// The markdown pipeline's `loom state verify` calls VerifyCompletedStages on
// state it never adopts a digest for. State with no digest has no record of
// which spec its stages came from, so verification must not demote them.
func TestVerifyLeavesStateWithoutASpecDigestAlone(t *testing.T) {
	_, _, store, input := completedRun(t) // no spec on disk: no digest recorded
	writeSpecFile(t, input, "# Feature\n")
	state := mustLoad(t, store)

	if stale := orchestrator.VerifyCompletedStages(state); len(stale) != 0 {
		t.Errorf("stale = %+v, want nothing: no digest means nothing to compare", stale)
	}
}

// --- failure paths: a demotion that cannot be persisted must stop the run ---

// makeStateDirUnwritable makes the next Save fail. Root ignores directory
// permissions, so the test cannot run as root — CI runs as an ordinary user.
func makeStateDirUnwritable(t *testing.T, store *orchestrator.StateStore) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions; needs an ordinary user")
	}
	dir := filepath.Dir(store.Path())
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod state dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func TestAnAdoptedSpecDigestThatCannotBeSavedStopsTheRun(t *testing.T) {
	executor, _, store, input := completedRun(t) // no spec on disk: no digest recorded
	writeSpecFile(t, input, "# Feature\n")
	makeStateDirUnwritable(t, store)

	err := executor.Run(context.Background(), threeStagePlan(), input)
	if err == nil || !strings.Contains(err.Error(), "persist adopted spec digest") {
		t.Errorf("Run error = %v, want the adoption's save failure", err)
	}
}

func TestASpecDemotionThatCannotBeSavedStopsTheRun(t *testing.T) {
	executor, provider, store, input := completedSpecRun(t)
	writeSpecFile(t, input, "# Feature, rewritten\n")
	makeStateDirUnwritable(t, store)

	err := executor.Run(context.Background(), threeStagePlan(), input)
	if err == nil || !strings.Contains(err.Error(), "persist verification result") {
		t.Errorf("Run error = %v, want the demotion's save failure", err)
	}
	assertInvocations(t, provider, []string{"analyst", "developer", "qa-engineer"})
}

// The timeline is how a demotion is reported; one that cannot be recorded
// stops the run rather than re-running work nobody was told about. A
// directory where the timeline file belongs fails the append for any user.
func TestASpecDemotionThatCannotBeRecordedStopsTheRun(t *testing.T) {
	executor, provider, store, input := completedSpecRun(t)
	writeSpecFile(t, input, "# Feature, rewritten\n")
	timeline := filepath.Join(filepath.Dir(store.Path()), orchestrator.RunEventsFileName)
	if err := os.Remove(timeline); err != nil {
		t.Fatalf("remove timeline: %v", err)
	}
	if err := os.Mkdir(timeline, 0o755); err != nil {
		t.Fatalf("block timeline: %v", err)
	}

	if err := executor.Run(context.Background(), threeStagePlan(), input); err == nil {
		t.Error("Run succeeded although the demotions could not be recorded")
	}
	assertInvocations(t, provider, []string{"analyst", "developer", "qa-engineer"})
}

func TestSpecDigestOfNoPathIsEmpty(t *testing.T) {
	if digest := orchestrator.SpecDigest(""); digest != "" {
		t.Errorf("SpecDigest(\"\") = %q, want empty: no spec, nothing to verify", digest)
	}
}
