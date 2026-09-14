package orchestrator_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
	"github.com/orieken/loom/internal/state"
)

// loopPlan is the L2.17 shape: developer → code-reviewer repeating until
// the review approves, bounded, with a gate on exhaustion.
func loopPlan(maxIterations int) orchestrator.Plan {
	return orchestrator.Plan{
		Name: "loop-plan",
		Stages: []orchestrator.Stage{
			{ID: "developer", Agent: "developer", Consumes: []string{"code-reviewer"}, Timeout: 5 * time.Second},
			{ID: "code-reviewer", Agent: "code-reviewer",
				StateKind: string(state.KindReview), Timeout: 5 * time.Second},
			{ID: "qa-engineer", Agent: "qa-engineer", Timeout: 5 * time.Second},
		},
		Loops: []orchestrator.Loop{{
			ID: "review", From: "developer", To: "code-reviewer",
			Condition: orchestrator.ReviewApprovedCondition,
			Gate:      orchestrator.GateConfirmUnresolvedReview, MaxIterations: maxIterations,
		}},
	}
}

func reviewPayload(t *testing.T, verdict state.Verdict) []byte {
	t.Helper()
	raw, err := json.Marshal(mock.SampleReview(verdict))
	if err != nil {
		t.Fatalf("marshal review: %v", err)
	}
	return raw
}

// reviewerScript scripts a reviewer that rejects every round. Tests that
// want an approval install a hook for the round it should arrive on.
func reviewerScript(t *testing.T) map[string]mock.Script {
	t.Helper()
	return map[string]mock.Script{
		"developer":     {ArtifactContent: "# implementation"},
		"code-reviewer": {Payload: reviewPayload(t, state.VerdictChangesRequested)},
		"qa-engineer":   {ArtifactContent: "# qa"},
	}
}

// TestApprovingReviewerRunsTheLoopOnce is the baseline: nothing loops when
// the first review approves.
func TestApprovingReviewerRunsTheLoopOnce(t *testing.T) {
	scripts := reviewerScript(t)
	scripts["code-reviewer"] = mock.Script{Payload: reviewPayload(t, state.VerdictApproved)}
	executor, provider, store, input := newHarness(t, scripts)

	if err := executor.Run(context.Background(), loopPlan(3), input); err != nil {
		t.Fatalf("Run: %v", err)
	}

	assertInvocations(t, provider, []string{"developer", "code-reviewer", "qa-engineer"})
	if got := mustLoad(t, store).Stages["code-reviewer"].Iteration; got != 1 {
		t.Errorf("iteration = %d, want 1", got)
	}
}

// TestChangesRequestedSendsTheDeveloperBack is the L2.17 done-when: the
// reviewer's verdict now changes what the executor does.
func TestChangesRequestedSendsTheDeveloperBack(t *testing.T) {
	executor, provider, store, input := newHarness(t, reviewerScript(t))
	provider.SetHook(func(stageID string, invocation int) *mock.Script {
		// Approve on the reviewer's second visit.
		if stageID == "code-reviewer" && invocation == 2 {
			return &mock.Script{Payload: reviewPayload(t, state.VerdictApproved)}
		}
		return nil
	})

	if err := executor.Run(context.Background(), loopPlan(3), input); err != nil {
		t.Fatalf("Run: %v", err)
	}

	assertInvocations(t, provider, []string{
		"developer", "code-reviewer", "developer", "code-reviewer", "qa-engineer",
	})
	if got := mustLoad(t, store).Stages["developer"].Iteration; got != 2 {
		t.Errorf("developer iteration = %d, want 2", got)
	}
}

// TestEveryIterationIsRetainedAndDigested is what makes "why did this take
// three rounds?" answerable, and what L4.5 will mine.
func TestEveryIterationIsRetainedAndDigested(t *testing.T) {
	executor, provider, store, input := newHarness(t, reviewerScript(t))
	provider.SetHook(func(stageID string, invocation int) *mock.Script {
		if stageID == "code-reviewer" && invocation == 3 {
			return &mock.Script{Payload: reviewPayload(t, state.VerdictApproved)}
		}
		return nil
	})

	if err := executor.Run(context.Background(), loopPlan(3), input); err != nil {
		t.Fatalf("Run: %v", err)
	}

	assertRetainedRounds(t, mustLoad(t, store).Stages["code-reviewer"], 2)
}

