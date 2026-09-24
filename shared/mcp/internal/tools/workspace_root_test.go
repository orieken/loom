package tools

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// escapeFixture builds <base>/workspace/project as the root, with a real
// <base>/etc beside it, so "../../etc" names a directory that exists outside
// the root — rejected for escaping, not for being missing.
func escapeFixture(t *testing.T) (WorkspaceRoot, string) {
	t.Helper()
	base := t.TempDir()
	rootDir := filepath.Join(base, "workspace", "project")
	for _, dir := range []string{filepath.Join(rootDir, "src"), filepath.Join(base, "etc")} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if err := os.Symlink(filepath.Join(base, "etc"), filepath.Join(rootDir, "escape")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	root, err := NewWorkspaceRoot(rootDir)
	if err != nil {
		t.Fatalf("NewWorkspaceRoot: %v", err)
	}
	return root, base
}

func TestPathsOutsideTheRootAreRejected(t *testing.T) {
	root, base := escapeFixture(t)
	for name, argument := range map[string]string{
		"the filesystem root":            "/",
		"a relative climb":               "../../etc",
		"an absolute path beside it":     filepath.Join(base, "etc"),
		"a symbolic link pointing out":   "escape",
		"a climb that re-enters nothing": "src/../../..",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := root.Resolve(argument); !errors.Is(err, ErrOutsideWorkspace) {
				t.Errorf("Resolve(%q) = %v, want ErrOutsideWorkspace", argument, err)
			}
		})
	}
}

func TestPathsInsideTheRootResolveToTheirRealLocation(t *testing.T) {
	root, _ := escapeFixture(t)
	want := filepath.Join(root.Dir(), "src")
	for _, argument := range []string{"src", "./src", "src/../src", want, "."} {
		got, err := root.Resolve(argument)
		if err != nil {
			t.Errorf("Resolve(%q) = %v, want it accepted", argument, err)
			continue
		}
		if argument != "." && got != want {
			t.Errorf("Resolve(%q) = %q, want %q", argument, got, want)
		}
	}
}

func TestEmptyAndMissingPathsAreRejected(t *testing.T) {
	root, _ := escapeFixture(t)
	for _, argument := range []string{"", "   ", "no-such-dir"} {
		if _, err := root.Resolve(argument); err == nil {
			t.Errorf("Resolve(%q) accepted", argument)
		}
	}
}

// A server that cannot say where its workspace is must read nowhere.
func TestTheZeroRootRejectsEverything(t *testing.T) {
	root, _ := escapeFixture(t)
	for _, argument := range []string{"/", root.Dir(), filepath.Join(root.Dir(), "src")} {
		if _, err := (WorkspaceRoot{}).Resolve(argument); err == nil {
			t.Errorf("the zero root accepted %q", argument)
		}
	}
}

func TestARootMustBeAnExistingDirectory(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	for _, dir := range []string{file, filepath.Join(t.TempDir(), "missing")} {
		if _, err := NewWorkspaceRoot(dir); err == nil {
			t.Errorf("NewWorkspaceRoot(%q) accepted", dir)
		}
	}
}
