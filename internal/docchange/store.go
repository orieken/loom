package docchange

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/orieken/loom/internal/repograph"
)

const (
	changeFile   = "change.json"
	approvalFile = "approval.json"
	appliedFile  = "applied.json"
)

// ErrNoSuchChange is returned for an id with no changeset.
var ErrNoSuchChange = errors.New("no such changeset")

// Save writes change.json.
func Save(root string, change Change) error {
	raw, err := json.MarshalIndent(change, "", "  ")
	if err != nil {
		return fmt.Errorf("encode changeset: %w", err)
	}
	return repograph.WriteFileAtomically(filepath.Join(Dir(root, change.ID), changeFile), append(raw, '\n'))
}

// Load reads a changeset.
func Load(root, id string) (Change, error) {
	raw, err := os.ReadFile(filepath.Join(Dir(root, id), changeFile))
	if errors.Is(err, os.ErrNotExist) {
		return Change{}, fmt.Errorf("%w %q: run `loom docs plan` first", ErrNoSuchChange, id)
	}
	if err != nil {
		return Change{}, err
	}
	var change Change
	if err := json.Unmarshal(raw, &change); err != nil {
		return Change{}, fmt.Errorf("parse changeset %s: %w", id, err)
	}
	return change, nil
}

// stage writes an artifact's final content under the change's own directory.
func stage(root, id, target string, content []byte) error {
	return repograph.WriteFileAtomically(StagedPath(root, id, target), content)
}

// readTarget returns a target file's content and whether it exists.
func readTarget(root, target string) (string, bool, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(target)))
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	return string(raw), err == nil, err
}

// digestOfTarget is the digest of a target file, "" if it does not exist.
func digestOfTarget(root, target string) (string, error) {
	content, exists, err := readTarget(root, target)
	if err != nil || !exists {
		return "", err
	}
	return Digest([]byte(content)), nil
}

// LatestChange returns the id of the newest changeset, or "".
func LatestChange(root string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(root, repograph.OutputDir, ChangesDir))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	latest := ""
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() > latest {
			latest = entry.Name()
		}
	}
	return latest, nil
}

// changeExists says a changeset directory already holds this id.
func changeExists(root, id string) bool {
	_, err := os.Stat(Dir(root, id))
	return err == nil
}
