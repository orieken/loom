package mutation_test

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"

	"github.com/orieken/loom/internal/diffcover"
	"github.com/orieken/loom/internal/mutation"
)

func touch(t *testing.T, root string, paths ...string) {
	t.Helper()
	for _, name := range paths {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		// Each file declares the package its directory names, as Go code
		// normally does; the mismatch case is written explicitly below.
		clause := "package " + filepath.Base(filepath.Dir(full)) + "\n"
		if filepath.Dir(full) == root {
			// The temporary root's name is not an identifier; like this
			// repository (directory ai-assistant-dot-files, package loom),
			// the root package is named unlike its directory.
			clause = "package root\n"
		}
		if err := os.WriteFile(full, []byte(clause), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func TestPlanConfinesEachPackageToItsChangedFiles(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "pkg/a.go", "pkg/data.go", "pkg/a_test.go", "pkg/sub/c.go", "main.go", "other.go")

	targets, err := mutation.Plan(root, diffcover.Changes{
		"pkg/a.go": {3, 4},
		"main.go":  {1},
	}, nil)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	want := []mutation.Target{
		{Dir: ".", Exclude: []string{`^other\.go$`, "/"}, Integration: true},
		{Dir: "pkg", Exclude: []string{`^data\.go$`, "/"}},
	}
	if !reflect.DeepEqual(targets, want) {
		t.Errorf("targets = %+v\nwant      %+v", targets, want)
	}
}

// gremlins matches --exclude-files against the path relative to the target.
// An unanchored "a.go" would also exclude "data.go" — the changed file's
// neighbour or, worse, the changed file itself.
func TestExclusionsMatchOnlyTheFileTheyName(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "pkg/data.go", "pkg/a.go")

	targets, err := mutation.Plan(root, diffcover.Changes{"pkg/data.go": {1}}, nil)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, pattern := range targets[0].Exclude {
		if regexp.MustCompile(pattern).MatchString("data.go") {
			t.Errorf("pattern %q excludes the changed file data.go", pattern)
		}
	}
	if !regexp.MustCompile(targets[0].Exclude[0]).MatchString("a.go") {
		t.Errorf("pattern %q does not exclude the unchanged a.go", targets[0].Exclude[0])
	}
}

func TestPlanSkipsWhatIsNotMutableProductionCode(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "pkg/a_test.go", "examples/embedding/main.go", "pkg/testdata/f.go")

	targets, err := mutation.Plan(root, diffcover.Changes{
		"pkg/a_test.go":              {1},
		"pkg/testdata/f.go":          {1},
		"examples/embedding/main.go": {1},
		"docs/readme.md":             {1},
		"pkg/deleted_only.go":        {},
	}, map[string]bool{"examples/embedding/": true})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("targets = %+v, want none", targets)
	}
}

// gremlins v0.6.0 tests the wrong package when a package's name differs from
// its directory's, reporting every mutant as LIVED; those targets must run in
// integration mode.
func TestPackagesNamedUnlikeTheirDirectoryRunInIntegrationMode(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "pkg/a.go")
	for name, clause := range map[string]string{"cmd/tool/main.go": "package main\n", "fs/fs.go": "package frameworkfs\n"} {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(clause), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	targets, err := mutation.Plan(root, diffcover.Changes{"pkg/a.go": {1}, "cmd/tool/main.go": {1}, "fs/fs.go": {1}}, nil)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	integration := map[string]bool{}
	for _, target := range targets {
		integration[target.Dir] = target.Integration
	}
	if want := map[string]bool{"cmd/tool": true, "fs": true, "pkg": false}; !reflect.DeepEqual(integration, want) {
		t.Errorf("integration by dir = %v, want %v", integration, want)
	}
}

func TestPlanReportsAChangedFileItCannotParse(t *testing.T) {
	root := t.TempDir()
	full := filepath.Join(root, "pkg", "a.go")
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte("not go at all"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := mutation.Plan(root, diffcover.Changes{"pkg/a.go": {1}}, nil); err == nil {
		t.Error("Plan accepted a changed file whose package clause it could not read")
	}
}

func TestPlanReportsAChangedPackageItCannotRead(t *testing.T) {
	if _, err := mutation.Plan(t.TempDir(), diffcover.Changes{"missing/a.go": {1}}, nil); err == nil {
		t.Error("Plan accepted a changed package directory that does not exist")
	}
}
