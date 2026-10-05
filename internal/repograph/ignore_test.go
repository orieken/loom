package repograph

import "testing"

func TestIgnorePatterns(t *testing.T) {
	ignore := ParseIgnore(`
# comment
vendor/
*.gen.go
/docs/api
build/**/out
!vendor/keep
`)
	cases := map[string]bool{
		"vendor/x/y.go":      true,
		"a/vendor/y.go":      true,
		"vendor/keep":        false,
		"src/foo.gen.go":     true,
		"docs/api/x.md":      true,
		"sub/docs/api/x.md":  false,
		"build/a/b/out":      true,
		"src/main.go":        false,
		"vendorish/main.go":  false,
		"notes/mygen.go.txt": false,
	}
	for path, want := range cases {
		if got := ignore.Matches(path); got != want {
			t.Errorf("Matches(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestEmptyIgnoreMatchesNothing(t *testing.T) {
	if ParseIgnore("").Matches("anything.go") {
		t.Error("an empty .loomignore must ignore nothing")
	}
}
