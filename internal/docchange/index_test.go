package docchange

import (
	"reflect"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/contextindex"
)

func appliedIndex(t *testing.T) (string, contextindex.Index) {
	t.Helper()
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	approveAndApply(t, root, plan(t, root, Options{}).ID)
	index, found, err := contextindex.Load(root)
	if err != nil || !found {
		t.Fatalf("%v %v", found, err)
	}
	return root, index
}

func TestApplyingRecordsWhatTheDocumenterSaidInTheIndex(t *testing.T) {
	_, index := appliedIndex(t)
	lib := index.Entries["lib"]
	if lib.Status != contextindex.StatusFresh || lib.Purpose != "does lib" || lib.Layer != "application" || !reflect.DeepEqual(lib.Tags, []string{"api"}) {
		t.Errorf("lib = %+v", lib)
	}
}

func TestApplyingRecordsHashesATimestampAndOpenQuestions(t *testing.T) {
	_, index := appliedIndex(t)
	lib := index.Entries["lib"]
	if lib.InputHash == "" || !strings.HasPrefix(lib.DocHash, "sha256:") {
		t.Errorf("hashes are recorded: %+v", lib)
	}
	if lib.GeneratedAt == "" || len(lib.OpenQuestions) != 1 {
		t.Errorf("timestamp and open questions are recorded: %+v", lib)
	}
}

func TestAfterApplyingLoomHealthWouldReportEverythingFresh(t *testing.T) {
	root, index := appliedIndex(t)
	sheets, _ := contextindex.CurrentSheets(root, gitLister())
	if report := contextindex.Check(index, sheets); report.NeedsReindex() || len(report.Undocumented) != 0 || report.Fresh != 4 {
		t.Errorf("`loom health` would report: %+v", report)
	}
}

func TestADocHashIsTheDigestOfTheSidecarThatWasWritten(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	approveAndApply(t, root, plan(t, root, Options{}).ID)
	index, _, _ := contextindex.Load(root)
	if want := Digest([]byte(readFile(t, root, ".loom/docs/lib/README.md"))); index.Entries["lib"].DocHash != want {
		t.Errorf("doc_hash %q != digest of the written sidecar %q", index.Entries["lib"].DocHash, want)
	}
}

func TestTheIndexIsNotTouchedUntilTheChangesetIsApplied(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	plan(t, root, Options{})
	if _, found, _ := contextindex.Load(root); found {
		t.Error("planning must not write the index")
	}
}

func TestADirectoryLeftOutOfTheChangesetStaysUndocumentedInTheIndex(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	writeFile(t, root, "lib/lib.go", "package lib\n\n// Changed.\nfunc Do() {}\n")
	git(t, root, "add", "-A")
	approveAndApply(t, root, plan(t, root, Options{}).ID)
	index, _, _ := contextindex.Load(root)
	if entry := index.Entries["lib"]; entry.Status != contextindex.StatusUndocumented || entry.Purpose != "" {
		t.Errorf("a stale result must not be recorded: %+v", entry)
	}
}

func TestAnExistingIndexEntryOutsideTheChangesetIsKept(t *testing.T) {
	root := newRepo(t, goRepo)
	documentedRun(t, root, nil)
	approveAndApply(t, root, plan(t, root, Options{}).ID)
	before, _, _ := contextindex.Load(root)
	writeFile(t, root, "lib/lib.go", "package lib\n\n// Changed.\nfunc Do() {}\n")
	git(t, root, "add", "-A")
	approveAndApply(t, root, plan(t, root, Options{}).ID)
	after, _, _ := contextindex.Load(root)
	if after.Entries["lib"].Purpose != before.Entries["lib"].Purpose || after.Entries["lib"].Status != contextindex.StatusStale {
		t.Errorf("an entry that was documented and then changed is stale, and keeps what was recorded: %+v", after.Entries["lib"])
	}
}
