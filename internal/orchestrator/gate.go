package orchestrator

import (
	"errors"
	"fmt"
	"os/user"
	"strings"
	"time"
)

// ApprovalMethod records which CLI channel recorded an approval. Provider
// output is never a channel: nothing an agent returns can create an
// approval (roadmap L2.13 — enforcement lives below the model).
type ApprovalMethod string

// The only two approval channels that exist today. Webhook and queue
// channels are later roadmap items.
const (
	ApprovalMethodTTY  ApprovalMethod = "tty"
	ApprovalMethodFlag ApprovalMethod = "flag"
	// ApprovalMethodCLI records an approval entered with `loom state
	// approve` for a markdown-pipeline run. It is an audit record of a human
	// decision, not an enforced gate — the markdown pipeline can proceed
	// without it (roadmap L2.13 covers `loom run` only).
	ApprovalMethodCLI ApprovalMethod = "cli"
)

// Approval is the durable record that a named gate was unlocked by a human,
// bound to the artifacts that human was shown (roadmap L2.14). An approval
// with no digests approves nothing in particular; one with digests approves
// exactly that state of the run.
type Approval struct {
	ApprovedAt time.Time      `json:"approvedAt"`
	Method     ApprovalMethod `json:"method"`
	Approver   string         `json:"approver"`
	// ArtifactDigests is the SHA-256 of every completed stage's artifact at
	// the moment of approval. A human approving a gate is approving the
	// state of the run, not one file — so an edit to any of them resets the
	// gate, which is what all eight prose gates have always declared.
	ArtifactDigests map[string]string `json:"artifactDigests,omitempty"`
	// InvalidatedAt and InvalidatedBy are set when a bound artifact changed
	// after approval. The record is kept rather than deleted: it is the
	// audit trail of what was approved, and when it stopped being true.
	InvalidatedAt *time.Time `json:"invalidatedAt,omitempty"`
	InvalidatedBy string     `json:"invalidatedBy,omitempty"`
}

// IsValid reports whether this approval still stands.
func (a Approval) IsValid() bool { return a.InvalidatedAt == nil }

// ErrWaitingApproval is the sentinel every gate halt wraps, so callers can
// branch with errors.Is without depending on the concrete error type.
var ErrWaitingApproval = errors.New("waiting for gate approval")

// WaitingApprovalError names the gate the run stopped at and the stage that
// gate guards. The CLI turns this into a prompt or a resume instruction.
type WaitingApprovalError struct {
	Gate  string
	Stage string
	// SkipReason is set when the gated stage was already routed out of this
	// run, so a caller can say the gate guards work that will not happen
	// (roadmap L3.32). Empty for the ordinary case.
	SkipReason string
}

func (e *WaitingApprovalError) Error() string {
	return fmt.Sprintf("stage %q is gated by %q: %s", e.Stage, e.Gate, ErrWaitingApproval)
}

// Unwrap makes errors.Is(err, ErrWaitingApproval) true for every gate halt.
func (e *WaitingApprovalError) Unwrap() error { return ErrWaitingApproval }

// IsStageSettled reports whether a stage needs no further work: it either
// completed successfully or was routed around. Both are terminal for the
// run loop; RUNNING, INTERRUPTED, FAILED, and STALE are not.
func (s *RunState) IsStageSettled(stageID string) bool {
	status := s.Stages[stageID].Status
	return status == StageStatusCompleted || status == StageStatusSkipped
}

// IsGateApproved reports whether the named gate is unlocked for this run.
// An approval that was invalidated by a later edit does not count — that is
// L2.14's whole point.
func (s *RunState) IsGateApproved(gate string) bool {
	approval, ok := s.Approvals[gate]
	return ok && approval.IsValid()
}

