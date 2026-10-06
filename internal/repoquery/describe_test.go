package repoquery

import (
	"errors"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/contextindex"
)

func TestDescribeReturnsTheEntryAndItsRenderedSummary(t *testing.T) {
	root := indexedRepo(t, layered)
	document(t, root, "store", "Persists records.", "adapter", "persistence")
	writeFile(t, root, ".loom/docs/store/README.md", "# store\n\nPersists records.\n")
	got, err := snapshotOf(t, root).Describe("store")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "store" || got.Entry.Purpose != "Persists records." || !strings.Contains(got.Summary, "# store") || got.Stale || got.SummaryTruncated {
		t.Errorf("%+v", got)
	}
}

func TestDescribeResolvesAFileToItsDirectory(t *testing.T) {
	root := indexedRepo(t, layered)
	document(t, root, "store", "Persists records.", "adapter")
	got, err := snapshotOf(t, root).Describe("store/store.go")
	if err != nil || got.Path != "store" || got.ResolvedFrom != "store/store.go" {
		t.Errorf("%+v %v", got, err)
	}
	if _, err := snapshotOf(t, root).Describe("store/not-a-file.go"); err == nil {
		t.Error("a file the directory does not hold must not resolve")
	}
}

func TestDescribeOfAnUndocumentedDirectoryHasAnEmptyEntryNotAnError(t *testing.T) {
	got, err := snapshotOf(t, indexedRepo(t, layered)).Describe("api")
	if err != nil || got.Entry.Status != contextindex.StatusUndocumented || got.Entry.Purpose != "" || got.Summary != "" || got.Stale {
		t.Errorf("%+v %v", got, err)
	}
}

func TestDescribeFlagsAnEntryDocumentedAgainstAnOlderVersionOfTheDirectory(t *testing.T) {
	root := indexedRepo(t, layered)
	document(t, root, "store", "Persists records.", "adapter")
	writeFile(t, root, "store/store.go", "package store\n\nfunc Get() {}\nfunc Put() {}\n")
	git(t, root, "add", "-A")
	reindex(t, root)
	got, err := snapshotOf(t, root).Describe("store")
	if err != nil || !got.Stale {
		t.Errorf("the directory changed after it was documented: %+v %v", got, err)
	}
}

func TestDescribeCapsALongSummaryAndSaysSo(t *testing.T) {
	root := indexedRepo(t, layered)
	document(t, root, "store", "x", "adapter")
	writeFile(t, root, ".loom/docs/store/README.md", strings.Repeat("é", MaxSummaryBytes))
	got, _ := snapshotOf(t, root).Describe("store")
	if !got.SummaryTruncated || len(got.Summary) > MaxSummaryBytes {
		t.Errorf("len %d truncated %v", len(got.Summary), got.SummaryTruncated)
	}
	for _, r := range got.Summary {
		if r == '�' {
			t.Error("truncation must not leave a broken character")
		}
	}
}

func TestDescribeOfTheRootReadsTheRootSummary(t *testing.T) {
	root := indexedRepo(t, layered)
	writeFile(t, root, ".loom/docs/README.md", "# the repo\n")
	got, err := snapshotOf(t, root).Describe(".")
	if err != nil || got.Summary != "# the repo\n" {
		t.Errorf("%+v %v", got, err)
	}
}

func TestDescribeNeverReadsAFileForAPathThatIsNotAnIndexKey(t *testing.T) {
	root := indexedRepo(t, layered)
	writeFile(t, root, ".loom/docs/../secret/README.md", "do not return me")
	for _, attempt := range []string{"../secret", "../../etc/passwd", "store/../../x"} {
		if _, err := snapshotOf(t, root).Describe(attempt); err == nil {
			t.Errorf("%q must not resolve", attempt)
		}
	}
	if summary, _ := snapshotOf(t, root).readSummary("../secret"); summary != "" {
		t.Error("a traversing directory name is refused at the read, too")
	}
}

func TestDescribeOfAnUnknownPathSuggestsNearMisses(t *testing.T) {
	_, err := snapshotOf(t, indexedRepo(t, layered)).Describe("stor")
	var unknown *UnknownPathError
	if !errors.As(err, &unknown) {
		t.Errorf("%v", err)
	}
}
