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
	"github.com/orieken/loom/cmd/loom/internal/platform"
)

func TestExecuteInstallWritesManifestAndFilteredRules(t *testing.T) {
	target, cache := t.TempDir(), t.TempDir()
	rules, err := platform.ParseRuleSet("go")
	if err != nil {
		t.Fatalf("parse rules: %v", err)
	}
	request := installRequest{target: target, cache: cache, frameworkVersion: "v3.3.14", platforms: []string{"claude-code"}, rules: rules, isCopy: true}
	var output bytes.Buffer
	reporter := installOutput{writer: &output}
	files := frameworkfs.NewWriter(loom.FrameworkFS, target, cache, true, false, reporter.action)
	if err := executeInstall(request, loom.FrameworkFS, loom.MCPFS, files, reporter); err != nil {
		t.Fatalf("execute install: %v", err)
	}
	assertInstallArtifact(t, filepath.Join(target, manifest.Filename))
	assertInstallArtifact(t, filepath.Join(target, ".claude/rules/go-conventions.md"))
	if _, err := os.Stat(filepath.Join(target, ".claude/rules/python-conventions.md")); !os.IsNotExist(err) {
		t.Fatalf("unexpected Python rule in Go-filtered install: %v", err)
	}
	if !strings.Contains(output.String(), "1 platforms, 39 agents, 69 skills, 6 rules installed") {
		t.Fatalf("unexpected summary: %s", output.String())
	}
}

func TestExecuteInstallDryRunWritesNothing(t *testing.T) {
	target, cache := t.TempDir(), t.TempDir()
	request := installRequest{target: target, cache: cache, frameworkVersion: "v3.3.14", isCopy: true, isDryRun: true}
	var output bytes.Buffer
	reporter := installOutput{writer: &output, isDryRun: true}
	files := frameworkfs.NewWriter(loom.FrameworkFS, target, cache, true, true, reporter.action)
	if err := executeInstall(request, loom.FrameworkFS, loom.MCPFS, files, reporter); err != nil {
		t.Fatalf("execute dry run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, manifest.Filename)); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote manifest: %v", err)
	}
}

func TestExecuteInstallRetainsPreviouslyOwnedSkippedPaths(t *testing.T) {
	target, cache := t.TempDir(), t.TempDir()
	request := installRequest{target: target, cache: cache, frameworkVersion: "v3.3.14", isCopy: true, withConfig: true, withMCP: true}
	for iteration := 0; iteration < 2; iteration++ {
		var output bytes.Buffer
		reporter := installOutput{writer: &output}
		files := frameworkfs.NewWriter(loom.FrameworkFS, target, cache, true, false, reporter.action)
		if err := executeInstall(request, loom.FrameworkFS, loom.MCPFS, files, reporter); err != nil {
			t.Fatalf("execute install %d: %v", iteration, err)
		}
	}
	installed, err := manifest.Read(target)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	assertOwnedPath(t, installed.Platforms, sharedOwnership, ".golangci.yml")
	assertOwnedPath(t, installed.Platforms, sharedOwnership, sanitizeProjectName(filepath.Base(target))+"-mcp")
}

func assertInstallArtifact(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected install artifact %s: %v", path, err)
	}
}

func assertOwnedPath(t *testing.T, records []manifest.PlatformRecord, owner, expected string) {
	t.Helper()
	for _, record := range records {
		if record.Name == owner {
			for _, path := range record.Paths {
				if path == expected {
					return
				}
			}
		}
	}
	t.Fatalf("owner %s in records %v does not contain %s", owner, records, expected)
}
