package orchestrator_test

// Roadmap L3.36: a run whose tree changed after the review approved it must
// say so. The case that matters is a stage that DECLARES a write tool —
// after L3.50 every post-review stage does, so the posture check (which
// exempts declared writers, correctly, because it asks a different
// question) no longer notices this at all.

import (
	"context"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
	"github.com/orieken/loom/internal/state"
)

// editingTree changes its digest from the nth call onward, standing in for
// a stage that modified source while it ran.
//
// Coupled to the digest CALL SEQUENCE, like the changingTree fake this
// package already uses: the executor takes a digest before a stage, again
// for the posture check, and again to anchor the review boundary, so which
// call falls inside which stage depends on that pattern. A change to it
// makes these tests fail rather than pass quietly, which is the right
// direction — but read this comment before assuming the production code
// broke. Measured: this plan takes ten digests, and the final stage spans
// calls 8 through 10.
type editingTree struct {
	calls     int
	changeAt  int
	head      string
	diffLines int
}

func (t *editingTree) Digest() (string, error) {
	t.calls++
	if t.calls >= t.changeAt {
		return "after", nil
	}
	return "before", nil
}

func (t *editingTree) ChangedPaths() ([]string, error)    { return nil, nil }
func (t *editingTree) HeadRef() (string, error)           { return t.head, nil }
func (t *editingTree) DiffLinesSince(string) (int, error) { return t.diffLines, nil }

// A declared writer editing after the review is exactly run 4's
// sre-engineer: entitled to write, correct in what it wrote, and invisible.
func TestAStageEditingAfterTheReviewIsRecorded(t *testing.T) {
	executor, _, store, input := newHarness(t, map[string]mock.Script{
		"analyst":     {ArtifactContent: "# analysis"},
		"developer":   {Payload: reviewPayload(t, state.VerdictApproved)},
		"qa-engineer": {ArtifactContent: "# qa"},
	})
	plan := reviewThenWriterPlan()
	// Steady through the review; changes during the final stage.
	executor = executor.WithWorkTree(&editingTree{changeAt: 8})

	if err := executor.Run(context.Background(), plan, input); err != nil {
		t.Fatalf("Run: %v", err)
	}

	runState := mustLoad(t, store)
	if runState.ReviewedDigest == "" {
		t.Fatal("no reviewed digest recorded, so nothing anchors the review boundary")
	}
	edits := runState.PostReviewEdits()
	if len(edits) == 0 {
		t.Fatal("a stage changed the tree after the review approved it and nothing recorded it")
	}
	if edits[0] != "qa-engineer" {
		t.Errorf("post-review edits = %v, want the stage that ran after the review", edits)
	}
}

// A run where nothing changes after the review must stay silent. A warning
// that fires on every run is one people learn to skip.
func TestAQuietRunRecordsNoPostReviewEdits(t *testing.T) {
	executor, _, store, input := newHarness(t, map[string]mock.Script{
		"analyst":     {ArtifactContent: "# analysis"},
		"developer":   {Payload: reviewPayload(t, state.VerdictApproved)},
		"qa-engineer": {ArtifactContent: "# qa"},
	})
	// Never changes: changeAt beyond any call this plan makes.
	executor = executor.WithWorkTree(&editingTree{changeAt: 1000})

	if err := executor.Run(context.Background(), reviewThenWriterPlan(), input); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if edits := mustLoad(t, store).PostReviewEdits(); len(edits) != 0 {
		t.Errorf("post-review edits = %v, want none on a run that changed nothing after review", edits)
	}
}

// The review stage itself is not a post-review edit. It necessarily changes
// the tree as it writes its own report, and flagging that would fire on
// every run and mean nothing.
func TestTheReviewStageIsNotItsOwnPostReviewEdit(t *testing.T) {
	executor, _, store, input := newHarness(t, map[string]mock.Script{
		"analyst":     {ArtifactContent: "# analysis"},
		"developer":   {Payload: reviewPayload(t, state.VerdictApproved)},
		"qa-engineer": {ArtifactContent: "# qa"},
	})
	// Changes from the very first digest, so the review stage itself is
	// running across a changing tree.
	executor = executor.WithWorkTree(&editingTree{changeAt: 1})

	if err := executor.Run(context.Background(), reviewThenWriterPlan(), input); err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, id := range mustLoad(t, store).PostReviewEdits() {
		if id == "developer" {
			t.Error("the reviewing stage was recorded as editing after its own review")
		}
	}
}

// reviewThenWriterPlan makes the middle stage the reviewer, leaving a
// stage after it that can modify source — the shape the built-in plan has
// and the one L3.36 is about.
func reviewThenWriterPlan() orchestrator.Plan {
	plan := threeStagePlan()
	plan.Stages[1].StateKind = string(state.KindReview)
	return plan
}
