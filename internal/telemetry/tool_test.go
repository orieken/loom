package telemetry_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/telemetry"
)

// traceTool drives one tool call and returns the spans it produced.
func traceTool(ctx context.Context, t *testing.T, call telemetry.ToolCall, result telemetry.ToolResult) []otlpSpan {
	t.Helper()
	path := filepath.Join(t.TempDir(), telemetry.TracesFileName)
	session, err := telemetry.Start(telemetry.Options{Version: "test-version", TraceFile: path})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	_, span := session.StartTool(ctx, call)
	span.End(result)
	if err := session.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	return decodeSpans(t, path)
}

func stageSpanForTest() orchestrator.StageSpan {
	return orchestrator.StageSpan{ID: "developer", Agent: "developer"}
}

func toolSpan(t *testing.T, spans []otlpSpan) otlpSpan {
	t.Helper()
	return findSpan(t, spans, "loom.tool search_ki")
}

// A secret-shaped argument name must never put its value in a span. The key
// survives — knowing a token was passed is useful, knowing which is not.
func TestSecretArgumentsAreRedactedByName(t *testing.T) {
	secrets := []string{"apiKey", "API_KEY", "authToken", "password", "db_credential", "sessionId", "Authorization"}
	arguments := map[string]string{"query": "clean architecture"}
	for _, key := range secrets {
		arguments[key] = "super-secret-value"
	}

	span := toolSpan(t, traceTool(context.Background(), t,
		telemetry.ToolCall{Name: "search_ki", Arguments: arguments,
			SafeArguments: []string{"query"}},
		telemetry.ToolResult{Bytes: 9, Blocks: 1}))

	assertNoAttributeContains(t, span, "super-secret-value")
	for _, key := range secrets {
		assertStringAttribute(t, span, "loom.tool.arg."+key, telemetry.RedactedPlaceholder)
	}
	// A declared-safe argument must still come through intact, or redaction
	// has been bought at the price of the span being useless.
	assertStringAttribute(t, span, "loom.tool.arg.query", "clean architecture")
}

func assertNoAttributeContains(t *testing.T, span otlpSpan, unwanted string) {
	t.Helper()
	for _, attr := range span.Attributes {
		if attr.Value.StringValue != nil && strings.Contains(*attr.Value.StringValue, unwanted) {
			t.Errorf("attribute %q leaked %q", attr.Key, unwanted)
		}
	}
}

func assertStringAttribute(t *testing.T, span otlpSpan, key, want string) {
	t.Helper()
	got := span.attribute(t, key).Value.StringValue
	if got == nil || *got != want {
		t.Errorf("%s = %v, want %q", key, got, want)
	}
}

// Oversized values are truncated with a marker, not dropped: a reader must
// be able to tell truncation from a genuinely short value.
func TestOversizedValuesAreTruncatedWithAMarker(t *testing.T) {
	long := strings.Repeat("x", telemetry.AttributeValueLimit*3)

	span := toolSpan(t, traceTool(context.Background(), t,
		telemetry.ToolCall{Name: "search_ki", Arguments: map[string]string{"query": long},
			SafeArguments: []string{"query"}},
		telemetry.ToolResult{Bytes: len(long), Blocks: 1}))

	for _, key := range []string{"loom.tool.arg.query"} {
		value := span.attribute(t, key).Value.StringValue
		if value == nil {
			t.Fatalf("%s missing", key)
		}
		if !strings.HasSuffix(*value, telemetry.TruncationMarker) {
			t.Errorf("%s was not marked as truncated", key)
		}
		if len([]rune(*value)) > telemetry.AttributeValueLimit+len([]rune(telemetry.TruncationMarker)) {
			t.Errorf("%s is %d runes, over the cap", key, len([]rune(*value)))
		}
	}
}

