package telemetry

// The retrieval.queried event (roadmap L3.4, ADR-002). ADR-002 declared
// retrieval quality a judgment-only fitness function whose evidence would be
// a log of retrieval events; nothing ever emitted one, so retrieval quality
// was unmeasured. Each corpus search now records what it was asked — as
// properties, never the text — and what it found.
//
// It is a span beneath the tool call, with the event on it, rather than an
// entry in the executor's event vocabulary: the searches run in the MCP
// server's process, not the executor's, and the vocabulary holds only what
// the executor emits (L3.9). TRACEPARENT carries the span back to the run.

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// RetrievalQueriedEvent names the event a corpus search records.
const RetrievalQueriedEvent = "retrieval.queried"

// RetrievalQuery is one search of one corpus.
type RetrievalQuery struct {
	// Corpus names the corpus searched, e.g. "project-features".
	Corpus string
	// Backend names how it was searched: bm25, vector, hybrid.
	Backend string
	// Query is the caller's text. It is never recorded — only its length and,
	// with a salt configured, its salted hash (guardrail #9).
	Query string
}

// RetrievalResult is what a search found.
type RetrievalResult struct {
	Hits int
	// TopHit is the best reference's path relative to the corpus root: an
	// absolute path would carry the user's home directory into the span.
	TopHit string
	Err    error
}

// RetrievalSpan is an open retrieval span. A nil one is valid and records
// nothing, as for an untraced call.
type RetrievalSpan struct {
	span trace.Span
}

// StartRetrieval opens a retrieval span beneath the span in ctx. With no
// span in ctx — an untraced server — it records nothing, so a search never
// starts a trace of its own.
func StartRetrieval(ctx context.Context, query RetrievalQuery) (context.Context, *RetrievalSpan) {
	parent := trace.SpanFromContext(ctx)
	if !parent.SpanContext().IsValid() {
		return ctx, nil
	}
	ctx, span := parent.TracerProvider().Tracer(ScopeName).Start(ctx, "loom.retrieval "+query.Corpus,
		trace.WithAttributes(retrievalQueryAttributes(query)...))
	return ctx, &RetrievalSpan{span: span}
}

func retrievalQueryAttributes(query RetrievalQuery) []attribute.KeyValue {
	attributes := []attribute.KeyValue{
		attribute.String("loom.retrieval.corpus", query.Corpus),
		attribute.String("loom.retrieval.backend", query.Backend),
		attribute.Int("loom.retrieval.query.length", len([]rune(query.Query))),
	}
	if digest := HashValue(query.Query); digest != "" {
		attributes = append(attributes, attribute.String("loom.retrieval.query.hash", digest))
	}
	return attributes
}

// End records the retrieval.queried event and closes the span.
func (s *RetrievalSpan) End(result RetrievalResult) {
	if s == nil {
		return
	}
	s.span.AddEvent(RetrievalQueriedEvent, trace.WithAttributes(retrievalResultAttributes(result)...))
	if result.Err != nil {
		s.span.RecordError(result.Err)
		s.span.SetStatus(codes.Error, "retrieval failed")
	} else {
		s.span.SetStatus(codes.Ok, "")
	}
	s.span.End()
}

// retrievalResultAttributes: outcome is "miss" for zero hits — the signal
// ADR-002 names for a corpus that should graduate, or a missing document.
func retrievalResultAttributes(result RetrievalResult) []attribute.KeyValue {
	outcome := "hit"
	if result.Hits == 0 {
		outcome = "miss"
	}
	attributes := []attribute.KeyValue{
		attribute.Int("loom.retrieval.hits", result.Hits),
		attribute.String("loom.retrieval.outcome", outcome),
	}
	if result.TopHit != "" {
		attributes = append(attributes, attribute.String("loom.retrieval.top_hit", result.TopHit))
	}
	return attributes
}
