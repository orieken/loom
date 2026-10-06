package repodoc

import (
	"fmt"

	"github.com/orieken/loom/internal/orchestrator"
)

// callOutcome describes how a model call ended, with what the provider
// reported it consumed. Usage rides on a failed call too: it was billed.
func callOutcome(output orchestrator.StageOutput, err error) orchestrator.SpanOutcome {
	if err != nil {
		return orchestrator.SpanOutcome{Status: orchestrator.StageStatusFailed, Err: err, Usage: output.Usage}
	}
	return orchestrator.SpanOutcome{Status: orchestrator.StageStatusCompleted, Usage: output.Usage}
}

// runOutcome describes a whole documentation run. The reason carries the
// counts a trace reader asks for first: how many directories ended where.
func runOutcome(record RunRecord, usage *orchestrator.Usage) orchestrator.SpanOutcome {
	outcome := orchestrator.SpanOutcome{
		Status: orchestrator.StageStatusCompleted, Usage: usage,
		Reason: fmt.Sprintf("%d verified, %d unverified, %d failed", record.Count(StatusVerified), record.Unverified(), record.Failed()),
	}
	switch {
	case record.Interrupted, record.StoppedBecause != "":
		outcome.Status = orchestrator.StageStatusInterrupted
	case record.Failed()+record.Unverified() > 0:
		outcome.Status = orchestrator.StageStatusFailed
	}
	return outcome
}
