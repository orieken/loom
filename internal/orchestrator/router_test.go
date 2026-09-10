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

// routedPlan is the shape L3.0 introduces: analyst, the internal router,
// one skippable stage, one that is not, and a gate behind the skippable one.
func routedPlan() orchestrator.Plan {
	return orchestrator.Plan{
		Name: "routed-plan",
		Stages: []orchestrator.Stage{
			{ID: "analyst", Agent: "analyst", StateKind: string(state.KindAnalysis), Timeout: 5 * time.Second},
			{ID: orchestrator.RouterStageID, Agent: orchestrator.RouterStageID,
				StateKind: string(state.KindRoute), Internal: true, Timeout: 5 * time.Second},
			{ID: "data-engineer", Agent: "data-engineer", Skippable: true, Timeout: 5 * time.Second},
			{ID: "developer", Agent: "developer", Timeout: 5 * time.Second},
			{ID: "devops-engineer", Agent: "devops-engineer", Skippable: true,
				Gate: "confirm-ship", Timeout: 5 * time.Second},
		},
	}
}

func routedScripts(t *testing.T, analysis state.AnalysisState) map[string]mock.Script {
	t.Helper()
	payload, err := json.Marshal(analysis)
	if err != nil {
		t.Fatalf("marshal analysis: %v", err)
	}
	return map[string]mock.Script{
		"analyst":         {Payload: payload},
		"data-engineer":   {ArtifactContent: "# data"},
		"developer":       {ArtifactContent: "# implementation"},
		"devops-engineer": {ArtifactContent: "# devops"},
	}
}

// analysisNeedingNothing declares no migration and no devops work, so both
// skippable stages should be routed out.
func analysisNeedingNothing() state.AnalysisState {
	return state.AnalysisState{
		SchemaVersion:      state.SchemaVersion,
		Feature:            "user-auth",
		Summary:            "A feature that needs neither a migration nor infrastructure work.",
		AcceptanceCriteria: []state.AcceptanceCriterion{{Statement: "it works"}},
		BoundedContext:     state.BoundedContext{Owning: "identity"},
		AffectedComponents: []state.AffectedComponent{{Path: "internal/identity/session.go", Reason: "the change"}},
		Tasks:              state.TaskList{Developer: []string{"write it"}},
		DefinitionOfDone:   []string{"tests green"},
	}
}

// TestRouterSkipsStagesTheAnalysisDoesNotCallFor is the L3.0 done-when: a
// feature with no infra work skips devops by a recorded decision, visible
// before the developer stage runs.
func TestRouterSkipsStagesTheAnalysisDoesNotCallFor(t *testing.T) {
	executor, provider, store, input := newHarness(t, routedScripts(t, analysisNeedingNothing()))

	err := executor.Run(context.Background(), routedPlan(), input)

	// The run halts at confirm-ship: routing a gated stage out must not
	// delete the human checkpoint that guarded it.
	if !errors.Is(err, orchestrator.ErrWaitingApproval) {
		t.Fatalf("Run error = %v, want a halt at confirm-ship", err)
	}
	assertSkippedWithReason(t, mustLoad(t, store), "data-engineer")
	assertRoutedOut(t, mustLoad(t, store), "devops-engineer")
	// data-engineer was never invoked; the router is internal, so it never
	// reaches the provider either.
	assertInvocations(t, provider, []string{"analyst", "developer"})
}

// assertRoutedOut allows for a routed-out stage that the run is currently
// halted in front of: its gate still stops the run, so the record reads
// WAITING_APPROVAL with the skip preserved underneath.
func assertRoutedOut(t *testing.T, runState *orchestrator.RunState, stageID string) {
	t.Helper()
	record := runState.Stages[stageID]
	waitingAtItsGate := record.Status == orchestrator.StageStatusWaitingApproval &&
		record.PreviousStatus == orchestrator.StageStatusSkipped
	if record.Status != orchestrator.StageStatusSkipped && !waitingAtItsGate {
		t.Errorf("stage %q status = %q, want SKIPPED (or waiting at its gate with the skip preserved)", stageID, record.Status)
	}
	if record.SkipReason == "" {
		t.Errorf("stage %q was routed out with no recorded reason", stageID)
	}
}

func assertSkippedWithReason(t *testing.T, runState *orchestrator.RunState, stageID string) {
	t.Helper()
	record := runState.Stages[stageID]
	if record.Status != orchestrator.StageStatusSkipped {
		t.Errorf("stage %q status = %q, want SKIPPED", stageID, record.Status)
	}
	if record.SkipReason == "" {
		t.Errorf("stage %q was skipped with no recorded reason", stageID)
	}
}

// TestSkipsAreRecordedBeforeTheDeveloperRuns is the "visible early"
// requirement: the shape of the run is known once the router finishes, not
// discovered one stage at a time.
func TestSkipsAreRecordedBeforeTheDeveloperRuns(t *testing.T) {
	scripts := routedScripts(t, analysisNeedingNothing())
	scripts["developer"] = mock.Script{Err: errors.New("stop here")}
	executor, _, store, input := newHarness(t, scripts)

	_ = executor.Run(context.Background(), routedPlan(), input)

	runState := mustLoad(t, store)
	assertSkippedWithReason(t, runState, "data-engineer")
	assertSkippedWithReason(t, runState, "devops-engineer")
	if runState.Stages["developer"].Status != orchestrator.StageStatusFailed {
		t.Fatalf("developer status = %q; the skips must be recorded before it runs", runState.Stages["developer"].Status)
	}
}

