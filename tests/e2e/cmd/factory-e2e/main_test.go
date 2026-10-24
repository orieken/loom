package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runMain(args ...string) (int, string) {
	var out bytes.Buffer
	return run(args, &out), out.String()
}

func TestUsageErrorsExitTwo(t *testing.T) {
	for _, args := range [][]string{nil, {"bogus"}, {"run"}, {"deliver", "--no-such-flag"}} {
		if code, output := runMain(args...); code != exitBroken {
			t.Errorf("%v exit %d, want %d\n%s", args, code, exitBroken, output)
		}
	}
}

func TestAHarnessThatCannotRunExitsTwo(t *testing.T) {
	code, output := runMain("verify", "--fixture", t.TempDir(), "--workdir", t.TempDir())
	if code != exitBroken || !strings.Contains(output, "no delivery to verify") {
		t.Errorf("exit %d, output %q", code, output)
	}
}

func TestAFailedCheckExitsOne(t *testing.T) {
	fixture, err := filepath.Abs(filepath.Join("..", "..", "testdata"))
	if err != nil {
		t.Fatal(err)
	}
	code, output := runMain("deliver", "--loom", "/bin/false", "--fixture", fixture, "--workdir", t.TempDir(),
		"--run-arg", "--mock-stage-cost=0.01")
	if code != exitFailed || !strings.Contains(output, "FAIL  the factory shipped the order") {
		t.Errorf("exit %d, output:\n%s", code, output)
	}
}

func TestVerifyingARunThatNeverHappenedExitsOne(t *testing.T) {
	workdir := t.TempDir()
	report := `{"order":"divide","checks":[{"name":"x","passed":true}]}`
	if err := os.WriteFile(filepath.Join(workdir, "report.json"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	fixture, _ := filepath.Abs(filepath.Join("..", "..", "testdata"))
	// A delivery report that passed, but no project behind it: verification
	// finds nothing archived and nothing pushed.
	if code, _ := runMain("verify", "--fixture", fixture, "--workdir", workdir); code != exitFailed {
		t.Errorf("a report with failing verification exited %d, want %d", code, exitFailed)
	}
}
