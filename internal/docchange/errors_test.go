package docchange

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestACorruptCandidateIsLeftOutWithItsReason(t *testing.T) {
	root := newRepo(t, goRepo)
	runID := documentedRun(t, root, nil)
	writeFile(t, root, ".loom/runs/"+runID+"/docs/lib.json", "{not json")
	change := plan(t, root, Options{})
	reasons := map[string]string{}
	for _, skipped := range change.Skipped {
		reasons[skipped.Path] = skipped.Reason
	}
	if !strings.HasPrefix(reasons["lib"], "invalid:") {
		t.Errorf("a result that does not decode must be left out: %+v", change.Skipped)
	}
}

func skipReasonFor(change Change, dir string) string {
	for _, skipped := range change.Skipped {
		if skipped.Path == dir {
			return skipped.Reason
		}
	}
	return ""
}

func TestAnUnreadableCandidateIsLeftOutWithItsReason(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("file permissions are not enforced here")
	}
	root := newRepo(t, goRepo)
	runID := documentedRun(t, root, nil)
	path := filepath.Join(root, ".loom", "runs", runID, "docs", "lib.json")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	if reason := skipReasonFor(plan(t, root, Options{}), "lib"); !strings.HasPrefix(reason, "unreadable:") {
		t.Errorf("reason = %q", reason)
	}
}

func TestACandidateForADirectoryThatNoLongerExistsIsLeftOut(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	git(t, root, "rm", "-rq", "lib")
	if reason := skipReasonFor(plan(t, root, Options{}), "lib"); !strings.Contains(reason, "no longer exists") {
		t.Errorf("reason = %q", reason)
	}
}

func TestPlanFailsLoudlyOnACorruptIndex(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	writeFile(t, root, ".loom/index.json", "{broken")
	runCounter++
	if _, err := Plan(PlanInput{Root: root, Vocabulary: vocabulary(), Lister: gitLister(), Now: fixedNow}); err == nil {
		t.Error("a corrupt index must not be silently replaced by a changeset")
	}
}

func TestPlanFailsWhenATargetCannotBeRead(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	if err := os.MkdirAll(filepath.Join(root, "CODEMAP.md"), 0o755); err != nil { // a directory where the file belongs
		t.Fatal(err)
	}
	runCounter++
	if _, err := Plan(PlanInput{Root: root, Vocabulary: vocabulary(), Lister: gitLister(), Now: fixedNow}); err == nil {
		t.Error("an unreadable target must fail the plan rather than be treated as absent")
	}
}

func TestPlanFailsOutsideAGitRepository(t *testing.T) {
	root := t.TempDir()
	if _, err := Plan(PlanInput{Root: root, RunID: "r", Vocabulary: vocabulary(), Lister: gitLister(), Now: fixedNow}); err == nil {
		t.Error("there is nothing to plan from")
	}
}

func TestPlanFailsWhenTheRunsDirectoryIsUnreadable(t *testing.T) {
	root := newRepo(t, goRepo)
	writeFile(t, root, ".loom/runs", "a file where the runs directory belongs")
	if _, err := Plan(PlanInput{Root: root, Vocabulary: vocabulary(), Lister: gitLister(), Now: fixedNow}); err == nil {
		t.Error("expected an error")
	}
}

func TestACorruptChangesetOrApprovalIsAnErrorNotASilentPass(t *testing.T) {
	root, change := plannedChange(t, Options{})
	writeFile(t, root, ".loom/changes/"+change.ID+"/approval.json", "{broken")
	if _, _, err := LoadApproval(root, change.ID); err == nil {
		t.Error("a corrupt approval must be an error")
	}
	if _, err := Apply(root, change.ID, clock); err == nil {
		t.Error("apply must not proceed past a corrupt approval")
	}
	writeFile(t, root, ".loom/changes/"+change.ID+"/change.json", "{broken")
	if _, err := Load(root, change.ID); err == nil || errors.Is(err, ErrNoSuchChange) {
		t.Errorf("a corrupt changeset is not 'no such changeset': %v", err)
	}
}

func TestLatestChangeReportsAnUnreadableChangesDirectory(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".loom/changes", "a file where the directory belongs")
	if _, err := LatestChange(root); err == nil {
		t.Error("expected an error")
	}
}

func TestApplyFailsCleanlyWhenATargetCannotBeWritten(t *testing.T) {
	root, change := plannedChange(t, Options{})
	if _, err := Approve(root, change.ID, clock); err != nil {
		t.Fatal(err)
	}
	// A file where CODEMAP.md's... parent would be needed is not applicable, so block .loom/docs instead.
	if err := os.RemoveAll(filepath.Join(root, ".loom", "docs")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, ".loom/docs", "a file where the sidecar directory belongs")
	if _, err := Apply(root, change.ID, clock); err == nil {
		t.Error("an unwritable target must fail the apply")
	}
	if _, statErr := os.Stat(filepath.Join(Dir(root, change.ID), appliedFile)); statErr == nil {
		t.Error("a failed apply must not be recorded as applied")
	}
}

func TestADocCommentEditThatCannotBeStagedFailsThePlan(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	if err := os.MkdirAll(filepath.Join(root, "lib", "doc.go"), 0o755); err != nil {
		t.Fatal(err)
	}
	runCounter++
	// doc.go is a directory: goDoc reads it as absent, then staging reads the target and fails.
	if _, err := Plan(PlanInput{Root: root, Options: Options{DocComments: true}, Vocabulary: vocabulary(), Lister: gitLister(), Now: fixedNow}); err == nil {
		t.Error("expected an error")
	}
}

func TestAGoFileThatDoesNotParseIsIgnoredWhenFindingThePackage(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "lib", map[string]string{"bad.go": "this is not go", "ok.go": "package lib\n"})
	name, documentedIn := goPackageFacts(root, sheetOf("lib", "bad.go", "ok.go"))
	if name != "lib" || documentedIn != "" {
		t.Errorf("%q %q", name, documentedIn)
	}
}
