// Package manifest records the paths owned by a loom installation.
package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/orieken/loom/cmd/loom/internal/ownership"
)

const Filename = ".loom-manifest.json"

// OwnedPath is one path loom installed, with the digest of the content it
// wrote there. The digest is what lets a later install tell a file it owns
// from one the project wrote, and an unmodified one from an edited one
// (roadmap L3.26).
type OwnedPath struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

// PlatformRecord describes the paths owned by one installed platform.
//
// Paths lists the install roots for human-readable output and stays
// directory-granular. Owned is the authoritative per-file record: uninstall
// removes what is listed there and nothing else.
type PlatformRecord struct {
	Name  string      `json:"name"`
	Paths []string    `json:"paths"`
	Owned []OwnedPath `json:"owned,omitempty"`
}

// Manifest describes one completed loom installation.
type Manifest struct {
	Version     string           `json:"version"`
	InstalledAt time.Time        `json:"installedAt"`
	Platforms   []PlatformRecord `json:"platforms"`
}

// Read loads the loom manifest from a target directory.
func Read(target string) (Manifest, error) {
	content, err := os.ReadFile(filepath.Join(target, Filename))
	if err != nil {
		return Manifest{}, fmt.Errorf("read install manifest: %w", err)
	}
	var installed Manifest
	if err := json.Unmarshal(content, &installed); err != nil {
		return Manifest{}, fmt.Errorf("decode install manifest: %w", err)
	}
	return installed, nil
}

// ReadIfExists loads a manifest when one is present.
func ReadIfExists(target string) (Manifest, bool, error) {
	installed, err := Read(target)
	if err == nil {
		return installed, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return Manifest{}, false, nil
	}
	return Manifest{}, false, err
}

// Write persists a manifest atomically in the target directory.
func Write(target string, installed Manifest) error {
	content, err := json.MarshalIndent(installed, "", "  ")
	if err != nil {
		return fmt.Errorf("encode install manifest: %w", err)
	}
	destination := filepath.Join(target, Filename)
	if err := os.WriteFile(destination, append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("write install manifest: %w", err)
	}
	return nil
}

// Ledger collapses every platform's owned paths into the lookup install
// consults. A path installed for two platforms carries the same content, so
// a later record overwriting an earlier one loses nothing.
func (installed Manifest) Ledger() ownership.Ledger {
	ledger := make(ownership.Ledger)
	for _, record := range installed.Platforms {
		for _, owned := range record.Owned {
			ledger[owned.Path] = owned.Digest
		}
	}
	return ledger
}
