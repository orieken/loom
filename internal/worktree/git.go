// Package worktree reports what a run has changed in the repository it is
// running against (roadmap L3.30).
//
// The executor needs this to notice a stage editing source it never declared
// it would edit. Run 4's accessibility-engineer declares
// `tools: Read, Glob, Grep, Bash` — no edit tool anywhere — and modified
// handlers.go, correctly and helpfully, through Bash. `--allowed-tools` is an
// allowlist over named tools, not a write barrier: any stage holding Bash can
// write through a heredoc or `sed -i`, so "read-only stage" was a property
// nothing enforced and nothing checked.
//
// Checking is the honest response. Enforcing would mean removing Bash from
// six reviewing stages that legitimately use it to run checks, which costs
// more than the defect.
package worktree

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
)

// Git digests a git working tree.
type Git struct {
	root string
}

// New returns a digester rooted at a repository.
func New(root string) *Git { return &Git{root: root} }

// Digest fingerprints every uncommitted change in the tree.
//
// It hashes content, not just status. `git status --porcelain` alone would
// have missed run 4's violation entirely: accessibility-engineer edited a
// file the developer had already modified, so the file's status never
// changed — only its bytes did.
func (g *Git) Digest() (string, error) {
	status, err := g.run("status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return "", err
	}
	tracked, err := g.trackedDiff()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append(tracked, status...))
	return hex.EncodeToString(sum[:]), nil
}

// trackedDiff is empty in a repository with no commits, where `git diff
// HEAD` fails because there is no HEAD to diff against. That is an ordinary
// state — a freshly initialised project — and every file in it is untracked,
// so the status output above already describes the whole tree.
func (g *Git) trackedDiff() ([]byte, error) {
	if _, err := g.run("rev-parse", "--verify", "HEAD"); err != nil {
		return nil, nil
	}
	return g.run("diff", "HEAD")
}

func (g *Git) run(args ...string) ([]byte, error) {
	command := exec.Command("git", args...)
	command.Dir = g.root
	command.Stderr = nil
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s in %s: %w", args[0], g.root, err)
	}
	return output, nil
}

// ChangedPaths lists every path with uncommitted changes, repo-relative and
// slash-separated (roadmap L2.24). Rename entries carry "old -> new" and
// both halves are reported: a stage may legitimately claim either.
func (g *Git) ChangedPaths() ([]string, error) {
	output, err := g.run("status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(string(output), "\n") {
		if len(line) < 4 {
			continue
		}
		// Porcelain v1: two status characters, a space, then the path.
		paths = append(paths, splitRename(strings.TrimSpace(line[3:]))...)
	}
	return paths, nil
}

func splitRename(entry string) []string {
	entry = strings.Trim(entry, `"`)
	if before, after, found := strings.Cut(entry, " -> "); found {
		return []string{strings.Trim(before, `"`), strings.Trim(after, `"`)}
	}
	return []string{entry}
}
