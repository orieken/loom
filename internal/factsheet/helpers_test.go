package factsheet

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/repograph"
)

const fixturesDir = "../../tests/fixtures/repograph"

type walkLister struct{}

func (walkLister) TrackedFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if relative, relErr := filepath.Rel(root, path); relErr == nil && !isSidecar(relative) {
			files = append(files, filepath.ToSlash(relative))
		}
		return nil
	})
	return files, err
}

func isSidecar(relative string) bool { return strings.HasPrefix(relative, ".loom/") }

func (walkLister) HeadSHA(string) string { return "fixture" }

func copyFixture(t *testing.T, name string) string {
	t.Helper()
	source := filepath.Join(fixturesDir, name)
	target := t.TempDir()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(source, path)
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(target, relative), 0o755)
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(filepath.Join(target, relative), content, 0o644)
	})
	if err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return target
}

// sheetsFor indexes a repo directory and builds its fact sheets.
func sheetsFor(t *testing.T, root string) []FactSheet {
	t.Helper()
	now := func() time.Time { return time.Unix(0, 0) }
	result, err := repograph.Index(root, walkLister{}, repograph.DefaultExtractors(), now)
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	return Build(result.Repo, result.Graph)
}

func sheetFor(t *testing.T, sheets []FactSheet, dir string) FactSheet {
	t.Helper()
	for _, sheet := range sheets {
		if sheet.Path == dir {
			return sheet
		}
	}
	t.Fatalf("no sheet for %q in %d sheets", dir, len(sheets))
	return FactSheet{}
}

func hashes(sheets []FactSheet) map[string]string {
	out := map[string]string{}
	for _, sheet := range sheets {
		out[sheet.Path] = sheet.InputHash
	}
	return out
}

func changedDirectories(before, after []FactSheet) []string {
	old, now := hashes(before), hashes(after)
	var changed []string
	for dir, hash := range now {
		if old[dir] != hash {
			changed = append(changed, dir)
		}
	}
	sortStrings(changed)
	return changed
}

func sortStrings(items []string) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j] < items[j-1]; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

func writeText(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
