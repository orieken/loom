package state_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/state"
)

func validRepoDoc() state.RepoDocState {
	return state.RepoDocState{
		SchemaVersion:    state.SchemaVersion,
		Path:             "internal/state",
		InputHash:        "sha256:abc",
		Purpose:          "Records and verifies pipeline checkpoint digests.",
		Responsibilities: []string{"Verifies digests"},
		Layer:            state.LayerApplication,
		Invariants:       []state.DocInvariant{{Statement: "Imports nothing outer", Evidence: []string{"imports"}}},
		Evidence: []state.DocEvidence{
			{Claim: "Records and verifies pipeline checkpoint digests.", Fields: []string{"exports:Verify", "exports:Digest"}},
			{Claim: "Verifies digests", Fields: []string{"exports:Verify"}},
		},
		FilesRead: []state.DocFileRead{{Path: "internal/state/state.go", StartLine: 10, EndLine: 40, Reason: "Verify's contract was ambiguous"}},
	}
}

func TestValidRepoDocPasses(t *testing.T) {
	if err := validRepoDoc().Validate(); err != nil {
		t.Fatalf("a valid document was rejected: %v", err)
	}
}

func TestRepoDocValidationNamesTheBrokenField(t *testing.T) {
	cases := map[string]func(*state.RepoDocState){
		"schemaVersion":           func(d *state.RepoDocState) { d.SchemaVersion = 0 },
		"path":                    func(d *state.RepoDocState) { d.Path = "" },
		"inputHash":               func(d *state.RepoDocState) { d.InputHash = "" },
		"layer":                   func(d *state.RepoDocState) { d.Layer = "mystery" },
		"evidence":                func(d *state.RepoDocState) { d.Evidence = nil },
		"evidence[0].claim":       func(d *state.RepoDocState) { d.Evidence[0].Claim = "" },
		"evidence[0].fields":      func(d *state.RepoDocState) { d.Evidence[0].Fields = nil },
		"invariants[0].statement": func(d *state.RepoDocState) { d.Invariants[0].Statement = "" },
		"invariants[0].evidence":  func(d *state.RepoDocState) { d.Invariants[0].Evidence = nil },
		"filesRead[0]":            func(d *state.RepoDocState) { d.FilesRead[0].Reason = "" },
	}
	for field, breakIt := range cases {
		t.Run(field, func(t *testing.T) {
			doc := validRepoDoc()
			breakIt(&doc)
			assertValidationNames(t, doc.Validate(), field)
		})
	}
}

func TestEvidenceMustCiteARealFactSheetField(t *testing.T) {
	for _, field := range []string{"exports", "exports:Verify", "child_summaries:svc/api", "approximate_imports:x"} {
		doc := validRepoDoc()
		doc.Evidence[0].Fields = []string{field}
		if err := doc.Validate(); err != nil {
			t.Errorf("%q is a fact-sheet field: %v", field, err)
		}
	}
	for _, field := range []string{"source", "line 4", "Exports", ""} {
		doc := validRepoDoc()
		doc.Evidence[0].Fields = []string{field}
		assertValidationNames(t, doc.Validate(), "evidence[0].fields")
	}
}

// "Unknown" is a valid answer — but only when it says what is missing.
func TestAnUnknownPurposeIsValidOnlyWithAnOpenQuestion(t *testing.T) {
	doc := state.RepoDocState{SchemaVersion: state.SchemaVersion, Path: "misc", InputHash: "h", Layer: state.LayerUnknown}
	assertValidationNames(t, doc.Validate(), "purpose")
	doc.OpenQuestions = []string{"The directory holds one file with no exports; its role is not established"}
	if err := doc.Validate(); err != nil {
		t.Errorf("an honest unknown must pass: %v", err)
	}
}

func TestEvidenceIsOnlyRequiredWhenThereIsAClaim(t *testing.T) {
	doc := state.RepoDocState{SchemaVersion: state.SchemaVersion, Path: "p", InputHash: "h", Layer: state.LayerUnknown, OpenQuestions: []string{"?"}}
	if err := doc.Validate(); err != nil {
		t.Errorf("no claims, no evidence needed: %v", err)
	}
	doc.Responsibilities = []string{"does a thing"}
	assertValidationNames(t, doc.Validate(), "evidence")
}

func TestFilesReadNeedsAValidLineRange(t *testing.T) {
	for _, bad := range []state.DocFileRead{
		{Path: "a.go", StartLine: 0, EndLine: 3, Reason: "r"},
		{Path: "a.go", StartLine: 5, EndLine: 4, Reason: "r"},
		{Path: "", StartLine: 1, EndLine: 2, Reason: "r"},
	} {
		doc := validRepoDoc()
		doc.FilesRead = []state.DocFileRead{bad}
		assertValidationNames(t, doc.Validate(), "filesRead[0]")
	}
}

func TestRepoDocDecodesAndRejectsUnknownFields(t *testing.T) {
	raw, _ := json.Marshal(validRepoDoc())
	if _, err := state.Decode(state.KindRepoDoc, raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, err := state.Decode(state.KindRepoDoc, []byte(`{"schemaVersion":2,"invented":1}`)); err == nil {
		t.Error("a field the struct does not have is a field the agent invented")
	}
}

func TestRepoDocSchemaIsGeneratedAndListsTheLayers(t *testing.T) {
	schema, ok := state.SchemaForKind(state.KindRepoDoc)
	if !ok {
		t.Fatal("no schema for repo-doc")
	}
	for _, layer := range state.Layer("").Values() {
		if !strings.Contains(string(schema), `"`+layer+`"`) {
			t.Errorf("schema omits layer %q: the agent would be validated against a rule it was never told", layer)
		}
	}
}

func TestRepoDocRendersAViewWithoutInventingText(t *testing.T) {
	name, body, err := state.RenderView(state.KindRepoDoc, mustJSON(t, validRepoDoc()))
	if err != nil || name != "repo-doc.md" {
		t.Fatalf("render: %q %v", name, err)
	}
	for _, want := range []string{"# internal/state", "Records and verifies", "Layer: application", "## Responsibilities", "## Invariants", "evidence: imports"} {
		if !strings.Contains(body, want) {
			t.Errorf("view lacks %q:\n%s", want, body)
		}
	}
}

func TestRenderingAnUnknownPurposeSaysSoInsteadOfGuessing(t *testing.T) {
	doc := state.RepoDocState{SchemaVersion: state.SchemaVersion, Path: "misc", InputHash: "h", Layer: state.LayerUnknown, OpenQuestions: []string{"what is it?"}}
	_, body, err := state.RenderView(state.KindRepoDoc, mustJSON(t, doc))
	if err != nil || !strings.Contains(body, "Purpose unknown") || !strings.Contains(body, "what is it?") {
		t.Errorf("%v\n%s", err, body)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestFilesReadLineRangeBoundariesAreInclusive(t *testing.T) {
	for _, ok := range []state.DocFileRead{
		{Path: "a.go", StartLine: 1, EndLine: 1, Reason: "r"},
		{Path: "a.go", StartLine: 7, EndLine: 7, Reason: "r"},
	} {
		doc := validRepoDoc()
		doc.FilesRead = []state.DocFileRead{ok}
		if err := doc.Validate(); err != nil {
			t.Errorf("a one-line read at %d is valid: %v", ok.StartLine, err)
		}
	}
}
