package orchestrator_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/policy"
	"github.com/orieken/loom/internal/provider/mock"
	"github.com/orieken/loom/internal/state"
)

// approvedRunWithEditedArtifact drives a run to its gate, approves it, then
// edits the artifact the approval bound — the exact sequence that should
// invalidate an approval (L2.14).
func approvedRunWithEditedArtifact(t *testing.T, edit bool) (*orchestrator.Executor, *orchestrator.StateStore) {
	t.Helper()
	executor, _, store, input := newHarness(t, map[string]mock.Script{
		"analyst":     {ArtifactContent: "# analysis"},
		"developer":   {ArtifactContent: "# implementation"},
		"qa-engineer": {ArtifactContent: "# qa report"},
	})
	waiting := runUntilGate(t, executor, gatedPlan(), input)
	if err := executor.Approve(waiting.Gate, orchestrator.ApprovalMethodFlag); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if edit {
		path := filepath.Join(input.WorkspaceDir, "analyst.md")
		if err := os.WriteFile(path, []byte("# analysis, edited after approval\n"), 0o644); err != nil {
			t.Fatalf("edit approved artifact: %v", err)
		}
	}
	return executor, store
}

// The CLI asks this before recording an approval, so it never writes one the
// same command is about to destroy. Untested until roadmap L3.45 — and it
// guards the property L2.14 exists for.
func TestWouldInvalidateApprovalsNamesTheGateAndTheStage(t *testing.T) {
	executor, _ := approvedRunWithEditedArtifact(t, true)

	stale, err := executor.WouldInvalidateApprovals()
	if err != nil {
		t.Fatalf("WouldInvalidateApprovals: %v", err)
	}
	if stale == nil {
		t.Fatal("editing an approved artifact invalidated nothing")
	}
	if stale.Gate != "confirm-design" || stale.ChangedStage != "analyst" {
		t.Errorf("stale = %+v, want gate confirm-design changed by analyst", stale)
	}
	// The message is what a human reads when a run stops; it has to name both.
	if message := stale.Error(); !strings.Contains(message, "confirm-design") ||
		!strings.Contains(message, "analyst") {
		t.Errorf("Error() = %q, want it to name the gate and the stage", message)
	}
}

func TestWouldInvalidateApprovalsIsSilentWhenNothingChanged(t *testing.T) {
	executor, _ := approvedRunWithEditedArtifact(t, false)

	stale, err := executor.WouldInvalidateApprovals()
	if err != nil {
		t.Fatalf("WouldInvalidateApprovals: %v", err)
	}
	if stale != nil {
		t.Errorf("WouldInvalidateApprovals = %+v, want nil when no artifact changed", stale)
	}
}

// The check is a dry run: asking whether an approval WOULD be invalidated
// must not itself demote a stage. A regression here turns a question into an
// action, which is the worst failure available to a read-only query the CLI
// runs before deciding what to do.
//
// What this catches, stated precisely because mutation testing showed the
// obvious claim was too broad: it fails if the check ever PERSISTS what it
// inspected. It does not fail if cloneForInspection is removed, because
// WouldInvalidateApprovals loads fresh state and never saves it, so an
// in-memory demotion is discarded and unobservable. The clone is defence
// against a future caller that does persist; this test guards the persisting
// failure itself.
func TestWouldInvalidateApprovalsDoesNotMutateRunState(t *testing.T) {
	executor, store := approvedRunWithEditedArtifact(t, true)

	before := mustLoad(t, store)
	beforeStatus := before.Stages["analyst"].Status
	beforeApprovals := len(before.Approvals)

	if _, err := executor.WouldInvalidateApprovals(); err != nil {
		t.Fatalf("WouldInvalidateApprovals: %v", err)
	}

	after := mustLoad(t, store)
	if after.Stages["analyst"].Status != beforeStatus {
		t.Errorf("analyst status changed from %q to %q by a read-only check",
			beforeStatus, after.Stages["analyst"].Status)
	}
	if len(after.Approvals) != beforeApprovals {
		t.Errorf("approvals went from %d to %d by a read-only check",
			beforeApprovals, len(after.Approvals))
	}
}

// A dry run must see exactly what a live gate saw, or a policy that would
// have fired in production silently does not when it is replayed.
func TestPolicyContextForReadsTheSameFactsAsALiveGate(t *testing.T) {
	scripts := reviewerScript(t)
	scripts["code-reviewer"] = mock.Script{Payload: reviewPayload(t, state.VerdictApproved)}
	executor, _, store, input := newHarness(t, scripts)
	if err := executor.Run(context.Background(), loopPlan(3), input); err != nil {
		t.Fatalf("Run: %v", err)
	}

	runState := mustLoad(t, store)
	context := orchestrator.PolicyContextFor(store, runState, policy.GateID("confirm-design"))

	if context.Gate != policy.GateID("confirm-design") {
		t.Errorf("gate = %q, want confirm-design", context.Gate)
	}
	if context.ReviewVerdict == nil {
		t.Fatal("review verdict is absent, but a completed review stage recorded one")
	}
	if *context.ReviewVerdict != string(state.VerdictApproved) {
		t.Errorf("review verdict = %q, want %q", *context.ReviewVerdict, state.VerdictApproved)
	}
}

