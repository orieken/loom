package intake

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// FileReader reads a project-root-relative path's bytes. internal/intake
// performs no filesystem I/O itself: cmd/loom/cmd builds the real one,
// closing over the project root it already resolves for plans and specs.
type FileReader func(path string) ([]byte, error)

// digestPrefix names the hash function a slice.digest commits to. SHA-256 is
// the only one this build verifies.
const digestPrefix = "sha256:"

// Criterion is one entry in a spec package's criteria[], the unit a slice's
// covers list refers to.
type Criterion struct {
	ID            string `json:"id"`
	ObservableVia string `json:"observableVia"`
}

// SpecPackage is the spec a group of slices covers together.
type SpecPackage struct {
	Criteria []Criterion `json:"criteria"`
}

// levelsOf returns the levels criterion must be covered at, and false when
// its observableVia is missing or unrecognized: a defect in the spec
// package, reported against it rather than guessed at.
func levelsOf(criterion Criterion) ([]string, bool) {
	if !knownObservableVia[criterion.ObservableVia] {
		return nil, false
	}
	if criterion.ObservableVia == "both" {
		return []string{"api", "ui"}, true
	}
	return []string{criterion.ObservableVia}, true
}

// coversEntry is one parsed element of a slice's covers list: a criterion id
// and the one level it names, or an empty level for a bare entry that covers
// every level the criterion has.
type coversEntry struct {
	criterionID string
	level       string
}

// parseCoversEntry splits "AC1:ui" into its criterion id and level, and
// "AC2" into its id with no level.
func parseCoversEntry(raw string) coversEntry {
	id, level, found := strings.Cut(raw, ":")
	if !found {
		return coversEntry{criterionID: id}
	}
	return coversEntry{criterionID: id, level: level}
}

// isWithinProjectRoot reports whether path is a project-root-relative path
// that stays inside the root. It is a lexical decision — no file is opened
// to make it — so an escaping path never reaches FileReader.
func isWithinProjectRoot(path string) bool {
	if path == "" || filepath.IsAbs(path) {
		return false
	}
	cleaned := filepath.ToSlash(filepath.Clean(path))
	return cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}

// readRooted reads path via read, refusing — without calling read — a path
// that would resolve outside the project root.
func readRooted(path string, read FileReader) ([]byte, error) {
	if !isWithinProjectRoot(path) {
		return nil, fmt.Errorf("%q leaves the project root", path)
	}
	content, err := read(path)
	if err != nil {
		return nil, fmt.Errorf("%q: %w", path, err)
	}
	return content, nil
}

// readSpecPackage reads and parses the spec package at path.
func readSpecPackage(path string, read FileReader) (SpecPackage, error) {
	raw, err := readRooted(path, read)
	if err != nil {
		return SpecPackage{}, err
	}
	var pkg SpecPackage
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return SpecPackage{}, fmt.Errorf("%q: %w", path, err)
	}
	return pkg, nil
}

// manifestFile is a manifest's parsed path -> lowercase hex SHA-256 entries,
// and the raw bytes they were parsed from — digest verification hashes the
// bytes, not the parsed form.
type manifestFile struct {
	entries map[string]string
	raw     []byte
}

// readManifest reads path's sha256sum-style listing: one "<hex digest>
// <path>" pair per line.
func readManifest(path string, read FileReader) (manifestFile, error) {
	raw, err := readRooted(path, read)
	if err != nil {
		return manifestFile{}, err
	}
	entries := map[string]string{}
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		hash, file, ok := splitManifestLine(line)
		if !ok {
			return manifestFile{}, fmt.Errorf("%q: %q is not a %q line", path, line, "<sha256>  <path>")
		}
		entries[file] = strings.ToLower(hash)
	}
	return manifestFile{entries: entries, raw: raw}, nil
}

func splitManifestLine(line string) (hash, file string, ok bool) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return "", "", false
	}
	return fields[0], fields[1], true
}

// sha256Hex is content's digest as lowercase hex, the form a manifest
// records a listed file's hash in.
func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// sha256Digest is content's digest in slice.digest's own "sha256:<hex>" form.
func sha256Digest(content []byte) string {
	return digestPrefix + sha256Hex(content)
}
