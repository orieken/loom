package orchestrator_test

// Roadmap L2.26 — the payload a stage was rejected for is kept on disk and
// the error says where. A typed stage is deliberately not retried, so the
// rejection is the only account of what the agent produced; before this,
// that account was decoded, rejected and dropped.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
	"github.com/orieken/loom/internal/state"
)

func rejectedStatePath(input orchestrator.StageInput, stageID string) string {
	return filepath.Join(input.WorkspaceDir, state.TypedStateDir, stageID+".rejected.json")
}

// TestRejectedPayloadIsKeptAndNamed is the done-when: a stage failure caused
// by an invalid payload leaves that payload on disk, and the error names
// where. The invented field matters twice — it must be in the error AND
// still in the kept file, because the point is reading what the agent said,
// not what the validator managed to quote.
func TestRejectedPayloadIsKeptAndNamed(t *testing.T) {
	scripts := typedScripts(t)
	valid, _ := mock.TypedScript(string(state.KindAnalysis))
	withExtra := strings.Replace(string(valid.Payload), `{"schemaVersion"`, `{"developerHandoffNotesNote":"oops","schemaVersion"`, 1)
	scripts["analyst"] = mock.Script{Payload: []byte(withExtra)}
	executor, _, _, input := newHarness(t, scripts)

	err := executor.Run(context.Background(), typedPlan(), input)
	if err == nil {
		t.Fatal("Run succeeded; want rejection of the invented field")
	}

	kept := rejectedStatePath(input, "analyst")
	if !strings.Contains(err.Error(), kept) {
		t.Errorf("error does not name where the payload was kept:\n%v\nwant it to name %s", err, kept)
	}
	body, readErr := os.ReadFile(kept)
	if readErr != nil {
		t.Fatalf("rejected payload was not kept: %v", readErr)
	}
	if !strings.Contains(string(body), "developerHandoffNotesNote") {
		t.Error("kept payload does not hold the field the stage was rejected for")
	}
}

// TestRejectedPayloadIsKeptWholeNotTruncated is the specific gap the run-4
// audit recorded as "partial": the payload was echoed into the error and cut
// at 800 characters, so the tail — `knownGaps`, in that run — was lost. The
// error may still quote a prefix; the file may not.
func TestRejectedPayloadIsKeptWholeNotTruncated(t *testing.T) {
	scripts := typedScripts(t)
	valid, _ := mock.TypedScript(string(state.KindAnalysis))
	padded := strings.Replace(string(valid.Payload),
		`{"schemaVersion"`,
		`{"invented":"`+strings.Repeat("x", 2000)+`","tailMarker":"the-part-truncation-eats","schemaVersion"`, 1)
	scripts["analyst"] = mock.Script{Payload: []byte(padded)}
	executor, _, _, input := newHarness(t, scripts)

	if err := executor.Run(context.Background(), typedPlan(), input); err == nil {
		t.Fatal("Run succeeded; want rejection")
	}

	body, err := os.ReadFile(rejectedStatePath(input, "analyst"))
	if err != nil {
		t.Fatalf("rejected payload was not kept: %v", err)
	}
	if len(body) != len(padded) {
		t.Errorf("kept payload is %d bytes, want the whole %d-byte payload", len(body), len(padded))
	}
	if !strings.Contains(string(body), "the-part-truncation-eats") {
		t.Error("the tail of the payload was lost — this is the run-4 'partial' finding, unfixed")
	}
}

// TestASucceedingStageClearsItsRejectedPayload guards the failure mode this
// mechanism introduces: evidence of a run that no longer happened. A stage
// that failed, was fixed and re-ran must not leave a rejected payload
// describing the earlier attempt.
func TestASucceedingStageClearsItsRejectedPayload(t *testing.T) {
	executor, _, _, input := newHarness(t, typedScripts(t))
	stale := rejectedStatePath(input, "analyst")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(stale, []byte(`{"from":"the previous attempt"}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := executor.Run(context.Background(), typedPlan(), input); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("a stale rejected payload survived a successful run of the same stage")
	}
}

// TestAnUnparseablePayloadIsStillValidJSONOnDisk keeps the kept file useful
// to the tools a person reaches for: it is written byte-for-byte as the
// agent produced it, so `jq` reads it when the failure was a schema
// violation rather than a syntax error.
func TestAnUnparseablePayloadIsStillValidJSONOnDisk(t *testing.T) {
	scripts := typedScripts(t)
	valid, _ := mock.TypedScript(string(state.KindAnalysis))
	withExtra := strings.Replace(string(valid.Payload), `{"schemaVersion"`, `{"vibes":"immaculate","schemaVersion"`, 1)
	scripts["analyst"] = mock.Script{Payload: []byte(withExtra)}
	executor, _, _, input := newHarness(t, scripts)

	if err := executor.Run(context.Background(), typedPlan(), input); err == nil {
		t.Fatal("Run succeeded; want rejection")
	}

	body, err := os.ReadFile(rejectedStatePath(input, "analyst"))
	if err != nil {
		t.Fatalf("rejected payload was not kept: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Errorf("kept payload is not parseable JSON: %v", err)
	}
}
