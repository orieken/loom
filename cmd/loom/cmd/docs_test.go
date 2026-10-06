package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/docchange"
	"github.com/spf13/cobra"
)

// documentedRepo is a git repo whose directories have verified candidates.
func documentedRepo(t *testing.T) string {
	t.Helper()
	root := committedGoRepo(t)
	provider := answerEverything(t, root)
	useDocumentFixtures(t, provider)
	if output, err := runDocumentWith(t, documentFlags{root: root}); err != nil {
		t.Fatalf("documenting: %v\n%s", err, output)
	}
	return root
}

// runDocsCommand runs one `loom docs` subcommand function with captured IO.
func runDocsCommand(t *testing.T, run func(*cobra.Command, []string) error, flags docsFlags, input string, args ...string) (string, error) {
	t.Helper()
	docsArgs = flags
	command := &cobra.Command{}
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetIn(strings.NewReader(input))
	err := run(command, args)
	return out.String(), err
}

func planChange(t *testing.T, root string, flags docsFlags) string {
	t.Helper()
	flags.root = root
	output, err := runDocsCommand(t, runDocsPlan, flags, "")
	if err != nil {
		t.Fatalf("plan: %v\n%s", err, output)
	}
	id, _ := docchange.LatestChange(root)
	return id
}

func TestDocsPlanStagesAChangesetAndSaysNothingWasWrittenToTheRepo(t *testing.T) {
	root := documentedRepo(t)
	output, err := runDocsCommand(t, runDocsPlan, docsFlags{root: root}, "")
	if err != nil {
		t.Fatal(err)
	}
	assertContainsAll(t, output, "directories documented", "to create", "Nothing was written to your repository", "loom docs approve")
	if _, statErr := os.Stat(filepath.Join(root, "CODEMAP.md")); statErr == nil {
		t.Error("`plan` wrote CODEMAP.md into the repository")
	}
}

func TestDocsApproveWithoutATerminalRefusesUnlessToldYouReviewedIt(t *testing.T) {
	root := documentedRepo(t)
	id := planChange(t, root, docsFlags{})
	approvalIsInteractive = func() bool { return false }
	t.Cleanup(func() { approvalIsInteractive = stdinIsInteractive })
	_, err := runDocsCommand(t, runDocsApprove, docsFlags{root: root}, "y\n")
	if !errors.Is(err, errApprovalNeedsTerminal) {
		t.Fatalf("approval must come from a person: %v", err)
	}
	if _, found, _ := docchange.LoadApproval(root, id); found {
		t.Error("an approval was recorded without anyone approving")
	}
}

func asTerminal(t *testing.T) {
	t.Helper()
	approvalIsInteractive = func() bool { return true }
	t.Cleanup(func() { approvalIsInteractive = stdinIsInteractive })
}

func TestDocsApproveOnATerminalDeclinesOnAnythingButYes(t *testing.T) {
	root := documentedRepo(t)
	id := planChange(t, root, docsFlags{})
	asTerminal(t)
	output, err := runDocsCommand(t, runDocsApprove, docsFlags{root: root}, "n\n")
	if err != nil || !strings.Contains(output, "not approved") {
		t.Fatalf("%v %s", err, output)
	}
	if _, found, _ := docchange.LoadApproval(root, id); found {
		t.Error("declining must not record an approval")
	}
}

func TestDocsApproveOnATerminalRecordsTheApprovalOnYes(t *testing.T) {
	root := documentedRepo(t)
	id := planChange(t, root, docsFlags{})
	asTerminal(t)
	output, err := runDocsCommand(t, runDocsApprove, docsFlags{root: root}, "Y\n")
	if err != nil || !strings.Contains(output, "approved "+id) {
		t.Fatalf("%v %s", err, output)
	}
	if approval, found, _ := docchange.LoadApproval(root, id); !found || !approval.IsValid() {
		t.Error("approval not recorded")
	}
}

func TestDocsFullFlowWritesTheApprovedChangesetAndNothingBefore(t *testing.T) {
	root := documentedRepo(t)
	id := planChange(t, root, docsFlags{inTreeReadmes: true})
	if _, err := runDocsCommand(t, runDocsApply, docsFlags{root: root}, ""); !errors.Is(err, docchange.ErrNotApproved) {
		t.Fatalf("apply before approval: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "a", "README.md")); err == nil {
		t.Fatal("nothing may be written before approval")
	}
	if _, err := runDocsCommand(t, runDocsApprove, docsFlags{root: root, yes: true}, "", id); err != nil {
		t.Fatal(err)
	}
	output, err := runDocsCommand(t, runDocsApply, docsFlags{root: root}, "", id)
	if err != nil {
		t.Fatal(err)
	}
	assertContainsAll(t, output, "applied "+id, "written")
	for _, target := range []string{"a/README.md", "CODEMAP.md", ".loom/index.json"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(target))); err != nil {
			t.Errorf("%s not written: %v", target, err)
		}
	}
}

func TestDocsResolvesTheLatestChangesetAndExplainsWhenThereIsNone(t *testing.T) {
	root := t.TempDir()
	docsArgs = docsFlags{root: root}
	if _, err := resolveChange(nil); !errors.Is(err, docchange.ErrNoSuchChange) || !strings.Contains(err.Error(), "loom docs plan") {
		t.Errorf("err = %v", err)
	}
	if id, err := resolveChange([]string{"explicit"}); err != nil || id != "explicit" {
		t.Errorf("%q %v", id, err)
	}
}

func TestDocsPlanWithoutARunSaysToRunDocument(t *testing.T) {
	root := committedGoRepo(t)
	useDocumentFixtures(t, answerEverything(t, root))
	if _, err := runDocsCommand(t, runDocsPlan, docsFlags{root: root}, ""); !errors.Is(err, docchange.ErrNoRun) {
		t.Errorf("err = %v", err)
	}
}

func TestDocsPlanFailsOnAMissingVocabulary(t *testing.T) {
	root := documentedRepo(t)
	frameworkFS = emptyFS()
	if _, err := runDocsCommand(t, runDocsPlan, docsFlags{root: root}, ""); err == nil {
		t.Error("planning re-verifies against the vocabulary, so it must exist")
	}
}

func TestDocsApproveAndApplyOfAnUnknownChangesetFailWithAClearError(t *testing.T) {
	root := t.TempDir()
	for name, run := range map[string]func(*cobra.Command, []string) error{"approve": runDocsApprove, "apply": runDocsApply} {
		if _, err := runDocsCommand(t, run, docsFlags{root: root, yes: true}, "", "nope"); !errors.Is(err, docchange.ErrNoSuchChange) {
			t.Errorf("%s: %v", name, err)
		}
		if _, err := runDocsCommand(t, run, docsFlags{root: root, yes: true}, ""); !errors.Is(err, docchange.ErrNoSuchChange) {
			t.Errorf("%s with no changeset at all: %v", name, err)
		}
	}
}

func TestDocsApplyReportsAResetApprovalToThePerson(t *testing.T) {
	root := documentedRepo(t)
	id := planChange(t, root, docsFlags{})
	if _, err := runDocsCommand(t, runDocsApprove, docsFlags{root: root, yes: true}, "", id); err != nil {
		t.Fatal(err)
	}
	staged := docchange.StagedPath(root, id, "CODEMAP.md")
	if err := os.WriteFile(staged, []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := runDocsCommand(t, runDocsApply, docsFlags{root: root}, "", id)
	var reset *docchange.ResetError
	if !errors.As(err, &reset) || !strings.Contains(err.Error(), "CODEMAP.md") {
		t.Errorf("err = %v", err)
	}
}
