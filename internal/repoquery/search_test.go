package repoquery

import (
	"reflect"
	"strings"
	"testing"
)

func documented(t *testing.T) *Snapshot {
	t.Helper()
	root := indexedRepo(t, layered)
	document(t, root, "store", "Persists records and reads them back.", "adapter", "persistence", "state")
	document(t, root, "svc", "Coordinates store access for the program.", "application", "orchestration")
	return snapshotOf(t, root)
}

func matchPaths(matches []Match) []string {
	out := []string{}
	for _, match := range matches {
		out = append(out, match.Path)
	}
	return out
}

func TestFindFilesByTagReturnsOnlyTaggedDirectories(t *testing.T) {
	matches, err := documented(t).FindFiles("persistence", "")
	if err != nil || !reflect.DeepEqual(matchPaths(matches), []string{"store"}) {
		t.Errorf("%v %v", matches, err)
	}
	if matches[0].Purpose == "" || matches[0].Layer != "adapter" || matches[0].Status != "fresh" {
		t.Errorf("%+v", matches[0])
	}
}

func TestFindFilesRanksPathAboveTagAbovePurpose(t *testing.T) {
	matches, err := documented(t).FindFiles("", "store")
	if err != nil {
		t.Fatal(err)
	}
	if got := matchPaths(matches); !reflect.DeepEqual(got, []string{"store", "svc"}) {
		t.Errorf("a path hit beats a purpose hit: %v", got)
	}
	if matches[0].Score != pathWeight || matches[1].Score != purposeWeight {
		t.Errorf("scores = %d, %d", matches[0].Score, matches[1].Score)
	}
}

func TestFindFilesCombinesTagAndQueryAndIsCaseInsensitive(t *testing.T) {
	matches, _ := documented(t).FindFiles("STATE", "Records")
	if !reflect.DeepEqual(matchPaths(matches), []string{"store"}) {
		t.Errorf("the tag filter narrows the query: %v", matches)
	}
	if none, _ := documented(t).FindFiles("state", "orchestrate"); len(none) != 0 {
		t.Errorf("a lexical search does not stem: %v", none)
	}
}

func TestFindFilesMatchesUndocumentedDirectoriesByPathAlone(t *testing.T) {
	matches, _ := documented(t).FindFiles("", "api")
	if !reflect.DeepEqual(matchPaths(matches), []string{"api"}) || matches[0].Status != "undocumented" || matches[0].Purpose != "" {
		t.Errorf("%+v", matches)
	}
}

func TestFindFilesNeedsATagOrAQuery(t *testing.T) {
	if _, err := documented(t).FindFiles("", "  "); err == nil || !strings.Contains(err.Error(), "tag, a query") {
		t.Errorf("err = %v", err)
	}
}

func TestFindFilesIsDeterministic(t *testing.T) {
	snapshot := documented(t)
	first, _ := snapshot.FindFiles("", "records store")
	second, _ := snapshot.FindFiles("", "records store")
	if !reflect.DeepEqual(first, second) {
		t.Error("the same query must rank the same way")
	}
}

func TestSearchNeedsAnIndexNotJustAGraph(t *testing.T) {
	snapshot := documented(t)
	snapshot.HasIndex = false
	if _, err := snapshot.FindFiles("x", ""); err == nil || !strings.Contains(err.Error(), "no context index") {
		t.Errorf("err = %v", err)
	}
	if _, err := snapshot.Describe("store"); err == nil {
		t.Error("describe needs the index too")
	}
}

func TestTermsSplitOnPunctuationAndDropEmpties(t *testing.T) {
	if got := terms("  Hello, World!  foo_bar "); !reflect.DeepEqual(got, []string{"hello", "world", "foo", "bar"}) {
		t.Errorf("%v", got)
	}
}
