package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/shared/mcp/internal/analyzers"
	"github.com/orieken/loom/shared/mcp/internal/domain"
)

const toolTestContract = "# Contract: analysis.md\n\n" +
	"## Required Sections (exact heading text and level)\n" +
	"- `## Summary`\n" +
	"- `## Definition of Done`\n"

func TestValidateArtifactToolInfersContractFromFilename(t *testing.T) {
	contractsDir := t.TempDir()
	WriteFile(t, filepath.Join(contractsDir, "analysis-contract.md"), toolTestContract)
	artifactPath := filepath.Join(t.TempDir(), "analysis.md")
	WriteFile(t, artifactPath, "## Summary\n\n## Definition of Done\n")

	result := executeValidateArtifact(t, contractsDir, filepath.Dir(artifactPath), map[string]any{"artifactPath": artifactPath})

	var parsed analyzers.ArtifactValidationResult
	if err := json.Unmarshal([]byte(ExtractText(t, result)), &parsed); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if parsed.Status != analyzers.ArtifactStatusPass {
		t.Errorf("status = %q, want PASS (violations %v)", parsed.Status, parsed.Violations)
	}
}

func TestValidateArtifactToolErrors(t *testing.T) {
	workspace := t.TempDir()
	artifactPath := filepath.Join(workspace, "analysis.md")
	WriteFile(t, artifactPath, "## Summary\n")
	// An artifact whose name maps to no contract. It must exist inside the
	// workspace root, or path resolution (L2.3) rejects it before the
	// contract lookup this case is about.
	mysteryPath := filepath.Join(workspace, "mystery.md")
	WriteFile(t, mysteryPath, "## Anything\n")
	cases := []struct {
		name         string
		contractsDir string
		args         map[string]any
		wantSubstr   string
	}{
		{
			name:       "missing artifactPath",
			args:       map[string]any{},
			wantSubstr: "artifactPath is required",
		},
		{
			name:       "unknown artifact with no explicit contract",
			args:       map[string]any{"artifactPath": mysteryPath},
			wantSubstr: "no known contract",
		},
		{
			name:       "known artifact but contracts dir unknown",
			args:       map[string]any{"artifactPath": artifactPath},
			wantSubstr: "AI_ASSISTANT_DOTFILES_PATH",
		},
		{
			name: "explicit contract path that does not exist",
			args: map[string]any{"artifactPath": artifactPath, "contractPath": "/nonexistent/contract.md"},
			// Rejected at path resolution (L2.3) rather than when the
			// analyzer failed to open it; still an error for a missing contract.
			wantSubstr: "does not exist",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := executeValidateArtifact(t, tc.contractsDir, workspace, tc.args)
			assertErrorResult(t, result, tc.wantSubstr)
		})
	}
}

func executeValidateArtifact(t *testing.T, contractsDir, workspace string, args map[string]any) *domain.ToolResult {
	t.Helper()
	root, err := NewWorkspaceRoot(workspace)
	if err != nil {
		t.Fatalf("workspace root: %v", err)
	}
	tool := NewValidateArtifactTool(SilentLogger(), analyzers.NewArtifactContractAnalyzer(), contractsDir, root)
	result, err := tool.Execute(context.Background(), BuildRequest(args))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return result
}

func assertErrorResult(t *testing.T, result *domain.ToolResult, wantSubstr string) {
	t.Helper()
	if !result.IsError {
		t.Fatalf("want error result, got success: %s", ExtractText(t, result))
	}
	if text := ExtractText(t, result); !strings.Contains(text, wantSubstr) {
		t.Errorf("error text %q missing %q", text, wantSubstr)
	}
}

// An explicit contract is read and echoed back as headings, so it is a path
// argument like any other (L2.3): accepted inside the workspace or the
// framework's contracts directory, refused anywhere else.
func TestAnExplicitContractIsConfinedToTheWorkspaceOrTheContractsDirectory(t *testing.T) {
	workspace, contractsDir, elsewhere := t.TempDir(), t.TempDir(), t.TempDir()
	artifactPath := filepath.Join(workspace, "notes.md")
	WriteFile(t, artifactPath, "## Summary\n\n## Definition of Done\n")
	for dir, name := range map[string]string{workspace: "local-contract.md", contractsDir: "framework-contract.md", elsewhere: "stray-contract.md"} {
		WriteFile(t, filepath.Join(dir, name), toolTestContract)
	}

	for _, accepted := range []string{filepath.Join(workspace, "local-contract.md"), filepath.Join(contractsDir, "framework-contract.md")} {
		result := executeValidateArtifact(t, contractsDir, workspace, map[string]any{"artifactPath": artifactPath, "contractPath": accepted})
		if result.IsError {
			t.Errorf("contract %s refused: %s", accepted, ExtractText(t, result))
		}
	}
	stray := filepath.Join(elsewhere, "stray-contract.md")
	result := executeValidateArtifact(t, contractsDir, workspace, map[string]any{"artifactPath": artifactPath, "contractPath": stray})
	assertErrorResult(t, result, "outside the workspace root")
}
