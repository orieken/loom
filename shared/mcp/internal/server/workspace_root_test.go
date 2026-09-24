package server

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
	"github.com/orieken/loom/shared/mcp/internal/tools"
)

// pathArguments names, for every tool that reads the filesystem, each argument
// a model supplies a path in, plus a valid value for every argument so that
// one path at a time can be made to escape. A tool that resolved only its
// first path passed when this table listed one argument per tool — the
// ubiquitous-language tool's dictionaryPath (a mutant, W8, found it).
var pathArguments = map[string]struct {
	paths []string
	valid map[string]any
}{
	"analyze_complexity":        {paths: []string{"projectPath"}, valid: map[string]any{"projectPath": "."}},
	"check_accessibility":       {paths: []string{"projectPath"}, valid: map[string]any{"projectPath": "."}},
	"check_ubiquitous_language": {paths: []string{"projectPath", "dictionaryPath"}, valid: map[string]any{"projectPath": ".", "dictionaryPath": "DOMAIN_DICTIONARY.md"}},
	"verify_dependencies":       {paths: []string{"projectPath"}, valid: map[string]any{"projectPath": "."}},
	"search_docs":               {paths: []string{"docsPath"}, valid: map[string]any{"docsPath": ".", "query": "anything"}},
	"validate_artifact":         {paths: []string{"artifactPath"}, valid: map[string]any{"artifactPath": "DOMAIN_DICTIONARY.md"}},
}

// Tools that take no filesystem path from the model.
var pathlessTools = map[string]bool{"search_ki": true}

// The L2.3 done-when, through the real registry: every tool that reads the
// filesystem refuses "/" and "../../etc".
func TestEveryPathTakingToolRejectsEscapes(t *testing.T) {
	registry := FrameworkRegistryAt(logging.NewLogger(&bytes.Buffer{}), escapeRoot(t))
	for name, spec := range pathArguments {
		for _, path := range spec.paths {
			for _, escape := range []string{"/", "../../etc"} {
				t.Run(name+" "+path+"="+escape, func(t *testing.T) {
					args := map[string]any{}
					for key, value := range spec.valid {
						args[key] = value
					}
					args[path] = escape
					result := execute(t, registry, name, args)
					if !result.IsError || !strings.Contains(text(result), "outside the workspace root") {
						t.Errorf("%s accepted %s=%q: %s", name, path, escape, text(result))
					}
				})
			}
		}
	}
}

func TestEveryRegisteredToolIsAccountedFor(t *testing.T) {
	registry := FrameworkRegistryAt(logging.NewLogger(&bytes.Buffer{}), tools.WorkspaceRoot{})
	for _, registration := range registry.All() {
		name := registration.Tool.Name()
		if _, covered := pathArguments[name]; !covered && !pathlessTools[name] {
			t.Errorf("tool %q is in neither pathArguments nor pathlessTools", name)
		}
	}
}

// escapeRoot is <base>/workspace/project, with a real <base>/etc beside it and
// a DOMAIN_DICTIONARY.md inside, so an escape is refused for escaping and not
// for naming something missing.
func escapeRoot(t *testing.T) tools.WorkspaceRoot {
	t.Helper()
	base := t.TempDir()
	rootDir := filepath.Join(base, "workspace", "project")
	for _, dir := range []string{rootDir, filepath.Join(base, "etc")} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(rootDir, "DOMAIN_DICTIONARY.md"), []byte("# terms\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	root, err := tools.NewWorkspaceRoot(rootDir)
	if err != nil {
		t.Fatalf("NewWorkspaceRoot: %v", err)
	}
	return root
}

func execute(t *testing.T, registry *domain.Registry, name string, args map[string]any) *domain.ToolResult {
	t.Helper()
	for _, registration := range registry.All() {
		if registration.Tool.Name() != name {
			continue
		}
		result, err := registration.Tool.Execute(context.Background(), domain.ToolRequest{Args: args})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return result
	}
	t.Fatalf("tool %q is not registered", name)
	return nil
}

func text(result *domain.ToolResult) string {
	var parts []string
	for _, content := range result.Content {
		parts = append(parts, content.Text)
	}
	return strings.Join(parts, " ")
}

func TestTheDefaultRootIsTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	root := WorkingDirectoryRoot(logging.NewLogger(&bytes.Buffer{}))
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if root.Dir() != resolved {
		t.Errorf("default root = %q, want the working directory %q", root.Dir(), resolved)
	}
	if count := len(FrameworkRegistry(logging.NewLogger(&bytes.Buffer{})).All()); count != len(pathArguments)+len(pathlessTools) {
		t.Errorf("FrameworkRegistry registered %d tools", count)
	}
}

// A server that cannot say where its workspace is must read nowhere: when the
// working directory is gone, the default root rejects every path.
func TestAVanishedWorkingDirectoryYieldsARootThatRejectsEverything(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vanishing")
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Chdir(dir)
	if err := os.Remove(dir); err != nil {
		t.Fatalf("remove: %v", err)
	}
	var logs bytes.Buffer
	root := WorkingDirectoryRoot(logging.NewLogger(&logs))
	if root.Dir() != "" {
		t.Skipf("this platform still resolves a removed working directory (%q)", root.Dir())
	}
	if _, err := root.Resolve("/"); err == nil {
		t.Error("the fallback root accepted a path")
	}
	if !strings.Contains(logs.String(), "every path argument will be rejected") {
		t.Errorf("the fallback was not logged: %s", logs.String())
	}
}
