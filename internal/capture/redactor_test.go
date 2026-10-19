package capture

import (
	"strings"
	"testing"
)

func TestRedactorDefaultList(t *testing.T) {
	redactor := newTestRedactor(t)
	tests := []struct {
		path       string
		isRedacted bool
	}{
		{".env", true},
		{".env.local", true},
		{".env.d/key.txt", true},
		{"config/.env.production", true},
		{"secrets/key.txt", true},
		{"deploy/secrets/token", true},
		{"server.pem", true},
		{"certs/tls.key", true},
		{"deploy/credentials.json", true},
		{"credentials", true},
		{"src/main.go", false},
		{"envelope.txt", false},
		{"README.md", false},
		{"docs/secretsauce/recipe.md", false},
		{"internal/keys/store.go", false},
		{"keyboard.go", false},
		{"pemfile.go", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := redactor.IsRedacted(tt.path, ""); got != tt.isRedacted {
				t.Errorf("IsRedacted(%q) = %v, want %v", tt.path, got, tt.isRedacted)
			}
		})
	}
}

func TestRedactorHomePatternUsesTheInjectedHome(t *testing.T) {
	redactor := newTestRedactor(t)

	if !redactor.IsRedacted("", "/home/user/.ssh/id_ed25519") {
		t.Error("a key under the injected home .ssh must be redacted")
	}
	if redactor.IsRedacted("", "/home/other/.ssh/id_ed25519") {
		t.Error("another home must not match the injected home pattern")
	}
}

func TestRedactorWithoutHomeDropsTheHomePattern(t *testing.T) {
	redactor, err := NewRedactor(nil, "")
	if err != nil {
		t.Fatal(err)
	}

	if redactor.IsRedacted("", "/home/user/.ssh/id_rsa") {
		t.Error("with no home the ~ pattern has nothing to expand to")
	}
}

func TestRedactorAddsConfiguredGlobs(t *testing.T) {
	redactor := newTestRedactor(t, "private/**", "*.secret")

	for _, path := range []string{"private/notes.md", "a/b/token.secret"} {
		if !redactor.IsRedacted(path, "") {
			t.Errorf("%q must be redacted by the configured glob", path)
		}
	}
	if redactor.IsRedacted("public/notes.md", "") {
		t.Error("an unrelated path must not be redacted")
	}
}

func TestRedactorRejectsABadGlobSoTheListFailsClosed(t *testing.T) {
	_, err := NewRedactor([]string{"[broken"}, "/home/user")

	if err == nil || !strings.Contains(err.Error(), "bad glob") {
		t.Errorf("error = %v, want a bad glob error", err)
	}
}
