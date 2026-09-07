package frameworkfs

import (
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"

	"github.com/orieken/loom/cmd/loom/internal/ownership"
)

// WithOwnership makes the writer install only where loom is allowed to
// (roadmap L3.26). The prior ledger says which paths a previous install
// wrote; isForced overrides the refusal to overwrite a locally edited file.
//
// A writer without a ledger installs nothing it does not own, which is the
// correct default for a first install into an occupied project.
func (writer *Writer) WithOwnership(prior ownership.Ledger, isForced bool) *Writer {
	writer.prior = prior
	writer.isForced = isForced
	writer.written = make(ownership.Ledger)
	return writer
}

// Written returns the paths this writer installed, with their digests, for
// recording in the manifest.
func (writer *Writer) Written() ownership.Ledger { return writer.written }

// entry is one embedded file and where it lands in the target.
type entry struct {
	source      string
	destination string
}

// expand lists the embedded files under a source. A directory source becomes
// one entry per file, because a directory is not the granularity at which a
// foreign file can be spared.
func (writer *Writer) expand(source, destination string) ([]entry, error) {
	info, err := iofs.Stat(writer.content, source)
	if err != nil {
		return nil, fmt.Errorf("inspect embedded %s: %w", source, err)
	}
	if !info.IsDir() {
		return []entry{{source, destination}}, nil
	}
	return writer.expandDirectory(source, destination)
}

func (writer *Writer) expandDirectory(source, destination string) ([]entry, error) {
	var entries []entry
	err := iofs.WalkDir(writer.content, source, func(path string, item iofs.DirEntry, walkErr error) error {
		if walkErr != nil || item.IsDir() {
			return walkErr
		}
		relative, relErr := embeddedRelative(source, path)
		if relErr != nil {
			return relErr
		}
		entries = append(entries, entry{path, destination + "/" + relative})
		return nil
	})
	return entries, err
}

// decide resolves one destination against the prior ledger.
func (writer *Writer) decide(destination string) (ownership.Decision, error) {
	path, err := safeDestination(writer.target, destination)
	if err != nil {
		return ownership.Skip, err
	}
	state, err := inspectState(path)
	if err != nil {
		return ownership.Skip, err
	}
	return writer.prior.Decide(destination, state), nil
}

// inspectState reads what occupies a path, resolving through any symlink so
// that a linked install compares against the content it points at.
func inspectState(path string) (ownership.State, error) {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return ownership.State{}, nil
	} else if err != nil {
		return ownership.State{}, fmt.Errorf("inspect %s: %w", path, err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		// Unreadable or a dangling link: present, but not provably ours.
		return ownership.State{Exists: true}, nil
	}
	return ownership.State{Exists: true, Digest: ownership.Digest(content)}, nil
}

// isAllowed reports whether install may write to a destination, reporting
// the reason when it may not.
func (writer *Writer) isAllowed(destination string) (bool, error) {
	decision, err := writer.decide(destination)
	if err != nil {
		return false, err
	}
	if !ownership.IsSkip(decision) {
		return true, nil
	}
	if writer.isForced && decision == ownership.SkipModified {
		return true, nil
	}
	writer.report("skipped " + destination + " (" + ownership.Reason(decision) + ")")
	return false, nil
}

// record marks a destination as loom's own, digesting the embedded content
// installed there. A directory source records every file beneath it, because
// ownership is per path and a directory is not a path loom can own as a unit.
func (writer *Writer) record(source, destination string) error {
	if writer.written == nil {
		return nil
	}
	entries, err := writer.expand(source, destination)
	if err != nil {
		return err
	}
	for _, item := range entries {
		if err := writer.recordFile(item); err != nil {
			return err
		}
	}
	return nil
}

func (writer *Writer) recordFile(item entry) error {
	data, err := iofs.ReadFile(writer.content, item.source)
	if err != nil {
		return fmt.Errorf("digest embedded %s: %w", item.source, err)
	}
	writer.written[item.destination] = ownership.Digest(data)
	return nil
}

// recordContent marks a generated (non-embedded) destination as loom's own.
func (writer *Writer) recordContent(destination string, content []byte) {
	if writer.written != nil {
		writer.written[destination] = ownership.Digest(content)
	}
}

// clearLinkedDirectory replaces a directory symlink from an older, directory
// granular install with a real directory, so per-file installs can populate
// it alongside whatever the project keeps there. A symlink loom did not
// record is left alone and reported.
func (writer *Writer) clearLinkedDirectory(destination string) (bool, error) {
	path, err := safeDestination(writer.target, destination)
	if err != nil {
		return false, err
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("inspect %s: %w", destination, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return true, nil
	}
	return writer.replaceLinkedDirectory(path, destination)
}

func (writer *Writer) replaceLinkedDirectory(path, destination string) (bool, error) {
	if _, isOwned := writer.prior[destination]; !isOwned {
		writer.report("skipped " + destination + " (" + ownership.Reason(ownership.Skip) + ")")
		return false, nil
	}
	if writer.isDryRun {
		writer.report("would replace the linked directory " + destination)
		return true, nil
	}
	if err := os.Remove(path); err != nil {
		return false, fmt.Errorf("remove linked directory %s: %w", destination, err)
	}
	writer.report("replaced the linked directory " + destination)
	return true, os.MkdirAll(path, 0o755)
}

// dirEntryPaths lists the destination directories an entry set populates.
func entryDestinations(entries []entry) []string {
	seen := make(map[string]bool, len(entries))
	var directories []string
	for _, item := range entries {
		parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(item.destination)))
		if parent != "." && !seen[parent] {
			seen[parent] = true
			directories = append(directories, parent)
		}
	}
	return directories
}
