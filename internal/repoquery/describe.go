package repoquery

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/repograph"
)

// MaxSummaryBytes caps the rendered summary returned with a description:
// progressive disclosure, not the whole README.
const MaxSummaryBytes = 3000

// Description is what the index knows about one directory.
type Description struct {
	Path  string             `json:"path"`
	Entry contextindex.Entry `json:"entry"`
	// Summary is the rendered document, if one was generated and applied.
	Summary          string `json:"summary,omitempty"`
	SummaryTruncated bool   `json:"summaryTruncated,omitempty"`
	// Stale says the entry was documented against a directory that has since
	// changed. It is judged against the stored fact sheet, so it can lag the
	// working tree: the envelope's fresh flag says whether the graph does.
	Stale bool `json:"stale"`
	// ResolvedFrom is the file path asked about, when it was resolved to its
	// directory.
	ResolvedFrom string `json:"resolvedFrom,omitempty"`
}

// Describe returns the index entry for a directory, or for the directory a
// source file lives in, plus its rendered summary. Only a path that is a key of
// the index is ever turned into a file read.
func (s *Snapshot) Describe(raw string) (Description, error) {
	if !s.HasIndex {
		return Description{}, fmt.Errorf("%w (there is a graph but no context index yet)", ErrNoIndex)
	}
	dir, resolvedFrom, err := s.resolveDirectory(raw)
	if err != nil {
		return Description{}, err
	}
	entry := s.Index.Entries[dir]
	facts, err := s.Facts()
	if err != nil {
		return Description{}, err
	}
	summary, truncated := s.readSummary(dir)
	sheet, hasSheet := facts[dir]
	return Description{Path: dir, Entry: entry, Summary: summary, SummaryTruncated: truncated, ResolvedFrom: resolvedFrom,
		Stale: hasSheet && entry.InputHash != "" && entry.InputHash != sheet.InputHash}, nil
}

// resolveDirectory maps a path to an index key: itself, or — for a file — the
// directory that lists it.
func (s *Snapshot) resolveDirectory(raw string) (dir, resolvedFrom string, err error) {
	target := cleanPath(raw)
	if _, ok := s.Index.Entries[target]; ok {
		return target, "", nil
	}
	if parent := path.Dir(target); parent != target {
		if _, ok := s.Index.Entries[parent]; ok && s.directoryHoldsFile(parent, path.Base(target)) {
			return parent, raw, nil
		}
	}
	return "", "", &UnknownPathError{Path: raw, Suggestions: s.suggest(target)}
}

func (s *Snapshot) directoryHoldsFile(dir, name string) bool {
	facts, err := s.Facts()
	if err != nil {
		return false
	}
	for _, file := range facts[dir].Files {
		if file == name {
			return true
		}
	}
	return false
}

// readSummary returns the rendered document for an indexed directory, capped.
func (s Snapshot) readSummary(dir string) (string, bool) {
	if strings.Contains(dir, "..") {
		return "", false
	}
	target := filepath.Join(s.root, repograph.OutputDir, "docs", filepath.FromSlash(dir), "README.md")
	if dir == "." {
		target = filepath.Join(s.root, repograph.OutputDir, "docs", "README.md")
	}
	raw, err := os.ReadFile(target)
	if err != nil {
		return "", false
	}
	if len(raw) > MaxSummaryBytes {
		return strings.ToValidUTF8(string(raw[:MaxSummaryBytes]), ""), true
	}
	return string(raw), false
}
