package repodoc

import (
	"reflect"
	"testing"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/factsheet"
)

func emptyIndex() contextindex.Index {
	return contextindex.Index{Entries: map[string]contextindex.Entry{}}
}

func paths(tasks []Task) []string {
	out := []string{}
	for _, task := range tasks {
		out = append(out, task.Path)
	}
	return out
}

func TestPlanDocumentsChildrenBeforeParents(t *testing.T) {
	got := paths(Plan(emptyIndex(), tree(), Options{}))
	want := []string{"svc/api", "svc/store", "svc", "."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestPlanSendsOnlyDirectoriesWhoseHashChanged(t *testing.T) {
	index := contextindex.Index{Entries: map[string]contextindex.Entry{
		"svc/api":   {InputHash: "h-api"},     // fresh: never sent
		"svc/store": {InputHash: "old-store"}, // stale
		"svc":       {InputHash: "old-svc"},   // stale
		// "." has no entry: undocumented
	}}
	tasks := Plan(index, tree(), Options{})
	if want := []string{"svc/store", "svc", "."}; !reflect.DeepEqual(paths(tasks), want) {
		t.Errorf("tasks = %v, want %v", paths(tasks), want)
	}
	if tasks[0].Status != contextindex.StatusStale || tasks[2].Status != contextindex.StatusUndocumented {
		t.Errorf("statuses = %v %v", tasks[0].Status, tasks[2].Status)
	}
}

func TestPlanOfAFullyFreshTreeIsEmpty(t *testing.T) {
	entries := map[string]contextindex.Entry{}
	for _, sheet := range tree() {
		entries[sheet.Path] = contextindex.Entry{InputHash: sheet.InputHash}
	}
	if tasks := Plan(contextindex.Index{Entries: entries}, tree(), Options{}); len(tasks) != 0 {
		t.Errorf("a fresh tree costs nothing: %v", paths(tasks))
	}
}

func TestPlanLimitKeepsTheDeepestDirectories(t *testing.T) {
	if got := paths(Plan(emptyIndex(), tree(), Options{Limit: 2})); !reflect.DeepEqual(got, []string{"svc/api", "svc/store"}) {
		t.Errorf("a limit must keep leaves, never a parent without its children: %v", got)
	}
	if got := len(Plan(emptyIndex(), tree(), Options{Limit: 99})); got != 4 {
		t.Errorf("a limit above the count changes nothing: %d", got)
	}
}

func TestPlanPathPrefixKeepsTheDirectoryAndItsDescendants(t *testing.T) {
	sheets := append(tree(), factsheet.FactSheet{Path: "svcs", InputHash: "h"})
	if got := paths(Plan(emptyIndex(), sheets, Options{PathPrefix: "svc"})); !reflect.DeepEqual(got, []string{"svc/api", "svc/store", "svc"}) {
		t.Errorf("svc and below, but not the sibling svcs: %v", got)
	}
	if got := len(Plan(emptyIndex(), sheets, Options{PathPrefix: "."})); got != 5 {
		t.Errorf("the root prefix keeps everything: %d", got)
	}
}

func TestDepthOfTheRootIsZero(t *testing.T) {
	if depth(".") != 0 || depth("a") != 1 || depth("a/b/c") != 3 {
		t.Error("depth")
	}
}
