package orchestrator

// Reset-on-edit (roadmap L2.14). All eight prose gates declare "any edit to
// the pending artifact resets the gate"; until now nothing enforced it, so
// an edited artifact demoted its stage while the approval survived and the
// gated stage proceeded on a decision about content that no longer exists.
//
// An approval binds to the digests it was given. Before a gated stage
// starts, those digests are checked against what is on disk: a mismatch,
// or a vanished artifact, invalidates the approval and the run halts at the
// gate again. A stage that re-runs to a byte-identical artifact leaves its
// digest unchanged and keeps the approval — the rule is "any edit resets
// the gate", not "any re-run".

import "fmt"

// invalidateApprovalsFor resets every approval bound to a stage that
// digest verification just demoted. This is the primary detection point,
// not the barrier: verification runs before the demoted stage re-runs, and
// a re-run overwrites the edited file — so checking only at the barrier
// would let an agent's fresh output mask the edit that caused it.
//
// A write that changes nothing demotes nothing (the digest still matches),
// so an identical re-run still leaves the approval standing.
func (e *Executor) invalidateApprovalsFor(state *RunState, stale []StaleStage) error {
	for _, item := range stale {
		if err := e.invalidateApprovalsBoundTo(state, item.StageID); err != nil {
			return err
		}
	}
	return nil
}

func (e *Executor) invalidateApprovalsBoundTo(state *RunState, stageID string) error {
	for gate, approval := range state.Approvals {
		if !approval.IsValid() {
			continue
		}
		if _, bound := approval.ArtifactDigests[stageID]; !bound {
			continue
		}
		state.InvalidateApproval(gate, stageID)
		if e.onReset != nil {
			e.onReset(&StaleApprovalError{Gate: gate, ChangedStage: stageID})
		}
		if err := e.emit(Event{Kind: EventGateInvalidated, Gate: gate, Stage: stageID}); err != nil {
			return err
		}
	}
	return nil
}

// StaleApprovalError says an approval no longer holds and names the
// artifact that changed, so a human who edited a file on purpose is not
// left guessing why the run stopped.
type StaleApprovalError struct {
	Gate         string
	ChangedStage string
}

func (e *StaleApprovalError) Error() string {
	return fmt.Sprintf("approval for gate %q was reset: stage %q's artifact changed after it was approved",
		e.Gate, e.ChangedStage)
}

// WouldInvalidateApprovals reports the first gate whose approval an
// impending verification would reset, and the stage responsible. The CLI
// calls it before recording an approval so it never writes one that the
// same command is about to destroy. Verification cascades by recorded
// sequence, so this needs no plan.
func (e *Executor) WouldInvalidateApprovals() (*StaleApprovalError, error) {
	state, err := e.store.Load()
	if err != nil || state == nil {
		return nil, err
	}
	for _, stale := range VerifyCompletedStages(cloneForInspection(state)) {
		if gate := gateBoundTo(state, stale.StageID); gate != "" {
			return &StaleApprovalError{Gate: gate, ChangedStage: stale.StageID}, nil
		}
	}
	return nil, nil
}

// cloneForInspection copies the parts VerifyCompletedStages mutates, so a
// dry-run check cannot demote anything in the real state.
func cloneForInspection(state *RunState) *RunState {
	stages := make(map[string]StageRecord, len(state.Stages))
	for id, record := range state.Stages {
		stages[id] = record
	}
	return &RunState{SchemaVersion: state.SchemaVersion, PlanName: state.PlanName, Stages: stages,
		Approvals: state.Approvals, SpecPath: state.SpecPath, SpecSHA256: state.SpecSHA256}
}

func gateBoundTo(state *RunState, stageID string) string {
	for gate, approval := range state.Approvals {
		if !approval.IsValid() {
			continue
		}
		if _, bound := approval.ArtifactDigests[stageID]; bound {
			return gate
		}
	}
	return ""
}

// checkApprovalBinding re-verifies a bound approval. It returns the stage
// whose artifact changed, or "" when the approval still holds.
func checkApprovalBinding(state *RunState, gate string) string {
	approval, ok := state.Approvals[gate]
	if !ok || !approval.IsValid() {
		return ""
	}
	for stageID, bound := range approval.ArtifactDigests {
		if changedSince(state, stageID, bound) {
			return stageID
		}
	}
	return ""
}

// changedSince reports whether a bound stage's artifact differs from the
// digest recorded when the human approved. A missing file counts as
// changed: content that vanished is not the content that was approved.
func changedSince(state *RunState, stageID, bound string) bool {
	record, ok := state.Stages[stageID]
	if !ok || record.ArtifactPath == "" {
		return true
	}
	found, err := ArtifactSHA256(record.ArtifactPath)
	if err != nil {
		// Unreadable for any reason — deleted, permissions, replaced by a
		// directory — is not the content that was approved.
		return true
	}
	return found != bound
}

// enforceApprovalBinding invalidates a stale approval and records why,
// so the barrier can halt as if the gate had never been approved.
func (e *Executor) enforceApprovalBinding(state *RunState, stage Stage) (bool, error) {
	changedStage := checkApprovalBinding(state, stage.Gate)
	if changedStage == "" {
		return false, nil
	}
	state.InvalidateApproval(stage.Gate, changedStage)
	if e.onReset != nil {
		e.onReset(&StaleApprovalError{Gate: stage.Gate, ChangedStage: changedStage})
	}
	if err := e.store.Save(state); err != nil {
		return false, err
	}
	return true, e.emit(Event{Kind: EventGateInvalidated, Gate: stage.Gate, Stage: changedStage})
}
