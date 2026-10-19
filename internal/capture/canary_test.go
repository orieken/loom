package capture_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
	"github.com/orieken/loom/internal/capture/claudehook"
)

const (
	canaryPrompt  = "CANARY-PROMPT-7f3a91"
	canaryOld     = "CANARY-OLDSTRING-2b8c44"
	canaryNew     = "CANARY-NEWSTRING-9d1e05"
	canaryContent = "CANARY-CONTENT-5a6f72"
	canaryCommand = "CANARY-COMMAND-c03b18"
	canaryKey     = "CANARY-KEYNAME-e47d26"
	canarySecret  = "CANARY-SECRETDIR-81fa33"
)

type clockAt struct{ now time.Time }

func (c clockAt) Now() time.Time { return c.now }

func newAssembler(t *testing.T) *capture.Assembler {
	t.Helper()
	salt, err := capture.NewSalt([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	redactor, err := capture.NewRedactor(nil, "/home/user")
	if err != nil {
		t.Fatal(err)
	}
	next := 0
	return capture.NewAssembler(capture.AssemblerDeps{
		Hasher: capture.NewHasher(salt),
		Paths:  capture.NewPathPolicy("/proj", redactor),
		Clock:  clockAt{now: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)},
		NewEventID: func() (string, error) {
			next++
			return fmt.Sprintf("id-%d", next), nil
		},
	})
}

// payloads hold a unique canary in every place a payload value can live.
func canaryPayloads() map[string]string {
	return map[string]string{
		"PostToolUse": `{"session_id":"s1","tool_name":"Edit","tool_input":{` +
			`"file_path":"/proj/src/a.go","old_string":"` + canaryOld + `","new_string":"` + canaryNew + `",` +
			`"content":"` + canaryContent + `","` + canaryKey + `=x":1}}`,
		"PreToolUse":       `{"tool_name":"Bash","tool_input":{"command":"echo ` + canaryCommand + `"}}`,
		"UserPromptSubmit": `{"session_id":"s1","prompt":"no, ` + canaryPrompt + `"}`,
	}
}

// TestCanaryNoPayloadReachesAStoredEvent is the no-payload-at-rest fitness
// function for the domain: payload text must not appear in a normalized
// event, in its day-file line, or in any common printed form.
func TestCanaryNoPayloadReachesAStoredEvent(t *testing.T) {
	assembler := newAssembler(t)
	canaries := []string{canaryPrompt, canaryOld, canaryNew, canaryContent, canaryCommand, canaryKey}
	for hookEvent, payload := range canaryPayloads() {
		t.Run(hookEvent, func(t *testing.T) {
			obs, err := (claudehook.Decoder{}).Decode(hookEvent, []byte(payload))
			if err != nil {
				t.Fatal(err)
			}
			event, err := assembler.Assemble(obs)
			if err != nil {
				t.Fatal(err)
			}
			assertNoCanary(t, event, canaries)
		})
	}
}

func assertNoCanary(t *testing.T, event capture.SessionEvent, canaries []string) {
	t.Helper()
	line, err := capture.MarshalLine(event)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(event)
	forms := []string{string(line), string(encoded), fmt.Sprintf("%v", event), fmt.Sprintf("%+v", event), fmt.Sprintf("%#v", event)}
	for _, form := range forms {
		for _, canary := range canaries {
			if strings.Contains(form, canary) {
				t.Errorf("canary %q found in %q", canary, form)
			}
		}
	}
}

func TestCanaryRedactedPathNameIsNotStoredInPlain(t *testing.T) {
	assembler := newAssembler(t)
	payload := `{"tool_name":"Write","tool_input":{"file_path":"/proj/` + canarySecret + `/secrets/key.txt"}}`
	obs, _ := (claudehook.Decoder{}).Decode("PostToolUse", []byte(payload))

	event, err := assembler.Assemble(obs)

	if err != nil {
		t.Fatal(err)
	}
	if event.Path != nil {
		t.Errorf("redacted path stored as %q", *event.Path)
	}
	assertNoCanary(t, event, []string{canarySecret})
}

func TestCanaryNonRedactedPathIsStoredInPathByDesign(t *testing.T) {
	assembler := newAssembler(t)
	payload := `{"tool_name":"Edit","tool_input":{"file_path":"/proj/` + canarySecret + `/a.go"}}`
	obs, _ := (claudehook.Decoder{}).Decode("PostToolUse", []byte(payload))

	event, _ := assembler.Assemble(obs)

	if event.Path == nil || *event.Path != canarySecret+"/a.go" {
		t.Errorf("Path = %v, want the project-relative path", event.Path)
	}
}
