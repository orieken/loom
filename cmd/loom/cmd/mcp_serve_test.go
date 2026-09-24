package cmd

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/orieken/loom/cmd/loom/internal/mcpprobe"
)

var expectedMCPToolNames = []string{
	"analyze_complexity",
	"check_accessibility",
	"check_ubiquitous_language",
	"search_docs",
	"search_ki",
	"validate_artifact",
	"verify_dependencies",
}

// TestMCPServeAnswersToolsList spawns the built loom binary, performs an MCP
// initialize + tools/list handshake over stdio, and asserts all framework
// tool names are present. Doubles as live coverage of the mcpprobe package
// that `loom health` uses for its maturity assessment.
func TestMCPServeAnswersToolsList(t *testing.T) {
	binary := buildLoomBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	names, err := mcpprobe.ToolsList(ctx, t.TempDir(), binary, "mcp", "serve")
	if err != nil {
		t.Fatalf("MCP handshake: %v", err)
	}
	assertToolNames(t, names)
}

func buildLoomBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "loom")
	build := exec.Command("go", "build", "-o", binary, "github.com/orieken/loom/cmd/loom")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build loom binary: %v\n%s", err, output)
	}
	return binary
}

func assertToolNames(t *testing.T, names []string) {
	t.Helper()
	present := make(map[string]bool, len(names))
	for _, name := range names {
		present[name] = true
	}
	for _, expected := range expectedMCPToolNames {
		if !present[expected] {
			t.Errorf("tools/list missing %q (got %v)", expected, names)
		}
	}
	if len(names) != len(expectedMCPToolNames) {
		t.Errorf("tools/list returned %d tools, want %d: %v", len(names), len(expectedMCPToolNames), names)
	}
}

// A --root that does not exist stops the server before it serves (L2.3),
// rather than surfacing on the first tool call.
func TestMCPServeRefusesARootThatDoesNotExist(t *testing.T) {
	previous := mcpServeArgs
	t.Cleanup(func() { mcpServeArgs = previous })
	mcpServeArgs = mcpServeFlags{root: filepath.Join(t.TempDir(), "missing")}
	if err := runMCPServe(nil, nil); err == nil {
		t.Error("mcp serve started with a root that does not exist")
	}
}