func assertRetainedRounds(t *testing.T, record orchestrator.StageRecord, want int) {
	t.Helper()
	if len(record.IterationArtifacts) != want {
		t.Fatalf("retained %d earlier rounds, want %d", len(record.IterationArtifacts), want)
	}
	for _, retained := range record.IterationArtifacts {
		if retained.SHA256 == "" {
			t.Errorf("round %d was retained without a digest", retained.Iteration)
		}
		if _, err := os.Stat(retained.Path); err != nil {
			t.Errorf("round %d's artifact is missing: %v", retained.Iteration, err)
		}
	}
}

// TestExhaustingTheBoundHaltsAtItsGate covers the decision that exhaustion
// is where a human looks, not a crash.
func TestExhaustingTheBoundHaltsAtItsGate(t *testing.T) {
	executor, provider, store, input := newHarness(t, reviewerScript(t))

	err := executor.Run(context.Background(), loopPlan(2), input)

	var waiting *orchestrator.WaitingApprovalError
	if !errors.As(err, &waiting) || waiting.Gate != orchestrator.GateConfirmUnresolvedReview {
		t.Fatalf("Run error = %v, want a halt at confirm-unresolved-review", err)
	}
	// Two rounds ran; qa-engineer never did.
	assertInvocations(t, provider, []string{"developer", "code-reviewer", "developer", "code-reviewer"})
	if got := mustLoad(t, store).Stages["code-reviewer"].Iteration; got != 2 {
		t.Errorf("iteration = %d, want the bound of 2", got)
	}
}

// TestApprovingTheExhaustedLoopProceeds is the other half: a human who
// approves is accepting the outstanding findings, and the run goes on
// without looping again.
func TestApprovingTheExhaustedLoopProceeds(t *testing.T) {
	executor, provider, store, input := newHarness(t, reviewerScript(t))
	if err := executor.Run(context.Background(), loopPlan(2), input); !errors.Is(err, orchestrator.ErrWaitingApproval) {
		t.Fatalf("first leg = %v, want a halt", err)
	}
	if err := executor.Approve(orchestrator.GateConfirmUnresolvedReview, orchestrator.ApprovalMethodTTY); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	if err := executor.Run(context.Background(), loopPlan(2), input); err != nil {
		t.Fatalf("run after approving the exhausted loop: %v", err)
	}

	assertInvocations(t, provider, []string{
		"developer", "code-reviewer", "developer", "code-reviewer", "qa-engineer",
	})
	if !mustLoad(t, store).IsGateApproved(orchestrator.GateConfirmUnresolvedReview) {
		t.Error("the approval did not stick")
	}
}

// TestGateBeforeTheLoopDoesNotRehaltEachRound verifies the property the
// epic said to check rather than build: an approval binds the artifacts
// complete when it was given, and the developer's own output was not among
// them.
func TestGateBeforeTheLoopDoesNotRehaltEachRound(t *testing.T) {
	plan := loopPlan(3)
	plan.Stages[0].Gate = "confirm-design"
	executor, provider, _, input := newHarness(t, reviewerScript(t))
	provider.SetHook(func(stageID string, invocation int) *mock.Script {
		if stageID == "code-reviewer" && invocation == 2 {
			return &mock.Script{Payload: reviewPayload(t, state.VerdictApproved)}
		}
		return nil
	})

	if err := executor.Run(context.Background(), plan, input); !errors.Is(err, orchestrator.ErrWaitingApproval) {
		t.Fatalf("first leg = %v, want a halt at confirm-design", err)
	}
	if err := executor.Approve("confirm-design", orchestrator.ApprovalMethodTTY); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	// One approval, two rounds through the gated stage, no second halt.
	if err := executor.Run(context.Background(), plan, input); err != nil {
		t.Fatalf("the loop re-halted at a gate already approved: %v", err)
	}
	assertInvocations(t, provider, []string{
		"developer", "code-reviewer", "developer", "code-reviewer", "qa-engineer",
	})
}

func TestLoopEventsAppearOnTheTimeline(t *testing.T) {
	executor, _, store, input := newHarness(t, reviewerScript(t))
	_ = executor.Run(context.Background(), loopPlan(2), input)

	found := firstOfEachKind(readTimeline(t, store))
	iterated, ok := found[orchestrator.EventLoopIterated]
	if !ok || iterated.Loop != "review" || iterated.Iteration != 2 {
		t.Errorf("loop.iterated event = %+v", iterated)
	}
	exhausted, ok := found[orchestrator.EventLoopExhausted]
	if !ok || exhausted.Loop != "review" {
		t.Errorf("loop.exhausted event = %+v", exhausted)
	}
}