func TestRouterIncludesStagesTheAnalysisCallsFor(t *testing.T) {
	analysis := analysisNeedingNothing()
	analysis.DataModelChanges = []state.DataModelChange{{Description: "add sessions", Phase: state.MigrationPhaseExpand}}
	analysis.Tasks.DevOps = []string{"add a deploy step"}
	executor, provider, store, input := newHarness(t, routedScripts(t, analysis))

	if err := executor.Run(context.Background(), routedPlan(), input); !errors.Is(err, orchestrator.ErrWaitingApproval) {
		t.Fatalf("Run error = %v, want a halt at confirm-ship", err)
	}

	if mustLoad(t, store).Stages["data-engineer"].Status != orchestrator.StageStatusCompleted {
		t.Error("data-engineer was skipped despite a declared migration")
	}
	assertInvocations(t, provider, []string{"analyst", "data-engineer", "developer"})
}

// TestApprovingAGateDoesNotResurrectASkippedStage covers the interaction
// that a gate halt must not undo the route: the checkpoint is about
// reaching that point, not about running the stage behind it.
func TestApprovingAGateDoesNotResurrectASkippedStage(t *testing.T) {
	executor, provider, store, input := newHarness(t, routedScripts(t, analysisNeedingNothing()))
	if err := executor.Run(context.Background(), routedPlan(), input); !errors.Is(err, orchestrator.ErrWaitingApproval) {
		t.Fatalf("first leg = %v, want a halt", err)
	}
	if err := executor.Approve("confirm-ship", orchestrator.ApprovalMethodTTY); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	if err := executor.Run(context.Background(), routedPlan(), input); err != nil {
		t.Fatalf("run after approval: %v", err)
	}

	assertSkippedWithReason(t, mustLoad(t, store), "devops-engineer")
	assertInvocations(t, provider, []string{"analyst", "developer"})
}

// TestEditingTheRouteResetsTheGate verifies the property this design was
// chosen for — it must fall out of L2.14's binding, not be special-cased.
func TestEditingTheRouteResetsTheGate(t *testing.T) {
	executor, _, store, input := newHarness(t, routedScripts(t, analysisNeedingNothing()))
	if err := executor.Run(context.Background(), routedPlan(), input); !errors.Is(err, orchestrator.ErrWaitingApproval) {
		t.Fatalf("first leg = %v, want a halt", err)
	}
	if err := executor.Approve("confirm-ship", orchestrator.ApprovalMethodTTY); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	routePath := filepath.Join(input.WorkspaceDir, state.TypedStateDir, orchestrator.RouterStageID+".json")
	if err := os.WriteFile(routePath, []byte(`{"hand":"edited"}`), 0o644); err != nil {
		t.Fatalf("edit route: %v", err)
	}
	err := executor.Run(context.Background(), routedPlan(), input)

	if !errors.Is(err, orchestrator.ErrWaitingApproval) {
		t.Fatalf("Run error = %v, want a halt: editing the route must reset the gate", err)
	}
	if mustLoad(t, store).IsGateApproved("confirm-ship") {
		t.Error("editing the route left the approval standing")
	}
}

func TestRouteIsNotRecomputedOnAPlainResume(t *testing.T) {
	executor, provider, store, input := newHarness(t, routedScripts(t, analysisNeedingNothing()))
	if err := executor.Run(context.Background(), routedPlan(), input); !errors.Is(err, orchestrator.ErrWaitingApproval) {
		t.Fatalf("first leg = %v, want a halt", err)
	}
	firstDigest := mustLoad(t, store).Stages[orchestrator.RouterStageID].ArtifactSHA256

	if err := executor.Approve("confirm-ship", orchestrator.ApprovalMethodTTY); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := executor.Run(context.Background(), routedPlan(), input); err != nil {
		t.Fatalf("resume: %v", err)
	}

	if got := mustLoad(t, store).Stages[orchestrator.RouterStageID].ArtifactSHA256; got != firstDigest {
		t.Error("the route was recomputed on resume; it is an artifact like any other")
	}
	assertInvocations(t, provider, []string{"analyst", "developer"})
}

