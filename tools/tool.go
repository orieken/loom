// Package tools is loom's public, transport-free embedding API (roadmap D.2).
// It defines the Tool abstraction and the Registry an external MCP server
// merges loom's framework tools into — with no mcp-go, jsonschema, or
// internal types in any exported signature, so consumers never inherit
// loom's dependency pins. TestPublicAPIImportsOnlyStdlib enforces that.
//
// Obtain the built-in framework registrations from
// shared/mcp/register.Frameworks and combine them with your own tools via
// Registry.Merge.
package tools

import (
	"context"
	"encoding/json"
)

// ToolRequest is a transport-free tool invocation.
type ToolRequest struct {
	Name string
	Args map[string]any
}

// StringArg returns the string argument named key, or "" when it is absent
// or not a string.
func (r ToolRequest) StringArg(key string) string {
	value, _ := r.Args[key].(string)
	return value
}

// ContentBlock is one piece of tool output. Text is the only content kind
// the framework tools emit today.
type ContentBlock struct {
	Text string
}

// ToolResult is a transport-free tool outcome. IsError marks a failure the
// calling LLM should repair (bad arguments, analysis failure); transport-level
// failures are returned as Go errors from Execute instead.
type ToolResult struct {
	Content []ContentBlock
	IsError bool
}

// NewTextResult wraps text in a successful single-block result.
func NewTextResult(text string) *ToolResult {
	return &ToolResult{Content: []ContentBlock{{Text: text}}}
}

// NewErrorResult wraps message in a single-block result flagged as an error.
func NewErrorResult(message string) *ToolResult {
	return &ToolResult{Content: []ContentBlock{{Text: message}}, IsError: true}
}

// Tool is the framework's first-class abstraction for every capability exposed
// over MCP. Implement it to contribute a capability; register it via
// Registry.Register. Schemas are raw JSON Schema documents; OutputSchema may
// return nil when the tool declares none.
type Tool interface {
	Name() string
	Description() string
	InputSchema() json.RawMessage
	OutputSchema() json.RawMessage
	Execute(ctx context.Context, request ToolRequest) (*ToolResult, error)
}

// SafeArguments is an optional interface a Tool may implement to declare
// which of its arguments may appear verbatim in telemetry (guardrail #9).
//
// It is opt-in because only the tool's author knows which of its arguments
// carry content. `text` is a harmless echo payload in one tool and a user's
// message in another, and no list maintained here could tell them apart —
// which matters because the registry is consumer-extensible (see
// examples/embedding). A tool that does not implement this has every
// argument value recorded as a hash and a length instead, so the safe
// default costs a value in a trace rather than leaking one.
//
// Declaring an argument safe is not a way to record a credential: an
// argument whose name looks secret is redacted regardless of what this
// returns.
type SafeArguments interface {
	// SafeArgumentNames returns the argument names whose values are
	// non-sensitive. Names not listed are hashed.
	SafeArgumentNames() []string
}
