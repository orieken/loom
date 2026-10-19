package claudehook

import (
	"testing"

	"github.com/orieken/loom/internal/capture"
)

func TestEditTextsCountTheEditsOfAPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    int
	}{
		{name: "edit", payload: `{"tool_input":{"file_path":"a","old_string":"x","new_string":"y"}}`, want: 1},
		{name: "multi edit", payload: `{"tool_input":{"edits":[{"old_string":"a","new_string":"b"},{"old_string":"c","new_string":"d"}]}}`, want: 2},
		{name: "multi edit wins over single fields", payload: `{"tool_input":{"new_string":"z","edits":[{"old_string":"a","new_string":"b"}]}}`, want: 1},
		{name: "write has no edit", payload: `{"tool_input":{"file_path":"a","content":"body"}}`, want: 0},
		{name: "deletion has no new text", payload: `{"tool_input":{"old_string":"x","new_string":""}}`, want: 0},
		{name: "not json", payload: `{"tool_input":`, want: 0},
		{name: "tool input is a string", payload: `{"tool_input":"x"}`, want: 0},
		{name: "empty", payload: ``, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := EditTexts{}.EditTextOf([]byte(tt.payload))

			if text.Len() != tt.want {
				t.Fatalf("Len = %d, want %d", text.Len(), tt.want)
			}
		})
	}
}

// TestEditTextsHandOverTheTextToTheLocatorOnly proves the text arrives in the
// locator: the hunk is found from the text, though the text is not readable
// from the EditText.
func TestEditTextsHandOverTheTextToTheLocatorOnly(t *testing.T) {
	salt, _ := capture.NewSalt([]byte("0123456789abcdef0123456789abcdef"))
	hasher := capture.NewHasher(salt)
	content := []byte("a\nneedle\nb\n")
	text := EditTexts{}.EditTextOf([]byte(`{"tool_input":{"old_string":"o","new_string":"needle"}}`))

	found := capture.NewHunkLocator(hasher).LocateAll(content, capture.SplitLines(content), text)

	if len(found) != 1 || found[0].RangeStart != 2 {
		t.Fatalf("found %+v, want one hunk at line 2", found)
	}
}
