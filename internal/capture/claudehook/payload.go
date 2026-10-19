// Package claudehook is the anti-corruption layer between the hook payload of
// Claude Code and the neutral capture.Observation. Every host-tool name
// lives in this one file, so a verified contract changes one spot.
//
// PRECONDITION: the hook event names (PreToolUse, PostToolUse,
// UserPromptSubmit, Stop) and the payload field names (session_id,
// tool_name, tool_input, file_path, prompt) match the installed Claude Code.
// They are UNVERIFIED in this repository. How a permission denial is
// reported is also UNVERIFIED, so no hook event maps to capture.EventToolDenied
// yet and the denied-call signal stays off.
// ENFORCED-BY: judgment-only (manual verification against installed Claude
// Code before release; open question 1 in analysis.md).
package claudehook

import (
	"encoding/json"
	"errors"
	"sort"

	"github.com/orieken/loom/internal/capture"
)

// Errors of the decoder. None of them carries payload text.
var (
	ErrUnknownHookEvent = errors.New("claudehook: unknown hook event")
	ErrPayloadTooLarge  = errors.New("claudehook: payload exceeds the size cap")
	ErrMalformedPayload = errors.New("claudehook: malformed payload")
)

// Hook event names of the host tool (UNVERIFIED).
const (
	hookPreToolUse       = "PreToolUse"
	hookPostToolUse      = "PostToolUse"
	hookUserPromptSubmit = "UserPromptSubmit"
	hookStop             = "Stop"
)

// Payload field names of the host tool (UNVERIFIED).
const filePathKey = "file_path"

// The value types stored beside an argument key.
const (
	typeObject = "object"
	typeArray  = "array"
	typeString = "string"
	typeBool   = "bool"
	typeNull   = "null"
	typeNumber = "number"
)

// typeByFirstByte maps the first byte of a raw JSON value to its type. Any
// other first byte (a digit or '-') is a number.
var typeByFirstByte = map[byte]string{
	'{': typeObject,
	'[': typeArray,
	'"': typeString,
	't': typeBool,
	'f': typeBool,
	'n': typeNull,
}

var _ capture.PayloadDecoder = Decoder{}

var eventKinds = map[string]capture.EventKind{
	hookPreToolUse:       capture.EventToolUse,
	hookPostToolUse:      capture.EventToolResult,
	hookUserPromptSubmit: capture.EventPrompt,
	hookStop:             capture.EventStop,
}

// payload lists the only fields the decoder reads. Unknown fields are
// ignored by encoding/json. ToolInput stays raw so a value is never decoded
// into a Go value: the decoder reads its first byte only.
type payload struct {
	SessionID string          `json:"session_id"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
	Prompt    string          `json:"prompt"`
}

// Decoder implements capture.PayloadDecoder for Claude Code.
type Decoder struct{}

// Decode turns one untrusted payload into an Observation. It keeps argument
// keys and types and drops every argument value. It keeps the prompt text in
// memory for the classifier and nowhere else. It never executes or
// interpolates a value.
func (Decoder) Decode(hookEvent string, raw []byte) (capture.Observation, error) {
	kind, isKnown := eventKinds[hookEvent]
	if !isKnown {
		return capture.Observation{}, ErrUnknownHookEvent
	}
	if len(raw) > capture.MaxStdinBytes {
		return capture.Observation{}, ErrPayloadTooLarge
	}
	var in payload
	if err := json.Unmarshal(raw, &in); err != nil {
		return capture.Observation{}, ErrMalformedPayload
	}
	return observationFrom(kind, in), nil
}

func observationFrom(kind capture.EventKind, in payload) capture.Observation {
	obs := capture.Observation{Kind: kind, Tool: in.ToolName, SessionID: in.SessionID}
	if kind == capture.EventPrompt {
		obs.Prompt = in.Prompt
		return obs
	}
	fields := objectFields(in.ToolInput)
	obs.ArgKeys, obs.ArgTypes = shapeOf(fields)
	obs.FilePath = stringValue(fields[filePathKey])
	return obs
}

// objectFields reads a JSON object into raw values. A value that is not an
// object gives no fields, because the input is untrusted.
func objectFields(input json.RawMessage) map[string]json.RawMessage {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(input, &fields); err != nil {
		return nil
	}
	return fields
}

// shapeOf returns the sorted keys and the type of each value. The type comes
// from the first byte of the raw value. Sorting makes the result stable.
func shapeOf(fields map[string]json.RawMessage) ([]string, []string) {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	types := make([]string, len(keys))
	for i, key := range keys {
		types[i] = jsonType(fields[key])
	}
	return keys, types
}

func jsonType(value json.RawMessage) string {
	if len(value) == 0 {
		return typeNull
	}
	if known, ok := typeByFirstByte[value[0]]; ok {
		return known
	}
	return typeNumber
}

// stringValue reads a JSON string, or returns "" for any other value.
func stringValue(value json.RawMessage) string {
	var text string
	if err := json.Unmarshal(value, &text); err != nil {
		return ""
	}
	return text
}
