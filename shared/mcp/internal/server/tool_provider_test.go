package server

import (
	"bytes"
	"testing"

	"github.com/orieken/loom/shared/mcp/internal/domain"
	"github.com/orieken/loom/shared/mcp/internal/logging"
	"github.com/orieken/loom/shared/mcp/internal/tools"
)

func TestBuildFrameworkRegistryRegistersAllFrameworkTools(t *testing.T) {
	registry := buildFrameworkRegistry(logging.NewLogger(&bytes.Buffer{}), tools.WorkspaceRoot{})

	want := []string{
		"analyze_complexity",
		"check_accessibility",
		"check_ubiquitous_language",
		"search_docs",
		"search_ki",
		"validate_artifact",
		"verify_dependencies",
	}
	registrations := registry.All()
	if len(registrations) != len(want) {
		t.Fatalf("registry has %d tools, want %d", len(registrations), len(want))
	}
	for i, name := range want {
		assertFrameworkRegistration(t, registrations[i], name)
	}
}

func assertFrameworkRegistration(t *testing.T, registration domain.ToolRegistration, wantName string) {
	t.Helper()
	if got := registration.Tool.Name(); got != wantName {
		t.Errorf("tool name = %q, want %q", got, wantName)
	}
	if registration.Timeout <= 0 {
		t.Errorf("tool %s has no timeout budget declared", wantName)
	}
	if registration.Retry != domain.RetryIdempotent {
		t.Errorf("tool %s retry class = %q, want %q", wantName, registration.Retry, domain.RetryIdempotent)
	}
	if registration.Permission != domain.ScopeReadOnly {
		t.Errorf("tool %s permission = %q, want %q", wantName, registration.Permission, domain.ScopeReadOnly)
	}
}
