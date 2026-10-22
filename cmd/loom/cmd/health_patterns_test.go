package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// patternSpecText is a valid local spec. verifyLine, when set, adds one
// verify reference.
func patternSpecText(name, verifyLine string) string {
	lines := []string{
		"schemaVersion: 1", "name: " + name, "version: 1.0.0", "intent: Do the thing.", "owner: team",
		"triggers: {keywords: [], intentPatterns: []}",
		"steps: [{id: run, description: Run it., executor: tool}]",
		"provenance: {source: hand-written, status: candidate}",
	}
	if verifyLine != "" {
		lines = append(lines, "verify: ["+verifyLine+"]")
	}
	return strings.Join(lines, "\n")
}

func writeProjectFile(t *testing.T, target, relative, content string) {
	t.Helper()
	path := filepath.Join(target, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// patternHealthContent is the framework content of these tests: the health
// fixture plus one shipped skill.
func patternHealthContent() fstest.MapFS {
	content := healthContent()
	content["shared/skills/review-pr/SKILL.md"] = &fstest.MapFile{Data: []byte("---\nname: review-pr\n---\n")}
	return content
}

// runHealthOn runs the whole health command against target, which has no
// install manifest: pattern specs are checked without one.
func runHealthOn(t *testing.T, target string) string {
	t.Helper()
	var output bytes.Buffer
	_ = executeHealth(healthRequest{target: target, display: target}, patternHealthContent(), &output)
	return output.String()
}

// runPatternCheck runs only the pattern line, so its own failures and
// warnings can be counted apart from the rest of health.
func runPatternCheck(t *testing.T, target string) (string, *healthCheck) {
	t.Helper()
	var output bytes.Buffer
	check := &healthCheck{
		request: healthRequest{target: target, display: target}, content: patternHealthContent(), output: healthOutput{writer: &output},
	}
	check.verifyPatterns()
	return output.String(), check
}

// @issue F2 @ac a project without pattern specs passes and reports zero
func TestHealthReportsNoPatternSpecsAsAPass(t *testing.T) {
	output := runHealthOn(t, t.TempDir())

	if !strings.Contains(output, "✓ pattern specs: none (shared/patterns, .claude/patterns)") {
		t.Errorf("output has no zero-spec pass line:\n%s", output)
	}
}

// @issue F2 @ac a valid local spec passes and is counted
func TestHealthCountsAValidLocalSpec(t *testing.T) {
	target := t.TempDir()
	writeProjectFile(t, target, ".claude/patterns/do-thing.pattern.yaml", patternSpecText("do-thing", ""))

	output, check := runPatternCheck(t, target)

	if !strings.Contains(output, "✓ pattern specs valid (1)") || check.failures != 0 || check.warnings != 0 {
		t.Errorf("want one counted spec and no issue, got %d failure(s), %d warning(s):\n%s", check.failures, check.warnings, output)
	}
}

// The pattern line runs on an installed project too, not only without a
// manifest.
func TestHealthChecksPatternSpecsOnAnInstalledProject(t *testing.T) {
	target := t.TempDir()
	writeHealthFixture(t, target, []string{"alpha.md", "beta.md"})
	writeProjectFile(t, target, ".claude/patterns/do-thing.pattern.yaml", patternSpecText("do-thing", ""))

	output := runHealthOn(t, target)

	if !strings.Contains(output, "manifest found") || !strings.Contains(output, "✓ pattern specs valid (1)") {
		t.Errorf("output lacks the manifest or the pattern line:\n%s", output)
	}
}

// @issue F2 @ac an invalid spec fails health with the file and field path
func TestHealthFailsAnInvalidSpecAndNamesTheFileAndField(t *testing.T) {
	target := t.TempDir()
	text := strings.Replace(patternSpecText("do-thing", ""), "executor: tool", "executor: robot", 1)
	writeProjectFile(t, target, ".claude/patterns/do-thing.pattern.yaml", text)

	output, check := runPatternCheck(t, target)

	if check.failures != 1 {
		t.Fatalf("failures = %d, want 1:\n%s", check.failures, output)
	}
	want := `✗ pattern spec .claude/patterns/do-thing.pattern.yaml: steps[0].executor: must be one of: "tool", "skill", "human" (step "run")`
	if !strings.Contains(output, want) {
		t.Errorf("output lacks %q:\n%s", want, output)
	}
	if strings.Contains(output, "pattern specs valid") {
		t.Error("a failing scan still printed the pass line")
	}
}

// @issue F2 @ac a local spec overriding a shared spec warns
func TestHealthWarnsWhenALocalSpecOverridesASharedOne(t *testing.T) {
	target := t.TempDir()
	shared := strings.Replace(patternSpecText("do-thing", ""), "status: candidate", "status: approved", 1)
	writeProjectFile(t, target, "shared/patterns/do-thing.pattern.yaml", shared)
	writeProjectFile(t, target, ".claude/patterns/do-thing.pattern.yaml", patternSpecText("do-thing", ""))

	output, check := runPatternCheck(t, target)

	if check.failures != 0 || check.warnings != 1 {
		t.Fatalf("want 0 failures and 1 warning, got %d and %d:\n%s", check.failures, check.warnings, output)
	}
	if !strings.Contains(output, `⚠ pattern spec .claude/patterns/do-thing.pattern.yaml: name: overrides the shared spec "do-thing"`) {
		t.Errorf("output has no override warning:\n%s", output)
	}
}

// @issue F2 @ac a verify reference to a missing skill or tool fails
func TestHealthResolvesVerifyReferences(t *testing.T) {
	tests := []struct {
		name       string
		verifyLine string
		wantFailed bool
	}{
		{"a skill the framework ships", "{kind: skill, name: review-pr}", false},
		{"a skill the project installed", "{kind: skill, name: local-skill}", false},
		{"a registered MCP tool", "{kind: tool, name: search_ki}", false},
		{"a missing skill", "{kind: skill, name: no-such-skill}", true},
		{"a missing tool", "{kind: tool, name: no_such_tool}", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target := t.TempDir()
			writeProjectFile(t, target, ".claude/skills/local-skill/SKILL.md", "---\nname: local-skill\n---\n")
			writeProjectFile(t, target, ".claude/patterns/do-thing.pattern.yaml", patternSpecText("do-thing", test.verifyLine))

			output, check := runPatternCheck(t, target)

			if failed := check.failures > 0; failed != test.wantFailed || failed != strings.Contains(output, "verify[0].name") {
				t.Errorf("verify failure = %v, want %v:\n%s", failed, test.wantFailed, output)
			}
		})
	}
}

func TestASkillsDirectoryEntryThatIsADirectoryIsNotASkill(t *testing.T) {
	project := fstest.MapFS{".claude/skills/odd/SKILL.md/inner": {Data: []byte("x")}}
	references := patternReferences{project: project}

	if references.HasSkill("odd") {
		t.Error("a directory named SKILL.md was taken for a skill")
	}
}

func TestNoFrameworkContentResolvesNoFrameworkSkill(t *testing.T) {
	references := patternReferences{project: fstest.MapFS{}}

	if references.HasSkill("review-pr") {
		t.Error("a skill resolved with no framework content")
	}
}
