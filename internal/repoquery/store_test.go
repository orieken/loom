package repoquery

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotOfAnUnindexedRepoIsErrNoIndex(t *testing.T) {
	if _, err := newTestStore(t.TempDir()).Snapshot(); !errors.Is(err, ErrNoIndex) {
		t.Errorf("err = %v", err)
	}
	if _, err := NewStore("", nil).Snapshot(); !errors.Is(err, ErrNoIndex) {
		t.Errorf("a server with no workspace root has nothing to read: %v", err)
	}
}

func TestSnapshotCarriesTheGraphRevisionAndIsFreshRightAfterIndexing(t *testing.T) {
	snapshot := snapshotOf(t, indexedRepo(t, layered))
	if !snapshot.Fresh || len(snapshot.GraphSHA()) != 40 || !snapshot.HasIndex {
		t.Errorf("fresh=%v sha=%q hasIndex=%v", snapshot.Fresh, snapshot.GraphSHA(), snapshot.HasIndex)
	}
}

func TestAChangeToImportsMakesTheSnapshotStale(t *testing.T) {
	root := indexedRepo(t, layered)
	store := newTestStore(root)
	store.ttl = 0
	writeFile(t, root, "api/api.go", "package api\n\nimport \"example.com/x/svc\"\n\nfunc Serve() { svc.Run() }\n")
	git(t, root, "add", "-A")
	snapshot, err := store.Snapshot()
	if err != nil || snapshot.Fresh {
		t.Errorf("the graph on disk no longer matches the tree: fresh=%v err=%v", snapshot.Fresh, err)
	}
	reindex(t, root)
	if again, _ := store.Snapshot(); !again.Fresh {
		t.Error("re-indexing makes it fresh again")
	}
}

func TestAChangeThatLeavesTheImportGraphAloneStaysFresh(t *testing.T) {
	root := indexedRepo(t, layered)
	store := newTestStore(root)
	store.ttl = 0
	writeFile(t, root, "store/store.go", "package store\n\n// Get gets, differently.\nfunc Get() {}\n")
	git(t, root, "add", "-A")
	if snapshot, _ := store.Snapshot(); !snapshot.Fresh {
		t.Error("the graph describes imports, so a body edit does not make it stale")
	}
}

func TestFreshnessIsRecheckedOnlyOncePerInterval(t *testing.T) {
	root := indexedRepo(t, layered)
	store := newTestStore(root)
	clock := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return clock }
	if snapshot, _ := store.Snapshot(); !snapshot.Fresh {
		t.Fatal("fresh to begin with")
	}
	writeFile(t, root, "api/api.go", "package api\n\nimport \"example.com/x/svc\"\n\nfunc Serve() { svc.Run() }\n")
	git(t, root, "add", "-A")
	if snapshot, _ := store.Snapshot(); !snapshot.Fresh {
		t.Error("within the interval the earlier extraction is reused")
	}
	clock = clock.Add(DefaultFreshnessTTL)
	if snapshot, _ := store.Snapshot(); snapshot.Fresh {
		t.Error("after the interval the tree is extracted again and the change is seen")
	}
}

func TestANewIndexInvalidatesTheCachedExtractionImmediately(t *testing.T) {
	root := indexedRepo(t, layered)
	store := newTestStore(root)
	if snapshot, _ := store.Snapshot(); !snapshot.Fresh {
		t.Fatal("fresh to begin with")
	}
	writeFile(t, root, "api/api.go", "package api\n\nimport \"example.com/x/svc\"\n\nfunc Serve() { svc.Run() }\n")
	git(t, root, "add", "-A")
	reindex(t, root)
	if snapshot, _ := store.Snapshot(); !snapshot.Fresh {
		t.Error("`loom index` ran, so the stored graph is new and must be compared with a new extraction")
	}
}

func TestFreshnessThatCannotBeCheckedReportsNotFreshRatherThanFailing(t *testing.T) {
	root := indexedRepo(t, layered)
	store := NewStore(root, failingLister{})
	snapshot, err := store.Snapshot()
	if err != nil || snapshot.Fresh {
		t.Errorf("the answer is still useful but must not claim to be current: fresh=%v err=%v", snapshot.Fresh, err)
	}
}

type failingLister struct{}

func (failingLister) TrackedFiles(string) ([]string, error) { return nil, errors.New("no git") }
func (failingLister) HeadSHA(string) string                 { return "" }

