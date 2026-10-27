package orchestrator

// Integrity is the L2.12 half of run state: artifact digests are computed
// and verified in Go, never by a model. A stage whose artifact changed on
// disk since it completed is not treated as complete — it re-runs, and so
// does every stage that consumed its output.

import (
	"fmt"
)

// StaleReason says why a completed stage stopped counting as complete.
type StaleReason string

// The three ways verification demotes a stage.
const (
	// StaleReasonEdited means the artifact's digest no longer matches the
	// one recorded when the stage completed.
	StaleReasonEdited StaleReason = "ARTIFACT_EDITED"
	// StaleReasonMissing means the recorded artifact is gone or unreadable.
	StaleReasonMissing StaleReason = "ARTIFACT_MISSING"
	// StaleReasonUpstream means an earlier stage went stale, so this
	// stage's own output was derived from content that no longer exists.
	StaleReasonUpstream StaleReason = "UPSTREAM_STALE"
	// StaleReasonSpecChanged means the spec the run was bound to changed,
	// and every stage's output derives from it (review item I-1).
	StaleReasonSpecChanged StaleReason = "SPEC_CHANGED"
)

// StaleStage reports one demotion, for the CLI to show the human.
type StaleStage struct {
	StageID string
	Reason  StaleReason
}

// VerifyCompletedStages walks the recorded stages in sequence order,
// demoting any completed stage whose artifact no longer matches its
// recorded digest, then cascading the demotion to every stage recorded
// after it. It returns the demotions in sequence order; the caller persists
// once. Sequence rather than plan order is what lets the markdown pipeline,
// which has no plan, share this code.
//
// Demotion never touches Approvals: unlocking a gate stays unlocked even
// when the stage behind it goes stale. Binding approvals to digests is
// roadmap L2.14, deliberately not built here.
//
// The spec is checked first: it is upstream of every stage, so a changed spec
// demotes them all and the per-artifact checks have nothing left to decide.
func VerifyCompletedStages(state *RunState) []StaleStage {
	if specChanged(state) {
		return demoteForSpecChange(state)
	}
	stale := make([]StaleStage, 0)
	cascading := false
	for _, stageID := range state.StagesInSequence() {
		reason, demote := stageStaleReason(state, stageID, cascading)
		if !demote {
			continue
		}
		markStale(state, stageID, reason)
		stale = append(stale, StaleStage{StageID: stageID, Reason: reason})
		cascading = true
	}
	return stale
}

// stageStaleReason decides one stage's fate: only completed stages can be
// demoted, an already-cascading run demotes them all, and otherwise the
// artifact on disk decides.
func stageStaleReason(state *RunState, stageID string, cascading bool) (StaleReason, bool) {
	if !state.IsStageCompleted(stageID) {
		return "", false
	}
	if cascading {
		return StaleReasonUpstream, true
	}
	return artifactStaleReason(state.Stages[stageID])
}

// artifactStaleReason compares the artifact on disk with the digest
// recorded when the stage completed. A stage that produced no artifact has
// nothing to verify and stays completed — the executor cannot check what
// was never written.
func artifactStaleReason(record StageRecord) (StaleReason, bool) {
	if record.ArtifactPath == "" {
		return "", false
	}
	found, err := ArtifactSHA256(record.ArtifactPath)
	if err != nil {
		return StaleReasonMissing, true
	}
	if found != record.ArtifactSHA256 {
		return StaleReasonEdited, true
	}
	return "", false
}

func markStale(state *RunState, stageID string, reason StaleReason) {
	record := state.Stages[stageID]
	record.PreviousStatus = record.Status
	record.Status = StageStatusStale
	record.StaleReason = reason
	record.FoundSHA256 = foundDigest(record.ArtifactPath)
	// FinishedAt keeps when the stage actually finished; when it was demoted
	// is the stage.stale event's timestamp.
	state.Stages[stageID] = record
}

// foundDigest records what was actually on disk at demotion time, so the
// state file shows the mismatch rather than only asserting it. An
// unreadable artifact records nothing.
func foundDigest(artifactPath string) string {
	if artifactPath == "" {
		return ""
	}
	found, err := ArtifactSHA256(artifactPath)
	if err != nil {
		return ""
	}
	return found
}

// Description renders one demotion for humans — the line the CLI prints
// before re-running the stage.
func (s StaleStage) Description() string {
	switch s.Reason {
	case StaleReasonEdited:
		return fmt.Sprintf("stage %q was COMPLETED but its artifact changed on disk — re-running", s.StageID)
	case StaleReasonMissing:
		return fmt.Sprintf("stage %q was COMPLETED but its artifact is missing — re-running", s.StageID)
	case StaleReasonSpecChanged:
		return fmt.Sprintf("stage %q was COMPLETED but the spec changed since — re-running", s.StageID)
	default:
		return fmt.Sprintf("stage %q re-runs because an earlier stage went stale", s.StageID)
	}
}
