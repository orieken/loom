package capture

import "testing"

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"**/secrets/**", "a/secrets/key.txt", true},
		{"**/secrets/**", "secrets/key.txt", true},
		{"**/secrets/**", "a/b/secrets/c/d", true},
		{"**/secrets/**", "a/secretsx/key.txt", false},
		{"**/secrets/**", "a/b/key.txt", false},
		{"*.go", "internal/a/b.go", true},
		{"*.go", "a.txt", false},
		{"a?c", "dir/abc", true},
		{"internal/*/x.go", "internal/a/x.go", true},
		{"internal/*/x.go", "internal/a/b/x.go", false},
		{"**", "a/b", true},
		{"a/**/z", "a/z", true},
		{"a/**/z", "a/b/c/z", true},
		{"a/**/z", "a/b/c/y", false},
		{"/abs/**", "/abs/x/y", true},
		{"/abs/**", "/other/x", false},
		{"src/*.go", "src", false},
		{"[", "a", false},
		{"*.go", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.pattern+" on "+tt.name, func(t *testing.T) {
			if got := MatchGlob(tt.pattern, tt.name); got != tt.want {
				t.Errorf("MatchGlob(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
			}
		})
	}
}

func TestValidateGlob(t *testing.T) {
	tests := []struct {
		pattern string
		isValid bool
	}{
		{"**/secrets/**", true},
		{"*.pem", true},
		{"[unclosed", false},
		{"ok/[bad", false},
	}
	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			if err := ValidateGlob(tt.pattern); (err == nil) != tt.isValid {
				t.Errorf("ValidateGlob(%q) error = %v, want valid=%v", tt.pattern, err, tt.isValid)
			}
		})
	}
}
