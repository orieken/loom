package tools

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/orieken/loom/tools"
)

// frameworkToolsForTest builds each tool as a zero value. Neither
// SafeArgumentNames nor InputSchema reads a field, so the constructors'
// loggers and analyzers are not needed to ask a tool what it declares.
func frameworkToolsForTest(t *testing.T) []tools.Tool {
	t.Helper()
	return []tools.Tool{
		&SearchKITool{},
		&SearchDocsTool{},
		&AnalyzeComplexityTool{},
		&CheckUbiquitousLanguageTool{},
		&CheckAccessibilityTool{},
		&VerifyDependenciesTool{},
		&ValidateArtifactTool{},
	}
}

// Guardrail #9 is an allowlist, and an allowlist is only a control while
// someone has to justify widening it. This pins what each framework tool
// declares safe: adding a name here is a deliberate edit in the same commit
// as the tool change, which is the review moment the guardrail exists for.
//
// `query` is absent from every entry on purpose. It is the free text a
// caller composed, and it is what roadmap L3.38 was raised to stop exporting.
var declaredSafeArguments = map[string][]string{
	"search_ki":                 {"domain", "tags"},
	"search_docs":               {"docsPath"},
	"analyze_complexity":        {"maxComplexity", "maxLines", "projectPath"},
	"check_ubiquitous_language": {"dictionaryPath", "projectPath"},
	"check_accessibility":       {"filePath", "projectPath"},
	"verify_dependencies":       {"projectPath"},
	"validate_artifact":         {"artifactPath", "contractPath"},
}

func TestFrameworkToolsDeclareTheExpectedSafeArguments(t *testing.T) {
	for _, tool := range frameworkToolsForTest(t) {
		declaring, ok := tool.(tools.SafeArguments)
		if !ok {
			t.Errorf("%s does not declare safe arguments, so every value it takes is hashed", tool.Name())
			continue
		}
		want, known := declaredSafeArguments[tool.Name()]
		if !known {
			t.Errorf("%s is not pinned in declaredSafeArguments — add it deliberately", tool.Name())
			continue
		}
		got := append([]string(nil), declaring.SafeArgumentNames()...)
		sort.Strings(got)
		if !equalStrings(got, want) {
			t.Errorf("%s safe arguments = %v, want %v", tool.Name(), got, want)
		}
	}
}

// A declared-safe name that is not an argument the tool accepts is dead
// permission: harmless today, and a trap the first time someone adds an
// argument by that name and finds it already exempt.
func TestSafeArgumentsAreRealArgumentsOfTheirTool(t *testing.T) {
	for _, tool := range frameworkToolsForTest(t) {
		declaring, ok := tool.(tools.SafeArguments)
		if !ok {
			continue
		}
		properties := schemaProperties(t, tool.InputSchema())
		for _, name := range declaring.SafeArgumentNames() {
			if !properties[name] {
				t.Errorf("%s declares %q safe but its schema has no such argument", tool.Name(), name)
			}
		}
	}
}

func schemaProperties(t *testing.T, schema json.RawMessage) map[string]bool {
	t.Helper()
	var document struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(schema, &document); err != nil {
		t.Fatalf("input schema is not valid JSON: %v", err)
	}
	properties := make(map[string]bool, len(document.Properties))
	for name := range document.Properties {
		properties[name] = true
	}
	return properties
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
