package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/planfile"
	"github.com/spf13/cobra"
)

// A composed plan is subject to exactly the rules a hand-written one is
// (roadmap L3.37).
//
// The form gathers a name and a stage set and nothing else. Validation is
// the loader's, so a rule added there covers this command for free — and a
// form with its own idea of what is legal is how the two drift.
func TestAnUnrunnablePlanIsRefusedWithTheLoadersReason(t *testing.T) {
	directory := t.TempDir()
	// qa-engineer reads the analyst's state, so dropping the analyst and
	// keeping qa-engineer is a plan the executor could not run.
	composed := &draft{name: "no-analyst", stages: []string{
		"context-engineer", "developer", "code-reviewer", "qa-engineer",
	}}

	err := composed.write(discardCommand(), directory)

	if err == nil {
		t.Fatal("an unrunnable plan was accepted")
	}
	if !strings.Contains(err.Error(), "reads state from") {
		t.Errorf("error %q is not the loader's — the form is validating on its own", err)
	}
	if _, statErr := os.Stat(filepath.Join(directory, "no-analyst.yaml")); statErr == nil {
		t.Error("a plan that cannot be run was still written to disk")
	}
}

// What the form writes must be what the loader reads, byte for byte — the
// file is the artifact, not an export of some in-memory plan.
func TestAComposedPlanRoundTripsThroughTheLoader(t *testing.T) {
	directory := t.TempDir()
	// security-reviewer consumes developer, so a genuine review-only plan
	// cannot include it — the loader refuses that combination, which is
	// correct: a security review of an implementation needs one.
	composed := &draft{
		name:        "review-only",
		description: "Read the code, change nothing.",
		stages:      []string{"context-engineer", "analyst", "code-reviewer"},
	}

	if err := composed.write(discardCommand(), directory); err != nil {
		t.Fatalf("write: %v", err)
	}

	source, err := os.ReadFile(filepath.Join(directory, "review-only.yaml"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	plan, err := planfile.Parse(source, "review-only.yaml")
	if err != nil {
		t.Fatalf("the file this command wrote does not parse: %v", err)
	}
	if plan.Name != "review-only" || len(plan.Stages) != 3 {
		t.Errorf("parsed plan = %q with %d stages", plan.Name, len(plan.Stages))
	}
}

// Writing over an existing plan would discard work with no way back.
func TestAnExistingPlanIsNotOverwritten(t *testing.T) {
	directory := t.TempDir()
	composed := &draft{name: "taken", stages: []string{"context-engineer", "analyst"}}
	if err := composed.write(discardCommand(), directory); err != nil {
		t.Fatalf("first write: %v", err)
	}

	err := composed.write(discardCommand(), directory)

	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v, want a refusal to overwrite", err)
	}
}

func TestPlanNamesThatCannotBeFilenamesAreRejected(t *testing.T) {
	cases := map[string]string{
		"empty":            "   ",
		"the built-in":     "deliver-feature",
		"has a slash":      "team/plan",
		"has a space":      "my plan",
		"has an extension": "plan.yaml",
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			if err := validatePlanName(value); err == nil {
				t.Errorf("accepted %q", value)
			}
		})
	}
	if err := validatePlanName("deliver-bugfix"); err != nil {
		t.Errorf("rejected a good name: %v", err)
	}
}

// The report says what was left out, which is the interesting half when
// comparing a custom plan against the built-in one.
func TestTheReportNamesWhatThePlanOmits(t *testing.T) {
	omitted := omittedStages([]string{"context-engineer", "analyst", "router", "developer"})

	if len(omitted) == 0 {
		t.Fatal("a four-stage plan omits nothing?")
	}
	joined := joinOrNone(omitted)
	if !strings.Contains(joined, "devops-engineer") {
		t.Errorf("omitted list %q does not name a stage that was left out", joined)
	}
	if joinOrNone(nil) != "nothing" {
		t.Error("an empty omission list must read as nothing, not as an empty string")
	}
}

func discardCommand() *cobra.Command {
	command := &cobra.Command{}
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	return command
}

// A name typed with surrounding whitespace must not become a filename with
// a space in it.
//
// validatePlanName trimmed before checking and the raw value then became the
// filename, so " my-plan " passed validation and produced "my-plan .yaml".
// Found by driving the real form rather than by reading the code — the two
// paths only meet there.
func TestAPlanNameIsNormalisedBeforeItBecomesAFilename(t *testing.T) {
	directory := t.TempDir()
	composed := &draft{name: "  spaced-plan  ", stages: []string{"context-engineer", "analyst"}}

	if err := composed.write(discardCommand(), directory); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := os.Stat(filepath.Join(directory, "spaced-plan.yaml")); err != nil {
		t.Errorf("expected spaced-plan.yaml: %v", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), " ") {
			t.Errorf("wrote a filename containing a space: %q", entry.Name())
		}
	}
}

// The write path validates too. The form's own validator runs while the
// human can still fix a name, but nothing guarantees the form ran at all —
// --name skips that prompt entirely.
func TestTheWritePathRejectsANameTheFormWouldHave(t *testing.T) {
	err := (&draft{name: "deliver-feature", stages: []string{"context-engineer"}}).
		write(discardCommand(), t.TempDir())

	if err == nil || !strings.Contains(err.Error(), "built-in") {
		t.Fatalf("error = %v, want the built-in name refused on the write path", err)
	}
}
