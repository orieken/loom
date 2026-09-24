package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/orieken/loom/internal/telemetry"
	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
)

type stubTool struct {
	name         string
	description  string
	inputSchema  json.RawMessage
	outputSchema json.RawMessage
	result       *domain.ToolResult
	err          error
	gotRequest   domain.ToolRequest
}

func (s *stubTool) Name() string        { return s.name }
func (s *stubTool) Description() string { return s.description }

// InputSchema defaults to an unconstrained object: every MCP tool declares a
// schema, and the server compiles it before any call (L2.1).
func (s *stubTool) InputSchema() json.RawMessage {
	if s.inputSchema == nil {
		return json.RawMessage(`{"type":"object"}`)
	}
	return s.inputSchema
}
func (s *stubTool) OutputSchema() json.RawMessage { return s.outputSchema }

func (s *stubTool) Execute(_ context.Context, request domain.ToolRequest) (*domain.ToolResult, error) {
	s.gotRequest = request
	return s.result, s.err
}

func TestMCPToolDefinitionMapsMetadata(t *testing.T) {
	stub := &stubTool{
		name:         "stub_tool",
		description:  "a stub",
		inputSchema:  json.RawMessage(`{"type":"object"}`),
		outputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}

	definition := mcpToolDefinition(stub)

	if definition.Name != "stub_tool" || definition.Description != "a stub" {
		t.Errorf("metadata not mapped: got name=%q description=%q", definition.Name, definition.Description)
	}
	if string(definition.RawInputSchema) != `{"type":"object"}` {
		t.Errorf("input schema not mapped: got %s", definition.RawInputSchema)
	}
	if string(definition.RawOutputSchema) != `{"type":"object","properties":{}}` {
		t.Errorf("output schema not mapped: got %s", definition.RawOutputSchema)
	}
	if _, err := json.Marshal(definition); err != nil {
		t.Errorf("wire definition does not marshal: %v", err)
	}
}

func TestMCPToolHandlerConvertsRequestAndResult(t *testing.T) {
	tests := []struct {
		name        string
		result      *domain.ToolResult
		wantText    string
		wantIsError bool
	}{
		{"text result", domain.NewTextResult("hello"), "hello", false},
		{"error result", domain.NewErrorResult("bad input"), "bad input", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubTool{name: "stub_tool", result: tt.result}
			request := mcp.CallToolRequest{}
			request.Params.Arguments = map[string]any{"projectPath": "/tmp/x"}

			got, err := New(logging.NewLogger(&bytes.Buffer{})).mcpToolHandler(domain.ToolRegistration{Tool: stub})(context.Background(), request)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertDomainRequest(t, stub.gotRequest)
			assertTextResult(t, got, tt.wantText, tt.wantIsError)
		})
	}
}

func assertDomainRequest(t *testing.T, got domain.ToolRequest) {
	t.Helper()
	if got.Name != "stub_tool" {
		t.Errorf("request name not set: got %q", got.Name)
	}
	if got.StringArg("projectPath") != "/tmp/x" {
		t.Errorf("arguments not converted: got %v", got.Args)
	}
}

func assertTextResult(t *testing.T, got *mcp.CallToolResult, wantText string, wantIsError bool) {
	t.Helper()
	if got.IsError != wantIsError {
		t.Errorf("IsError = %v, want %v", got.IsError, wantIsError)
	}
	text, ok := got.Content[0].(mcp.TextContent)
	if !ok || text.Text != wantText {
		t.Errorf("content = %#v, want text %q", got.Content, wantText)
	}
}

func TestMCPToolHandlerPropagatesExecuteError(t *testing.T) {
	executeErr := errors.New("transport failure")
	stub := &stubTool{name: "stub_tool", err: executeErr}

	_, err := New(logging.NewLogger(&bytes.Buffer{})).mcpToolHandler(domain.ToolRegistration{Tool: stub})(context.Background(), mcp.CallToolRequest{})
	if !errors.Is(err, executeErr) {
		t.Errorf("expected execute error to propagate, got %v", err)
	}
}

func TestMCPResultNilStaysNil(t *testing.T) {
	if got := mcpResult(nil); got != nil {
		t.Errorf("expected nil, got %#v", got)
	}
}

