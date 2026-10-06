package repoquery

import (
	"reflect"
	"strings"
	"testing"
)

func TestImpactDepthBoundariesAreInclusive(t *testing.T) {
	snapshot := snapshotOf(t, indexedRepo(t, layered))
	one, _ := snapshot.Impact([]string{"store"}, 1)
	two, _ := snapshot.Impact([]string{"store"}, 2)
	if len(one) != 2 || len(two) != 3 {
		t.Errorf("depth n reaches exactly n imports: depth 1 -> %d, depth 2 -> %d", len(one), len(two))
	}
	atMax, _ := snapshot.Impact([]string{"store"}, MaxImpactDepth)
	overMax, _ := snapshot.Impact([]string{"store"}, MaxImpactDepth+1)
	if !reflect.DeepEqual(atMax, overMax) {
		t.Error("a depth above the maximum behaves as the maximum")
	}
	negative, err := snapshot.Impact([]string{"store"}, -5)
	if err != nil || len(negative) != 3 {
		t.Errorf("a non-positive depth falls back to the default (3): %v %v", negative, err)
	}
}

func TestAnExactPathAtTheSameDistanceUpgradesAnApproximateOne(t *testing.T) {
	// seed <- viaFuzzy (approximate) <- target ; seed <- viaSure (exact) <- target.
	reverse := map[string][]importer{
		"seed":     {{from: "viaFuzzy", exact: false}, {from: "viaSure", exact: true}},
		"viaFuzzy": {{from: "target", exact: true}},
		"viaSure":  {{from: "target", exact: true}},
	}
	got := walkDependents(reverse, map[string]bool{"seed": true}, 5)
	if got["target"].distance != 2 || !got["target"].exact {
		t.Errorf("two shortest paths, one all-exact: the node is exact: %+v", got["target"])
	}
	if got["viaFuzzy"].exact || !got["viaSure"].exact {
		t.Errorf("each first hop keeps its own label: %+v %+v", got["viaFuzzy"], got["viaSure"])
	}
}

func TestALongerExactPathDoesNotOverrideAShorterApproximateOne(t *testing.T) {
	reverse := map[string][]importer{
		"seed": {{from: "target", exact: false}, {from: "mid", exact: true}},
		"mid":  {{from: "target", exact: true}},
	}
	got := walkDependents(reverse, map[string]bool{"seed": true}, 5)
	if got["target"].distance != 1 || got["target"].exact {
		t.Errorf("distance is the shortest, and its confidence is that of the shortest paths: %+v", got["target"])
	}
}

func TestACycleDoesNotLoopOrListTheSeed(t *testing.T) {
	reverse := map[string][]importer{"a": {{from: "b", exact: true}}, "b": {{from: "a", exact: true}}}
	got := walkDependents(reverse, map[string]bool{"a": true}, 10)
	if _, listed := got["a"]; listed || len(got) != 1 || got["b"].distance != 1 {
		t.Errorf("%+v", got)
	}
}

func TestASummaryOfExactlyTheCapIsNotTruncatedButOneByteMoreIs(t *testing.T) {
	root := indexedRepo(t, layered)
	writeFile(t, root, ".loom/docs/store/README.md", strings.Repeat("a", MaxSummaryBytes))
	if summary, truncated := snapshotOf(t, root).readSummary("store"); truncated || len(summary) != MaxSummaryBytes {
		t.Errorf("at the cap: %d %v", len(summary), truncated)
	}
	writeFile(t, root, ".loom/docs/store/README.md", strings.Repeat("a", MaxSummaryBytes+1))
	if summary, truncated := snapshotOf(t, root).readSummary("store"); !truncated || len(summary) != MaxSummaryBytes {
		t.Errorf("one over: %d %v", len(summary), truncated)
	}
}

func TestOnlyATagOrOnlyAQueryIsEnoughToSearch(t *testing.T) {
	snapshot := documented(t)
	if _, err := snapshot.FindFiles("persistence", ""); err != nil {
		t.Errorf("a tag alone: %v", err)
	}
	if _, err := snapshot.FindFiles("", "store"); err != nil {
		t.Errorf("a query alone: %v", err)
	}
	if _, err := snapshot.FindFiles("", ""); err == nil {
		t.Error("neither is an error")
	}
}

func TestSortedEdgesOrderByPathWithinAConfidence(t *testing.T) {
	edges := sortedEdges(map[string]string{"b": "exact", "a": "exact", "d": "approximate", "c": "approximate"})
	if !reflect.DeepEqual(paths(edges), []string{"a", "b", "c", "d"}) {
		t.Errorf("%v", paths(edges))
	}
}

func TestEntrypointsAreSortedByPath(t *testing.T) {
	files := map[string]string{"go.mod": layered["go.mod"], "b/main.go": "package main\n\nfunc main() {}\n", "a/main.go": "package main\n\nfunc main() {}\n"}
	got, _ := snapshotOf(t, indexedRepo(t, files)).Entrypoints()
	if len(got) != 2 || got[0].Path != "a/main.go" {
		t.Errorf("%v", got)
	}
}
