package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/shared/mcp/internal/analyzers"
	"github.com/orieken/loom/shared/mcp/internal/domain"
)

// rootedProject is a small workspace every path-taking tool can analyze, with
// a real directory beside it for "../outside" to name.
func rootedProject(t *testing.T) WorkspaceRoot {
	t.Helper()
	base := t.TempDir()
	dir := filepath.Join(base, "project")
	files := map[string]string{
		"main.go":              "package main\n\nfunc main() {}\n",
		"index.html":           "<html><body><img src=\"a.png\" alt=\"a\"></body></html>\n",
		"DOMAIN_DICTIONARY.md": "## Customer\n**Synonyms to avoid**: `client`\n",
		"docs/guide.md":        "# Guide\n",
	}
	for name, content := range files {
		WriteFile(t, filepath.Join(dir, name), content)
	}
	if err := os.MkdirAll(filepath.Join(base, "outside"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	root, err := NewWorkspaceRoot(dir)
	if err != nil {
		t.Fatalf("NewWorkspaceRoot: %v", err)
	}
	return root
}

func rootedTools(root WorkspaceRoot) map[string]domain.Tool {
	logger := SilentLogger()
	return map[string]domain.Tool{
		"analyze_complexity":        NewAnalyzeComplexityTool(logger, analyzers.NewComplexityAnalyzer(), root),
		"check_accessibility":       NewCheckAccessibilityTool(logger, analyzers.NewAccessibilityAnalyzer(), root),
		"check_ubiquitous_language": NewCheckUbiquitousLanguageTool(logger, analyzers.NewUbiquitousLanguageAnalyzer(), root),
		"verify_dependencies":       NewVerifyDependenciesTool(logger, analyzers.NewDependencyBoundaryAnalyzer(), root),
		"search_docs":               NewSearchDocsTool(logger, nil, nil, root),
	}
}

// valid arguments for each tool, every path relative to the workspace root.
var validArguments = map[string]map[string]any{
	"analyze_complexity":        {"projectPath": "."},
	"check_accessibility":       {"projectPath": "."},
	"check_ubiquitous_language": {"projectPath": ".", "dictionaryPath": "DOMAIN_DICTIONARY.md"},
	"verify_dependencies":       {"projectPath": "."},
	"search_docs":               {"query": "guide", "docsPath": "docs"},
}

func TestRootedToolsAnalyzeAPathInsideTheWorkspace(t *testing.T) {
	root := rootedProject(t)
	for name, tool := range rootedTools(root) {
		t.Run(name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), BuildRequest(validArguments[name]))
			if err != nil || result.IsError {
				t.Errorf("%s refused a path inside the workspace: %v %s", name, err, ExtractText(t, result))
			}
		})
	}
}

func TestRootedToolsRefuseEveryPathArgumentThatEscapes(t *testing.T) {
	root := rootedProject(t)
	for name, tool := range rootedTools(root) {
		for _, key := range pathKeys(validArguments[name]) {
			t.Run(name+" "+key, func(t *testing.T) {
				args := withArgument(validArguments[name], key, "../outside")
				result, err := tool.Execute(context.Background(), BuildRequest(args))
				if err != nil || !result.IsError || !strings.Contains(ExtractText(t, result), "outside the workspace root") {
					t.Errorf("%s accepted %s=../outside: %v %s", name, key, err, ExtractText(t, result))
				}
			})
		}
	}
}

// pathKeys are the arguments that carry a path.
func pathKeys(args map[string]any) []string {
	var keys []string
	for key := range args {
		if strings.HasSuffix(key, "Path") {
			keys = append(keys, key)
		}
	}
	return keys
}

// withArgument copies args with one value replaced.
func withArgument(args map[string]any, key string, value any) map[string]any {
	copied := map[string]any{key: value}
	for k, v := range args {
		if k != key {
			copied[k] = v
		}
	}
	return copied
}

// accessibility takes filePath as an alternative to projectPath; both are
// confined.
func TestAccessibilityConfinesItsFilePath(t *testing.T) {
	tool := rootedTools(rootedProject(t))["check_accessibility"]
	inside, err := tool.Execute(context.Background(), BuildRequest(map[string]any{"filePath": "index.html"}))
	if err != nil || inside.IsError {
		t.Errorf("filePath inside the workspace refused: %v %s", err, ExtractText(t, inside))
	}
	outside, err := tool.Execute(context.Background(), BuildRequest(map[string]any{"filePath": "/"}))
	if err != nil || !outside.IsError {
		t.Errorf("filePath=/ accepted: %v %s", err, ExtractText(t, outside))
	}
}
