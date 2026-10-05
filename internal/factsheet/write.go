package factsheet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// rootFile is the file name of the repo root's sheet.
const rootFile = "_root.json"

// Outcome counts what Write did.
type Outcome struct {
	Written int
	Removed int
}

// FilePath is where a directory's sheet lives, mirroring the directory path.
func FilePath(root, dir string) string {
	if dir == RootPath {
		return filepath.Join(root, OutputDir, rootFile)
	}
	return filepath.Join(root, OutputDir, filepath.FromSlash(dir)+".json")
}

// Write stores the sheets, touching only files whose bytes changed, and
// removes sheets of directories that no longer exist. A second run over an
// unchanged repo writes nothing.
func Write(root string, sheets []FactSheet) (Outcome, error) {
	var outcome Outcome
	keep := map[string]bool{}
	for _, sheet := range sheets {
		target := FilePath(root, sheet.Path)
		keep[target] = true
		changed, err := writeSheet(target, sheet)
		if err != nil {
			return outcome, err
		}
		if changed {
			outcome.Written++
		}
	}
	removed, err := prune(filepath.Join(root, OutputDir), keep)
	outcome.Removed = removed
	return outcome, err
}

func writeSheet(target string, sheet FactSheet) (bool, error) {
	raw, err := json.MarshalIndent(sheet, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode fact sheet %s: %w", sheet.Path, err)
	}
	raw = append(raw, '\n')
	if existing, readErr := os.ReadFile(target); readErr == nil && bytes.Equal(existing, raw) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return false, fmt.Errorf("create %s: %w", filepath.Dir(target), err)
	}
	temp := target + ".tmp"
	if err := os.WriteFile(temp, raw, 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", temp, err)
	}
	return true, os.Rename(temp, target)
}

// prune deletes sheet files outside keep, then any directory left empty.
func prune(facts string, keep map[string]bool) (int, error) {
	removed := 0
	err := filepath.WalkDir(facts, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".json") || keep[path] {
			return err
		}
		removed++
		return os.Remove(path)
	})
	if os.IsNotExist(err) {
		return 0, nil
	}
	removeEmptyDirectories(facts)
	return removed, err
}

func removeEmptyDirectories(facts string) {
	var directories []string
	_ = filepath.WalkDir(facts, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && entry.IsDir() && path != facts {
			directories = append(directories, path)
		}
		return nil
	})
	for i := len(directories) - 1; i >= 0; i-- {
		_ = os.Remove(directories[i]) // fails, harmlessly, when not empty
	}
}
