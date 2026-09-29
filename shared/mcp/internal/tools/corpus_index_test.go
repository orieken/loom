package tools

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
)

func refs(paths ...string) []Reference {
	found := make([]Reference, 0, len(paths))
	for _, path := range paths {
		found = append(found, Reference{Path: path, Title: "title of " + path})
	}
	return found
}

func paths(found []Reference) []string {
	names := make([]string, 0, len(found))
	for _, reference := range found {
		names = append(names, reference.Path)
	}
	return names
}

// A document both backends rank well beats one only a single backend ranks
// first — the point of fusing ranks instead of interleaving lists.
func TestFusionFavoursWhatBothRankingsAgreeOn(t *testing.T) {
	lexical := refs("only-lexical", "both", "c")
	semantic := refs("only-semantic", "both", "d")
	fused := fuseReciprocalRank(lexical, semantic)
	assertNames(t, "fused", paths(fused)[:1], "both")
	if len(fused) != 5 {
		t.Errorf("fused %v, want each path once", paths(fused))
	}
}

func TestFusionScoresByRankAndKeepsTheFirstListsReference(t *testing.T) {
	first := []Reference{{Path: "p", Title: "from first", Relevance: 99}}
	second := []Reference{{Path: "p", Title: "from second", Relevance: 0.1}}
	fused := fuseReciprocalRank(first, second)
	want := 2.0 / float64(rrfConstant+1)
	if len(fused) != 1 || fused[0].Title != "from first" || fused[0].Relevance != want {
		t.Errorf("fused = %+v, want the first list's reference scored %v", fused, want)
	}
}

// Equal fused scores keep the order paths were first seen, so the result is
// deterministic.
func TestFusionBreaksTiesByFirstAppearance(t *testing.T) {
	fused := fuseReciprocalRank(refs("a", "b"), refs("b", "a"))
	assertNames(t, "tied", paths(fused), "a", "b")
}

func TestFusionIsBoundedByTheContractsTopK(t *testing.T) {
	var many []string
	for index := range 15 {
		many = append(many, fmt.Sprintf("doc-%02d", index))
	}
	if fused := fuseReciprocalRank(refs(many...)); len(fused) != maxCorpusResults {
		t.Errorf("fused %d references, want at most %d", len(fused), maxCorpusResults)
	}
}

// scriptedIndex answers with fixed references, or fails, recording the
// roots it was asked about.
type scriptedIndex struct {
	found     []Reference
	err       error
	refreshed [][]string
	searched  []string
}

func (s *scriptedIndex) EnsureIndex(_ context.Context, roots []string) error {
	s.refreshed = append(s.refreshed, roots)
	return s.err
}

func (s *scriptedIndex) SearchWithin(_ context.Context, root, _ string) ([]Reference, error) {
	s.searched = append(s.searched, root)
	return s.found, s.err
}

// search_features searches the feature archive by default, and whichever
// root it resolved otherwise — never a wider one.
func TestSearchFeaturesSearchesTheRootItResolved(t *testing.T) {
	root := rootedProject(t)
	WriteFile(t, filepath.Join(root.Dir(), "docs", "features", "a", "README.md"), "# A\n")
	index := &scriptedIndex{}
	tool := NewSearchFeaturesTool(SilentLogger(), index, root)
	for _, args := range []map[string]any{{"query": "q"}, {"query": "q", "featuresPath": "docs"}} {
		if result, err := tool.Execute(context.Background(), BuildRequest(args)); err != nil || result.IsError {
			t.Fatalf("Execute(%v): %v %s", args, err, ExtractText(t, result))
		}
	}
	want := []string{filepath.Join(root.Dir(), "docs", "features"), filepath.Join(root.Dir(), "docs")}
	if len(index.searched) != 2 || index.searched[0] != want[0] || index.searched[1] != want[1] {
		t.Errorf("searched %v, want %v", index.searched, want)
	}
}

func TestAHybridIndexRefreshesEveryBackend(t *testing.T) {
	lexical, semantic := &scriptedIndex{}, &scriptedIndex{}
	if err := NewHybridIndex(lexical, semantic).EnsureIndex(context.Background(), []string{"/root"}); err != nil {
		t.Fatalf("EnsureIndex: %v", err)
	}
	for _, index := range []*scriptedIndex{lexical, semantic} {
		if len(index.refreshed) != 1 || index.refreshed[0][0] != "/root" {
			t.Errorf("refreshed %v, want once with the root", index.refreshed)
		}
	}
}

func TestAHybridIndexSearchesEveryBackend(t *testing.T) {
	hybrid := NewHybridIndex(&scriptedIndex{found: refs("a", "shared")}, &scriptedIndex{found: refs("shared", "b")})
	found, err := hybrid.SearchWithin(context.Background(), "/root", "q")
	if err != nil || paths(found)[0] != "shared" || len(found) != 3 {
		t.Errorf("found %v (err %v), want shared first of three", paths(found), err)
	}
}

// One backend failing fails the search: half the evidence is not a result.
func TestAHybridIndexFailsWhenAnyBackendFails(t *testing.T) {
	broken := errors.New("backend down")
	healthy := &scriptedIndex{found: refs("a")}
	failing := &scriptedIndex{err: broken}
	hybrid := NewHybridIndex(healthy, failing)
	if _, err := hybrid.SearchWithin(context.Background(), "/root", "q"); !errors.Is(err, broken) {
		t.Errorf("search err = %v, want the backend's failure", err)
	}
	if err := hybrid.EnsureIndex(context.Background(), []string{"/root"}); !errors.Is(err, broken) {
		t.Errorf("refresh err = %v, want the backend's failure", err)
	}
	stops := NewHybridIndex(&scriptedIndex{err: broken}, healthy)
	healthy.refreshed = nil
	_ = stops.EnsureIndex(context.Background(), []string{"/root"})
	if len(healthy.refreshed) != 0 {
		t.Error("a refresh continued past a failed backend")
	}
}
