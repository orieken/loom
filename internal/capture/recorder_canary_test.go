package capture

import (
	"errors"
	"strings"
	"testing"
)

var errAppend = errors.New("disk full")

// Canary strings: unique text that must never reach a stored line.
const (
	canaryOld    = "CANARY-OLD-TEXT-7c1e"
	canaryNew    = "CANARY-NEW-TEXT-9b42"
	canaryPrompt = "CANARY-PROMPT-3d8f"
	canaryFile   = "CANARY-FILE-BODY-5a60"
)

// TestRecorderCanaryTextNeverReachesAStoredLine feeds the old and new text of
// an edit, a prompt and the body of a file, and searches every stored line.
func TestRecorderCanaryTextNeverReachesAStoredLine(t *testing.T) {
	file := "head\n" + canaryNew + " " + canaryFile + "\ntail\n"
	edits := EditText{}.With(canaryOld, canaryNew)
	editObs := edit("a.go")
	promptObs := Observation{Kind: EventPrompt, SessionID: "s1", Prompt: "no, " + canaryPrompt}
	r := newRig(t, editObs, edits)
	r.files.put("a.go", file)

	calls := []Observation{promptObs, editObs, readOf("a.go")}
	for _, obs := range calls {
		r.mustCall(t, obs)
	}

	stored := storedLines(t, r.sink.events)
	equal(t, "prompt class stored", strings.Contains(stored, `"promptClass":"corrective"`), true)
	equal(t, "a canary reached a stored line", containsAny(stored, canaryOld, canaryNew, canaryPrompt, canaryFile), false)
}

func TestRecorderCanaryTextNeverReachesAnError(t *testing.T) {
	obs := Observation{Kind: EventPrompt, Prompt: canaryPrompt}
	r := newRig(t, obs, EditText{}.With(canaryOld, canaryNew))
	r.sink.failWith = errAppend

	err := r.call(t, obs)

	if err == nil || containsAny(err.Error(), canaryOld, canaryNew, canaryPrompt) {
		t.Fatalf("err = %v, want an error without payload text", err)
	}
}
