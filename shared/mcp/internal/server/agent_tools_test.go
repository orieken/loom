package server

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/orieken/loom/shared/mcp/internal/logging"
	"github.com/orieken/loom/shared/mcp/internal/tools"
)

const repoRoot = "../../../.."

var mcpToolMention = regexp.MustCompile(`mcp__loom__([a-z_]+)`)

func registeredToolNames(t *testing.T) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	for _, registration := range FrameworkRegistryAt(logging.NewLogger(&bytes.Buffer{}), tools.WorkspaceRoot{}).All() {
		names[registration.Tool.Name()] = true
	}
	return names
}

func readText(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot, path))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func mentioned(text string) []string {
	seen := map[string]bool{}
	for _, match := range mcpToolMention.FindAllStringSubmatch(text, -1) {
		seen[match[1]] = true
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func frontmatterTools(text string) []string {
	for _, line := range strings.Split(strings.SplitN(text, "\n---", 2)[0], "\n") {
		if value, found := strings.CutPrefix(line, "tools:"); found {
			return strings.Split(strings.ReplaceAll(value, " ", ""), ",")
		}
	}
	return nil
}

func declaredMCP(text string) []string {
	var names []string
	for _, tool := range frontmatterTools(text) {
		if name, found := strings.CutPrefix(tool, "mcp__loom__"); found {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// A prompt that names a tool the server does not register sends an agent to a
// call that cannot succeed.
func TestEveryLoomMCPToolAnAgentOrSkillNamesIsRegistered(t *testing.T) {
	registered := registeredToolNames(t)
	files, _ := filepath.Glob(filepath.Join(repoRoot, "shared", "agents", "*.md"))
	skills, _ := filepath.Glob(filepath.Join(repoRoot, "shared", "skills", "*", "SKILL.md"))
	for _, file := range append(files, skills...) {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range mentioned(string(raw)) {
			if !registered[name] {
				t.Errorf("%s names mcp__loom__%s, which the server does not register", file, name)
			}
		}
	}
}

// Declared tools are permissions. Declaring one the prompt never uses is a
// permission nobody reviewed; using one the frontmatter omits fails under
// `loom run`, whose allowlist is that line.
func TestAgentsDeclareExactlyTheLoomMCPToolsTheirPromptsUse(t *testing.T) {
	want := map[string][]string{
		"context-engineer":  {"dependents_of", "find_files", "impact"},
		"architect":         {"dependencies_of", "dependents_of", "describe"},
		"refactor-engineer": {"dependents_of", "impact"},
		"unit-tester":       {"dependents_of"},
	}
	for agent, expected := range want {
		text := readText(t, "shared/agents/"+agent+".md")
		if got := declaredMCP(text); !equal(got, expected) {
			t.Errorf("%s declares %v, want %v", agent, got, expected)
		}
		body := strings.SplitN(text, "\n---\n", 2)[1]
		if got := mentioned(body); !equal(got, expected) {
			t.Errorf("%s's prompt uses %v but declares %v", agent, got, expected)
		}
	}
}

func TestNoOtherAgentDeclaresAnMCPToolItDoesNotUse(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(repoRoot, "shared", "agents", "*.md"))
	for _, file := range files {
		raw, _ := os.ReadFile(file)
		declared := declaredMCP(string(raw))
		if len(declared) > 0 && !equal(declared, mentioned(strings.SplitN(string(raw), "\n---\n", 2)[1])) {
			t.Errorf("%s declares %v but its prompt does not use exactly those", file, declared)
		}
	}
}

func TestEveryAgentThatUsesTheGraphSaysItIsOptionalAndNamesTheFallback(t *testing.T) {
	for _, agent := range []string{"context-engineer", "architect", "refactor-engineer", "unit-tester"} {
		text := readText(t, "shared/agents/"+agent+".md")
		for _, want := range []string{"The repo graph is optional", "`fresh`", "`approximate`", "say in your output that you did"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s does not say %q", agent, want)
			}
		}
	}
}

func TestContextEngineerRecordsOnlyTheQuestionAndImpactIsItsFirstChoice(t *testing.T) {
	text := strings.Join(strings.Fields(readText(t, "shared/agents/context-engineer.md")), " ")
	for _, want := range []string{"mcp__loom__impact", "only `seeds`", "replaces anything else you write there", "distance-1", "**omit `blastRadius`**"} {
		if !strings.Contains(text, want) {
			t.Errorf("context-engineer lacks %q", want)
		}
	}
	if !strings.Contains(readText(t, "shared/contracts/context-manifest-contract.md"), "## Blast Radius") {
		t.Error("the contract must say the section exists and is optional")
	}
}

func TestVerifyDependenciesChecksLayersAsGraphQueriesAndKeepsItsStandaloneFallback(t *testing.T) {
	text := readText(t, "shared/skills/verify-dependencies/SKILL.md")
	for _, want := range []string{"mcp__loom__dependencies_of", "`layer`", "`domain` → `application` → `adapter` → `framework`", "possible* violation", "standalone: true", "## Standalone Mode"} {
		if !strings.Contains(text, want) {
			t.Errorf("verify-dependencies lacks %q", want)
		}
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}
