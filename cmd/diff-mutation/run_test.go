package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/mutation"
)

// gremlins reports a.go's mutants relative to the target directory: line 3
// (changed) killed, line 4 (changed) lived, line 9 (unchanged) lived.
const pkgReport = `{"files": [{"file_name": "a.go", "mutations": [
  {"type": "CONDITIONALS_NEGATION", "status": "KILLED", "line": 3, "column": 5},
  {"type": "ARITHMETIC_BASE", "status": "LIVED", "line": 4, "column": 7},
  {"type": "ARITHMETIC_BASE", "status": "LIVED", "line": 9, "column": 7}
]}]}`

const pkgDiff = "+++ b/pkg/a.go\n@@ -3,0 +3,2 @@\n"

func diffOf(text string) diffSource {
	return func(string) (io.Reader, error) { return strings.NewReader(text), nil }
}

// writesReport is a mutationRunner that records its target and writes body
// where gremlins would.
func writesReport(body string, seen *[]mutation.Target) mutationRunner {
	return func(_ string, target mutation.Target, output string) error {
		*seen = append(*seen, target)
		return os.WriteFile(output, []byte(body), 0o600)
	}
}

func repository(t *testing.T, files ...string) {
	t.Helper()
	root := t.TempDir()
	for _, name := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte("package pkg\n"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	t.Chdir(root)
}

func runWith(diff diffSource, mutate mutationRunner, args ...string) (int, string) {
	var out bytes.Buffer
	code := run(args, &out, diff, mutate)
	return code, out.String()
}

func TestReportOnlyScoresChangedLinesAndNamesSurvivors(t *testing.T) {
	repository(t, "pkg/a.go", "pkg/b.go")
	var seen []mutation.Target
	code, out := runWith(diffOf(pkgDiff), writesReport(pkgReport, &seen), "--base", "main")

	if code != exitPass {
		t.Errorf("exit %d, want %d (report-only); output:\n%s", code, exitPass, out)
	}
	for _, want := range []string{"50.0% (1 of 2 tested killed; report-only)", "LIVED       pkg/a.go:4:7 ARITHMETIC_BASE"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "a.go:9") {
		t.Errorf("a mutant on an unchanged line was reported:\n%s", out)
	}
	if len(seen) != 1 || seen[0].Dir != "pkg" {
		t.Errorf("targets = %+v, want one run on pkg", seen)
	}
}

func TestAFloorFailsAScoreBelowIt(t *testing.T) {
	repository(t, "pkg/a.go")
	var seen []mutation.Target
	code, out := runWith(diffOf(pkgDiff), writesReport(pkgReport, &seen), "--base", "main", "--floor", "60")
	if code != exitFail || !strings.Contains(out, "floor 60.0%") {
		t.Errorf("exit %d, want %d; output:\n%s", code, exitFail, out)
	}
}

func TestAFloorPassesAScoreAtIt(t *testing.T) {
	repository(t, "pkg/a.go")
	var seen []mutation.Target
	code, out := runWith(diffOf(pkgDiff), writesReport(pkgReport, &seen), "--base", "main", "--floor", "50")
	if code != exitPass {
		t.Errorf("exit %d at a floor equal to the score; output:\n%s", code, out)
	}
}

// gremlins writes no report when it finds no mutable operator. That is a
// package with nothing to mutate, not an error — and it is said out loud.
func TestAPackageWithNoMutantsHasNothingToScore(t *testing.T) {
	repository(t, "pkg/a.go")
	silent := func(string, mutation.Target, string) error { return nil }
	code, out := runWith(diffOf(pkgDiff), silent, "--base", "main", "--floor", "90")
	if code != exitPass || !strings.Contains(out, "nothing to score") {
		t.Errorf("exit %d; output:\n%s", code, out)
	}
}

func TestNoChangedGoMeansNoRun(t *testing.T) {
	repository(t)
	var seen []mutation.Target
	code, out := runWith(diffOf(""), writesReport(pkgReport, &seen), "--base", "main")
	if code != exitPass || len(seen) != 0 || !strings.Contains(out, "mutating 0 changed package(s)") {
		t.Errorf("exit %d, runs %d; output:\n%s", code, len(seen), out)
	}
}

func TestErrorsExitTwo(t *testing.T) {
	var seen []mutation.Target
	failingDiff := func(string) (io.Reader, error) { return nil, errors.New("unknown revision") }
	failingRun := func(string, mutation.Target, string) error { return errors.New("tests failed before mutation") }
	for _, test := range []struct {
		name   string
		diff   diffSource
		mutate mutationRunner
		args   []string
	}{
		{"no base", diffOf(pkgDiff), writesReport(pkgReport, &seen), nil},
		{"unknown flag", diffOf(pkgDiff), writesReport(pkgReport, &seen), []string{"--nope"}},
		{"git fails", failingDiff, writesReport(pkgReport, &seen), []string{"--base", "x"}},
		{"unreadable diff", diffOf("+++ b/pkg/a.go\n@@ nonsense\n"), writesReport(pkgReport, &seen), []string{"--base", "x"}},
		{"changed package missing", diffOf("+++ b/gone/a.go\n@@ -0,0 +1 @@\n"), writesReport(pkgReport, &seen), []string{"--base", "x"}},
		{"gremlins fails", diffOf(pkgDiff), failingRun, []string{"--base", "x"}},
		{"report unreadable", diffOf(pkgDiff), writesReport("{", &seen), []string{"--base", "x"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository(t, "pkg/a.go")
			if code, out := runWith(test.diff, test.mutate, test.args...); code != exitUsage {
				t.Errorf("exit %d, want %d; output:\n%s", code, exitUsage, out)
			}
		})
	}
}

// A report that exists but cannot be opened is an error, not "no mutants":
// only a missing report means gremlins found nothing. The first version made
// the report path a directory — which os.Open accepts on Unix — so the error
// came from JSON decoding and a mutant treating every open error as "no
// mutants" survived. An unreadable file is a real open failure.
func TestAnUnreadableReportIsAnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a 0o000 file, so there is no open error to provoke")
	}
	repository(t, "pkg/a.go")
	unreadable := func(_ string, _ mutation.Target, output string) error {
		return os.WriteFile(output, []byte(pkgReport), 0o000)
	}
	if code, out := runWith(diffOf(pkgDiff), unreadable, "--base", "x"); code != exitUsage {
		t.Errorf("exit %d, want %d; output:\n%s", code, exitUsage, out)
	}
}

func TestTimedOutNotViableAndNotCoveredAreListed(t *testing.T) {
	repository(t, "pkg/a.go")
	report := `{"files": [{"file_name": "a.go", "mutations": [
	  {"type": "T", "status": "TIMED OUT", "line": 3, "column": 1},
	  {"type": "V", "status": "NOT VIABLE", "line": 3, "column": 2},
	  {"type": "C", "status": "NOT COVERED", "line": 4, "column": 3}]}]}`
	var seen []mutation.Target
	_, out := runWith(diffOf(pkgDiff), writesReport(report, &seen), "--base", "main")
	for _, want := range []string{"TIMED OUT   pkg/a.go:3:1", "NOT VIABLE  pkg/a.go:3:2", "NOT COVERED pkg/a.go:4:3", "0.0% (0 of 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}
