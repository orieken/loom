package docchange

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestPlanWritesNothingOutsideTheChangesDirectory(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	before := snapshot(t, root)
	plan(t, root, Options{InTreeReadmes: true, DocComments: true})
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Error("`plan` changed the repository or the index; it may only stage under .loom/changes/")
	}
}

func TestDefaultChangesetHoldsSidecarDocsTheIndexAndCodemapOnly(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	change := plan(t, root, Options{})
	want := []string{".loom/docs/README.md", ".loom/docs/cmd/README.md", ".loom/docs/cmd/app/README.md", ".loom/docs/lib/README.md", ".loom/index.json", "CODEMAP.md"}
	if got := targets(change, ActionCreate); !reflect.DeepEqual(got, want) {
		t.Errorf("created = %v\nwant      %v", got, want)
	}
	for _, artifact := range change.Artifacts {
		if artifact.Kind == KindReadme || artifact.Kind == KindDocComment {
			t.Errorf("an in-tree artifact appeared without its flag: %+v", artifact)
		}
	}
	if change.Documented != 4 {
		t.Errorf("documented = %d", change.Documented)
	}
}

func TestInTreeReadmesAreOptInAndLandInsideMarkers(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	change := plan(t, root, Options{InTreeReadmes: true})
	readme := artifactFor(t, change, "lib/README.md")
	if readme.Kind != KindReadme || readme.Action != ActionCreate {
		t.Fatalf("%+v", readme)
	}
	staged := readFile(t, root, ".loom/changes/"+change.ID+"/files/lib/README.md")
	if !strings.Contains(staged, "<!-- loom:generated:start hash=sha256:") || !strings.Contains(staged, "<!-- loom:generated:end -->") || !strings.Contains(staged, "does lib") {
		t.Errorf("%s", staged)
	}
}

func TestPlanCollectsEveryOpenQuestionForTheGate(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	change := plan(t, root, Options{})
	if len(change.OpenQuestions) != 4 || change.OpenQuestions[0].Question == "" {
		t.Errorf("open questions = %+v", change.OpenQuestions)
	}
	if summary := Summary(change); !strings.Contains(summary, "Open questions the documents leave unanswered (4)") || !strings.Contains(summary, "why lib?") {
		t.Errorf("the gate must show what the documents leave open:\n%s", summary)
	}
}

func TestPlanWithoutAnyDocumenterRunSaysSo(t *testing.T) {
	root := newRepo(t, goRepo)
	_, err := Plan(PlanInput{Root: root, Vocabulary: vocabulary(), Lister: gitLister(), Now: fixedNow})
	if err == nil || err.Error() != ErrNoRun.Error() {
		t.Errorf("err = %v", err)
	}
}

func TestACandidateForADirectoryThatChangedSinceIsLeftOut(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	writeFile(t, root, "lib/lib.go", "package lib\n\n// Do does something different now.\nfunc Do() {}\n")
	git(t, root, "add", "-A")
	change := plan(t, root, Options{})
	skipped := map[string]string{}
	for _, item := range change.Skipped {
		skipped[item.Path] = item.Reason
	}
	if !strings.Contains(skipped["lib"], "changed since it was documented") {
		t.Errorf("a stale candidate must not be accepted: %+v", change.Skipped)
	}
	if exists(root, ".loom/changes/"+change.ID+"/files/.loom/docs/lib/README.md") {
		t.Error("a stale result must not be staged")
	}
}