// WaitingGate returns the gate this run is currently halted on, or "" when
// it is not waiting on anything.
// A run can hold more than one WAITING_APPROVAL record: halting at a later
// gate, then being sent back to an earlier one by a reset approval, leaves
// the later stage's record behind. The run is waiting on the *earliest*
// barrier it has reached, so this walks stages in recorded sequence rather
// than in map order — which was nondeterministic, and could report (and
// let a human approve) a gate the run was not actually sitting at.
func (s *RunState) WaitingGate() string {
	for _, stageID := range s.StagesInSequence() {
		if s.Stages[stageID].Status == StageStatusWaitingApproval {
			return s.Stages[stageID].Gate
		}
	}
	return ""
}

// Approve records a human approval for the gate the run is currently
// waiting on and persists it atomically. It refuses any gate the run is not
// actually halted at, which is what stops a caller from pre-approving every
// gate in one command line and hollowing out the interrupt.
func (e *Executor) Approve(gate string, method ApprovalMethod) error {
	state, err := e.store.Load()
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("no run state at %s — nothing to approve", e.store.Path())
	}
	if err := checkWaitingOn(state, gate); err != nil {
		return err
	}
	// Detect BEFORE recording: an approval is the moment the schema's
	// "checksum changed between gate-presented and gate-approved" is
	// finally answerable, and refreshing the baseline first would erase the
	// difference being measured.
	corrections := e.detectCorrections(state)
	state.RecordApproval(gate, method)
	// Refresh the baseline: approving means accepting the state as it
	// stands right now, so that state is what a *later* edit is measured
	// against (roadmap L4.5).
	e.captureBaseline(state, gate, BaselineApproved)
	if err := e.recordCorrections(state, corrections); err != nil {
		return err
	}
	if err := e.store.Save(state); err != nil {
		return fmt.Errorf("persist approval for gate %q: %w", gate, err)
	}
	return e.emit(Event{Kind: EventGateApproved, Gate: gate, ApprovalMethod: method})
}

// RecordApproval writes an approval for a named gate, binding it to the
// digests of everything completed so far. Re-approving after an
// invalidation replaces the record with a fresh binding. Callers that must
// enforce "the run is actually waiting on this gate" go through
// Executor.Approve; this is the plain record used by the markdown
// pipeline, which has no executor barrier to wait at.
func (s *RunState) RecordApproval(gate string, method ApprovalMethod) {
	s.Approvals[gate] = Approval{
		ApprovedAt: time.Now().UTC(), Method: method, Approver: currentApprover(),
		ArtifactDigests: s.completedDigests(),
	}
}

// completedDigests snapshots what a human is approving: the recorded digest
// of every completed stage's artifact. Stages that produced no artifact
// bind nothing — there is no content to have changed.
func (s *RunState) completedDigests() map[string]string {
	digests := make(map[string]string, len(s.Stages))
	for stageID, record := range s.Stages {
		if record.Status == StageStatusCompleted && record.ArtifactSHA256 != "" {
			digests[stageID] = record.ArtifactSHA256
		}
	}
	return digests
}

// InvalidateApproval marks an approval stale because a bound artifact
// changed, naming the stage responsible.
func (s *RunState) InvalidateApproval(gate, changedStage string) {
	approval, ok := s.Approvals[gate]
	if !ok || !approval.IsValid() {
		return
	}
	now := time.Now().UTC()
	approval.InvalidatedAt = &now
	approval.InvalidatedBy = changedStage
	s.Approvals[gate] = approval
}

func checkWaitingOn(state *RunState, gate string) error {
	waiting := state.WaitingGate()
	if waiting == "" {
		return fmt.Errorf("run is not waiting on any gate — cannot approve %q", gate)
	}
	if waiting != gate {
		return fmt.Errorf("run is waiting on gate %q, not %q", waiting, gate)
	}
	return nil
}

// currentApprover identifies the human at the CLI by OS username. An
// unresolvable user is recorded literally rather than failing the approval.
func currentApprover() string {
	current, err := user.Current()
	if err != nil || strings.TrimSpace(current.Username) == "" {
		return "unknown"
	}
	return current.Username
}
