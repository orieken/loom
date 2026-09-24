package analyzers

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func goFiles(path string) bool { return strings.HasSuffix(path, ".go") }

// A link inside the workspace must not lead the walk outside it (L2.3).
func TestCollectFilesNeverFollowsASymbolicLink(t *testing.T) {
	outside, root := t.TempDir(), t.TempDir()
	write(t, filepath.Join(outside, "secret.go"), "package secret")
	write(t, filepath.Join(root, "real.go"), "package real")
	if err := os.Symlink(filepath.Join(outside, "secret.go"), filepath.Join(root, "linked.go")); err != nil {
		t.Fatalf("symlink file: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked-dir")); err != nil {
		t.Fatalf("symlink dir: %v", err)
	}

	files, err := CollectFiles(root, goFiles)
	if err != nil {
		t.Fatalf("CollectFiles: %v", err)
	}
	if len(files) != 1 || filepath.Base(files[0]) != "real.go" {
		t.Errorf("files = %v, want only real.go — no link followed", files)
	}
}

func TestCollectFilesSkipsUninterestingDirectoriesAndFiltersByInclude(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.go", "b.txt", "sub/c.go", "vendor/d.go", "node_modules/e.go", ".hidden/f.go"} {
		write(t, filepath.Join(root, name), "x")
	}
	files, err := CollectFiles(root, goFiles)
	if err != nil {
		t.Fatalf("CollectFiles: %v", err)
	}
	var names []string
	for _, file := range files {
		relative, _ := filepath.Rel(root, file)
		names = append(names, filepath.ToSlash(relative))
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "a.go,sub/c.go" {
		t.Errorf("files = %v, want a.go and sub/c.go", names)
	}
}

func TestCollectFilesReturnsASingleFileAsItself(t *testing.T) {
	file := filepath.Join(t.TempDir(), "only.go")
	write(t, file, "package only")
	files, err := CollectFiles(file, goFiles)
	if err != nil || len(files) != 1 || files[0] != file {
		t.Errorf("CollectFiles(file) = %v, %v; want the file itself", files, err)
	}
	if _, err := CollectFiles(filepath.Join(t.TempDir(), "missing"), goFiles); err == nil {
		t.Error("a missing path was walked without error")
	}
}

// Before the ceilings, `projectPath: "/"` walked the whole disk.
func TestAWalkPastItsCeilingAborts(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		write(t, filepath.Join(root, name), "0123456789")
	}
	for name, limits := range map[string]walkLimits{
		"file count": {files: 2, bytes: 1 << 20},
		"byte total": {files: 100, bytes: 25},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := collectWithin(root, goFiles, limits); !errors.Is(err, ErrWalkTooLarge) {
				t.Errorf("err = %v, want ErrWalkTooLarge", err)
			}
		})
	}
	if files, err := collectWithin(root, goFiles, walkLimits{files: 3, bytes: 30}); err != nil || len(files) != 3 {
		t.Errorf("a walk exactly at its ceilings = %v, %v; want all three files", files, err)
	}
}
