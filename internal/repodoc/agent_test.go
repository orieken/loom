package repodoc

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/factsheet"
	"github.com/orieken/loom/internal/state"
)

func readRepoFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// jsonFields lists the JSON keys of a struct type, recursing into slices of
// structs, so a field added to the output type shows up here.
func jsonFields(typ reflect.Type) []string {
	var fields []string
	for i := 0; i < typ.NumField(); i++ {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		fields = append(fields, name)
		if inner := typ.Field(i).Type; inner.Kind() == reflect.Slice && inner.Elem().Kind() == reflect.Struct {
			fields = append(fields, jsonFields(inner.Elem())...)
		}
	}
	return fields
}

// The agent is told its output shape in prose. If the type grows a field and
// the prompt does not mention it, the agent is validated against a rule it was
// never told — the L2.25 failure, one level up.
func TestAgentPromptNamesEveryTopLevelOutputField(t *testing.T) {
	prompt := readRepoFile(t, "shared/agents/repo-documenter.md")
	typ := reflect.TypeOf(state.RepoDocState{})
	for i := 0; i < typ.NumField(); i++ {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		if !strings.Contains(prompt, name) {
			t.Errorf("repo-documenter.md never mentions the output field %q", name)
		}
	}
}

func TestAgentPromptNamesEveryFactSheetFieldAnEvidenceCitationMayUse(t *testing.T) {
	prompt := readRepoFile(t, "shared/agents/repo-documenter.md")
	for _, root := range state.FactFieldRoots() {
		if !strings.Contains(prompt, root) {
			t.Errorf("the agent is not told it may cite %q", root)
		}
	}
	for _, field := range jsonFields(reflect.TypeOf(factsheet.FactSheet{})) {
		if !strings.Contains(prompt, field) {
			t.Errorf("the agent is not told the fact sheet has a %q field", field)
		}
	}
}

func TestAgentIsReadOnlyAndNamedLikeItsFile(t *testing.T) {
	prompt := readRepoFile(t, "shared/agents/repo-documenter.md")
	frontmatter := strings.SplitN(prompt, "---", 3)[1]
	for _, want := range []string{"name: repo-documenter", "tools: Read\n", "model_tier: default", "version: 1.1.1"} {
		if !strings.Contains(frontmatter, want) {
			t.Errorf("frontmatter lacks %q:\n%s", want, frontmatter)
		}
	}
	if AgentName != "repo-documenter" {
		t.Errorf("AgentName = %q", AgentName)
	}
}

func TestContractNamesTheSchemaAndEveryLayer(t *testing.T) {
	contract := readRepoFile(t, "shared/contracts/repo-doc-contract.md")
	if !strings.Contains(contract, "shared/schemas/pipeline/repo-doc.schema.json") {
		t.Error("the contract does not point at the generated schema")
	}
	for _, layer := range state.Layer("").Values() {
		if !strings.Contains(contract, "`"+layer+"`") {
			t.Errorf("the contract omits layer %q", layer)
		}
	}
}

var fencedJSON = regexp.MustCompile("(?s)## ([a-z-]+)\n\n```json\n(.*?)\n```")

func fixtureBlocks(t *testing.T) map[string][]byte {
	t.Helper()
	blocks := map[string][]byte{}
	for _, match := range fencedJSON.FindAllStringSubmatch(readRepoFile(t, "tests/agents/repo-documenter/input-fact-sheet.md"), -1) {
		blocks[match[1]] = []byte(match[2])
	}
	return blocks
}

func fixtureSheet(t *testing.T) factsheet.FactSheet {
	t.Helper()
	var sheet factsheet.FactSheet
	if err := json.Unmarshal(fixtureBlocks(t)[inputFactSheet], &sheet); err != nil {
		t.Fatalf("fact-sheet block: %v", err)
	}
	return sheet
}

// The eval fixture must stay a faithful copy of what `loom document` sends.
func TestEvalFixtureInputIsAValidFactSheetWithTheTrapItNeeds(t *testing.T) {
	sheet := fixtureSheet(t)
	if sheet.Path != "internal/ledger" || sheet.InputHash == "" || len(sheet.ApproximateImports) == 0 {
		t.Errorf("fixture sheet: %+v", sheet)
	}
	if !strings.Contains(readRepoFile(t, "tests/agents/repo-documenter/expected-patterns.txt"), sheet.InputHash) {
		t.Error("expected-patterns.txt must pin the fixture's input_hash")
	}
}

func TestEvalFixtureOffersOnlyTagsTheShippedVocabularyHas(t *testing.T) {
	var tags []Tag
	if err := json.Unmarshal(fixtureBlocks(t)[inputAllowedTags], &tags); err != nil || len(tags) == 0 {
		t.Fatalf("allowed-tags block: %v", err)
	}
	shipped, err := parseVocabulary([]byte(readRepoFile(t, VocabularyFile)), VocabularyFile)
	if err != nil {
		t.Fatal(err)
	}
	known := newVocabulary(shipped)
	for _, tag := range tags {
		if !known.Contains(tag.Name) {
			t.Errorf("fixture tag %q is not in the shipped vocabulary", tag.Name)
		}
	}
}