// Truncation must cut on a rune boundary — a byte-wise cut through a
// multi-byte character would put invalid UTF-8 into a span.
func TestTruncationDoesNotSplitAMultiByteCharacter(t *testing.T) {
	span := toolSpan(t, traceTool(context.Background(), t,
		telemetry.ToolCall{Name: "search_ki", Arguments: map[string]string{
			"query": strings.Repeat("日", telemetry.AttributeValueLimit),
		}, SafeArguments: []string{"query"}},
		telemetry.ToolResult{}))

	value := span.attribute(t, "loom.tool.arg.query").Value.StringValue
	if value == nil {
		t.Fatal("query attribute missing")
	}
	trimmed := strings.TrimSuffix(*value, telemetry.TruncationMarker)
	if !strings.HasPrefix(trimmed, "日") || strings.ContainsRune(trimmed, '�') {
		t.Errorf("truncation produced invalid UTF-8: %q", trimmed)
	}
}

// Propagation is best-effort. With TRACEPARENT set, the tool call joins the
// caller's trace.
func TestToolCallAdoptsAnInheritedTraceParent(t *testing.T) {
	const parent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	t.Setenv(telemetry.TraceParentEnvVar, parent)

	ctx := telemetry.ContextFromEnvironment(context.Background())
	span := toolSpan(t, traceTool(ctx, t,
		telemetry.ToolCall{Name: "search_ki"}, telemetry.ToolResult{}))

	if span.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("traceId = %q, want the inherited trace", span.TraceID)
	}
	if span.ParentSpanID != "00f067aa0ba902b7" {
		t.Errorf("parentSpanId = %q, want the inherited span", span.ParentSpanID)
	}
}

// Without it, the tool call starts a clean trace of its own rather than
// failing or inventing a parent. This is the common case: loom does not
// spawn the MCP server and cannot guarantee the variable survives.
func TestToolCallWithoutATraceParentStartsItsOwnTrace(t *testing.T) {
	t.Setenv(telemetry.TraceParentEnvVar, "")

	span := toolSpan(t, traceTool(telemetry.ContextFromEnvironment(context.Background()), t,
		telemetry.ToolCall{Name: "search_ki"}, telemetry.ToolResult{}))

	if span.ParentSpanID != "" {
		t.Errorf("parentSpanId = %q, want no parent", span.ParentSpanID)
	}
	if span.TraceID == "" {
		t.Error("tool call produced no trace of its own")
	}
}

// A malformed TRACEPARENT from somewhere upstream must not stop a tool from
// running, and must not produce a span claiming a parent it does not have.
func TestMalformedTraceParentIsIgnored(t *testing.T) {
	t.Setenv(telemetry.TraceParentEnvVar, "not-a-valid-traceparent")

	span := toolSpan(t, traceTool(telemetry.ContextFromEnvironment(context.Background()), t,
		telemetry.ToolCall{Name: "search_ki"}, telemetry.ToolResult{}))

	if span.ParentSpanID != "" {
		t.Errorf("parentSpanId = %q, want none from an unparseable value", span.ParentSpanID)
	}
}

// TraceParentEnv is the export side of the same hop.
func TestTraceParentEnvRoundTripsThroughTheEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), telemetry.TracesFileName)
	session, err := telemetry.Start(telemetry.Options{Version: "test-version", TraceFile: path})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	ctx, _ := session.Tracer().StartStage(context.Background(), stageSpanForTest())

	entries := telemetry.TraceParentEnv(ctx)
	if len(entries) != 1 || !strings.HasPrefix(entries[0], telemetry.TraceParentEnvVar+"=00-") {
		t.Fatalf("TraceParentEnv = %v, want one W3C traceparent entry", entries)
	}
	// Feed it back the way a child process would receive it.
	value := strings.TrimPrefix(entries[0], telemetry.TraceParentEnvVar+"=")
	t.Setenv(telemetry.TraceParentEnvVar, value)
	if got := telemetry.ContextFromEnvironment(context.Background()); got == nil {
		t.Fatal("ContextFromEnvironment returned nil")
	}
	_ = os.Unsetenv(telemetry.TraceParentEnvVar)
}

