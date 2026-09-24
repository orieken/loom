package tools

import (
	"encoding/json"
	"fmt"
)

// ErrorKind classifies a failed tool call by what the caller should do next
// (roadmap L2.5). Every failure used to be a prose string: a caller could not
// tell "fix the arguments and retry" from "stop" from "back off and retry".
type ErrorKind string

const (
	// ErrorValidation: the arguments are wrong. Fix them and call again.
	ErrorValidation ErrorKind = "validation"
	// ErrorNotFound: something the call needs does not exist — a path, a
	// corpus, an index. Retrying the same call will not help.
	ErrorNotFound ErrorKind = "not_found"
	// ErrorPermission: the call was refused, e.g. a path outside the
	// workspace root. Do not retry.
	ErrorPermission ErrorKind = "permission"
	// ErrorTransient: may succeed later — a deadline passed. The only kind
	// that is retryable, after backing off.
	ErrorTransient ErrorKind = "transient"
	// ErrorCancelled: the caller stopped the call. Not a failure of the tool,
	// and not something to retry automatically: the caller chose to stop.
	ErrorCancelled ErrorKind = "cancelled"
	// ErrorInternal: a defect in the tool. Report it; do not retry.
	ErrorInternal ErrorKind = "internal"
)

// FieldViolation is one argument that failed validation.
type FieldViolation struct {
	Field   string `json:"field"`
	Problem string `json:"problem"`
}

// ToolError is a typed tool failure. Retryable is derived from Kind, so a
// caller's retry policy and the kind can never disagree.
type ToolError struct {
	Kind       ErrorKind        `json:"kind"`
	Message    string           `json:"message"`
	Field      string           `json:"field,omitempty"`
	Retryable  bool             `json:"retryable"`
	Violations []FieldViolation `json:"violations,omitempty"`
}

// NewToolError builds a ToolError of kind with message.
func NewToolError(kind ErrorKind, message string) ToolError {
	return ToolError{Kind: kind, Message: message, Retryable: kind == ErrorTransient}
}

// WithField names the argument the failure concerns.
func (e ToolError) WithField(field string) ToolError {
	e.Field = field
	return e
}

// Error satisfies the error interface.
func (e ToolError) Error() string {
	if e.Field == "" {
		return fmt.Sprintf("%s: %s", e.Kind, e.Message)
	}
	return fmt.Sprintf("%s: %s: %s", e.Kind, e.Field, e.Message)
}

// toolErrorEnvelope is the stable wire shape: {"error": {...}}.
type toolErrorEnvelope struct {
	Error ToolError `json:"error"`
}

// NewToolErrorResult is the failed outcome of a call: IsError set, the typed
// error on Error for Go callers, and the same error as a JSON envelope in the
// content for MCP clients. A failure is never encoded into a success payload.
func NewToolErrorResult(toolError ToolError) *ToolResult {
	// Strings, a bool and a slice of string pairs: this cannot fail to encode.
	body, _ := json.Marshal(toolErrorEnvelope{Error: toolError})
	return &ToolResult{Content: []ContentBlock{{Text: string(body)}}, IsError: true, Error: &toolError}
}
