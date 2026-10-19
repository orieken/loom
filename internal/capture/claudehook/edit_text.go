package claudehook

import (
	"encoding/json"

	"github.com/orieken/loom/internal/capture"
)

// Payload field names of the edit tools (UNVERIFIED, like every name in this
// package). They live in this file only, so a verified contract changes one
// spot.
//
// PRECONDITION: Edit sends tool_input.old_string and tool_input.new_string,
// and MultiEdit sends tool_input.edits, a list of such pairs.
// ENFORCED-BY: judgment-only (manual verification against installed Claude
// Code before release; open question 1 in analysis.md).
type editInput struct {
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

type editToolInput struct {
	editInput
	Edits []editInput `json:"edits"`
}

type editPayload struct {
	ToolInput editToolInput `json:"tool_input"`
}

var _ capture.EditTextSource = EditTexts{}

// EditTexts implements capture.EditTextSource for Claude Code. The Decoder
// drops every argument value on purpose. This type is the one door through
// which the text of an edit reaches the hook, and it hands the text over as
// a capture.EditText, a type that cannot be printed or stored.
type EditTexts struct{}

// EditTextOf reads the edits of one payload. A payload that does not parse,
// or that holds no edit, gives an empty EditText.
func (EditTexts) EditTextOf(payload []byte) capture.EditText {
	var in editPayload
	if err := json.Unmarshal(payload, &in); err != nil {
		return capture.EditText{}
	}
	text := capture.EditText{}
	for _, edit := range editsOf(in.ToolInput) {
		text = text.With(edit.OldString, edit.NewString)
	}
	return text
}

func editsOf(input editToolInput) []editInput {
	if len(input.Edits) > 0 {
		return input.Edits
	}
	if input.NewString == "" {
		return nil
	}
	return []editInput{input.editInput}
}