// With no recording span there is nothing to propagate, and the caller must
// get nothing rather than an empty or malformed entry.
func TestTraceParentEnvIsEmptyWithoutASpan(t *testing.T) {
	if entries := telemetry.TraceParentEnv(context.Background()); entries != nil {
		t.Errorf("TraceParentEnv = %v, want nil with no span in context", entries)
	}
}

// The guardrail #9 headline: an argument the tool did not declare safe never
// reaches a span in any recoverable form. `query` is the live case — search_ki
// and search_docs both take one, and before roadmap L3.38 it was exported
// verbatim along with 512 characters of whatever KI body came back.
func TestUndeclaredArgumentsNeverAppearAsContent(t *testing.T) {
	t.Setenv(telemetry.SaltEnvVar, "a-salt")
	const secret = "how do I rotate the production database password"

	span := toolSpan(t, traceTool(context.Background(), t,
		telemetry.ToolCall{Name: "search_ki", Arguments: map[string]string{"query": secret}},
		telemetry.ToolResult{Bytes: 4096, Blocks: 3}))

	assertNoAttributeContains(t, span, secret)
	assertNoAttributeContains(t, span, "rotate")
	if attr := span.find("loom.tool.arg.query"); attr != nil {
		t.Errorf("undeclared argument was recorded verbatim as %v", attr.Value)
	}
	// The properties survive, or the span stops being diagnostic.
	assertIntAttribute(t, span, "loom.tool.arg.query.length", strconv.Itoa(len([]rune(secret))))
	if digest := span.attribute(t, "loom.tool.arg.query.hash").Value.StringValue; digest == nil ||
		!strings.HasPrefix(*digest, telemetry.HashPrefix) {
		t.Errorf("hash attribute = %v, want a %s digest", digest, telemetry.HashPrefix)
	}
}

// A salted hash is only worth recording if it correlates. The same input under
// the same salt must produce the same digest, and a different salt must not —
// otherwise one deployment's traces could be matched against another's.
func TestHashesCorrelateUnderOneSaltAndNotAcrossTwo(t *testing.T) {
	t.Setenv(telemetry.SaltEnvVar, "salt-one")
	first := telemetry.HashValue("clean architecture")
	again := telemetry.HashValue("clean architecture")
	t.Setenv(telemetry.SaltEnvVar, "salt-two")
	elsewhere := telemetry.HashValue("clean architecture")

	if first == "" || first != again {
		t.Errorf("same salt gave %q then %q, want a stable digest", first, again)
	}
	if first == elsewhere {
		t.Error("digest survived a salt change, so the salt is doing nothing")
	}
}

// No salt means no hash — never an unsalted one. A short query has little
// entropy and an unsalted digest of it is recoverable by brute force, so the
// choice is a salted hash or nothing. The length still goes on the span,
// because it leaks nothing and truncation is diagnosable without it.
func TestWithoutASaltNoHashIsEmittedAndTheLengthStillIs(t *testing.T) {
	t.Setenv(telemetry.SaltEnvVar, "")

	span := toolSpan(t, traceTool(context.Background(), t,
		telemetry.ToolCall{Name: "search_ki", Arguments: map[string]string{"query": "dark mode"}},
		telemetry.ToolResult{}))

	if attr := span.find("loom.tool.arg.query.hash"); attr != nil {
		t.Errorf("emitted a hash with no salt configured: %v", attr.Value)
	}
	assertIntAttribute(t, span, "loom.tool.arg.query.length", "9")
	assertNoAttributeContains(t, span, "dark mode")
}

