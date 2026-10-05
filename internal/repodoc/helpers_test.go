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
		{Path: "svc/api", Exports: []string{"Serve"}, InputHash: "h-api"},
		{Path: "svc/store", Exports: []string{"Get"}, InputHash: "h-store"},
	}
}

func validDoc(path, hash string) state.RepoDocState {
	return state.RepoDocState{
		SchemaVersion: state.SchemaVersion, Path: path, InputHash: hash,
		Purpose: "does " + path, Layer: state.LayerShared, Tags: []string{"api"},
		Evidence: []state.DocEvidence{{Claim: "does " + path, Fields: []string{"exports"}}},
	}
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
