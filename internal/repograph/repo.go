package repograph

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Repo is the view an Extractor gets: the files that survived listing and
// ignoring, and a way to read them.
type Repo interface {
	// Files returns repo-relative, slash-separated paths, sorted.
	Files() []string
	Read(path string) ([]byte, error)
}

// Lister names the files of a repository and its current revision. The git
// adapter is the production implementation; tests supply a directory walker.
type Lister interface {
	TrackedFiles(root string) ([]string, error)
	// HeadSHA returns the current revision, or "" when there is none.
	HeadSHA(root string) string
}

type dirRepo struct {
	root  string
	files []string
}

func (r dirRepo) Files() []string { return r.files }

func (r dirRepo) Read(path string) ([]byte, error) {
	return os.ReadFile(filepath.Join(r.root, filepath.FromSlash(path)))
}

// gitTimeout bounds each git call: they are local, but an unbounded one would
// hang a hook rather than fail it.
const gitTimeout = 60 * time.Second

// GitLister lists files with `git ls-files`, so untracked and ignored files
// never enter the graph.
type GitLister struct{}

// TrackedFiles returns every tracked path.
func (GitLister) TrackedFiles(root string) ([]string, error) {
	out, err := runGit(root, "ls-files", "-z")
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	var files []string
	for _, name := range strings.Split(out, "\x00") {
		if name != "" {
			files = append(files, name)
		}
	}
	return files, nil
}

// HeadSHA returns the current commit, or "" for a repo with no commits.
func (GitLister) HeadSHA(root string) string {
	out, err := runGit(root, "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func runGit(root string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	out, err := cmd.Output()
	return string(out), err
}
