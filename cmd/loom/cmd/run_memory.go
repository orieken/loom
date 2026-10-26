package cmd

// Episodic ingest at the end of a run (roadmap L3.5).
//
// The run's records are archived beside the artifacts they describe, then
// ingested into the project's episodic store. Both are best-effort and
// neither can fail a run: this preserves history, it does not control
// anything, and a learning store that can break a delivery is one people
// switch off.
//
// It runs on the gate-halt path too. Most runs a human looks at are halted
// ones, and a store that only knew about completed runs would miss the
// interesting half.

import (
	"path/filepath"

	"github.com/orieken/loom/internal/memory"
	"github.com/spf13/cobra"
)

// featureArchiveDir is where a delivery's artifacts are persisted, matching
// the layout deliver-feature already writes.
func featureArchiveDir(feature string) string {
	return filepath.Join("docs", "features", feature)
}

// archiveRoot is the repository root a run's archived records are made
// relative to: the expression run.go's prepareRunWorkspace uses to build
// StageInput.ProjectRoot, resolved independently rather than threaded
// through. The two agree because no production code calls os.Chdir (tests
// do, before either runs). A variable so a test can make it fail.
var archiveRoot = func() (string, error) { return filepath.Abs(".") }

// recordEpisode archives a run's records and ingests them. Failures are
// reported to stderr and otherwise ignored.
func recordEpisode(cmd *cobra.Command, workspaceDir, feature string) {
	root, err := archiveRoot()
	if err != nil {
		cmd.PrintErrf("memory: could not resolve repository root: %v\n", err)
		root = ""
	}
	if err := memory.ArchiveRecords(workspaceDir, featureArchiveDir(feature), root); err != nil {
		cmd.PrintErrf("memory: could not archive run records: %v\n", err)
	}
	if err := ingestWorkspace(workspaceDir); err != nil {
		cmd.PrintErrf("memory: could not record this run: %v\n", err)
	}
}

func ingestWorkspace(workspaceDir string) error {
	records, err := memory.ReadRecords(workspaceDir)
	if err != nil {
		return err
	}
	store, err := memory.Open(memory.DefaultPath("."))
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	_, err = store.Ingest(records.State, records.Events)
	return err
}
