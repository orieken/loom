package claudehook

import (
	"errors"
	"slices"
	"testing"
)

func TestRegisteredEventsAreExactlyTheFourTheDecoderKnows(t *testing.T) {
	events := RegisteredEvents()
	want := []string{"PreToolUse", "PostToolUse", "UserPromptSubmit", "Stop"}
	if !slices.Equal(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for _, event := range events {
		if _, err := (Decoder{}).Decode(event, []byte(`{}`)); errors.Is(err, ErrUnknownHookEvent) {
			t.Errorf("the decoder does not know registered event %s", event)
		}
	}
}