// Tool telemetry must be invisible to the tools: a handler with no session
// still calls Execute and returns its result. Tracing is off by default and
// must never be load-bearing.
func TestToolHandlerWorksWithoutATelemetrySession(t *testing.T) {
	stub := &stubTool{name: "stub_tool", result: domain.NewTextResult("ok")}
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{"projectPath": "/tmp/x"}

	handler := New(logging.NewLogger(&bytes.Buffer{}))
	got, err := handler.mcpToolHandler(domain.ToolRegistration{Tool: stub})(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertTextResult(t, got, "ok", false)
}

// Log lines carry trace and span IDs when a trace is in flight, so a log
// and a span can be joined without guessing from timestamps.
func TestToolCallLogsCarryTraceCorrelation(t *testing.T) {
	const parent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	t.Setenv(telemetry.TraceParentEnvVar, parent)
	session, err := telemetry.Start(telemetry.Options{
		Version: "test", TraceFile: filepath.Join(t.TempDir(), telemetry.TracesFileName),
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	logs := &bytes.Buffer{}
	handler := New(logging.NewLogger(logs))
	handler.WithTracing(session)

	stub := &stubTool{name: "stub_tool", result: domain.NewTextResult("ok")}
	ctx := telemetry.ContextFromEnvironment(context.Background())
	if _, err := handler.mcpToolHandler(domain.ToolRegistration{Tool: stub})(ctx, mcp.CallToolRequest{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = session.Shutdown(context.Background())

	if !strings.Contains(logs.String(), "4bf92f3577b34da6a3ce929d0e0e4736") {
		t.Errorf("tool call log carries no trace id:\n%s", logs.String())
	}
}

// Untraced calls must still log, without empty correlation fields that
// would read as a trace that failed rather than one that never started.
func TestUntracedToolCallLogsWithoutEmptyCorrelationFields(t *testing.T) {
	logs := &bytes.Buffer{}
	handler := New(logging.NewLogger(logs))
	stub := &stubTool{name: "stub_tool", result: domain.NewTextResult("ok")}

	if _, err := handler.mcpToolHandler(domain.ToolRegistration{Tool: stub})(context.Background(), mcp.CallToolRequest{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(logs.String(), "trace_id") {
		t.Errorf("untraced call logged a trace_id field:\n%s", logs.String())
	}
	if !strings.Contains(logs.String(), "tool.called") {
		t.Errorf("untraced call did not log at all:\n%s", logs.String())
	}
}

// A tool declares which arguments may be recorded verbatim. stubTool does
// not implement tools.SafeArguments, which is the common case for a
// consumer's own tool — and the safe default is that none of its values
// reach a span (guardrail #9).
func TestSafeArgumentNamesIsEmptyForAToolThatDeclaresNothing(t *testing.T) {
	if names := safeArgumentNames(&stubTool{name: "echo"}); len(names) != 0 {
		t.Errorf("safeArgumentNames(undeclaring tool) = %v, want none", names)
	}
}

func TestSafeArgumentNamesReturnsWhatTheToolDeclares(t *testing.T) {
	names := safeArgumentNames(&declaringTool{stubTool{name: "search_ki"}})
	if len(names) != 1 || names[0] != "domain" {
		t.Errorf("safeArgumentNames = %v, want [domain]", names)
	}
}

type declaringTool struct{ stubTool }

func (*declaringTool) SafeArgumentNames() []string { return []string{"domain"} }

// Arguments arrive as `any` off the wire. Strings stay themselves; anything
// else is JSON-encoded so a nested object arrives readable rather than as a
// Go %v dump — which matters because the rendered value is what the span's
// length and hash are computed from.
func TestRenderArgumentEncodesNonStrings(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		value any
		want  string
	}{
		{"string passes through", "clean architecture", "clean architecture"},
		{"number", float64(7), "7"},
		{"bool", true, "true"},
		{"nested object", map[string]any{"a": 1}, `{"a":1}`},
		{"list", []any{"x", "y"}, `["x","y"]`},
		{"nil", nil, "null"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := renderArgument(testCase.value); got != testCase.want {
				t.Errorf("renderArgument(%v) = %q, want %q", testCase.value, got, testCase.want)
			}
		})
	}
}
