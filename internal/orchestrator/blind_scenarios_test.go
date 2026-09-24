package orchestrator_test

// The fitness function for roadmap L3.62: acceptance scenarios are written
// from the analysis alone, before any implementation exists (ADR-009).
//
// It runs the built-in plan end to end and inspects what the stage actually
// received, rather than reading the plan's declaration back: the declaration
// is what a regression would edit, and a test that restates it would change
// with it.

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
	"github.com/orieken/loom/internal/state"
)

// analysisFieldsForScenarios are the only fields the stage may read: the
// criteria and what surrounds them, never anything describing the build.
var analysisFieldsForScenarios = map[string]bool{
	"feature": true, "acceptanceCriteria": true, "edgeCases": true, "qaTasks": true, "definitionOfDone": true,
}

func TestAcceptanceScenariosAreWrittenBlind(t *testing.T) {
	provider := runRecordingProvider(t, orchestrator.DefaultDeliverFeaturePlan())

	received := provider.InputFor(state.AcceptanceScenariosStageID)
	if upstreams := upstreamNames(received); strings.Join(upstreams, ",") != "analyst" {
		t.Errorf("acceptance-scenarios read %v, want the analysis and nothing else", upstreams)
	}
	for field := range projectedFields(t, received.UpstreamState["analyst"]) {
		if !analysisFieldsForScenarios[field] {
			t.Errorf("acceptance-scenarios was shown %q, which is not an acceptance-criteria field", field)
		}
	}

	invocations := provider.Invocations()
	scenarios, developer := position(invocations, state.AcceptanceScenariosStageID), position(invocations, "developer")
	if scenarios < 0 || developer < 0 || scenarios > developer {
		t.Errorf("invocations %v: acceptance-scenarios must run, and before the developer", invocations)
	}
}

// Plans choose their own order, so the executor enforces it for every plan.
func TestAPlanWritingScenariosAfterTheBuildIsRejected(t *testing.T) {
	plan := orchestrator.DefaultDeliverFeaturePlan()
	scenarios, developer := -1, -1
	for index, stage := range plan.Stages {
		switch stage.ID {
		case state.AcceptanceScenariosStageID:
			scenarios = index
		case "developer":
			developer = index
		}
	}
	plan.Stages[scenarios], plan.Stages[developer] = plan.Stages[developer], plan.Stages[scenarios]

	err := plan.Validate()
	if err == nil || !strings.Contains(err.Error(), "before the implementation exists") {
		t.Errorf("Validate() = %v, want the plan rejected for writing scenarios after the build", err)
	}
}

func TestQAEngineerAutomatesTheScenariosItWasGiven(t *testing.T) {
	provider := runRecordingProvider(t, orchestrator.DefaultDeliverFeaturePlan())
	var given state.QAScenariosInput
	payload := provider.InputFor("qa-engineer").UpstreamState[state.AcceptanceScenariosStageID]
	if err := json.Unmarshal(payload, &given); err != nil || len(given.Scenarios) == 0 {
		t.Errorf("qa-engineer did not receive the pre-written scenarios: %v %s", err, payload)
	}
}

// runRecordingProvider drives a plan past every gate, as runToCompletion
// does, but keeps the provider so a test can see what each stage received.
func runRecordingProvider(t *testing.T, plan orchestrator.Plan) *mock.Provider {
	t.Helper()
	executor, provider, _, input := newHarness(t, scriptsForPlan(plan))
	input.ProjectRoot = input.WorkspaceDir
	for attempt := 0; attempt <= len(plan.Stages); attempt++ {
		err := executor.Run(context.Background(), plan, input)
		if err == nil {
			return provider
		}
		var waiting *orchestrator.WaitingApprovalError
		if !errors.As(err, &waiting) {
			t.Fatalf("Run: %v", err)
		}
		if approveErr := executor.Approve(waiting.Gate, orchestrator.ApprovalMethodFlag); approveErr != nil {
			t.Fatalf("Approve %q: %v", waiting.Gate, approveErr)
		}
	}
	t.Fatalf("plan %q did not finish within %d approvals", plan.Name, len(plan.Stages))
	return nil
}

func upstreamNames(input orchestrator.StageInput) []string {
	names := make([]string, 0, len(input.UpstreamState))
	for name := range input.UpstreamState {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func projectedFields(t *testing.T, payload []byte) map[string]json.RawMessage {
	t.Helper()
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("projected analysis is not a JSON object: %v", err)
	}
	return fields
}

func position(invocations []string, stageID string) int {
	for index, invoked := range invocations {
		if invoked == stageID {
			return index
		}
	}
	return -1
}
