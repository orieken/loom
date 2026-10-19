package capture

import (
	"strings"
	"testing"
)

func TestPathPolicyDecide(t *testing.T) {
	policy := NewPathPolicy("/proj", newTestRedactor(t))
	longInside := strings.Repeat("a", MaxPathBytes)
	tests := []struct {
		name        string
		raw         string
		isRedacted  bool
		wantInput   string
		wantDisplay string
	}{
		{"absolute path inside the project", "/proj/src/a.go", false, "src/a.go", "src/a.go"},
		{"relative path inside the project", "src/a.go", false, "src/a.go", "src/a.go"},
		{"path is cleaned", "/proj/src/../src/a.go", false, "src/a.go", "src/a.go"},
		{"path on the redaction list", "/proj/.env.local", true, ".env.local", ""},
		{"path outside the project", "/etc/passwd", true, "/etc/passwd", ""},
		{"dot-dot escape after cleaning", "/proj/../etc/x", true, "/etc/x", ""},
		{"relative escape", "../x", true, "../x", ""},
		{"the project root itself", "/proj", true, "/proj", ""},
		{"path at the byte cap is kept", longInside, false, longInside, longInside},
		{"path over the byte cap is redacted", longInside + "b", true, longInside + "b", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := policy.Decide(tt.raw)
			if got.IsRedacted != tt.isRedacted {
				t.Errorf("IsRedacted = %v, want %v", got.IsRedacted, tt.isRedacted)
			}
			assertDisplay(t, got.Display, tt.wantDisplay)
			if got.HashInput != tt.wantInput {
				t.Errorf("HashInput = %q, want %q", got.HashInput, tt.wantInput)
			}
		})
	}
}

func assertDisplay(t *testing.T, got *string, want string) {
	t.Helper()
	if want == "" {
		if got != nil {
			t.Errorf("Display = %q, want nil", *got)
		}
		return
	}
	if got == nil || *got != want {
		t.Errorf("Display = %v, want %q", got, want)
	}
}
