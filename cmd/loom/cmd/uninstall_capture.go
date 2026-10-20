package cmd

// Removal of everything `loom install --capture` made. Capture is not a
// platform, so the platform records do not cover it (architecture-notes.md
// decision 7).
//
// Which uninstall removes capture:
//   - no --platform: yes, with everything else;
//   - --platform claude-code: yes, because the hooks live in a Claude Code
//     file and make no sense without that platform;
//   - any other --platform: no, capture stays.
//
// The stage removes the hook entries, the sessions directory, the salt, the
// failure log, the purge stamp and the session table in episodes.db.
// It leaves a capture.yaml that the user wrote, and says so.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/orieken/loom/cmd/loom/internal/manifest"
	"github.com/orieken/loom/cmd/loom/internal/settings"
	"github.com/orieken/loom/internal/capture"
	"github.com/orieken/loom/internal/memory"
)

type captureRemoval struct {
	layout   capture.Layout
	record   *manifest.CaptureRecord
	isDryRun bool
	output   uninstallOutput
}

// removesCapture says whether this uninstall takes capture with it.
func (r uninstallRequest) removesCapture() bool {
	return r.platform == "" || r.platform == claudePlatform
}

// uninstallCapture removes the capture artifacts and reports how many it
// removed and whether it ran. It does nothing when the manifest has no
// capture record and no capture artifact exists.
func uninstallCapture(request uninstallRequest, installed manifest.Manifest, output uninstallOutput) (int, bool, error) {
	if !request.removesCapture() {
		return 0, false, nil
	}
	removal := captureRemoval{layout: capture.NewLayout(request.target), record: installed.Capture, isDryRun: request.isDryRun, output: output}
	if installed.Capture == nil && !removal.hasArtifacts() {
		return 0, false, nil
	}
	removed, err := removal.run()
	return removed, true, err
}

// ownedCapturePaths lists the files and directories that capture creates.
// The failure log and the counter of an earlier install are both listed, so
// uninstall removes either format.
func ownedCapturePaths(layout capture.Layout) []string {
	return []string{layout.SessionsDir(), layout.SaltPath(), layout.FailureLogPath(), layout.FailureLogTempPath(), layout.LegacyFailuresPath(), layout.PurgeStampPath()}
}

func (r captureRemoval) selector() settings.Selector { return captureSelector(r.record) }

// hasArtifacts looks for capture leftovers when the manifest has no record.
func (r captureRemoval) hasArtifacts() bool {
	for _, path := range ownedCapturePaths(r.layout) {
		if _, err := os.Lstat(path); err == nil {
			return true
		}
	}
	entries, _ := settings.CountEntries(r.layout.SettingsLocalPath(), r.selector())
	tables, _ := memory.PlanSessionTableDrop(r.layout.DatabasePath())
	return entries > 0 || len(tables) > 0
}

func (r captureRemoval) run() (int, error) {
	removed, err := r.removeHooks()
	if err != nil {
		return removed, err
	}
	for _, path := range ownedCapturePaths(r.layout) {
		didRemove, removeErr := r.removePath(path)
		if removeErr != nil {
			return removed, removeErr
		}
		if didRemove {
			removed++
		}
	}
	tables, err := r.dropTables()
	removed += tables
	r.removeEmptyMemoryDirectory()
	r.mentionConfig()
	return removed, err
}

// removeHooks takes out the capture entries. A settings file the adapter
// does not understand is not an error here: uninstall tells the user and
// goes on, because the rest of the removal does not depend on it.
func (r captureRemoval) removeHooks() (int, error) {
	path := r.layout.SettingsLocalPath()
	display := relativeSlash(r.layout.Root(), path)
	if r.isDryRun {
		return r.planHooks(path, display)
	}
	result, err := settings.Unmerge(path, r.selector(), r.origin())
	if r.tellAboutRefusal(err) {
		return 0, nil
	}
	if result.Removed > 0 {
		r.output.removed(fmt.Sprintf("%d capture hook entries in %s", result.Removed, display))
	}
	if result.DeletedFile {
		r.output.removed(display)
	}
	return result.Removed, err
}

func (r captureRemoval) planHooks(path, display string) (int, error) {
	count, err := settings.CountEntries(path, r.selector())
	if r.tellAboutRefusal(err) {
		return 0, nil
	}
	if count > 0 {
		r.output.removed(fmt.Sprintf("%d capture hook entries in %s", count, display))
	}
	return count, err
}

// origin tells whether the install made the settings file.
func (r captureRemoval) origin() settings.FileOrigin {
	if r.record != nil && r.record.SettingsCreated {
		return settings.CreatedByLoom
	}
	return settings.ExistedBefore
}

// tellAboutRefusal prints the manual instructions for a settings file that
// the adapter will not edit. It reports whether err was such a refusal.
func (r captureRemoval) tellAboutRefusal(err error) bool {
	var refusal *settings.NotMergeableError
	if !errors.As(err, &refusal) {
		return false
	}
	r.output.note(fmt.Sprintf("⚠ left %s as it is (%s). Remove the entries whose command starts with %q by hand", relativeSlash(r.layout.Root(), refusal.Path), refusal.Reason, captureCommandPrefix))
	return true
}

// removePath removes one capture file or directory. A missing path is fine.
func (r captureRemoval) removePath(path string) (bool, error) {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	display := relativeSlash(r.layout.Root(), path)
	if r.isDryRun {
		r.output.removed(display)
		return true, nil
	}
	if err := os.RemoveAll(path); err != nil {
		return false, fmt.Errorf("remove %s: %w", display, err)
	}
	r.output.removed(display)
	return true, nil
}

// dropTables drops the session table in episodes.db. The human accepted
// this DROP (internal/memory/drop_sessions.go). A dry run only lists it.
func (r captureRemoval) dropTables() (int, error) {
	database := r.layout.DatabasePath()
	drop := memory.DropSessionTables
	if r.isDryRun {
		drop = memory.PlanSessionTableDrop
	}
	tables, err := drop(database)
	for _, table := range tables {
		r.output.removed(fmt.Sprintf("table %s in %s", table, relativeSlash(r.layout.Root(), database)))
	}
	return len(tables), err
}

// removeEmptyMemoryDirectory removes .claude/memory only when the removal
// left it empty. A directory that holds anything else stays.
func (r captureRemoval) removeEmptyMemoryDirectory() {
	if r.isDryRun {
		return
	}
	_ = os.Remove(filepath.Dir(r.layout.SaltPath()))
}

// mentionConfig lists a capture.yaml that the user wrote. Uninstall does not
// delete user configuration.
func (r captureRemoval) mentionConfig() {
	if _, err := os.Lstat(r.layout.ConfigPath()); err == nil {
		r.output.leftInPlace(relativeSlash(r.layout.Root(), r.layout.ConfigPath()))
	}
}
