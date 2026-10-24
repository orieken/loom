// Package factory is the software factory's supervisor (roadmap L3.65–L3.70):
// it reads work orders committed on a branch, claims the ready ones, runs
// each through `loom run` in its own git worktree, and records every claim,
// outcome and approval in an append-only ledger.
//
// The supervisor owns three things and nothing else: the ledger, the
// worktrees, and the claim snapshots inside them. It never writes an intake
// file (the human owns intent) and never writes a run's state (the executor
// owns progress). Everything it knows about a run it reads from the run's
// own state file after `loom run` exits.
package factory

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths, relative to the repository root unless noted.
const (
	// StateDir holds everything the factory owns in a repository. It
	// ignores itself, so the root checkout stays clean.
	StateDir = ".claude/factory"
	// IntakeDir is where work orders are committed.
	IntakeDir = "intake"
	// FeaturesDir holds hand-written specs, whose stems an order must not reuse.
	FeaturesDir = "features"
	// BranchPrefix names the branch each order's run works on.
	BranchPrefix = "factory/"
)

// Layout is where the factory keeps its files in one repository.
type Layout struct {
	Root string
}

// LedgerPath is the append-only record of every claim and outcome.
func (l Layout) LedgerPath() string { return filepath.Join(l.Root, StateDir, "ledger.jsonl") }

// LockPath keeps a second supervisor from running against the same ledger.
func (l Layout) LockPath() string { return filepath.Join(l.Root, StateDir, "supervisor.lock") }

// WorktreePath is where an order's run executes.
func (l Layout) WorktreePath(id string) string {
	return filepath.Join(l.Root, StateDir, "worktrees", id)
}

// RunArchivePath keeps a shipped run's workspace after its worktree is gone.
func (l Layout) RunArchivePath(id string) string { return filepath.Join(l.Root, StateDir, "runs", id) }

// LogPath collects what an order's `loom run` invocations printed.
func (l Layout) LogPath(id string) string { return filepath.Join(l.Root, StateDir, "logs", id+".log") }

// ClaimPath is the snapshot a run reads as its spec, relative to the
// order's worktree (ADR-010, 1C). Relative, because `loom run` resolves its
// workspace from the working directory and the agents are scoped to it.
func ClaimPath(id string) string { return filepath.Join(StateDir, "claims", id+".md") }

// RunStatePath is where `loom run` keeps the state of an order's run.
func RunStatePath(worktree, id string) string {
	return filepath.Join(worktree, ".claude", "feature-workspace", id, "run-state.json")
}

// BranchFor is the branch an order's run works on.
func BranchFor(id string) string { return BranchPrefix + id }

// Prepare creates the factory's directory in the root, ignoring itself.
func (l Layout) Prepare() error {
	return ignoredDirectory(filepath.Join(l.Root, StateDir))
}

// ignoredDirectory creates dir with a .gitignore that ignores everything in
// it, so nothing the factory writes is ever mistaken for a run's change.
func ignoredDirectory(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	ignore := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(ignore, []byte("*\n"), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", ignore, err)
	}
	return nil
}
