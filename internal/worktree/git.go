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
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

// HeadRef is the commit a run starts from, recorded so a gate can later
// ask what the run changed. Empty with no error in a repository with no
// commits: there is nothing to measure from, and that is an ordinary state
// for a freshly initialised project rather than a failure.
func (g *Git) HeadRef() (string, error) {
	output, err := g.run("rev-parse", "--verify", "HEAD")
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(output)), nil
}

// DiffLinesSince counts lines added and removed since a commit, across
// both tracked changes and new files.
//
// `git diff` alone would miss every file the run created, because an
// untracked file is not in the diff — a run that added three new packages
// would report zero. Counting them separately is the difference between
// measuring the change and measuring the part of it git happened to be
// tracking already.
func (g *Git) DiffLinesSince(ref string) (int, error) {
	if ref == "" {
		return 0, fmt.Errorf("no base commit to measure from")
	}
	tracked, err := g.trackedDiffLines(ref)
	if err != nil {
		return 0, err
	}
	untracked, err := g.untrackedLines()
	if err != nil {
		return 0, err
	}
	return tracked + untracked, nil
}

// trackedDiffLines sums the added and removed columns of `git diff
// --numstat`. A binary file reports "-" in both columns and contributes
// nothing, which is correct: its line count is not a meaningful number.
func (g *Git) trackedDiffLines(ref string) (int, error) {
	output, err := g.run("diff", "--numstat", ref)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		total += numstatLines(line)
	}
	return total, nil
}

// numstatLines reads the added and removed counts from one numstat row.
func numstatLines(row string) int {
	fields := strings.Fields(row)
	if len(fields) < 2 {
		return 0
	}
	total := 0
	for _, field := range fields[:2] {
		if count, err := strconv.Atoi(field); err == nil {
			total += count
		}
	}
	return total
}

// untrackedLines counts every line in every file git is not yet tracking.
func (g *Git) untrackedLines() (int, error) {
	output, err := g.run("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return 0, err
	}
	total := 0
	for _, name := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if name == "" {
			continue
		}
		total += countLines(filepath.Join(g.root, name))
	}
	return total, nil
}

// countLines treats an unreadable file as zero. A file that vanished
// between listing and reading is a race, not a reason to fail a gate.
//
// The trailing newline is why this is not just a count of '\n' plus one.
// A well-formed text file ends with one, so "line1\nline2\n" holds two
// lines and contains two newlines; adding one over-counted every such
// file. Only a file whose last line is unterminated needs the extra.
func countLines(path string) int {
	content, err := os.ReadFile(path)
	if err != nil || len(content) == 0 {
		return 0
	}
	lines := bytes.Count(content, []byte{'\n'})
	if content[len(content)-1] != '\n' {
		lines++
	}
	return lines
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
