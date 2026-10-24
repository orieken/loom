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
	// artifact that broke its contract, an input that cannot be projected.
	FailureTerminal FailureClass = "terminal"
)

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
