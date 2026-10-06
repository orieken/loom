package docchange

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func plannedChange(t *testing.T, options Options) (string, Change) {
	t.Helper()
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	return root, plan(t, root, options)
}

func TestApplyWithoutApprovalWritesNothingAndSaysWhatToDo(t *testing.T) {
	root, change := plannedChange(t, Options{InTreeReadmes: true})
	before := snapshot(t, root)
	_, err := Apply(root, change.ID, clock)
	if !errors.Is(err, ErrNotApproved) || !strings.Contains(err.Error(), "loom docs approve "+change.ID) {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Error("nothing may be written to the target repo before approval")
	}
}

func TestApprovedChangesetIsWrittenAndRecordedAsApplied(t *testing.T) {
	root, change := plannedChange(t, Options{InTreeReadmes: true})
	result := approveAndApply(t, root, change.ID)
	if len(result.Written) != len(targets(change, ActionCreate)) || len(result.Withheld) != 0 {
		t.Errorf("result = %+v", result)
	}
	for _, target := range []string{"lib/README.md", ".loom/docs/lib/README.md", ".loom/index.json", "CODEMAP.md"} {
		if !exists(root, target) {
			t.Errorf("%s was not written", target)
		}
	}
	if _, err := Apply(root, change.ID, clock); !errors.Is(err, ErrAlreadyApplied) {
		t.Errorf("a changeset is applied once: %v", err)
	}
}