func TestACandidateThatNoLongerVerifiesIsLeftOut(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	runCounter++
	change, err := Plan(PlanInput{Root: root, Vocabulary: repodocEmptyVocabulary(), Lister: gitLister(), Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	if len(change.Skipped) != 4 || !strings.Contains(change.Skipped[0].Reason, "no longer verifies (unknown-tag)") {
		t.Errorf("results are re-verified at plan time: %+v", change.Skipped)
	}
}

func TestPlanFromANamedRunIgnoresNewerOnes(t *testing.T) {
	root := newRepo(t, goRepo)
	first := documentedRun(t, root, nil)
	documentedRun(t, root, func(d *stateDoc) {
		d.Purpose = "second run says " + d.Path
		d.Evidence[0].Claim = d.Purpose
	})
	runCounter++
	change, err := Plan(PlanInput{Root: root, RunID: first, Vocabulary: vocabulary(), Lister: gitLister(), Now: fixedNow})
	if err != nil || change.RunID != first {
		t.Fatalf("%v %v", change.RunID, err)
	}
	staged := readFile(t, root, ".loom/changes/"+change.ID+"/files/.loom/docs/lib/README.md")
	if strings.Contains(staged, "second run says") {
		t.Error("planned from the wrong run")
	}
}

func twoPlansInOneSecond(t *testing.T) (root string, first, second Change) {
	t.Helper()
	root = newRepo(t, goRepo)
	documentedRun(t, root, nil)
	input := PlanInput{Root: root, Vocabulary: vocabulary(), Lister: gitLister(), Now: fixedNow}
	first, err := Plan(input)
	if err != nil {
		t.Fatal(err)
	}
	approveAndApply(t, root, first.ID)
	second, err = Plan(input)
	if err != nil {
		t.Fatal(err)
	}
	return root, first, second
}

// Two plans in one second must not share a directory: the second would inherit
// the first's approval and applied record.
func TestTwoPlansInTheSameSecondGetDistinctChangesetIds(t *testing.T) {
	_, first, second := twoPlansInOneSecond(t)
	if second.ID != first.ID+"-2" {
		t.Errorf("ids = %q and %q", first.ID, second.ID)
	}
}

func TestASecondPlanInTheSameSecondInheritsNeitherApprovalNorAppliedState(t *testing.T) {
	root, _, second := twoPlansInOneSecond(t)
	if _, approved, _ := LoadApproval(root, second.ID); approved {
		t.Error("the second changeset inherited the first's approval")
	}
	if _, err := Apply(root, second.ID, clock); !errors.Is(err, ErrNotApproved) {
		t.Errorf("it must not read as applied or approved: %v", err)
	}
}

func TestASuffixedChangesetIdStillSortsNewest(t *testing.T) {
	root, first, _ := twoPlansInOneSecond(t)
	third, _ := Plan(PlanInput{Root: root, Vocabulary: vocabulary(), Lister: gitLister(), Now: fixedNow})
	if third.ID != first.ID+"-3" {
		t.Errorf("third = %q", third.ID)
	}
	if latest, _ := LatestChange(root); latest != third.ID {
		t.Errorf("latest = %q", latest)
	}
}

// Documenting in batches leaves one run per batch; a changeset draws on all of them.
func TestPlanDrawsOnEveryRunNewestFirstSoBatchesAreNotDropped(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRunOf(t, root, "cmd", nil)
	newest := documentedRunOf(t, root, "lib", func(d *stateDoc) {
		d.Purpose = "batch two says " + d.Path
		d.Evidence[0].Claim = d.Purpose
	})
	change := plan(t, root, Options{})
	if change.Documented != 3 || len(change.Runs) != 2 || change.RunID != newest {
		t.Fatalf("documented=%d runs=%v newest=%s", change.Documented, change.Runs, change.RunID)
	}
	if !strings.Contains(readFile(t, root, ".loom/changes/"+change.ID+"/files/.loom/docs/lib/README.md"), "batch two says lib") ||
		!exists(root, ".loom/changes/"+change.ID+"/files/.loom/docs/cmd/app/README.md") {
		t.Error("lib from the newest batch, cmd/app from the earlier one")
	}
	if summary := Summary(change); !strings.Contains(summary, "from 2 runs, newest "+newest) {
		t.Errorf("%s", summary)
	}
}

func TestADirectoryTakesTheNewestRunThatHasAValidResultForIt(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRunOf(t, root, "lib", func(d *stateDoc) {
		d.Purpose = "older but valid for " + d.Path
		d.Evidence[0].Claim = d.Purpose
	})
	broken := documentedRunOf(t, root, "lib", nil)
	writeFile(t, root, ".loom/runs/"+broken+"/docs/lib.json", "{broken")
	change := plan(t, root, Options{})
	if !strings.Contains(readFile(t, root, ".loom/changes/"+change.ID+"/files/.loom/docs/lib/README.md"), "older but valid for lib") {
		t.Error("a corrupt newest candidate falls back to the older valid one")
	}
	if reason := skipReasonFor(change, "lib"); reason != "" {
		t.Errorf("a directory with a valid result is not skipped: %q", reason)
	}
}

func TestADirectoryWhoseEveryCandidateFailsIsSkippedWithTheNewestReason(t *testing.T) {
	root := newRepo(t, goRepo)
	only := documentedRunOf(t, root, "lib", nil)
	writeFile(t, root, ".loom/runs/"+only+"/docs/lib.json", "{broken")
	if reason := skipReasonFor(plan(t, root, Options{}), "lib"); !strings.HasPrefix(reason, "invalid:") {
		t.Errorf("reason = %q", reason)
	}
}