// A tool cannot opt its own credentials into a span. Declaring a
// secret-shaped name safe is either a mistake or an attack, and the name
// check wins either way.
func TestDeclaringASecretArgumentSafeDoesNotRecordIt(t *testing.T) {
	span := toolSpan(t, traceTool(context.Background(), t,
		telemetry.ToolCall{
			Name:          "search_ki",
			Arguments:     map[string]string{"apiToken": "super-secret-value"},
			SafeArguments: []string{"apiToken"},
		}, telemetry.ToolResult{}))

	assertNoAttributeContains(t, span, "super-secret-value")
	assertStringAttribute(t, span, "loom.tool.arg.apiToken", telemetry.RedactedPlaceholder)
}

// The result records its shape and never its text. Zero blocks on a
// successful call is the "found nothing" signal — the retrieval.hit_count
// equivalent, and the one worth alerting on.
func TestResultRecordsShapeNotText(t *testing.T) {
	span := toolSpan(t, traceTool(context.Background(), t,
		telemetry.ToolCall{Name: "search_ki"},
		telemetry.ToolResult{Bytes: 2048, Blocks: 0}))

	assertIntAttribute(t, span, "loom.tool.result.bytes", "2048")
	assertIntAttribute(t, span, "loom.tool.result.blocks", "0")
	if attr := span.find("loom.tool.result"); attr != nil {
		t.Errorf("result text attribute survived: %v", attr.Value)
	}
}

// otlpStatusError is STATUS_CODE_ERROR in the OTLP/JSON encoding.
const otlpStatusError = 2

// TraceIDs is how a log line carries correlation IDs without the logging
// package learning what OpenTelemetry is. Both branches matter: an untraced
// call must yield empty strings rather than a zero-valued ID that looks real.
func TestTraceIDsReportsTheActiveSpanAndNothingWithoutOne(t *testing.T) {
	if traceID, spanID := telemetry.TraceIDs(context.Background()); traceID != "" || spanID != "" {
		t.Errorf("TraceIDs(no span) = %q/%q, want empty", traceID, spanID)
	}

	path := filepath.Join(t.TempDir(), telemetry.TracesFileName)
	session, err := telemetry.Start(telemetry.Options{Version: "test-version", TraceFile: path})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = session.Shutdown(context.Background()) }()

	ctx, span := session.StartTool(context.Background(), telemetry.ToolCall{Name: "search_ki"})
	defer span.End(telemetry.ToolResult{})

	traceID, spanID := telemetry.TraceIDs(ctx)
	if len(traceID) != 32 {
		t.Errorf("traceID = %q, want 32 hex characters", traceID)
	}
	if len(spanID) != 16 {
		t.Errorf("spanID = %q, want 16 hex characters", spanID)
	}
}

// A tool that ran and reported failure is distinct from one that failed to
// run at all, and the span records both as errors while keeping the
// distinction in is_error.
func TestToolStatusSeparatesAToolErrorFromATransportError(t *testing.T) {
	reported := toolSpan(t, traceTool(context.Background(), t,
		telemetry.ToolCall{Name: "search_ki"},
		telemetry.ToolResult{IsError: true, Bytes: 12, Blocks: 1}))
	if reported.Status.Code != otlpStatusError {
		t.Errorf("status = %d, want error (%d) for a tool that reported failure", reported.Status.Code, otlpStatusError)
	}

	failed := toolSpan(t, traceTool(context.Background(), t,
		telemetry.ToolCall{Name: "search_ki"},
		telemetry.ToolResult{Err: errors.New("transport exploded")}))
	if failed.Status.Code != otlpStatusError {
		t.Errorf("status = %d, want error (%d) for a transport failure", failed.Status.Code, otlpStatusError)
	}
}

// A nil session must be usable: the MCP server runs untraced far more often
// than traced, and its call sites should not each branch on that.
func TestNilSessionTracesNothingAndDoesNotPanic(t *testing.T) {
	var session *telemetry.Session
	ctx, span := session.StartTool(context.Background(), telemetry.ToolCall{Name: "search_ki"})
	if ctx == nil {
		t.Fatal("StartTool on a nil session returned a nil context")
	}
	span.End(telemetry.ToolResult{Bytes: 1, Blocks: 1})
}
