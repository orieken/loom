package contextindex

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/orieken/loom/internal/factsheet"
	"github.com/orieken/loom/internal/repograph"
)

// Path is where the index lives for a repo root.
func Path(root string) string { return filepath.Join(root, repograph.OutputDir, File) }

// Load reads the index. found is false when none exists yet, which is not an
// error: indexing is opt-in.
func Load(root string) (index Index, found bool, err error) {
	raw, err := os.ReadFile(Path(root))
	if errors.Is(err, os.ErrNotExist) {
		return Index{}, false, nil
	}
	if err != nil {
		return Index{}, false, fmt.Errorf("read %s: %w", Path(root), err)
	}
	if err := json.Unmarshal(raw, &index); err != nil {
		return Index{}, false, fmt.Errorf("parse %s: %w", Path(root), err)
	}
	if index.SchemaVersion != SchemaVersion {
		return Index{}, false, fmt.Errorf("%s has schema_version %d; this build reads %d", Path(root), index.SchemaVersion, SchemaVersion)
	}
	return index, true, nil
}

// Marshal renders an index exactly as Write stores it.
func Marshal(index Index) ([]byte, error) {
	raw, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode index: %w", err)
	}
	return append(raw, '\n'), nil
}

// Write stores the index when its bytes changed and reports whether it did.
func Write(root string, index Index) (bool, error) {
	raw, err := Marshal(index)
	if err != nil {
		return false, err
	}
	if existing, readErr := os.ReadFile(Path(root)); readErr == nil && bytes.Equal(existing, raw) {
		return false, nil
	}
	return true, repograph.WriteFileAtomically(Path(root), raw)
}

// Refresh loads the previous index, brings it up to date with the sheets and
// writes it if anything changed.
func Refresh(root string, sheets []factsheet.FactSheet) (Index, bool, error) {
	previous, _, err := Load(root)
	if err != nil {
		return Index{}, false, err
	}
	index := Build(previous, sheets)
	written, err := Write(root, index)
	return index, written, err
}

// CurrentSheets computes the fact sheets of the working tree as it is now,
// without writing anything: the other side of a freshness comparison.
func CurrentSheets(root string, lister repograph.Lister) ([]factsheet.FactSheet, error) {
	repo, err := repograph.OpenRepo(root, lister)
	if err != nil {
		return nil, err
	}
	graph, err := repograph.Build(repo, repograph.DefaultExtractors(), repograph.Meta{GeneratedAt: time.Time{}.Format(time.RFC3339)})
	if err != nil {
		return nil, err
	}
	return factsheet.Build(repo, graph), nil
}
