package contextindex

import (
	"reflect"
	"testing"

	"github.com/orieken/loom/internal/repograph"
)

func TestRefreshOverAnUnchangedRepoWritesNothingTheSecondTime(t *testing.T) {
	root := gitRepo(t, goRepoFiles)
	sheets := currentSheets(t, root)
	index, written, err := Refresh(root, sheets)
	if err != nil || !written || len(index.Entries) != len(sheets) {
		t.Fatalf("first refresh: written=%v err=%v entries=%d", written, err, len(index.Entries))
	}
	if _, written, err := Refresh(root, currentSheets(t, root)); written || err != nil {
		t.Errorf("second refresh: written=%v err=%v", written, err)
	}
}

func TestRefreshFailsOnACorruptExistingIndex(t *testing.T) {
	root := gitRepo(t, goRepoFiles)
	writeFile(t, root, ".loom/index.json", "{broken")
	if _, _, err := Refresh(root, currentSheets(t, root)); err == nil {
		t.Error("a corrupt index must not be silently replaced")
	}
}

// The scenario Phase 3 exists for: document a tree, edit one file, and the
// check names that directory and its ancestors — and nothing else.
func TestEditingAFileStalesItsDirectoryAndAncestorsOnly(t *testing.T) {
	root := gitRepo(t, goRepoFiles)
	documented := documentAll(Build(Index{}, currentSheets(t, root)), currentSheets(t, root))
	if report := Check(documented, currentSheets(t, root)); report.NeedsReindex() || report.Fresh != 3 {
		t.Fatalf("a just-documented tree must be fresh: %+v", report)
	}
	writeFile(t, root, "b/b.go", "package b\n\nvar V = 2\n")
	report := Check(documented, currentSheets(t, root))
	if want := []string{".", "b"}; !reflect.DeepEqual(report.Stale, want) {
		t.Errorf("stale = %v, want %v", report.Stale, want)
	}
	if report.Fresh != 1 {
		t.Errorf("a, which only imports b, must stay fresh: %+v", report)
	}
}

func TestANewDirectoryIsUnindexedUntilIndexingRuns(t *testing.T) {
	root := gitRepo(t, goRepoFiles)
	indexed := Build(Index{}, currentSheets(t, root))
	writeFile(t, root, "c/c.go", "package c\n")
	runGit(t, root, "add", "-A")
	if report := Check(indexed, currentSheets(t, root)); !reflect.DeepEqual(report.Unindexed, []string{"c"}) {
		t.Errorf("unindexed = %v", report.Unindexed)
	}
}

func TestCurrentSheetsFailsOutsideAGitRepository(t *testing.T) {
	if _, err := CurrentSheets(t.TempDir(), repograph.GitLister{}); err == nil {
		t.Error("a directory that is not a git repo has no tracked files to index")
	}
}