// TestEditingALoopedArtifactDemotesTheStage checks the loop did not opt
// out of L2.12: a stage that iterated is still digest-verified on its
// current artifact.
func TestEditingALoopedArtifactDemotesTheStage(t *testing.T) {
	executor, provider, store, input := newHarness(t, reviewerScript(t))
	provider.SetHook(func(stageID string, invocation int) *mock.Script {
		if stageID == "code-reviewer" && invocation == 2 {
			return &mock.Script{Payload: reviewPayload(t, state.VerdictApproved)}
		}
		return nil
	})
	if err := executor.Run(context.Background(), loopPlan(3), input); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The stage's current artifact is what integrity tracks.
	current := mustLoad(t, store).Stages["code-reviewer"].ArtifactPath
	if err := os.WriteFile(current, []byte(`{"hand":"edited"}`), 0o644); err != nil {
		t.Fatalf("edit artifact: %v", err)
	}
	var reported []orchestrator.StaleStage
	executor.OnStale(func(stale []orchestrator.StaleStage) { reported = stale })
	_ = executor.Run(context.Background(), loopPlan(3), input)

	if len(reported) == 0 || reported[0].StageID != "code-reviewer" {
		t.Errorf("stale report = %+v, want the edited reviewer", reported)
	}
}

func TestLoopRefusesAnUnknownCondition(t *testing.T) {
	plan := loopPlan(3)
	plan.Loops[0].Condition = "vibes-are-good"
	executor, _, _, input := newHarness(t, reviewerScript(t))

	err := executor.Run(context.Background(), plan, input)

	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error = %v, want a refusal naming the unknown condition", err)
	}
}

func TestDefaultPlanDeclaresTheReviewLoop(t *testing.T) {
	plan := orchestrator.DefaultDeliverFeaturePlan()

	if len(plan.Loops) != 1 {
		t.Fatalf("default plan declares %d loops, want the review loop", len(plan.Loops))
	}
	loop := plan.Loops[0]
	if loop.From != "developer" || loop.To != "code-reviewer" {
		t.Errorf("loop spans %s..%s, want developer..code-reviewer", loop.From, loop.To)
	}
	if loop.MaxIterations < 1 {
		t.Error("the review loop has no bound; the prose's unbounded 'until APPROVED' is what this replaces")
	}
	if loop.Gate == "" {
		t.Error("the loop has no gate to halt at when it exhausts")
	}
}

func TestRetainedIterationsLiveBesideTheWorkspace(t *testing.T) {
	executor, _, _, input := newHarness(t, reviewerScript(t))
	_ = executor.Run(context.Background(), loopPlan(2), input)

	entries, err := os.ReadDir(filepath.Join(input.WorkspaceDir, orchestrator.IterationsDirName))
	if err != nil {
		t.Fatalf("read iterations directory: %v", err)
	}
	if len(entries) == 0 {
		t.Error("no iteration artifacts were retained")
	}
}

// loopOutcomeFor drives a run under a recording tracer and returns how the
// named loop was reported to have settled on the run span.
func loopOutcomeFor(t *testing.T, scripts map[string]mock.Script, loopID string) string {
	t.Helper()
	executor, _, _, input := newHarness(t, scripts)
	tracer := &recordingTracer{}
	executor.WithTracer(tracer)
	_ = executor.Run(context.Background(), loopPlan(2), input)
	for _, outcome := range tracer.ended {
		if reason, found := outcome.LoopOutcomes[loopID]; found {
			return reason
		}
	}
	return ""
}

// A run that converged and a run that hit its bound produce the same stages
// with the same statuses. Which of the two happened is the question anyone
// reading the trace is actually asking, and before roadmap L3.43 nothing
// answered it.
func TestRunSpanSaysHowTheReviewLoopSettled(t *testing.T) {
	approved := reviewerScript(t)
	approved["code-reviewer"] = mock.Script{Payload: reviewPayload(t, state.VerdictApproved)}

	if got := loopOutcomeFor(t, approved, "review"); got != orchestrator.LoopConverged {
		t.Errorf("approved review settled as %q, want %q", got, orchestrator.LoopConverged)
	}
	// reviewerScript keeps requesting changes, so the bound is what stops it.
	if got := loopOutcomeFor(t, reviewerScript(t), "review"); got != orchestrator.LoopRoundLimit {
		t.Errorf("exhausted review settled as %q, want %q", got, orchestrator.LoopRoundLimit)
	}
}
