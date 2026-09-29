package telemetry_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/telemetry"
)

const secretQuery = "sync preferences for acme-corp payroll"

// traceRetrieval runs one search beneath a tool span and returns every span.
func traceRetrieval(t *testing.T, query telemetry.RetrievalQuery, result telemetry.RetrievalResult) []otlpSpan {
	t.Helper()
	path := filepath.Join(t.TempDir(), telemetry.TracesFileName)
	session, err := telemetry.Start(telemetry.Options{Version: "test-version", TraceFile: path})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	ctx, tool := session.StartTool(context.Background(), telemetry.ToolCall{Name: "search_features"})
	_, retrieval := telemetry.StartRetrieval(ctx, query)
	retrieval.End(result)
	tool.End(telemetry.ToolResult{})
	if err := session.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	return decodeSpans(t, path)
}

// L3.4: each search records a retrieval.queried event — what was found, and
// the query as properties only (guardrail #9).
func TestASearchRecordsARetrievalQueriedEventBeneathItsTool(t *testing.T) {
	t.Setenv(telemetry.SaltEnvVar, "test-salt")
	spans := traceRetrieval(t,
		telemetry.RetrievalQuery{Corpus: "project-features", Backend: "hybrid", Query: secretQuery},
		telemetry.RetrievalResult{Hits: 3, TopHit: "settings-persistence/README.md"})
	tool := findSpan(t, spans, "loom.tool search_features")
	span := findSpan(t, spans, "loom.retrieval project-features")
	if span.ParentSpanID != tool.SpanID {
		t.Errorf("retrieval span's parent = %q, want the tool span %q", span.ParentSpanID, tool.SpanID)
	}
	assertStringAttribute(t, span, "loom.retrieval.corpus", "project-features")
	assertStringAttribute(t, span, "loom.retrieval.backend", "hybrid")
	assertIntAttribute(t, span, "loom.retrieval.query.length", "38")
	if hash := span.attribute(t, "loom.retrieval.query.hash").Value.StringValue; hash == nil || !strings.HasPrefix(*hash, telemetry.HashPrefix) {
		t.Errorf("query hash = %v, want a salted digest", hash)
	}
	if len(span.Events) != 1 || span.Events[0].Name != telemetry.RetrievalQueriedEvent {
		t.Fatalf("events = %+v, want one retrieval.queried", span.Events)
	}
	event := otlpSpan{Attributes: span.Events[0].Attributes}
	assertIntAttribute(t, event, "loom.retrieval.hits", "3")
	assertStringAttribute(t, event, "loom.retrieval.outcome", "hit")
	assertStringAttribute(t, event, "loom.retrieval.top_hit", "settings-persistence/README.md")
	assertNoAttributeContains(t, span, "acme-corp")
	assertNoAttributeContains(t, event, "acme-corp")
}

// Zero hits is the "miss" ADR-002 names as the signal to act on; without a
// salt no hash is emitted, only the length.
func TestAMissIsRecordedAndNoSaltMeansNoHash(t *testing.T) {
	t.Setenv(telemetry.SaltEnvVar, "")
	span := findSpan(t, traceRetrieval(t,
		telemetry.RetrievalQuery{Corpus: "project-docs", Backend: "bm25", Query: secretQuery},
		telemetry.RetrievalResult{}), "loom.retrieval project-docs")
	event := otlpSpan{Attributes: span.Events[0].Attributes}
	assertStringAttribute(t, event, "loom.retrieval.outcome", "miss")
	if event.find("loom.retrieval.top_hit") != nil || span.find("loom.retrieval.query.hash") != nil {
		t.Errorf("a miss without a salt recorded a top hit or a hash: %+v %+v", event.Attributes, span.Attributes)
	}
}

func TestAFailedSearchIsAnErrorSpan(t *testing.T) {
	span := findSpan(t, traceRetrieval(t,
		telemetry.RetrievalQuery{Corpus: "project-docs", Backend: "bm25", Query: "q"},
		telemetry.RetrievalResult{Err: errors.New("store unreadable")}), "loom.retrieval project-docs")
	if span.Status.Code != otlpStatusError {
		t.Errorf("status = %d, want error", span.Status.Code)
	}
}

// An untraced server records nothing, and a search never starts a trace of
// its own.
func TestAnUntracedSearchRecordsNothing(t *testing.T) {
	ctx, span := telemetry.StartRetrieval(context.Background(), telemetry.RetrievalQuery{Corpus: "c", Query: "q"})
	if span != nil || ctx != context.Background() {
		t.Errorf("an untraced search opened a span: %v", span)
	}
	span.End(telemetry.RetrievalResult{Hits: 1}) // nil-safe
}
