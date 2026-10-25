package orchestrator

import (
	"context"
	"errors"
)

// FailureClass is whether a failed stage is worth running again (roadmap
// L3.68). It is the first entry in the error taxonomy roadmap L4.2 will
// widen; until then it has exactly the two values a retry decision needs.
type FailureClass string

const (
	// FailureTransient is a failure a later attempt may not repeat: a
	// provider error, a timeout, an interruption.
	FailureTransient FailureClass = "transient"
	// FailureTerminal is a failure the same input will reproduce: an
	// input that cannot be projected, an internal stage's output.
	FailureTerminal FailureClass = "terminal"
)

// ErrInvalidState marks a typed stage's payload that does not conform to
// its schema.
var ErrInvalidState = errors.New("invalid state")

// classifyFailure settles a failure's class. Cancellation is always
// transient — the run was stopped, not broken — and an unclassified failure
// is terminal.
func classifyFailure(failure stageFailure) FailureClass {
	if errors.Is(failure.err, context.Canceled) || errors.Is(failure.err, context.DeadlineExceeded) {
		return FailureTransient
	}
	if failure.class == "" {
		return FailureTerminal
	}
	return failure.class
}

// invocationFailureClass classifies a provider call that returned an error.
// An internal stage runs here and is deterministic, so its failure repeats;
// a provider can fail for reasons outside the input.
func invocationFailureClass(stage Stage) FailureClass {
	if stage.Internal {
		return FailureTerminal
	}
	return FailureTransient
}

// artifactFailureClass classifies a stage whose output could not become its
// artifact. A provider stage's schema violation is transient: a model that
// invents a field usually does not invent it again, and the second factory
// dogfood run parked a $25 order over one stray field
// ("pruned_bounded_context_note") that a retry would have dropped. The
// order's max-attempts still bounds the retries, and the rejected payload
// is still kept, so this is a recorded retry rather than a silent repair.
// An internal stage is deterministic, and its violation would repeat.
func artifactFailureClass(stage Stage, err error) FailureClass {
	if !stage.Internal && errors.Is(err, ErrInvalidState) {
		return FailureTransient
	}
	return FailureTerminal
}
