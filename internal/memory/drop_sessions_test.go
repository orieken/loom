package memory_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/orieken/loom/internal/memory"
)

func storeWithSessionRows(t *testing.T) *memory.Store {
	t.Helper()
	store := newSessionStore(t)
	ingestSessions(t, store, mixedSource())
	return store
}

func TestPlanListsTheTableAndDropsNothing(t *testing.T) {
	store := storeWithSessionRows(t)

	planned, err := memory.PlanSessionTableDrop(store.Path())

	if err != nil || !reflect.DeepEqual(planned, []string{"session_corrections"}) {
		t.Fatalf("plan = %v, %v", planned, err)
	}
	if !hasTable(t, store, "session_corrections") || len(storedRows(t, store)) != 3 {
		t.Error("the dry run changed the database")
	}
}

func TestDropRemovesOnlyTheSessionTable(t *testing.T) {
	store := storeWithSessionRows(t)
	before := tableNames(t, store)

	dropped, err := memory.DropSessionTables(store.Path())

	if err != nil || !reflect.DeepEqual(dropped, []string{"session_corrections"}) {
		t.Fatalf("drop = %v, %v", dropped, err)
	}
	var want []string
	for _, name := range before {
		if name != "session_corrections" {
			want = append(want, name)
		}
	}
	if got := tableNames(t, store); !reflect.DeepEqual(got, want) {
		t.Errorf("tables after drop = %v, want %v", got, want)
	}
}

func TestDropIsANoOpWhenTheTableIsAbsent(t *testing.T) {
	store := newSessionStore(t)
	before := tableNames(t, store)

	dropped, err := memory.DropSessionTables(store.Path())
	planned, planErr := memory.PlanSessionTableDrop(store.Path())

	if err != nil || planErr != nil || len(dropped) != 0 || len(planned) != 0 {
		t.Errorf("drop %v %v, plan %v %v", dropped, err, planned, planErr)
	}
	if !reflect.DeepEqual(tableNames(t, store), before) {
		t.Error("a no-op drop changed the tables")
	}
}

func TestDropDoesNotCreateAMissingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent", "episodes.db")

	dropped, err := memory.DropSessionTables(path)
	planned, planErr := memory.PlanSessionTableDrop(path)

	if err != nil || planErr != nil || len(dropped) != 0 || len(planned) != 0 {
		t.Errorf("drop %v %v, plan %v %v", dropped, err, planned, planErr)
	}
	if _, isPresent := fileMode(t, path); isPresent {
		t.Error("the drop routine created the database file")
	}
	if _, isPresent := fileMode(t, filepath.Dir(path)); isPresent {
		t.Error("the drop routine created the directory")
	}
}
