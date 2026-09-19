package orchestrator

// Detecting edits made after the review approved (roadmap L3.36).
//
// `code-reviewer` runs before four stages that can modify source. In run 4
// it approved a 312-insertion tree; the tree that shipped was 343, and
// nothing recorded that they differed. "Review sees what ships" is a
// property the pipeline sells, and it was not being kept.
//
// This RECORDS the divergence rather than preventing it. Whether a
// post-review stage should be allowed to touch production source is a
// separate question this deliberately does not answer — `sre-engineer`
// implicated in run 4 did nothing wrong: it is entitled to write, its
// instrumentation was correct, and the suite passed. The defect is that a
// human could not tell, not that the edits were bad.
//
// It does not reuse the posture check, which asks a different question —
// did a stage change the tree WITHOUT declaring a write tool — and
// deliberately exempts declared writers. After L3.50 made every
// post-review stage declare `Write`, that exemption covers all of them, so
// the posture check no longer notices this case at all. It never should
// have been the thing noticing: it caught run 4 only because
// `accessibility-engineer`'s declaration was wrong.

import (
	"sort"

	"github.com/orieken/loom/internal/state"
)

// reviewStateKind anchors the review boundary to the typed kind the
// reviewing stage produces, not to a stage ID a plan may rename.
const reviewStateKind = string(state.KindReview)

// noteReviewApproved records the tree as the review left it. Everything
// that changes afterwards is what the approval did not cover.
//
// Keyed on the stage's typed kind rather than its ID so a plan that renames
// or duplicates the reviewing stage still anchors correctly.
func (e *Executor) noteReviewApproved(state *RunState, stage Stage, after string) {
	if after == "" || stage.StateKind != reviewStateKind {
		return
	}
	state.ReviewedDigest = after
}

// notePostReviewEdit marks a stage that changed the tree after the review
// had approved it. The mark is on the stage record so it survives into the
// archive, and is summarised by PostReviewEdits for the run to report.
func (e *Executor) notePostReviewEdit(state *RunState, stage Stage, before string) {
	if before == "" || state.ReviewedDigest == "" || stage.StateKind == reviewStateKind {
		return
	}
	if e.treeDigest() == before {
		return
	}
	record := state.Stages[stage.ID]
	record.EditedAfterReview = true
	state.Stages[stage.ID] = record
}

// PostReviewEdits lists the stages that changed the tree after the review
// approved it, sorted so the report is stable across runs.
//
// An empty list is the honest answer in two different situations: nothing
// changed after the review, or there was no review to change anything
// after. A caller that needs to tell them apart reads ReviewedDigest.
func (s *RunState) PostReviewEdits() []string {
	var stages []string
	for id, record := range s.Stages {
		if record.EditedAfterReview {
			stages = append(stages, id)
		}
	}
	sort.Strings(stages)
	return stages
}
