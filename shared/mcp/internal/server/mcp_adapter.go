// mcp_adapter.go owns every conversion between the transport-free domain
// types and the mcp-go wire types. No other file in the module may translate
// between the two — domain stays stdlib-only (roadmap M0.3).
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/orieken/loom/internal/telemetry"
	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/tools"
)

// mcpToolDefinition converts a domain.Tool's metadata into the MCP wire type.
func mcpToolDefinition(tool domain.Tool) mcp.Tool {
	return mcp.Tool{
		Name:            tool.Name(),
		Description:     tool.Description(),
		RawInputSchema:  tool.InputSchema(),
		RawOutputSchema: tool.OutputSchema(),
	}
}

// mcpToolHandler adapts a registration's Execute into an mcp-go handler,
// wrapping each call in a span and a correlated log line. This is the only
// place tool-call telemetry is emitted; the tools themselves stay unaware
// of it, and `internal/domain` stays stdlib-only (guardrail #8 and M0.3).
//
// Every call runs through the registration's circuit breaker and retry
// (roadmap L2.6, middleware.go), each attempt under its declared Timeout
// (L2.2). The span covers the whole call, attempts included.
func (h *Handler) mcpToolHandler(registration domain.ToolRegistration) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	validator, err := compileArgumentValidator(registration.Tool)
	if err != nil {
		return func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) { return nil, err }
	}
	return h.validatedToolHandler(registration, validator)
}

// validatedToolHandler is mcpToolHandler with the argument validator already
// compiled. A call whose arguments break the schema never reaches Execute —
// nor the breaker, since a malformed call says nothing about the tool's
// health: it returns the field-level violations instead (roadmap L2.1).
func (h *Handler) validatedToolHandler(registration domain.ToolRegistration, validator *argumentValidator) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tool := registration.Tool
	resilient := newResilientTool(registration, h.resilience, h.logger)
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if violations := validator.violations(request.GetArguments()); len(violations) > 0 {
			return mcpResult(invalidArgumentsResult(tool.Name(), violations)), nil
		}
		call := telemetry.ToolCall{
			Name:          tool.Name(),
			Arguments:     stringArguments(request.GetArguments()),
			SafeArguments: safeArgumentNames(tool),
		}
		ctx, span := h.session.StartTool(ctx, call)
		h.logToolCall(ctx, tool.Name())
		outcome := resilient.call(ctx, domainRequest(tool.Name(), request))
		span.End(toolResult(outcome))
		if outcome.err != nil {
			return nil, outcome.err
		}
		return mcpResult(outcome.result), nil
	}
}

// logToolCall correlates the server's own logs with the trace, so a log
// line and a span can be joined without guessing from timestamps.
func (h *Handler) logToolCall(ctx context.Context, name string) {
	traceID, spanID := telemetry.TraceIDs(ctx)
	if traceID == "" {
		h.logger.Info("tool.called", "tool", name)
		return
	}
	h.logger.Info("tool.called", "tool", name, "trace_id", traceID, "span_id", spanID)
}

// safeArgumentNames asks the tool which of its arguments may be recorded
// verbatim. A tool that does not implement tools.SafeArguments declares
// nothing, and every value it is called with is hashed (guardrail #9).
func safeArgumentNames(tool domain.Tool) []string {
	declaring, ok := tool.(tools.SafeArguments)
	if !ok {
		return nil
	}
	return declaring.SafeArgumentNames()
}

func toolResult(outcome callOutcome) telemetry.ToolResult {
	recorded := telemetry.ToolResult{Err: outcome.err, Attempts: outcome.attempts, Breaker: outcome.breaker}
	if outcome.result == nil {
		return recorded
	}
	recorded.Bytes = resultBytes(outcome.result)
	recorded.Blocks = len(outcome.result.Content)
	recorded.IsError = outcome.result.IsError
	recorded.ErrorKind = boundedKind(failureKind(outcome.result))
	return recorded
}

// knownKinds are the kinds a span may name. A kind an embedder's tool
// invented is recorded as "other", keeping the attribute a closed set.
var knownKinds = map[domain.ErrorKind]bool{
	domain.ErrorValidation: true, domain.ErrorNotFound: true, domain.ErrorPermission: true,
	domain.ErrorTransient: true, domain.ErrorCancelled: true, domain.ErrorInternal: true,
}

func boundedKind(kind domain.ErrorKind) string {
	if kind == "" || knownKinds[kind] {
		return string(kind)
	}
	return "other"
}

// resultBytes sizes the result without assembling it: the span records how
// much came back, never what.
func resultBytes(result *domain.ToolResult) int {
	total := 0
	for _, block := range result.Content {
		total += len(block.Text)
	}
	return total
}

// stringArguments renders each argument for a span attribute. Values are
// stringified here rather than in the telemetry package so that package's
// signatures stay free of `any`, per go-conventions.
func stringArguments(args map[string]any) map[string]string {
	rendered := make(map[string]string, len(args))
	for key, value := range args {
		rendered[key] = renderArgument(value)
	}
	return rendered
}

// renderArgument keeps strings as themselves and JSON-encodes everything
// else, so a nested object arrives readable rather than as a Go %v dump.
func renderArgument(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(encoded)
}

func domainRequest(name string, request mcp.CallToolRequest) domain.ToolRequest {
	return domain.ToolRequest{Name: name, Args: request.GetArguments()}
}

func mcpResult(result *domain.ToolResult) *mcp.CallToolResult {
	if result == nil {
		return nil
	}
	content := make([]mcp.Content, 0, len(result.Content))
	for _, block := range result.Content {
		content = append(content, mcp.NewTextContent(block.Text))
	}
	return &mcp.CallToolResult{Content: content, IsError: result.IsError}
}

// withDeadline bounds ctx by timeout. A registration with no timeout runs
// under the caller's context alone.
func withDeadline(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}
