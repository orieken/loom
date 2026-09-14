package orchestrator_test

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
)

// updateShape regenerates the committed baseline instead of asserting
// against it: `go test ./internal/orchestrator -run TestShape -update`.
//
// The flag exists because hand-editing JSON drifts from what the run
// actually emits. The discipline that keeps it from becoming
// regenerate-until-green is the same one the agent goldens use: a failure
// prints the whole old-and-new shape, so the change is visible in review,
// and the commit that moves the baseline says why.
var updateShape = flag.Bool("update", false, "rewrite the committed trace-shape baseline")

const shapeBaseline = "testdata/shape-deliver-feature.json"

// scriptsForPlan gives every stage in a plan something valid to return: the
// scripted state document for a typed stage, markdown for the rest.
func scriptsForPlan(plan orchestrator.Plan) map[string]mock.Script {
	scripts := map[string]mock.Script{}
	for _, stage := range plan.Stages {
		if stage.Internal {
			continue
		}
		if script, typed := mock.TypedScript(stage.StateKind); typed {
			scripts[stage.ID] = script
			continue
		}
		scripts[stage.ID] = mock.Script{ArtifactContent: "# " + stage.ID}
	}
	return scripts
}

// runToCompletion drives a plan past every gate it halts at, approving each
// one as a human would, and returns the shape of the whole run.
func runToCompletion(t *testing.T, plan orchestrator.Plan) orchestrator.Shape {
	t.Helper()
	executor, _, _, input := newHarness(t, scriptsForPlan(plan))
	// The claim check (L2.24) verifies reported files against ProjectRoot,
	// and the mock writes them there; with it unset the two disagree and a
	// stage is accused of fabricating work the mock did do.
	input.ProjectRoot = input.WorkspaceDir
	recorder := &orchestrator.ShapeRecorder{}
	executor.WithTracer(recorder)

	// One approval per gate the plan declares, plus one attempt to finish.
	// A bound rather than `for {}`: a plan that halts at the same gate twice
	// is a defect, and this test should report it rather than hang.
	for attempt := 0; attempt <= len(plan.Stages); attempt++ {
		err := executor.Run(context.Background(), plan, input)
		if err == nil {
			return recorder.Shape()
		}
		var waiting *orchestrator.WaitingApprovalError
		if !errors.As(err, &waiting) {
			t.Fatalf("Run: %v", err)
		}
		if approveErr := executor.Approve(waiting.Gate, orchestrator.ApprovalMethodFlag); approveErr != nil {
			t.Fatalf("Approve %q: %v", waiting.Gate, approveErr)
		}
	}
	t.Fatal("run never completed; it kept halting for approval")
	return orchestrator.Shape{}
}

// TestShapeOfTheBuiltInPlanIsStable is the L3.44 done-when: adding a stage
// to the built-in plan fails here until the baseline is updated in the same
// commit.
//
// What it protects is the run as a whole — which stages ran, in what order,
// how many times, how the review loop settled, and how many model calls it
// took. A routing change that quietly adds two stages, or a loop that starts
// terminating by its bound instead of converging, passes every other test in
// this repository as long as the artifacts validate.
func TestShapeOfTheBuiltInPlanIsStable(t *testing.T) {
	shape := runToCompletion(t, orchestrator.DefaultDeliverFeaturePlan())

	recorded, err := json.MarshalIndent(shape, "", "  ")
	if err != nil {
		t.Fatalf("encode shape: %v", err)
	}
	recorded = append(recorded, '\n')

	if *updateShape {
		if writeErr := os.WriteFile(shapeBaseline, recorded, 0o644); writeErr != nil {
			t.Fatalf("write baseline: %v", writeErr)
		}
		t.Logf("baseline rewritten: %s — put the diff in the commit and say why", shapeBaseline)
		return
	}

	want, err := os.ReadFile(shapeBaseline)
	if err != nil {
		t.Fatalf("read baseline (regenerate with -update): %v", err)
	}
	if string(recorded) != string(want) {
		t.Errorf("the run's shape changed.\n\n--- committed %s\n%s\n--- this run\n%s\n"+
			"If the change is intended, regenerate with:\n"+
			"  go test ./internal/orchestrator -run TestShapeOfTheBuiltInPlanIsStable -update\n"+
			"and put the diff in the commit with one sentence on why.",
			shapeBaseline, want, recorded)
	}
}

// The shape must exclude anything that changes between two identical runs.
// A baseline carrying a duration or a span ID would fail on the next run and
// train everyone to regenerate it, which is how a golden stops protecting
// anything.
func TestShapeIsStableAcrossTwoIdenticalRuns(t *testing.T) {
	plan := orchestrator.DefaultDeliverFeaturePlan()
	first, second := runToCompletion(t, plan), runToCompletion(t, plan)

	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	if string(firstJSON) != string(secondJSON) {
		t.Errorf("two identical runs produced different shapes:\n%s\n%s", firstJSON, secondJSON)
	}
}

func TestShapeBaselineIsCommitted(t *testing.T) {
	if _, err := os.Stat(filepath.FromSlash(shapeBaseline)); err != nil {
		t.Fatalf("no committed baseline at %s: %v", shapeBaseline, err)
	}
}
