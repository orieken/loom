package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
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
	plans := runnablePlans(".")
	if len(plans) != 1 || plans[0] != "deliver-feature" {
		t.Errorf("plans = %v, want only the built-in", plans)
	}
}

// Review item I-5: plans are found from the repository root, so the answer
// does not change with the directory the command runs in.
func TestIntakeCheckFindsProjectPlansFromASubdirectory(t *testing.T) {
	root := t.TempDir()
	gitForTest(t, root, "init", "-q")
	writeTestFile(t, filepath.Join(root, ".claude", "plans", "custom.yaml"), customPlanForTest)
	order := strings.Replace(validIntakeOrder, "plan: deliver-feature", "plan: custom", 1)
	writeTestFile(t, filepath.Join(root, "intake", "lockout.md"), order)
	t.Chdir(filepath.Join(root, "intake"))

	if output, err := runIntakeCheckFor(t, "lockout.md"); err != nil {
		t.Errorf("from a subdirectory the project's plan vanished: %v\n%s", err, output)
	}
}

// Review item I-10: an order sharing a feature spec's stem shares its workspace.
func TestIntakeCheckRejectsAnOrderNamedLikeAFeatureSpec(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestFile(t, filepath.Join("intake", "lockout.md"), validIntakeOrder)
	writeTestFile(t, filepath.Join("features", "lockout.md"), "# Feature: lockout\n")

	output, err := runIntakeCheckFor(t)
	if !errors.Is(err, errIntakeInvalid) || !strings.Contains(output, "features/lockout.md has the same name") {
		t.Errorf("err %v, output %q, want the shared stem reported", err, output)
	}
}

const reagentMapSpecJSONForTest = `{"criteria":[{"id":"AC1","observableVia":"ui"},{"id":"AC2","observableVia":"api"}]}`

// reagentMapManifestAndDigest builds a sha256sum-style manifest listing
// specBytes alone, and the sha256:<hex> digest of that manifest's own bytes
// — the two values a slice order's frontmatter must agree on.
func reagentMapManifestAndDigest(specBytes []byte) (manifestBytes []byte, digest string) {
	specSum := sha256.Sum256(specBytes)
	manifestBytes = []byte(hex.EncodeToString(specSum[:]) + "  docs/specs/reagent-map-spec/acceptance-criteria.json\n")
	manifestSum := sha256.Sum256(manifestBytes)
	return manifestBytes, "sha256:" + hex.EncodeToString(manifestSum[:])
}

func reagentMapOrderForTest(id, digest, covers string) string {
	return fmt.Sprintf(`---
intake: 1
id: %s
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
slice:
  of: reagent-map-spec
  spec: docs/specs/reagent-map-spec/acceptance-criteria.json
  digest: %s
  manifest: docs/specs/reagent-map-spec/package.sha256
  covers: [%s]
---
# %s
`, id, digest, covers, id)
}

// writeReagentMapSpecFixture writes the spec package and manifest under
// docs/specs/reagent-map-spec/ (relative to the current directory, which
// each caller has already t.Chdir'd into a fresh temp directory) and
// returns the digest every slice order must declare.
func writeReagentMapSpecFixture(t *testing.T) string {
	t.Helper()
	specBytes := []byte(reagentMapSpecJSONForTest)
	writeTestFile(t, filepath.Join("docs", "specs", "reagent-map-spec", "acceptance-criteria.json"), string(specBytes))
	manifestBytes, digest := reagentMapManifestAndDigest(specBytes)
	writeTestFile(t, filepath.Join("docs", "specs", "reagent-map-spec", "package.sha256"), string(manifestBytes))
	return digest
}

// End-to-end proof of AC7 and AC9's passing fixture: two slices that
// together cover every criterion level print one summary line and exit 0.
func TestIntakeCheckVerifiesASliceSplitEndToEnd(t *testing.T) {
	t.Chdir(t.TempDir())
	digest := writeReagentMapSpecFixture(t)
	writeTestFile(t, filepath.Join("intake", "reagent-map-ui.md"), reagentMapOrderForTest("reagent-map-ui", digest, "AC1"))
	writeTestFile(t, filepath.Join("intake", "reagent-map-api.md"), reagentMapOrderForTest("reagent-map-api", digest, "AC2"))

	output, err := runIntakeCheckFor(t)

	if err != nil {
		t.Fatalf("err = %v, output = %s", err, output)
	}
	if !strings.Contains(output, "reagent-map-spec: 2 slice(s), 2 level(s) covered") {
		t.Errorf("output = %q, want the spec summary line", output)
	}
}

// A cross-order violation fails the run even when every file is valid on
// its own, proving the exit-code contract folds in the new check (AC7).
func TestIntakeCheckFailsOnASliceCoverageGap(t *testing.T) {
	t.Chdir(t.TempDir())
	digest := writeReagentMapSpecFixture(t)
	// Both slices cover AC1 only: AC2:api is never covered.
	writeTestFile(t, filepath.Join("intake", "reagent-map-ui.md"), reagentMapOrderForTest("reagent-map-ui", digest, "AC1"))
	writeTestFile(t, filepath.Join("intake", "reagent-map-api.md"), reagentMapOrderForTest("reagent-map-api", digest, "AC1"))

	output, err := runIntakeCheckFor(t)

	if !errors.Is(err, errIntakeInvalid) {
		t.Errorf("err = %v, want invalid: zero per-file violations, one cross-order coverage gap", err)
	}
	if !strings.Contains(output, "AC2:api") {
		t.Errorf("output = %q, want the uncovered AC2:api level named", output)
	}
}

const customPlanForTest = `version: 1
name: custom
stages: [context-engineer, analyst]
`

func gitForTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