func TestDocumenterPromptNamesEveryInputBlockItCanReceive(t *testing.T) {
	prompt := readRepoFile(t, "shared/agents/repo-documenter.md")
	for _, block := range []string{inputFactSheet, inputChildSummaries, inputAllowedTags, inputFeedback} {
		if !strings.Contains(prompt, block) {
			t.Errorf("the documenter is sent a %q block its prompt never describes", block)
		}
	}
	for _, field := range []string{"source", "code", "where", "message", "findings", "previous"} {
		if !strings.Contains(prompt, field) {
			t.Errorf("the prompt does not explain the feedback field %q", field)
		}
	}
}

func TestAuditorPromptNamesEveryOutputField(t *testing.T) {
	prompt := readRepoFile(t, "shared/agents/repo-doc-auditor.md")
	for _, typ := range []reflect.Type{reflect.TypeOf(state.RepoDocAuditState{}), reflect.TypeOf(state.DocAuditFinding{})} {
		for _, field := range jsonFields(typ) {
			if !strings.Contains(prompt, field) {
				t.Errorf("repo-doc-auditor.md never mentions the output field %q", field)
			}
		}
	}
}

func TestAuditorPromptDescribesItsInputsAndIsReadOnly(t *testing.T) {
	prompt := readRepoFile(t, "shared/agents/repo-doc-auditor.md")
	frontmatter := strings.SplitN(prompt, "---", 3)[1]
	for _, want := range []string{"name: repo-doc-auditor", "tools: Read\n", "model_tier:", "version: 1.0.0"} {
		if !strings.Contains(frontmatter, want) {
			t.Errorf("frontmatter lacks %q", want)
		}
	}
	for _, block := range []string{inputFactSheet, inputChildSummaries, inputCandidate} {
		if !strings.Contains(prompt, block) {
			t.Errorf("the auditor is sent a %q block its prompt never describes", block)
		}
	}
	if AuditorName != "repo-doc-auditor" {
		t.Errorf("AuditorName = %q", AuditorName)
	}
}

func TestAuditContractNamesTheSchemaAndTheRules(t *testing.T) {
	contract := readRepoFile(t, "shared/contracts/repo-doc-audit-contract.md")
	for _, want := range []string{"shared/schemas/pipeline/repo-doc-audit.schema.json", "CHANGES_REQUESTED", "APPROVED", "blocking", "unverified"} {
		if !strings.Contains(contract, want) {
			t.Errorf("the contract does not mention %q", want)
		}
	}
}

func auditFixtureBlocks(t *testing.T) map[string][]byte {
	t.Helper()
	blocks := map[string][]byte{}
	for _, match := range fencedJSON.FindAllStringSubmatch(readRepoFile(t, "tests/agents/repo-doc-auditor/input-candidate.md"), -1) {
		blocks[match[1]] = []byte(match[2])
	}
	return blocks
}

// The auditor only ever sees documents the verifier passed. A fixture whose
// candidate the verifier rejects would test a situation that cannot happen.
func TestAuditorFixtureCandidatePassesTheDeterministicVerifier(t *testing.T) {
	blocks := auditFixtureBlocks(t)
	var sheet factsheet.FactSheet
	var candidate state.RepoDocState
	if err := json.Unmarshal(blocks[inputFactSheet], &sheet); err != nil {
		t.Fatalf("fact-sheet: %v", err)
	}
	if err := json.Unmarshal(blocks[inputCandidate], &candidate); err != nil {
		t.Fatalf("candidate: %v", err)
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("the candidate must be schema-valid: %v", err)
	}
	shipped, _ := parseVocabulary([]byte(readRepoFile(t, VocabularyFile)), VocabularyFile)
	if violations := Verify(sheet, candidate, newVocabulary(shipped)); len(violations) != 0 {
		t.Errorf("the auditor fixture's candidate must pass the verifier, so only judgement is left to test: %+v", violations)
	}
	if !strings.Contains(readRepoFile(t, "tests/agents/repo-doc-auditor/expected-patterns.txt"), sheet.InputHash) {
		t.Error("expected-patterns.txt must pin the fixture's input_hash")
	}
}

// The documenter fixture's whole point is that a careless answer fails the
// verifier; make sure the trap it plants is one the verifier can catch.
func TestDocumenterFixtureTrapIsCatchableByTheVerifier(t *testing.T) {
	sheet := fixtureSheet(t)
	doc := state.RepoDocState{SchemaVersion: state.SchemaVersion, Path: sheet.Path, InputHash: sheet.InputHash, Layer: state.LayerShared,
		Purpose: "Depends on `internal/legacy` and rolls back with `Rollback`.", OpenQuestions: []string{"q"},
		Evidence: []state.DocEvidence{{Claim: "Depends on `internal/legacy` and rolls back with `Rollback`.", Fields: []string{"exports"}}}}
	violations := Verify(sheet, doc, newVocabulary(nil))
	if !hasCode(violations, CodeUnknownName) || len(violations) != 2 {
		t.Errorf("an invented symbol and an approximate edge stated flat: %+v", violations)
	}
}