// TestAnEditedAnalysisReroutesTheRun asserts the whole chain: the edit
// demotes the analyst, the cascade demotes the router, and re-running both
// produces a route that now includes what the new analysis asks for.
func TestAnEditedAnalysisReroutesTheRun(t *testing.T) {
	executor, provider, store, input := newHarness(t, routedScripts(t, analysisNeedingNothing()))
	if err := executor.Run(context.Background(), routedPlan(), input); !errors.Is(err, orchestrator.ErrWaitingApproval) {
		t.Fatalf("first leg = %v, want a halt", err)
	}
	assertSkippedWithReason(t, mustLoad(t, store), "data-engineer")

	rerouted := analysisNeedingNothing()
	rerouted.DataModelChanges = []state.DataModelChange{{Description: "add sessions", Phase: state.MigrationPhaseExpand}}
	payload, err := json.Marshal(rerouted)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	analystStatePath := filepath.Join(input.WorkspaceDir, state.TypedStateDir, "analyst.json")
	if err := os.WriteFile(analystStatePath, payload, 0o644); err != nil {
		t.Fatalf("edit analysis: %v", err)
	}
	provider.SetScript("analyst", mock.Script{Payload: payload})
	executor.OnStale(func([]orchestrator.StaleStage) {})

	if err := executor.Run(context.Background(), routedPlan(), input); !errors.Is(err, orchestrator.ErrWaitingApproval) {
		t.Fatalf("rerouted run = %v, want a halt at confirm-ship", err)
	}

	if mustLoad(t, store).Stages["data-engineer"].Status != orchestrator.StageStatusCompleted {
		t.Error("the recomputed route did not pick up the new migration")
	}
}

// TestARerouteSkipsWorkTheNewAnalysisNoLongerNeeds documents how the
// "completed work survives a reroute" rule interacts with the L2.12
// cascade. Editing the analysis demotes everything recorded after it, so a
// downstream stage is STALE rather than COMPLETED by the time the new route
// is applied — and a stale stage the route excludes is skipped rather than
// re-run. The completed-survives rule therefore almost never fires; this
// pins what actually happens instead of what the rule alone would suggest.
func TestARerouteSkipsWorkTheNewAnalysisNoLongerNeeds(t *testing.T) {
	analysis := analysisNeedingNothing()
	analysis.DataModelChanges = []state.DataModelChange{{Description: "add sessions", Phase: state.MigrationPhaseExpand}}
	executor, provider, store, input := newHarness(t, routedScripts(t, analysis))
	if err := executor.Run(context.Background(), routedPlan(), input); !errors.Is(err, orchestrator.ErrWaitingApproval) {
		t.Fatalf("first leg = %v, want a halt", err)
	}

	// The analysis loses its migration; data-engineer has already run.
	payload, err := json.Marshal(analysisNeedingNothing())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(input.WorkspaceDir, state.TypedStateDir, "analyst.json"), payload, 0o644); err != nil {
		t.Fatalf("edit analysis: %v", err)
	}
	provider.SetScript("analyst", mock.Script{Payload: payload})
	executor.OnStale(func([]orchestrator.StaleStage) {})
	_ = executor.Run(context.Background(), routedPlan(), input)

	if got := mustLoad(t, store).Stages["data-engineer"].Status; got != orchestrator.StageStatusSkipped {
		t.Errorf("data-engineer status = %q, want SKIPPED: its work was invalidated by the edit and the new route does not ask for it", got)
	}
}

func TestSkippedStagesAppearOnTheTimeline(t *testing.T) {
	executor, _, store, input := newHarness(t, routedScripts(t, analysisNeedingNothing()))
	_ = executor.Run(context.Background(), routedPlan(), input)

	found := firstOfEachKind(readTimeline(t, store))
	event, ok := found[orchestrator.EventStageSkipped]
	if !ok {
		t.Fatal("no stage.skipped event was recorded")
	}
	if event.Stage == "" || event.Reason == "" {
		t.Errorf("stage.skipped event = %+v, want a stage and a reason", event)
	}
}

func TestRouterFailsClearlyWithoutAnAnalysis(t *testing.T) {
	plan := routedPlan()
	plan.Stages = plan.Stages[1:] // router first, no analyst
	executor, _, _, input := newHarness(t, map[string]mock.Script{})

	err := executor.Run(context.Background(), plan, input)

	if err == nil || !strings.Contains(err.Error(), "router needs the analyst") {
		t.Errorf("error = %v, want a clear failure about the missing analysis", err)
	}
}

// Every stage that can only review a surface must be skippable, or the
// router cannot route around it however clearly the analysis says there is
// no such surface (roadmap L3.24). visual-qa-engineer and sre-engineer were
// unskippable and cost $1.18 on a three-line array filter.
func TestSurfaceSpecificStagesAreSkippableInTheBuiltInPlan(t *testing.T) {
	plan := orchestrator.DefaultDeliverFeaturePlan()
	skippable := map[string]bool{}
	for _, stage := range plan.Stages {
		skippable[stage.ID] = stage.Skippable
	}

	for _, stage := range []string{"accessibility-engineer", "visual-qa-engineer", "sre-engineer"} {
		if !skippable[stage] {
			t.Errorf("stage %q reviews one kind of surface but cannot be routed around", stage)
		}
	}
	// The asymmetry the skippable set exists to protect: an unnecessary
	// review wastes an invocation, a skipped one does not fail so cheaply.
	for _, stage := range []string{"code-reviewer", "security-reviewer", "qa-engineer", "developer"} {
		if skippable[stage] {
			t.Errorf("stage %q must never be skippable by routing", stage)
		}
	}
}
