package repoquery

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestDependenciesAndDependentsFollowOneHopWithConfidence(t *testing.T) {
	snapshot := snapshotOf(t, indexedRepo(t, layered))
	deps, err := snapshot.DependenciesOf("svc")
	if err != nil || !reflect.DeepEqual(deps, []Edge{{Path: "store", Confidence: "exact"}}) {
		t.Errorf("deps = %v %v", deps, err)
	}
	dependents, err := snapshot.DependentsOf("store")
	if err != nil || !reflect.DeepEqual(paths(dependents), []string{"api", "svc"}) {
		t.Errorf("dependents = %v %v", dependents, err)
	}
	if none, _ := snapshot.DependenciesOf("store"); len(none) != 0 {
		t.Errorf("a leaf depends on nothing: %v", none)
	}
}

func TestASourceFilePathResolvesToItsDirectoryNode(t *testing.T) {
	snapshot := snapshotOf(t, indexedRepo(t, layered))
	deps, err := snapshot.DependenciesOf("cmd/app/main.go")
	if err != nil || !reflect.DeepEqual(paths(deps), []string{"svc"}) {
		t.Errorf("%v %v", deps, err)
	}
	if viaDot, _ := snapshot.DependentsOf("./store/"); !reflect.DeepEqual(paths(viaDot), []string{"api", "svc"}) {
		t.Errorf("a path is cleaned before lookup: %v", viaDot)
	}
}

func TestAnUnknownPathIsAnErrorWithNearMisses(t *testing.T) {
	snapshot := snapshotOf(t, indexedRepo(t, layered))
	_, err := snapshot.DependenciesOf("stor")
	var unknown *UnknownPathError
	if !errors.As(err, &unknown) || !reflect.DeepEqual(unknown.Suggestions, []string{"store"}) || !strings.Contains(err.Error(), "Did you mean: store") {
		t.Errorf("err = %v", err)
	}
	if _, err := snapshot.DependentsOf("nowhere/at/all"); err == nil || strings.Contains(err.Error(), "Did you mean") {
		t.Errorf("no near miss, no suggestion: %v", err)
	}
}

func TestImpactIsTransitiveNearestFirstAndExcludesTheChange(t *testing.T) {
	snapshot := snapshotOf(t, indexedRepo(t, layered))
	got, err := snapshot.Impact([]string{"store"}, 0)
	want := []Impacted{{"api", 1, "exact"}, {"svc", 1, "exact"}, {"cmd/app", 2, "exact"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("impact = %v %v\nwant     %v", got, err, want)
	}
}

func TestImpactDepthLimitsHowFarAChangeIsFollowed(t *testing.T) {
	snapshot := snapshotOf(t, indexedRepo(t, layered))
	got, _ := snapshot.Impact([]string{"store"}, 1)
	if !reflect.DeepEqual(paths2(got), []string{"api", "svc"}) {
		t.Errorf("depth 1 stops at direct importers: %v", got)
	}
	huge, _ := snapshot.Impact([]string{"store"}, 999)
	if len(huge) != 3 {
		t.Errorf("an absurd depth is capped, not rejected: %v", huge)
	}
}

func paths2(items []Impacted) []string {
	out := []string{}
	for _, item := range items {
		out = append(out, item.Path)
	}
	return out
}

func TestImpactOfSeveralPathsUnionsThem(t *testing.T) {
	snapshot := snapshotOf(t, indexedRepo(t, layered))
	got, err := snapshot.Impact([]string{"store", "svc"}, 3)
	if err != nil || !reflect.DeepEqual(paths2(got), []string{"api", "cmd/app"}) {
		t.Errorf("svc is itself a change, so only its importers and store's remain: %v %v", got, err)
	}
}

func TestImpactNamesEveryUnknownPathRatherThanShrinkingTheRadius(t *testing.T) {
	snapshot := snapshotOf(t, indexedRepo(t, layered))
	_, err := snapshot.Impact([]string{"store", "typo1", "typo2"}, 3)
	if err == nil || !strings.Contains(err.Error(), "typo1, typo2") {
		t.Errorf("err = %v", err)
	}
	if _, err := snapshot.Impact(nil, 3); err == nil {
		t.Error("no paths is an error")
	}
}

func TestOneApproximateEdgeMakesAChainApproximateButAnExactPathWins(t *testing.T) {
	files := map[string]string{
		"src/main/java/com/acme/app/Main.java": "package com.acme.app;\n\nimport com.acme.shared.*;\n\npublic class Main {}\n",
		"src/main/java/com/acme/shared/A.java": "package com.acme.shared;\n\npublic class A {}\n",
		"src/test/java/com/acme/shared/B.java": "package com.acme.shared;\n\npublic class B {}\n",
		"src/main/java/com/acme/top/Top.java":  "package com.acme.top;\n\nimport com.acme.app.Main;\n\npublic class Top {}\n",
	}
	snapshot := snapshotOf(t, indexedRepo(t, files))
	got, err := snapshot.Impact([]string{"src/main/java/com/acme/shared"}, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []Impacted{{"src/main/java/com/acme/app", 1, "approximate"}, {"src/main/java/com/acme/top", 2, "approximate"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("a wildcard import spanning two directories is approximate, and so is everything beyond it:\n got %v\nwant %v", got, want)
	}
	deps, _ := snapshot.DependenciesOf("src/main/java/com/acme/app")
	if len(deps) != 2 || deps[0].Confidence != "approximate" {
		t.Errorf("approximate edges are labelled, not hidden: %v", deps)
	}
}

func TestAnExactEdgeBeatsAnApproximateOneToTheSameNode(t *testing.T) {
	if got := strongest("approximate", "exact"); got != "exact" {
		t.Errorf("strongest = %q", got)
	}
	if got := strongest("exact", "approximate"); got != "exact" {
		t.Errorf("an exact edge is never downgraded: %q", got)
	}
	if got := strongest("", "approximate"); got != "approximate" {
		t.Errorf("%q", got)
	}
}

func TestExactEdgesSortBeforeApproximateOnes(t *testing.T) {
	edges := sortedEdges(map[string]string{"z": "exact", "a": "approximate", "m": "exact"})
	if !reflect.DeepEqual(paths(edges), []string{"m", "z", "a"}) {
		t.Errorf("%v", edges)
	}
}

func TestEntrypointsListEveryProgramStartSorted(t *testing.T) {
	files := map[string]string{
		"go.mod": layered["go.mod"], "cmd/app/main.go": layered["cmd/app/main.go"], "svc/svc.go": layered["svc/svc.go"], "store/store.go": layered["store/store.go"],
		"tools/gen.py": "if __name__ == \"__main__\":\n    pass\n",
	}
	snapshot := snapshotOf(t, indexedRepo(t, files))
	got, err := snapshot.Entrypoints()
	want := []Entrypoint{{"cmd/app/main.go", "cmd/app"}, {"tools/gen.py", "tools"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("%v %v", got, err)
	}
}

func TestAnEntrypointInTheRootHasNoDirectoryPrefix(t *testing.T) {
	snapshot := snapshotOf(t, indexedRepo(t, map[string]string{"go.mod": layered["go.mod"], "main.go": "package main\n\nfunc main() {}\n"}))
	got, _ := snapshot.Entrypoints()
	if !reflect.DeepEqual(got, []Entrypoint{{"main.go", "."}}) {
		t.Errorf("%v", got)
	}
}
