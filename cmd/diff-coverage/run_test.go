package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// profile: in pkg/a.go, lines 1-4 ran and 5-8 did not.
const profile = `mode: set
example.com/m/pkg/a.go:1.1,4.2 3 1
example.com/m/pkg/a.go:5.1,8.2 3 0
`

func diffOf(text string) diffSource {
	return func(string) (io.Reader, error) { return strings.NewReader(text), nil }
}

// project makes a temporary module with a profile and chdirs into it, since
// diff-coverage reads go.mod and the profile from the working directory.
func project(t *testing.T, goMod string) {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), goMod)
	write(t, filepath.Join(dir, "coverage.out"), profile)
	t.Chdir(dir)
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runWith(diff diffSource, args ...string) (int, string) {
	var out bytes.Buffer
	code := run(args, &out, diff)
	return code, out.String()
}

func TestChangedLinesThatRanPass(t *testing.T) {
	project(t, "module example.com/m\n")
	code, out := runWith(diffOf("+++ b/pkg/a.go\n@@ -1 +1,3 @@\n"), "--base", "main")
	if code != exitPass || !strings.Contains(out, "100.0%") || !strings.Contains(out, "PASS") {
		t.Errorf("exit %d, output:\n%s", code, out)
	}
}

func TestChangedLinesBelowThresholdFailAndAreNamed(t *testing.T) {
	project(t, "module example.com/m\n")
	code, out := runWith(diffOf("+++ b/pkg/a.go\n@@ -1 +3,4 @@\n"), "--base", "main")
	if code != exitFail {
		t.Errorf("exit %d, want %d; output:\n%s", code, exitFail, out)
	}
	for _, want := range []string{"50.0%", "UNCOVERED   pkg/a.go:5", "UNCOVERED   pkg/a.go:6", "FAIL"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestThresholdFlagMovesTheBar(t *testing.T) {
	project(t, "module example.com/m\n")
	code, out := runWith(diffOf("+++ b/pkg/a.go\n@@ -1 +3,4 @@\n"), "--base", "main", "--threshold", "50")
	if code != exitPass {
		t.Errorf("exit %d at a 50%% threshold, want pass; output:\n%s", code, out)
	}
}

func TestAChangedFileMissingFromTheProfileFails(t *testing.T) {
	project(t, "module example.com/m\n")
	code, out := runWith(diffOf("+++ b/pkg/untested.go\n@@ -1 +1,2 @@\n"), "--base", "main")
	if code != exitFail || !strings.Contains(out, "UNMEASURED  pkg/untested.go") {
		t.Errorf("exit %d, output:\n%s", code, out)
	}
}

func TestTheNestedExampleModuleIsExcludedByName(t *testing.T) {
	project(t, "module example.com/m\n")
	code, out := runWith(diffOf("+++ b/examples/embedding/main.go\n@@ -1 +1,9 @@\n"), "--base", "main")
	if code != exitPass || !strings.Contains(out, "EXCLUDED    examples/embedding/main.go") {
		t.Errorf("exit %d, output:\n%s", code, out)
	}
}

func TestUsageErrorsExitTwo(t *testing.T) {
	failingDiff := func(string) (io.Reader, error) { return nil, errors.New("unknown revision") }
	for _, test := range []struct {
		name  string
		goMod string
		diff  diffSource
		args  []string
		setup func(t *testing.T)
	}{
		{name: "no base", goMod: "module example.com/m\n", diff: diffOf(""), args: nil},
		{name: "unknown flag", goMod: "module example.com/m\n", diff: diffOf(""), args: []string{"--nope"}},
		{name: "git fails", goMod: "module example.com/m\n", diff: failingDiff, args: []string{"--base", "x"}},
		{name: "unreadable hunk", goMod: "module example.com/m\n", diff: diffOf("+++ b/a.go\n@@ nonsense\n"), args: []string{"--base", "x"}},
		{name: "no profile", goMod: "module example.com/m\n", diff: diffOf(""), args: []string{"--base", "x", "--profile", "missing.out"}},
		{name: "bad profile", goMod: "module example.com/m\n", diff: diffOf(""), args: []string{"--base", "x", "--profile", "go.mod"}},
		{name: "go.mod names no module", goMod: "go 1.26\n", diff: diffOf(""), args: []string{"--base", "x"}},
		{name: "no go.mod", goMod: "module example.com/m\n", diff: diffOf(""), args: []string{"--base", "x"},
			setup: func(t *testing.T) { t.Helper(); _ = os.Remove("go.mod") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			project(t, test.goMod)
			if test.setup != nil {
				test.setup(t)
			}
			if code, out := runWith(test.diff, test.args...); code != exitUsage {
				t.Errorf("exit %d, want %d; output:\n%s", code, exitUsage, out)
			}
		})
	}
}
