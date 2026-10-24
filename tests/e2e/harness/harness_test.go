package harness

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Roadmap L3.73: the factory, run end to end through the real binary with
// the mock provider, against a fixture and acceptance tests it never sees.
// A change that breaks gate approval, digest verification or intake parsing
// fails here, because the order then never ships or a check below fails.

var (
	buildOnce sync.Once
	loomPath  string
	buildErr  error
)

// loomBinary builds loom once per test binary. A build failure fails the
// test rather than skipping it: a harness that skips is a check that cannot
// fail.
func loomBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "loom-e2e-bin-")
		if err != nil {
			buildErr = err
			return
		}
		loomPath = filepath.Join(dir, "loom")
		command := exec.Command("go", "build", "-o", loomPath, "./cmd/loom")
		command.Dir = repositoryRoot(t)
		if output, err := command.CombinedOutput(); err != nil {
			buildErr = &buildError{err: err, output: string(output)}
		}
	})
	if buildErr != nil {
		t.Fatalf("build loom: %v", buildErr)
	}
	return loomPath
}

type buildError struct {
	err    error
	output string
}

func (e *buildError) Error() string { return e.err.Error() + "\n" + e.output }

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func mockConfig(t *testing.T, runArgs ...string) Config {
	t.Helper()
	var log bytes.Buffer
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("harness log:\n%s", log.String())
		}
	})
	return Config{
		Loom: loomBinary(t), Fixture: fixture(t, ""), Hidden: fixture(t, "hidden"), Workdir: t.TempDir(),
		Provider: "mock", RunArgs: append([]string{"--mock-stage-cost=0.01"}, runArgs...), Output: &log,
	}
}

func TestTheFactoryDeliversTheFixtureOrder(t *testing.T) {
	config := mockConfig(t, "--mock-solution="+fixture(t, "solution"))

	report, err := Run(config)
	if err != nil {
		t.Fatalf("harness: %v", err)
	}

	var printed bytes.Buffer
	Print(&printed, report)
	if !report.Passed() {
		t.Fatalf("the delivered order failed its checks:\n%s", printed.String())
	}
	for _, want := range []string{
		"the factory shipped the order", "recorded digests match every artifact",
		"verification catches a tampered artifact", "every stage settled", "every gate was approved explicitly",
		"the run stayed within the order's budget", "the branch was pushed", "the diff stays inside scope.paths",
		"the hidden acceptance tests pass",
	} {
		if !strings.Contains(printed.String(), "PASS  "+want) {
			t.Errorf("report lacks a passing %q:\n%s", want, printed.String())
		}
	}
	if strings.Join(report.Approved, ",") != "confirm-design,confirm-security,confirm-ship,commit" {
		t.Errorf("approved %v, want each run gate and then the commit", report.Approved)
	}
}

// The control: a run whose developer writes nothing useful must fail the
// harness — out of scope, and the hidden tests cannot even compile.
func TestTheHarnessFailsARunThatDeliversTheWrongThing(t *testing.T) {
	report, err := Run(mockConfig(t))
	if err != nil {
		t.Fatalf("harness: %v", err)
	}
	if report.Passed() {
		t.Fatal("a run that delivered placeholders passed the harness")
	}
	failed := map[string]bool{}
	for _, check := range report.Failed() {
		failed[check.Name] = true
	}
	for _, want := range []string{"the diff stays inside scope.paths", "the hidden acceptance tests pass"} {
		if !failed[want] {
			t.Errorf("%q passed for a run that delivered placeholders", want)
		}
	}
}
