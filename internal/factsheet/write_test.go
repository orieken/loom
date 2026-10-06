package factsheet

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSheetPathMirrorsTheDirectory(t *testing.T) {
	if got := FilePath("/r", "internal/state"); got != filepath.Join("/r", ".loom/facts/internal/state.json") {
		t.Errorf("got %s", got)
	}
	if got := FilePath("/r", "."); got != filepath.Join("/r", ".loom/facts/_root.json") {
		t.Errorf("got %s", got)
	}
}

func TestSecondWriteOverAnUnchangedRepoWritesNothing(t *testing.T) {
	root := copyFixture(t, "go")
	sheets := sheetsFor(t, root)
	first, err := Write(root, sheets)
	if err != nil || first.Written != len(sheets) {
		t.Fatalf("first write: %+v err=%v", first, err)
	}
	second, err := Write(root, sheetsFor(t, root))
	if err != nil || second.Written != 0 || second.Removed != 0 {
		t.Errorf("second write: %+v err=%v", second, err)
	}
}

func TestWriteTouchesOnlyTheSheetsThatChanged(t *testing.T) {
	root := copyFixture(t, "go")
	if _, err := Write(root, sheetsFor(t, root)); err != nil {
		t.Fatal(err)
	}
	writeText(t, root, "internal/store/store.go", "package store\n\nfunc Get() string { return \"z\" }\n")
	outcome, err := Write(root, sheetsFor(t, root))
	if err != nil || outcome.Written != 3 {
		t.Errorf("store, internal and the root change: %+v err=%v", outcome, err)
	}
}

func TestWriteRemovesSheetsOfDeletedDirectoriesAndEmptyFolders(t *testing.T) {
	root := copyFixture(t, "go")
	if _, err := Write(root, sheetsFor(t, root)); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "internal", "testsupport")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "cmd")); err != nil {
		t.Fatal(err)
	}
	outcome, err := Write(root, sheetsFor(t, root))
	if err != nil || outcome.Removed != 3 {
		t.Fatalf("testsupport, cmd and cmd/app sheets go: %+v err=%v", outcome, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".loom/facts/cmd")); !os.IsNotExist(err) {
		t.Error("an emptied sheet folder must be removed too")
	}
}

func TestWriteIgnoresAMissingFactsDirectory(t *testing.T) {
	outcome, err := Write(t.TempDir(), nil)
	if err != nil || outcome.Written != 0 || outcome.Removed != 0 {
		t.Errorf("%+v %v", outcome, err)
	}
}

func TestGeneratedSchemaIsValidJSON(t *testing.T) {
	raw, err := GenerateSchema()
	if err != nil || len(raw) == 0 || raw[0] != '{' {
		t.Errorf("schema: %v %q", err, raw)
	}
}

func TestLoadAllReadsEverySheetKeyedByDirectory(t *testing.T) {
	root := copyFixture(t, "go")
	sheets := sheetsFor(t, root)
	if _, err := Write(root, sheets); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadAll(root)
	if err != nil || len(loaded) != len(sheets) {
		t.Fatalf("%d of %d: %v", len(loaded), len(sheets), err)
	}
	if loaded["internal/store"].InputHash != sheetFor(t, sheets, "internal/store").InputHash {
		t.Error("a loaded sheet must equal the one written")
	}
}

func TestLoadAllOfARepoWithNoFactsIsEmptyNotAnError(t *testing.T) {
	loaded, err := LoadAll(t.TempDir())
	if err != nil || len(loaded) != 0 {
		t.Errorf("%v %v", loaded, err)
	}
}

func TestLoadAllFailsOnACorruptSheetRatherThanAnswerFromHalfTheFacts(t *testing.T) {
	root := copyFixture(t, "go")
	if _, err := Write(root, sheetsFor(t, root)); err != nil {
		t.Fatal(err)
	}
	writeText(t, root, ".loom/facts/internal/store.json", "{broken")
	if _, err := LoadAll(root); err == nil {
		t.Error("a partial map would answer confidently from incomplete facts")
	}
}