// The context reports only what state actually holds. A stage still waiting
// at its gate has settled nothing, so its verdict must be absent rather than
// guessed — an absent fact resolves a policy check to UNKNOWN, and a guessed
// one resolves it to a decision nobody made.
func TestPolicyContextOmitsFactsFromAnUnsettledStage(t *testing.T) {
	executor, _, store, input := newHarness(t, reviewerScript(t))
	// The reviewer rejects every round, so the bound halts the run at
	// confirm-unresolved-review with code-reviewer still waiting.
	_ = executor.Run(context.Background(), loopPlan(1), input)

	runState := mustLoad(t, store)
	context := orchestrator.PolicyContextFor(store, runState, policy.GateID("confirm-unresolved-review"))

	if context.ReviewVerdict != nil {
		t.Errorf("review verdict = %q from a stage that has not settled", *context.ReviewVerdict)
	}
}

// BuiltInLoops is what lets a plan file name a loop instead of restating its
// bound — so the bound cannot be edited away by writing a plan (L2.17).
func TestBuiltInLoopsCarriesTheReviewBound(t *testing.T) {
	loops := orchestrator.BuiltInLoops()

	review, declared := loops["review"]
	if !declared {
		t.Fatalf("BuiltInLoops has no review loop: %v", loops)
	}
	if review.MaxIterations < 1 {
		t.Errorf("review loop bound = %d, want a real bound", review.MaxIterations)
	}
	if review.From != "developer" || review.To != "code-reviewer" {
		t.Errorf("review loop spans %s..%s, want developer..code-reviewer", review.From, review.To)
	}
}

// The route and loop callbacks are how the CLI tells a human what is about
// to happen before a gate asks them to approve it. Registered and never
// fired, they would leave the human approving a plan they were not shown.
func TestRouteAndLoopCallbacksFireDuringARun(t *testing.T) {
	executor, _, _, input := newHarness(t, reviewerScript(t))
	var routes []orchestrator.RouteSummary
	var rounds []orchestrator.LoopRound
	executor.OnRoute(func(summary orchestrator.RouteSummary) { routes = append(routes, summary) })
	executor.OnLoopRound(func(round orchestrator.LoopRound) { rounds = append(rounds, round) })

	// The reviewer rejects every round, so the loop runs its bound and the
	// round callback must fire for each re-entry.
	_ = executor.Run(context.Background(), loopPlan(2), input)

	if len(rounds) == 0 {
		t.Fatal("the loop re-entered but OnLoopRound never fired")
	}
	for index, round := range rounds {
		if round.Loop != "review" || round.From != "developer" {
			t.Errorf("round %d = %+v, want the review loop re-entering at developer", index, round)
		}
		if round.Max != 2 {
			t.Errorf("round %d reports Max %d, want the plan's bound of 2", index, round.Max)
		}
	}
	// loopPlan declares no router stage, so no route is computed — asserting
	// the callback stayed silent is what makes the assertion above mean
	// "fired because the loop ran" rather than "fired because runs fire it".
	if len(routes) != 0 {
		t.Errorf("OnRoute fired %d times for a plan with no router stage", len(routes))
	}
}

// The router's decision must reach the human before the design gate. This is
// the same callback on a plan that does route.
func TestRouteCallbackReportsWhatTheRouterDecided(t *testing.T) {
	executor, _, _, input := newHarness(t, routedScripts(t, analysisNeedingNothing()))
	var summaries []orchestrator.RouteSummary
	executor.OnRoute(func(summary orchestrator.RouteSummary) { summaries = append(summaries, summary) })

	_ = executor.Run(context.Background(), routedPlan(), input)

	if len(summaries) != 1 {
		t.Fatalf("OnRoute fired %d times, want exactly once per run", len(summaries))
	}
	summary := summaries[0]
	if summary.Total == 0 {
		t.Fatal("the route summary counts no stages, so a human is shown nothing")
	}
	if summary.Included >= summary.Total {
		t.Errorf("summary = %+v, want the analysis needing nothing to skip at least one stage", summary)
	}
	// analysisNeedingNothing declares no migration and no infrastructure
	// work, so both skippable stages must appear by name — a count alone
	// would not tell a human what is about to be left out.
	if len(summary.Skipped) == 0 {
		t.Errorf("summary = %+v, want the skipped stages named", summary)
	}
}

// A gate halt's message is what a human reads when a run stops. It has to
// name the stage and the gate, and wrap the sentinel so `errors.Is` works
// for every caller that checks for a halt rather than a failure.
func TestWaitingApprovalErrorNamesTheStageAndGate(t *testing.T) {
	executor, _, _, input := newHarness(t, map[string]mock.Script{
		"analyst":     {ArtifactContent: "# analysis"},
		"developer":   {ArtifactContent: "# implementation"},
		"qa-engineer": {ArtifactContent: "# qa report"},
	})
	waiting := runUntilGate(t, executor, gatedPlan(), input)

	message := waiting.Error()
	for _, want := range []string{waiting.Stage, waiting.Gate} {
		if !strings.Contains(message, want) {
			t.Errorf("Error() = %q, want it to name %q", message, want)
		}
	}
}