func approvedThenEdited(t *testing.T) (string, Change, error) {
	t.Helper()
	root, change := plannedChange(t, Options{InTreeReadmes: true})
	if _, err := Approve(root, change.ID, clock); err != nil {
		t.Fatal(err)
	}
	staged := StagedPath(root, change.ID, "lib/README.md")
	raw, _ := os.ReadFile(staged)
	if err := os.WriteFile(staged, append(raw, []byte("\nsneaky addition\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Apply(root, change.ID, clock)
	return root, change, err
}

// Gate binding: editing a staged artifact after approval resets the approval.
func TestEditingAStagedFileAfterApprovalResetsTheGateAndWritesNothing(t *testing.T) {
	root, _, err := approvedThenEdited(t)
	var reset *ResetError
	if !errors.As(err, &reset) || !reflect.DeepEqual(reset.Changed, []string{"lib/README.md"}) {
		t.Fatalf("err = %v", err)
	}
	if exists(root, "lib/README.md") || exists(root, "CODEMAP.md") {
		t.Error("nothing may be written after the approval is reset")
	}
}

func TestAResetApprovalIsKeptNamingWhatChangedAndStaysReset(t *testing.T) {
	root, change, _ := approvedThenEdited(t)
	approval, _, _ := LoadApproval(root, change.ID)
	if approval.IsValid() || approval.Invalidated.Changed[0] != "lib/README.md" {
		t.Errorf("the record is kept, naming what changed: %+v", approval)
	}
	if _, err := Apply(root, change.ID, clock); !errors.Is(err, ErrNotApproved) || !strings.Contains(err.Error(), "lib/README.md") {
		t.Errorf("a reset approval stays reset: %v", err)
	}
}

func TestApprovingAgainAfterAResetBindsToTheEditedContent(t *testing.T) {
	root, change := plannedChange(t, Options{InTreeReadmes: true})
	if _, err := Approve(root, change.ID, clock); err != nil {
		t.Fatal(err)
	}
	staged := StagedPath(root, change.ID, "lib/README.md")
	if err := os.WriteFile(staged, []byte("<!-- reviewed -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(root, change.ID, clock); err == nil {
		t.Fatal("expected a reset")
	}
	if _, err := Approve(root, change.ID, clock); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(root, change.ID, clock); err != nil {
		t.Fatalf("a fresh approval of the edited file must apply: %v", err)
	}
	if got := readFile(t, root, "lib/README.md"); got != "<!-- reviewed -->\n" {
		t.Errorf("the reviewed edit is what was written: %q", got)
	}
}

func TestEditingChangeJSONOrAddingAStagedFileAlsoResetsTheGate(t *testing.T) {
	root, change := plannedChange(t, Options{})
	if _, err := Approve(root, change.ID, clock); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(Dir(root, change.ID), changeFile)
	raw, _ := os.ReadFile(path)
	if err := os.WriteFile(path, []byte(strings.Replace(string(raw), `"runId"`, `"runID"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	var reset *ResetError
	if _, err := Apply(root, change.ID, clock); !errors.As(err, &reset) || reset.Changed[0] != "change.json" {
		t.Errorf("the artifact list itself is bound: %v", err)
	}
}

func TestADeletedStagedFileResetsTheGateRatherThanCrashing(t *testing.T) {
	root, change := plannedChange(t, Options{})
	if _, err := Approve(root, change.ID, clock); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(StagedPath(root, change.ID, "CODEMAP.md")); err != nil {
		t.Fatal(err)
	}
	var reset *ResetError
	if _, err := Apply(root, change.ID, clock); !errors.As(err, &reset) || reset.Changed[0] != "CODEMAP.md" {
		t.Errorf("%v", err)
	}
}

func TestApproveRefusesAChangesetWhoseStagedFileIsMissing(t *testing.T) {
	root, change := plannedChange(t, Options{})
	if err := os.Remove(StagedPath(root, change.ID, "CODEMAP.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Approve(root, change.ID, clock); err == nil || !strings.Contains(err.Error(), "CODEMAP.md") {
		t.Errorf("err = %v", err)
	}
}

func TestATargetEditedAfterPlanningIsNeverOverwritten(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	writeFile(t, root, "lib/README.md", "# lib\n\nMy own notes.\n")
	change := plan(t, root, Options{InTreeReadmes: true})
	writeFile(t, root, "lib/README.md", "# lib\n\nMy own notes, edited after the plan.\n")
	if _, err := Approve(root, change.ID, clock); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	_, err := Apply(root, change.ID, clock)
	if !IsStale(err) || !strings.Contains(err.Error(), "lib/README.md") {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Error("a stale target must stop the whole apply before anything is written")
	}
}

func TestAnInterruptedApplyCanBeRepeated(t *testing.T) {
	root, change := plannedChange(t, Options{})
	if _, err := Approve(root, change.ID, clock); err != nil {
		t.Fatal(err)
	}
	// Simulate a first apply that wrote CODEMAP.md and then died.
	writeFile(t, root, "CODEMAP.md", readFile(t, root, ".loom/changes/"+change.ID+"/files/CODEMAP.md"))
	result, err := Apply(root, change.ID, clock)
	if err != nil || !reflect.DeepEqual(result.AlreadyPresent, []string{"CODEMAP.md"}) {
		t.Fatalf("%+v %v", result, err)
	}
	if len(result.Written) != len(targets(change, ActionCreate))-1 {
		t.Errorf("the rest must still be written: %+v", result)
	}
}

func TestApplyOfAnUnknownChangesetNamesTheFix(t *testing.T) {
	if _, err := Apply(t.TempDir(), "nope", clock); !errors.Is(err, ErrNoSuchChange) {
		t.Errorf("%v", err)
	}
	if _, err := Approve(t.TempDir(), "nope", clock); !errors.Is(err, ErrNoSuchChange) {
		t.Errorf("%v", err)
	}
}

func TestLatestChangeIsTheNewest(t *testing.T) {
	root := newRepo(t, goRepo)
	if id, err := LatestChange(root); id != "" || err != nil {
		t.Errorf("no changes yet: %q %v", id, err)
	}
	documentedRun(t, root, nil)
	first := plan(t, root, Options{})
	second := plan(t, root, Options{})
	if id, _ := LatestChange(root); id != second.ID || id <= first.ID {
		t.Errorf("latest = %q, first %q second %q", id, first.ID, second.ID)
	}
}
