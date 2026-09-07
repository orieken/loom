package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	loom "github.com/orieken/loom"
	frameworkfs "github.com/orieken/loom/cmd/loom/internal/fs"
)

func executeLevelInstall(t *testing.T, level int) (string, string) {
	t.Helper()
	target, cache := t.TempDir(), t.TempDir()
	flags := installFlags{target: target, platform: "claude-code", level: level, isCopy: true}
	request, err := prepareInstall(flags, "v3.3.14", loom.FrameworkFS)
	if err != nil {
		t.Fatalf("prepare level %d install: %v", level, err)
	}
	var output bytes.Buffer
	reporter := installOutput{writer: &output}
	files := frameworkfs.NewWriter(loom.FrameworkFS, target, cache, true, false, reporter.action)
	if err := executeInstall(request, loom.FrameworkFS, loom.MCPFS, files, reporter); err != nil {
		t.Fatalf("execute level %d install: %v", level, err)
	}
	return target, output.String()
}

// D.3 done-when: --level 1 installs only the core bundle.
func TestLevelOneInstallsOnlyCoreRules(t *testing.T) {
	target, _ := executeLevelInstall(t, 1)

	for _, core := range []string{"approval-gates.md", "architecture-guardrails.md", "design-principles.md", "memory-trust-boundary.md", "testing-conventions.md"} {
		assertInstallArtifact(t, filepath.Join(target, ".claude/rules", core))
	}
	entries, err := os.ReadDir(filepath.Join(target, ".claude/rules"))
	if err != nil {
		t.Fatalf("read installed rules: %v", err)
	}
	if len(entries) != 5 {
		t.Errorf("level 1 installed %d rules, want exactly the 5 core rules", len(entries))
	}
	for _, absent := range []string{".mcp.json", ".claude/workflows", ".claude/orchestration"} {
		if _, err := os.Stat(filepath.Join(target, absent)); !os.IsNotExist(err) {
			t.Errorf("level 1 must not install %s", absent)
		}
	}
}

// D.3 done-when: --level 2 adds exactly the L2 delta.
func TestLevelTwoAddsExactlyTheLevelTwoDelta(t *testing.T) {
	target, output := executeLevelInstall(t, 2)

	assertInstallArtifact(t, filepath.Join(target, ".mcp.json"))
	assertInstallArtifact(t, filepath.Join(target, ".claude/workflows"))
	assertInstallArtifact(t, filepath.Join(target, ".claude/orchestration"))
	entries, err := os.ReadDir(filepath.Join(target, ".claude/rules"))
	if err != nil {
		t.Fatalf("read installed rules: %v", err)
	}
	if len(entries) != 5 {
		t.Errorf("level 2 installed %d rules, want the same 5 core rules as level 1", len(entries))
	}
	// The executor bundle does not install, and says why. It used to claim
	// M0.4 had not landed; M0.4 shipped 2026-08-29, so the honest reason is
	// that the bundle's action has no implementation (roadmap L3.26).
	if !strings.Contains(output, `bundle "executor" action "executor" is not implemented yet`) {
		t.Errorf("expected the executor bundle to be skipped with a true reason, got:\n%s", output)
	}
	for _, absent := range []string{".claude/telemetry", ".claude/hooks", ".claude/evaluation"} {
		if _, err := os.Stat(filepath.Join(target, absent)); !os.IsNotExist(err) {
			t.Errorf("level 2 must not install level 3+ bundle %s", absent)
		}
	}
}

func TestLevelTwoRespectsExistingMCPConfig(t *testing.T) {
	target, cache := t.TempDir(), t.TempDir()
	existing := `{"mcpServers":{}}`
	if err := os.WriteFile(filepath.Join(target, ".mcp.json"), []byte(existing), 0644); err != nil {
		t.Fatalf("seed .mcp.json: %v", err)
	}
	flags := installFlags{target: target, platform: "claude-code", level: 2, isCopy: true}
	request, err := prepareInstall(flags, "v3.3.14", loom.FrameworkFS)
	if err != nil {
		t.Fatalf("prepare install: %v", err)
	}
	var output bytes.Buffer
	reporter := installOutput{writer: &output}
	files := frameworkfs.NewWriter(loom.FrameworkFS, target, cache, true, false, reporter.action)
	if err := executeInstall(request, loom.FrameworkFS, loom.MCPFS, files, reporter); err != nil {
		t.Fatalf("execute install: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(target, ".mcp.json"))
	if err != nil {
		t.Fatalf("read .mcp.json: %v", err)
	}
	if string(content) != existing {
		t.Errorf(".mcp.json was overwritten: %s", content)
	}
	if !strings.Contains(output.String(), ".mcp.json already exists") {
		t.Errorf("expected manual-merge warning, got:\n%s", output.String())
	}
}
