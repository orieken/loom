package memory

// Reading a run's records from a workspace or a feature archive.
//
// Both locations are supported because the archived copy is the durable
// record: `.claude/feature-workspace/<feature>/` is temporary and gets
// cleaned, while `docs/features/<name>/` is committed. A store rebuilt from
// the archive after someone deletes episodes.db is the reason ingest is
// idempotent.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/state"
)

// Records is one run's raw material.
type Records struct {
	State  *orchestrator.RunState
	Events []orchestrator.Event
	Source string
}

// ReadRecords loads a run's state and timeline from a directory holding
// them. A missing timeline is not an error — a run that halted before
// writing one still has state worth keeping, and refusing it would drop the
// runs a human most often looks at.
func ReadRecords(dir string) (Records, error) {
	statePath := filepath.Join(dir, orchestrator.RunStateFileName)
	state, err := orchestrator.NewStateStore(statePath).Load()
	if err != nil {
		return Records{}, fmt.Errorf("read run state: %w", err)
	}
	if state == nil {
		return Records{}, fmt.Errorf("no run state in %s — nothing to ingest", dir)
	}
	events, err := orchestrator.NewTimeline(statePath).Read()
	if err != nil {
		return Records{}, fmt.Errorf("read run events: %w", err)
	}
	return Records{State: state, Events: events, Source: dir}, nil
}

// ArchiveRecords copies a run's state, timeline and typed stage documents
// into the feature archive, so the history is version-controlled beside the
// artifacts it describes and survives the workspace being cleaned.
//
// It is best-effort at its call sites: losing the archive copy costs
// durability, not the run.
func ArchiveRecords(workspaceDir, archiveDir string) error {
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return fmt.Errorf("create archive directory: %w", err)
	}
	for _, name := range []string{orchestrator.RunStateFileName, orchestrator.RunEventsFileName} {
		if err := copyIfPresent(filepath.Join(workspaceDir, name), filepath.Join(archiveDir, name)); err != nil {
			return err
		}
	}
	return archiveTypedState(workspaceDir, archiveDir)
}

// archiveTypedState copies the typed stage documents the run produced.
//
// These are not a nice-to-have alongside run-state.json: they are where the
// facts live. `run-state.json` records that a stage completed and which
// KIND of document it produced; the document itself holds the review
// verdict, the security findings, the test results and the changed paths —
// everything the policy evaluator reads (see orchestrator's gateContext).
//
// Archiving only state and timeline made every finished run unanalysable
// for those questions. An L2.19 experiment on 2026-09-18 found all four
// sourceable policy facts resolving UNKNOWN against a recorded run: the
// stages had completed, their kinds were in run-state, and the documents
// were gone. `completedStageOfKind` succeeded and the read failed, which is
// indistinguishable from a fact that never existed. See
// docs/audits/l219-policy-dry-run-2026-09-18.md.
//
// Every kind is copied rather than the four a policy reads today. The set of
// questions asked of history grows, and re-deciding which facts were worth
// keeping is only possible while the runs still have them.
func archiveTypedState(workspaceDir, archiveDir string) error {
	sourceDir := filepath.Join(workspaceDir, state.TypedStateDir)
	entries, err := os.ReadDir(sourceDir)
	if os.IsNotExist(err) {
		// A run whose stages all produced markdown has no typed state, and
		// that is an ordinary outcome rather than a failure.
		return nil
	}
	if err != nil {
		return fmt.Errorf("read typed state %s: %w", sourceDir, err)
	}

	targetDir := filepath.Join(archiveDir, state.TypedStateDir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create typed state archive: %w", err)
	}
	return copyStageDocuments(entries, sourceDir, targetDir)
}

// copyStageDocuments copies the .json stage documents and nothing else. A
// directory or a stray file beside them is not a stage document, and
// copying it would put something in the archive that no reader decodes.
func copyStageDocuments(entries []os.DirEntry, sourceDir, targetDir string) error {
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		if err := copyIfPresent(filepath.Join(sourceDir, name), filepath.Join(targetDir, name)); err != nil {
			return err
		}
	}
	return nil
}

// copyIfPresent treats a missing source as nothing to do: a run with no
// timeline yet is a normal state, not a failure.
func copyIfPresent(source, target string) error {
	content, err := os.ReadFile(source)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", source, err)
	}
	if err := os.WriteFile(target, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}
	return nil
}

// ArchivedRunDirs lists every feature archive holding a run's records, so a
// store can be rebuilt from what is in git after the workspaces are gone —
// and so deliveries that predate the store can be imported.
func ArchivedRunDirs(featuresDir string) ([]string, error) {
	entries, err := os.ReadDir(featuresDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read feature archive %s: %w", featuresDir, err)
	}
	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		dir := filepath.Join(featuresDir, entry.Name())
		if entry.IsDir() && hasRunState(dir) {
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

// hasRunState is what separates an archived run from a feature directory
// that only ever held markdown — most existing archives are the latter, and
// skipping them quietly is correct rather than an error.
func hasRunState(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, orchestrator.RunStateFileName))
	return err == nil
}
