package register

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestFrameworksAtRegistersEveryToolUnderAValidRoot(t *testing.T) {
	registry, err := FrameworksAt(&bytes.Buffer{}, t.TempDir())
	if err != nil {
		t.Fatalf("FrameworksAt: %v", err)
	}
	if count := len(registry.All()); count != 7 {
		t.Errorf("registered %d tools, want 7", count)
	}
}

// A root that does not exist is refused up front, not discovered by the
// first tool call.
func TestARootThatIsNotADirectoryIsRefused(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := FrameworksAt(nil, missing); err == nil {
		t.Error("FrameworksAt accepted a root that does not exist")
	}
	if err := FrameworkToolsTracedAt(server.NewMCPServer("t", "0"), nil, nil, missing); err == nil {
		t.Error("FrameworkToolsTracedAt accepted a root that does not exist")
	}
}

func TestFrameworkToolsTracedAtRegistersOnTheServer(t *testing.T) {
	if err := FrameworkToolsTracedAt(server.NewMCPServer("t", "0"), &bytes.Buffer{}, nil, t.TempDir()); err != nil {
		t.Errorf("FrameworkToolsTracedAt: %v", err)
	}
}
