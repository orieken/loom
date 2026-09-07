// Package frameworkfs installs embedded framework content safely.
package frameworkfs

import (
	"fmt"
	iofs "io/fs"
	"os"
	"time"

	"github.com/orieken/loom/cmd/loom/internal/ownership"
)

// Reporter receives one human-readable filesystem action.
type Reporter func(message string)

// Writer installs embedded content beneath one validated target.
type Writer struct {
	content  iofs.FS
	target   string
	cache    string
	isCopy   bool
	isDryRun bool
	report   Reporter
	now      func() time.Time

	// prior is what a previous install recorded as loom's own; written
	// accumulates what this install writes. Both are nil until
	// WithOwnership is called, which leaves the pre-L3.26 behaviour
	// available to callers that have not been migrated.
	prior    ownership.Ledger
	written  ownership.Ledger
	isForced bool
}

// NewWriter creates a target-scoped embedded filesystem writer.
func NewWriter(content iofs.FS, target, cache string, isCopy, isDryRun bool, report Reporter) *Writer {
	return &Writer{content: content, target: target, cache: cache,
		isCopy: isCopy, isDryRun: isDryRun, report: report, now: time.Now}
}

// Reporter returns the writer's action reporter for related filesystem work.
func (writer *Writer) Reporter() Reporter { return writer.report }

// Install copies or links one embedded file or directory.
//
// A directory is installed file by file rather than as a unit, so that
// content the project already keeps alongside loom's — a team's own agent in
// .claude/agents — survives (roadmap L3.26). Without a ledger the writer
// keeps its pre-L3.26 whole-directory behaviour.
func (writer *Writer) Install(source, destination string) (bool, error) {
	if writer.written == nil {
		return writer.installUnowned(source, destination)
	}
	return writer.installOwned(source, destination)
}

func (writer *Writer) installUnowned(source, destination string) (bool, error) {
	if writer.isCopy {
		return writer.installCopy(source, destination)
	}
	return writer.installLink(source, destination)
}

func (writer *Writer) installOwned(source, destination string) (bool, error) {
	entries, err := writer.expand(source, destination)
	if err != nil {
		return false, err
	}
	if err := writer.prepareEntryDirectories(entries); err != nil {
		return false, err
	}
	return writer.installEntries(entries)
}

// prepareEntryDirectories converts any directory symlink left by an older
// install into a real directory before files are written beneath it.
func (writer *Writer) prepareEntryDirectories(entries []entry) error {
	for _, parent := range entryDestinations(entries) {
		if _, err := writer.clearLinkedDirectory(parent); err != nil {
			return err
		}
	}
	return nil
}

func (writer *Writer) installEntries(entries []entry) (bool, error) {
	isAnyInstalled := false
	for _, item := range entries {
		installed, err := writer.installEntry(item)
		if err != nil {
			return false, err
		}
		isAnyInstalled = isAnyInstalled || installed
	}
	return isAnyInstalled, nil
}

func (writer *Writer) installEntry(item entry) (bool, error) {
	isAllowed, err := writer.isAllowed(item.destination)
	if err != nil || !isAllowed {
		return false, err
	}
	if _, err := writer.installUnowned(item.source, item.destination); err != nil {
		return false, err
	}
	return true, writer.record(item.source, item.destination)
}

// Copy installs embedded content regardless of the selected link strategy.
func (writer *Writer) Copy(source, destination string) (bool, error) {
	if _, err := writer.installCopy(source, destination); err != nil {
		return false, err
	}
	return true, writer.record(source, destination)
}

// Write installs generated content beneath the target.
func (writer *Writer) Write(destination string, content []byte) (bool, error) {
	path, err := safeDestination(writer.target, destination)
	if err != nil {
		return false, err
	}
	written, err := writer.writeGenerated(path, destination, content)
	if written && err == nil {
		writer.recordContent(destination, content)
	}
	return written, err
}

// WriteIfMissing installs generated content only when the destination is absent.
func (writer *Writer) WriteIfMissing(destination string, content []byte) (bool, error) {
	path, err := safeDestination(writer.target, destination)
	if err != nil {
		return false, err
	}
	if _, err := os.Lstat(path); err == nil {
		writer.report("skipped " + destination + " (already exists)")
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("inspect %s: %w", destination, err)
	}
	written, err := writer.writeGenerated(path, destination, content)
	if written && err == nil {
		writer.recordContent(destination, content)
	}
	return written, err
}

// CopyIfMissing copies embedded content only when the destination is absent.
func (writer *Writer) CopyIfMissing(source, destination string) (bool, error) {
	path, err := safeDestination(writer.target, destination)
	if err != nil {
		return false, err
	}
	if _, err := os.Lstat(path); err == nil {
		writer.report("skipped " + destination + " (already exists)")
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("inspect %s: %w", destination, err)
	}
	if _, err := writer.installCopy(source, destination); err != nil {
		return false, err
	}
	return true, writer.record(source, destination)
}
