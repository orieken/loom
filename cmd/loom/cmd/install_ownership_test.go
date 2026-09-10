package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	loom "github.com/orieken/loom"
	frameworkfs "github.com/orieken/loom/cmd/loom/internal/fs"
	"github.com/orieken/loom/cmd/loom/internal/manifest"
)

// The scenario roadmap L3.26 was filed for: a project that already has its
// own agent and skill, and its own copy of a document loom also ships.
// Before L3.26 install moved .claude/agents and .claude/skills aside
// wholesale and every one of these files disappeared.
func TestInstallLeavesFilesItDidNotInstallAlone(t *testing.T) {
	target := occupiedProject(t)

	installInto(t, target)

	assertContent(t, target, ".claude/agents/our-team-reviewer.md", "OUR AGENT")
	assertContent(t, target, ".claude/skills/my-deploy/SKILL.md", "OUR SKILL")
	assertContent(t, target, "DOMAIN_DICTIONARY.md", "OUR TERMS")
	assertNoBackups(t, target)
}

// Loom's own content must still arrive, alongside the project's — the fix is
// worthless if "touch nothing" degrades into "install nothing".
func TestInstallStillInstallsItsOwnContentAlongsideTheProjects(t *testing.T) {
	target := occupiedProject(t)

	installInto(t, target)

	if _, err := os.Stat(filepath.Join(target, ".claude/agents/analyst.md")); err != nil {
		t.Fatalf("loom's own agent was not installed: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(target, ".claude/agents/our-team-reviewer.md")); err != nil {
		t.Fatalf("the project's agent did not survive alongside it: %v", err)
	}
}

// A second install must recognise its own files rather than treating them as
// foreign, or an upgrade from one level to the next would install nothing.
func TestSecondInstallUpdatesItsOwnFilesRatherThanSkippingThem(t *testing.T) {
	target := occupiedProject(t)
	installInto(t, target)

	output := installInto(t, target)

	if strings.Contains(output, ".claude/agents/analyst.md (not installed by loom") {
		t.Fatal("install treated its own file as foreign on the second run")
	}
}

// A loom file the project has edited is left alone: overwriting it would
// discard someone's work with no way to notice.
func TestInstallPreservesALoomFileTheProjectHasEdited(t *testing.T) {
	target := occupiedProject(t)
	installInto(t, target)
	edited := filepath.Join(target, ".claude/rules/design-principles.md")
	writeFile(t, edited, "OUR HOUSE RULES")

	output := installInto(t, target)

	assertContent(t, target, ".claude/rules/design-principles.md", "OUR HOUSE RULES")
	if !strings.Contains(output, "edited since loom installed it") {
		t.Fatalf("the skip was not explained to the user; got:\n%s", output)
	}
}

func occupiedProject(t *testing.T) string {
	t.Helper()
	target := t.TempDir()
	writeFile(t, filepath.Join(target, ".claude/agents/our-team-reviewer.md"), "OUR AGENT")
	writeFile(t, filepath.Join(target, ".claude/skills/my-deploy/SKILL.md"), "OUR SKILL")
	writeFile(t, filepath.Join(target, "DOMAIN_DICTIONARY.md"), "OUR TERMS")
	return target
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertContent(t *testing.T, target, relative, want string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(target, relative))
	if err != nil {
		t.Fatalf("%s: %v", relative, err)
	}
	if string(content) != want {
		t.Fatalf("%s: got %q, want %q", relative, content, want)
	}
}

// A backup is how the old whole-directory install announced a clobber. Their
// absence is the property, not a detail: a file that was never displaced
// needs no backup.
func assertNoBackups(t *testing.T, target string) {
	t.Helper()
	err := filepath.Walk(target, func(path string, _ os.FileInfo, walkErr error) error {
		if walkErr == nil && strings.Contains(filepath.Base(path), ".bak.") {
			t.Errorf("install created a backup, so it displaced something: %s", path)
		}
		return walkErr
	})
	if err != nil {
		t.Fatalf("walk target: %v", err)
	}
}

// installInto runs a real install into target, reusing one cache across
// calls so that a second install sees the state a repeat run really sees.
func installInto(t *testing.T, target string) string {
	t.Helper()
	cache := cacheFor(t, target)
	request := installRequest{target: target, cache: cache, frameworkVersion: "v3.3.14",
		platforms: []string{"claude-code"}, isCopy: true}
	var output bytes.Buffer
	reporter := installOutput{writer: &output}
	previous, _, err := manifest.ReadIfExists(target)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	files := frameworkfs.NewWriter(loom.FrameworkFS, target, cache, true, false, reporter.action).
		WithOwnership(previous.Ledger(), false)
	if err := executeInstall(request, loom.FrameworkFS, loom.MCPFS, files, reporter); err != nil {
		t.Fatalf("execute install: %v", err)
	}
	return output.String()
}

// cacheFor keeps one cache directory per target for the test's lifetime.
func cacheFor(t *testing.T, target string) string {
	t.Helper()
	cache := filepath.Join(filepath.Dir(target), filepath.Base(target)+"-cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatalf("create cache: %v", err)
	}
	return cache
}