func TestFactsAreLoadedOnceAndReused(t *testing.T) {
	snapshot := snapshotOf(t, indexedRepo(t, layered))
	first, err := snapshot.Facts()
	if err != nil || len(first) == 0 {
		t.Fatalf("%v %v", first, err)
	}
	first["injected"] = first["store"]
	second, _ := snapshot.Facts()
	if _, reused := second["injected"]; !reused {
		t.Error("the loaded sheets are cached on the snapshot")
	}
}

func TestASnapshotFailsLoudlyOnACorruptGraphOrIndex(t *testing.T) {
	root := indexedRepo(t, layered)
	writeFile(t, root, ".loom/index.json", "{broken")
	if _, err := newTestStore(root).Snapshot(); err == nil || errors.Is(err, ErrNoIndex) {
		t.Errorf("a corrupt index is not 'no index': %v", err)
	}
	writeFile(t, root, ".loom/graph.json", "{broken")
	if _, err := newTestStore(root).Snapshot(); err == nil || errors.Is(err, ErrNoIndex) {
		t.Errorf("a corrupt graph is not 'no index': %v", err)
	}
}

func TestQueriesThatNeedFactSheetsFailOnACorruptSheetInsteadOfAnsweringFromHalf(t *testing.T) {
	root := indexedRepo(t, layered)
	writeFile(t, root, ".loom/facts/store.json", "{broken")
	snapshot := snapshotOf(t, root)
	if _, err := snapshot.Entrypoints(); err == nil {
		t.Error("entrypoints needs every sheet")
	}
	if _, err := snapshot.Describe("store"); err == nil {
		t.Error("describe compares against the stored sheet")
	}
	if snapshot.directoryHoldsFile("store", "store.go") {
		t.Error("with unreadable facts a file cannot be confirmed to belong to a directory")
	}
}

func TestAnEmptyPathIsUnknownAndOffersNoSuggestions(t *testing.T) {
	_, err := snapshotOf(t, indexedRepo(t, layered)).DependenciesOf("")
	var unknown *UnknownPathError
	if !errors.As(err, &unknown) || len(unknown.Suggestions) != 0 {
		t.Errorf("%v", err)
	}
}

func TestSuggestionsAreCappedAndSorted(t *testing.T) {
	files := map[string]string{"go.mod": layered["go.mod"]}
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		files["pkg"+name+"/x.go"] = "package pkg" + name + "\n"
	}
	snapshot := snapshotOf(t, indexedRepo(t, files))
	_, err := snapshot.DependenciesOf("pkg")
	var unknown *UnknownPathError
	if !errors.As(err, &unknown) || len(unknown.Suggestions) != maxSuggestions || unknown.Suggestions[0] != "pkga" {
		t.Errorf("%v", unknown)
	}
}

func TestFactSheetsAreReadOncePerGraphNotOncePerCall(t *testing.T) {
	root := indexedRepo(t, layered)
	store := newTestStore(root)
	first, _ := store.Snapshot()
	sheets, err := first.Facts()
	if err != nil || len(sheets) == 0 {
		t.Fatalf("%v %v", sheets, err)
	}
	// Remove the sheets on disk. A second snapshot answering from them proves the
	// cache, not the disk, served it.
	if err := os.RemoveAll(filepath.Join(root, ".loom", "facts")); err != nil {
		t.Fatal(err)
	}
	second, _ := store.Snapshot()
	again, err := second.Facts()
	if err != nil || len(again) != len(sheets) {
		t.Errorf("a later call reuses the parsed sheets: %d of %d, %v", len(again), len(sheets), err)
	}
}

func TestFactSheetsAreReloadedAfterTheIntervalAndWhenTheGraphChanges(t *testing.T) {
	root := indexedRepo(t, layered)
	store := newTestStore(root)
	clock := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return clock }
	snapshot, _ := store.Snapshot()
	if _, err := snapshot.Facts(); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, ".loom", "facts")); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(DefaultFreshnessTTL)
	expired, _ := store.Snapshot()
	if facts, _ := expired.Facts(); len(facts) != 0 {
		t.Errorf("after the interval the sheets are read again, and these are gone: %d", len(facts))
	}
	writeFile(t, root, "api/api.go", "package api\n\nimport \"example.com/x/svc\"\n\nfunc Serve() { svc.Run() }\n")
	git(t, root, "add", "-A")
	reindex(t, root)
	changed, _ := store.Snapshot()
	if facts, _ := changed.Facts(); len(facts) == 0 {
		t.Error("a new graph reloads the sheets at once, without waiting out the interval")
	}
}
