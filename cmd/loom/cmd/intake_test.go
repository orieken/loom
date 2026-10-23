package cmd

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

const validIntakeOrder = `---
intake: 1
id: lockout
type: feature
plan: deliver-feature
status: ready
priority: 2
created: 2026-10-23
requested-by: oscar
source: manual
trust: internal
base-branch: main
finish: pr
autonomy: inherit
budget:
  max-attempts: 2
  max-usd: 10
  max-hours: 4
---
# Lock accounts after repeated failures
`

func runIntakeCheckFor(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var output bytes.Buffer
	intakeCheckCmd.SetOut(&output)
	defer intakeCheckCmd.SetOut(nil)
	err := runIntakeCheck(intakeCheckCmd, args)
	return output.String(), err
}

// The template and README describe orders; expanding a directory skips them.
func TestIntakeCheckSkipsTheTemplateAndReadmeInADirectory(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "lockout.md"), validIntakeOrder)
	writeTestFile(t, filepath.Join(dir, "TEMPLATE.md"), "---\nid: kebab-case-slug\n---\n")
	writeTestFile(t, filepath.Join(dir, "README.md"), "# intake/\n")
	output, err := runIntakeCheckFor(t, dir)
	if err != nil || !strings.Contains(output, "valid: 1 order(s)") {
		t.Errorf("err %v, output %q, want the one order valid", err, output)
	}
}

func TestIntakeCheckNamesEachViolationAndFails(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "lockout.md"), validIntakeOrder)
	broken := filepath.Join(dir, "renamed.md")
	writeTestFile(t, broken, validIntakeOrder)
	output, err := runIntakeCheckFor(t, dir)
	if !errors.Is(err, errIntakeInvalid) || !strings.Contains(err.Error(), "1 of 2") {
		t.Errorf("err = %v, want invalid in 1 of 2", err)
	}
	if !strings.Contains(output, broken+`: id: "lockout" does not match the file name "renamed"`) {
		t.Errorf("output = %q, want file: field: message", output)
	}
}

// Named explicitly, the template is checked — and its placeholders fail.
func TestIntakeCheckChecksAFileNamedExplicitly(t *testing.T) {
	template := filepath.Join(t.TempDir(), "TEMPLATE.md")
	writeTestFile(t, template, "---\nintake: 1\n---\n")
	if _, err := runIntakeCheckFor(t, template); !errors.Is(err, errIntakeInvalid) {
		t.Errorf("err = %v, want the template's placeholders reported", err)
	}
}

// An order that could not be read is never reported valid.
func TestIntakeCheckFailsOnAnUnreadableOrder(t *testing.T) {
	if _, err := runIntakeCheckFor(t, filepath.Join(t.TempDir(), "missing.md")); !errors.Is(err, errIntakeInvalid) {
		t.Errorf("missing path err = %v, want invalid", err)
	}
	locked := filepath.Join(t.TempDir(), "locked.md")
	writeTestFile(t, locked, validIntakeOrder)
	chmodForTest(t, locked, 0o000)
	if _, err := runIntakeCheckFor(t, locked); !errors.Is(err, errIntakeInvalid) {
		t.Errorf("unreadable file err = %v, want invalid", err)
	}
}

func TestIntakeCheckDefaultsToTheIntakeDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestFile(t, filepath.Join("intake", "lockout.md"), validIntakeOrder)
	if output, err := runIntakeCheckFor(t); err != nil || !strings.Contains(output, "valid: 1 order(s)") {
		t.Errorf("err %v, output %q, want intake/ checked by default", err, output)
	}
}

// A plan file that does not parse cannot run, so an order naming it is invalid.
func TestIntakeCheckOffersOnlyPlansThatParse(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestFile(t, filepath.Join(".claude", "plans", "broken.yaml"), "version: [\n")
	plans := runnablePlans()
	if len(plans) != 1 || plans[0] != "deliver-feature" {
		t.Errorf("plans = %v, want only the built-in", plans)
	}
}
