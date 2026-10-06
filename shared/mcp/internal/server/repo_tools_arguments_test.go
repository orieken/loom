package server

import (
	"bytes"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/repoquery/fixture"
	"github.com/orieken/loom/shared/mcp/internal/logging"
	"github.com/orieken/loom/shared/mcp/internal/tools"
)

// impact and entrypoints do not fit mainArgument's "one required string", so
// their arguments are checked here, through the same handler the MCP server
// registers (roadmap L2.1).
func TestImpactValidatesItsArrayArgument(t *testing.T) {
	for name, args := range map[string]map[string]any{
		"paths wrong type":    {"paths": 42},
		"paths missing":       {},
		"paths empty":         {"paths": []any{}},
		"path wrong type":     {"paths": []any{42}},
		"depth below minimum": {"paths": []any{"a"}, "depth": 0},
		"depth above maximum": {"paths": []any{"a"}, "depth": 99},
		"unknown argument":    {"paths": []any{"a"}, "pths": []any{"b"}},
	} {
		t.Run(name, func(t *testing.T) {
			result, report := callTool(t, "impact", args)
			if !result.IsError || len(report.Error.Violations) == 0 {
				t.Errorf("a malformed impact call must be a field-level validation error: %+v", report)
			}
		})
	}
	if _, report := callTool(t, "impact", map[string]any{"paths": []any{"a"}, "depth": 2}); strings.HasPrefix(report.Error.Message, "invalid arguments") {
		t.Errorf("a well-formed call was rejected: %+v", report.Error.Violations)
	}
}

func TestEntrypointsValidatesItsOptionalLimit(t *testing.T) {
	result, report := callTool(t, "entrypoints", map[string]any{"limit": "ten"})
	assertViolation(t, result, report, "limit", "integer")
	result, report = callTool(t, "entrypoints", map[string]any{"limt": 5})
	assertViolation(t, result, report, "limt", "not an argument")
	if _, report := callTool(t, "entrypoints", nil); strings.HasPrefix(report.Error.Message, "invalid arguments") {
		t.Errorf("entrypoints needs no arguments: %+v", report.Error.Violations)
	}
}

func TestFindFilesNeedsATagOrAQuery(t *testing.T) {
	result, report := callTool(t, "find_files", map[string]any{})
	if !result.IsError || len(report.Error.Violations) == 0 {
		t.Errorf("neither tag nor query must be rejected before the tool runs: %+v", report)
	}
	for _, args := range []map[string]any{{"tag": "state"}, {"query": "records"}, {"tag": "state", "query": "records"}} {
		if _, report := callTool(t, "find_files", args); strings.HasPrefix(report.Error.Message, "invalid arguments") {
			t.Errorf("%v rejected: %+v", args, report.Error.Violations)
		}
	}
}

// The tools take a repo path as a key into the stored graph, never as a file to
// open, so the workspace root has nothing to confine. This proves it through
// the real registry: an escape is refused, and nothing outside is read.
func TestTheRepoToolsRefuseEscapesThroughTheRealRegistry(t *testing.T) {
	root, err := tools.NewWorkspaceRoot(fixture.Indexed(t, map[string]string{
		"go.mod": "module example.com/x\n\ngo 1.22\n", "store/store.go": "package store\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	registry := FrameworkRegistryAt(logging.NewLogger(&bytes.Buffer{}), root)
	for _, name := range []string{"dependencies_of", "dependents_of", "describe"} {
		for _, escape := range []string{"/", "../../etc", "/etc/passwd"} {
			result := execute(t, registry, name, map[string]any{"path": escape})
			if !result.IsError || !strings.Contains(text(result), "not a directory or source file in the repo graph") {
				t.Errorf("%s accepted %q: %s", name, escape, text(result))
			}
		}
	}
	if result := execute(t, registry, "impact", map[string]any{"paths": []any{"../../etc"}}); !result.IsError {
		t.Error("impact accepted an escaping path")
	}
}

func TestTheRepoToolsAnswerThroughTheRealRegistryWithAFreshGraph(t *testing.T) {
	root, err := tools.NewWorkspaceRoot(fixture.Indexed(t, map[string]string{
		"go.mod": "module example.com/x\n\ngo 1.22\n", "main.go": "package main\n\nfunc main() {}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	registry := FrameworkRegistryAt(logging.NewLogger(&bytes.Buffer{}), root)
	result := execute(t, registry, "entrypoints", map[string]any{})
	if result.IsError || !strings.Contains(text(result), `"fresh":true`) || !strings.Contains(text(result), `"main.go"`) {
		t.Errorf("%s", text(result))
	}
}
