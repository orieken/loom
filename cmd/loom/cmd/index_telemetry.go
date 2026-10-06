package cmd

// Telemetry for indexing and documentation (ADR-010, Phase 8). Both use the
// same span vocabulary `loom run` does (roadmap L3.8), so a trace viewer reads
// them with no new knowledge: a root span per run, a span per stage, and for
// model calls a provider span carrying the tokens and cost the provider
// reported.

import (
	"context"
	"fmt"
	"io"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/repodoc"
	"github.com/orieken/loom/internal/repograph"
	"github.com/orieken/loom/internal/repoindex"
	"github.com/orieken/loom/internal/telemetry"
)

// indexStageID names the one stage of an indexing run.
const indexStageID = "index"

// startToolTelemetry opens a tracing session and returns its tracer and the
// shutdown to defer. traceFile may be empty, which leaves only the network
// exporter (OTEL_EXPORTER_OTLP_ENDPOINT); with neither, the tracer is nil and
// nothing is recorded. Problems go to errOut: telemetry never fails the work it
// observes.
func startToolTelemetry(errOut io.Writer, traceFile string) (orchestrator.Tracer, func()) {
	session, err := telemetry.Start(telemetry.Options{Version: version, TraceFile: traceFile})
	if err != nil {
		fmt.Fprintf(errOut, "telemetry: disabled: %v\n", err)
		return nil, func() {}
	}
	return session.Tracer(), func() {
		ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
		defer cancel()
		if err := session.Shutdown(ctx); err != nil {
			fmt.Fprintf(errOut, "telemetry: flush failed: %v\n", err)
		}
	}
}

// tracedIndex runs indexing inside a run span with one internal stage. There is
// no model call, so there is no provider span and no tokens: the spans record
// how long indexing took and what it found.
func tracedIndex(ctx context.Context, tracer orchestrator.Tracer, root string) (repoindex.Result, error) {
	tracer = orchestrator.OrNoop(tracer)
	ctx, runSpan := tracer.StartRun(ctx, orchestrator.RunSpan{Plan: repodoc.PlanName, SpecPath: root})
	_, stageSpan := tracer.StartStage(ctx, orchestrator.StageSpan{ID: indexStageID, Sequence: 1, Iteration: 1, Internal: true})
	result, err := repoindex.Run(root, repograph.GitLister{}, nowFunc)
	outcome := indexOutcome(result, err)
	stageSpan.End(outcome)
	runSpan.End(outcome)
	return result, err
}

func indexOutcome(result repoindex.Result, err error) orchestrator.SpanOutcome {
	if err != nil {
		return orchestrator.SpanOutcome{Status: orchestrator.StageStatusFailed, Err: err}
	}
	graph := result.Graph.Graph
	return orchestrator.SpanOutcome{
		Status: orchestrator.StageStatusCompleted,
		Reason: fmt.Sprintf("%d nodes, %d edges, %d fact sheets, %d index entries", len(graph.Nodes), len(graph.Edges), len(result.Sheets), len(result.Index.Entries)),
	}
}
