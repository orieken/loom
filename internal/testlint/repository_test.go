package testlint_test

// The fitness function for ADR-008's third clause, as far as it can be
// decided: no test in this module is unable to fail (roadmap L3.60).
//
// When a test is flagged, the fix is to make it assert something. If it
// genuinely must stay as it is — a compile-only smoke test, say — pin it in
// `permitted` with the reason, keyed by file and test name. Adding a pin is
// meant to be a visible, argued edit.

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/testlint"
)

const moduleRoot = "../.."

// permitted maps "file:TestName" to why that test may have no failure path.
// Empty on 2026-09-22: every one of the module's 600 tests could fail.
var permitted = map[string]string{}

func TestEveryTestInTheModuleCanFail(t *testing.T) {
	for _, finding := range scanModule(t) {
		if _, pinned := permitted[finding.Key()]; pinned {
			continue
		}
		t.Errorf("%s:%d — %s has no path to t.Error, t.Fatal or a helper that calls one, so it passes "+
			"whatever the code does (ADR-008). Make it assert; if it must stay as it is, pin it in "+
			"`permitted` with the reason", finding.File, finding.Line, finding.Test)
	}
}

// A pin for a test that has since been fixed, renamed or deleted is dead
// code that would silently excuse whatever test next takes its name.
func TestEveryPermittedEntryIsStillNeededAndExplained(t *testing.T) {
	flagged := map[string]bool{}
	for _, finding := range scanModule(t) {
		flagged[finding.Key()] = true
	}
	for key, reason := range permitted {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("permitted[%q] gives no reason", key)
		}
		if !flagged[key] {
			t.Errorf("permitted[%q] no longer matches a flagged test — remove the pin", key)
		}
	}
}

func scanModule(t *testing.T) []testlint.Finding {
	t.Helper()
	findings, err := testlint.Scan(moduleRoot, modulePath(t))
	if err != nil {
		t.Fatalf("scan module: %v", err)
	}
	return findings
}

func modulePath(t *testing.T) string {
	t.Helper()
	goMod, err := os.Open(filepath.Join(moduleRoot, "go.mod"))
	if err != nil {
		t.Fatalf("open go.mod: %v", err)
	}
	defer goMod.Close()
	scanner := bufio.NewScanner(goMod)
	for scanner.Scan() {
		if path, found := strings.CutPrefix(scanner.Text(), "module "); found {
			return strings.TrimSpace(path)
		}
	}
	t.Fatal("go.mod declares no module")
	return ""
}
