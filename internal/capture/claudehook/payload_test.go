package claudehook_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/capture"
	"github.com/orieken/loom/internal/capture/claudehook"
)

func decode(t *testing.T, hookEvent, payload string) capture.Observation {
	t.Helper()
	obs, err := (claudehook.Decoder{}).Decode(hookEvent, []byte(payload))
	if err != nil {
		t.Fatalf("Decode(%s): %v", hookEvent, err)
	}
	return obs
}

func TestDecodeMapsHookEventsToEventKinds(t *testing.T) {
	tests := map[string]capture.EventKind{
		"PreToolUse":       capture.EventToolUse,
		"PostToolUse":      capture.EventToolResult,
		"UserPromptSubmit": capture.EventPrompt,
		"Stop":             capture.EventStop,
	}
	for hookEvent, want := range tests {
		t.Run(hookEvent, func(t *testing.T) {
			if got := decode(t, hookEvent, `{}`).Kind; got != want {
				t.Errorf("Kind = %q, want %q", got, want)
			}
		})
	}
}

func TestDecodeKeepsKeysAndTypesAndDropsValues(t *testing.T) {
	payload := `{"session_id":"s1","tool_name":"Edit","tool_input":{
		"new_string":"NEWVALUE","old_string":"OLDVALUE","file_path":"/proj/a.go",
		"count":3,"flag":true,"off":false,"none":null,"nested":{"k":"v"},"list":[1]}}`

	obs := decode(t, "PostToolUse", payload)

	wantKeys := []string{"count", "file_path", "flag", "list", "nested", "new_string", "none", "off", "old_string"}
	wantTypes := []string{"number", "string", "bool", "array", "object", "string", "null", "bool", "string"}
	if !reflect.DeepEqual(obs.ArgKeys, wantKeys) || !reflect.DeepEqual(obs.ArgTypes, wantTypes) {
		t.Errorf("shape = %v / %v\nwant    %v / %v", obs.ArgKeys, obs.ArgTypes, wantKeys, wantTypes)
	}
	if obs.Tool != "Edit" || obs.SessionID != "s1" || obs.FilePath != "/proj/a.go" {
		t.Errorf("obs = %+v", obs)
	}
}

func TestDecodeDropsEveryArgumentValue(t *testing.T) {
	payload := `{"tool_name":"Edit","tool_input":{"new_string":"NEWVALUE","old_string":"OLDVALUE","content":"BODYVALUE"}}`

	shown := fmt.Sprintf("%+v", decode(t, "PostToolUse", payload))

	for _, value := range []string{"NEWVALUE", "OLDVALUE", "BODYVALUE"} {
		if strings.Contains(shown, value) {
			t.Errorf("argument value %s survived: %s", value, shown)
		}
	}
}

func TestDecodeKeepsPromptTextOnlyForAPromptEvent(t *testing.T) {
	prompt := decode(t, "UserPromptSubmit", `{"prompt":"no, do it","tool_input":{"a":1}}`)
	tool := decode(t, "PreToolUse", `{"prompt":"leaked text","tool_name":"Read"}`)

	if prompt.Prompt != "no, do it" || len(prompt.ArgKeys) != 0 {
		t.Errorf("prompt obs = %+v", prompt)
	}
	if tool.Prompt != "" {
		t.Errorf("a tool event must not keep prompt text, got %q", tool.Prompt)
	}
}

func TestDecodeIgnoresUnknownFieldsAndTreatsOddInputAsEmpty(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		wantKey int
		wantTo  string
	}{
		{"unknown fields", `{"tool_name":"Read","surprise":{"x":1},"tool_input":{"a":"b"}}`, 1, ""},
		{"tool input is a string", `{"tool_input":"rm -rf /"}`, 0, ""},
		{"tool input is null", `{"tool_input":null}`, 0, ""},
		{"file path is a number", `{"tool_input":{"file_path":42}}`, 1, ""},
		{"file path is an object", `{"tool_input":{"file_path":{"a":1}}}`, 1, ""},
		{"no tool input", `{"tool_name":"Read"}`, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs := decode(t, "PostToolUse", tt.payload)
			if len(obs.ArgKeys) != tt.wantKey || obs.FilePath != tt.wantTo {
				t.Errorf("keys=%v path=%q", obs.ArgKeys, obs.FilePath)
			}
		})
	}
}

func TestDecodeRejectsBadInputWithoutEchoingIt(t *testing.T) {
	tooLarge := make([]byte, capture.MaxStdinBytes+1)
	tests := []struct {
		name      string
		hookEvent string
		payload   []byte
		wantErr   error
	}{
		{"unknown hook event", "Surprise", []byte(`{}`), claudehook.ErrUnknownHookEvent},
		{"empty hook event", "", []byte(`{}`), claudehook.ErrUnknownHookEvent},
		{"over the size cap", "PreToolUse", tooLarge, claudehook.ErrPayloadTooLarge},
		{"not json", "PreToolUse", []byte(`CANARY-garbage`), claudehook.ErrMalformedPayload},
		{"empty stdin", "PreToolUse", nil, claudehook.ErrMalformedPayload},
		{"tool name is a number", "PreToolUse", []byte(`{"tool_name":7}`), claudehook.ErrMalformedPayload},
		{"torn json", "PreToolUse", []byte(`{"tool_name":"Ed`), claudehook.ErrMalformedPayload},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (claudehook.Decoder{}).Decode(tt.hookEvent, tt.payload)
			if err != tt.wantErr {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
			if err != nil && strings.Contains(err.Error(), "CANARY") {
				t.Errorf("error text echoes the payload: %v", err)
			}
		})
	}
}

func TestDecodeAcceptsAPayloadAtTheSizeCap(t *testing.T) {
	padding := strings.Repeat("x", capture.MaxStdinBytes-len(`{"prompt":""}`))
	payload := `{"prompt":"` + padding + `"}`

	if _, err := (claudehook.Decoder{}).Decode("UserPromptSubmit", []byte(payload)); err != nil {
		t.Errorf("a payload at the cap must decode, got %v", err)
	}
}
