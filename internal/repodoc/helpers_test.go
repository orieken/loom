package repodoc

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/orieken/loom/internal/factsheet"
	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
	"github.com/orieken/loom/internal/state"
)

// tree is a small repo: root > svc > {store, api}.
func tree() []factsheet.FactSheet {
	return []factsheet.FactSheet{
		{Path: ".", Children: []string{"svc"}, InputHash: "h-root"},
		{Path: "svc", Children: []string{"svc/api", "svc/store"}, InputHash: "h-svc"},
		{Path: "svc/api", Files: []string{"api.go"}, Exports: []string{"Serve"}, InputHash: "h-api"},
		{Path: "svc/store", Exports: []string{"Get"}, InputHash: "h-store"},
	}
}

// validDoc is a document that passes both the schema and the verifier for the
// matching directory of tree(): it cites `children` where there are children
// and `exports` where there are exports, because a citation of an empty field
// supports nothing.
func validDoc(path, hash string) state.RepoDocState {
	cited := "exports"
	for _, sheet := range tree() {
		if sheet.Path == path && len(sheet.Children) > 0 {
			cited = "children"
		}
	}
	return state.RepoDocState{
		SchemaVersion: state.SchemaVersion, Path: path, InputHash: hash,
		Purpose: "does " + path, Layer: state.LayerShared, Tags: []string{"api"},
		Evidence: []state.DocEvidence{{Claim: "does " + path, Fields: []string{cited}}},
	}
}

func approval(path, hash string) state.RepoDocAuditState {
	return state.RepoDocAuditState{SchemaVersion: state.SchemaVersion, Path: path, InputHash: hash, Verdict: state.VerdictApproved}
}

func rejection(path, hash string) state.RepoDocAuditState {
	return state.RepoDocAuditState{SchemaVersion: state.SchemaVersion, Path: path, InputHash: hash, Verdict: state.VerdictChangesRequested,
		Findings: []state.DocAuditFinding{{Claim: "does " + path, Problem: "the exports do not say that", Instruction: "soften it", Blocking: true}}}
}

func encodeAudit(t *testing.T, audit state.RepoDocAuditState) []byte {
	t.Helper()
	raw, err := json.Marshal(audit)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func encode(t *testing.T, doc state.RepoDocState) []byte {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// scriptedProvider answers every directory of tree() with a valid document.
func scriptedProvider(t *testing.T, sheets []factsheet.FactSheet) *mock.Provider {
	t.Helper()
	scripts := map[string]mock.Script{}
	for _, sheet := range sheets {
		scripts[stageID(sheet.Path)] = mock.Script{Payload: encode(t, validDoc(sheet.Path, sheet.InputHash))}
		scripts[auditStageID(sheet.Path)] = mock.Script{Payload: encodeAudit(t, approval(sheet.Path, sheet.InputHash))}
	}
	return mock.New(scripts)
}

func fixedClock() time.Time { return time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC) }

func newRunner(provider orchestrator.Provider, root string) Runner {
	return Runner{Provider: provider, Root: root, Now: fixedClock, Vocabulary: newVocabulary([]Tag{{Name: "api"}, {Name: "state"}})}
}

func tasksFor(sheets []factsheet.FactSheet) []Task {
	return Plan(emptyIndex(), sheets, Options{})
}

func jsonMarshal(value any) ([]byte, error) { return json.Marshal(value) }
