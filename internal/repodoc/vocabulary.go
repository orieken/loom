// Package repodoc drives the repo-documenter (ADR-010, Phase 4). The executor
// — this package, not the agent — decides which directories to document, in
// what order, and what each invocation is given. The agent sees one fact
// sheet and its children's summaries and returns typed JSON; nothing it
// returns is trusted until validated here.
package repodoc

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"go.yaml.in/yaml/v4"
)

// VocabularyFile is the framework's tag list, relative to the framework FS.
const VocabularyFile = "shared/vocabulary/context-tags.yaml"

// ProjectVocabularyFile is where a project extends it, relative to the repo.
const ProjectVocabularyFile = ".loom/tags.yaml"

// Tag is one allowed tag and what it means.
type Tag struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
}

type vocabularyFile struct {
	Version int   `yaml:"version"`
	Tags    []Tag `yaml:"tags"`
}

// Vocabulary is the closed set of tags the documenter may assign.
type Vocabulary struct {
	tags []Tag
}

// LoadVocabulary reads the framework's tags and merges the project's over
// them. A project may add tags and re-describe one, never remove one.
func LoadVocabulary(framework fs.FS, projectRoot string) (Vocabulary, error) {
	base, err := fs.ReadFile(framework, VocabularyFile)
	if err != nil {
		return Vocabulary{}, fmt.Errorf("read %s: %w", VocabularyFile, err)
	}
	merged, err := parseVocabulary(base, VocabularyFile)
	if err != nil {
		return Vocabulary{}, err
	}
	extra, err := os.ReadFile(filepath.Join(projectRoot, filepath.FromSlash(ProjectVocabularyFile)))
	if os.IsNotExist(err) {
		return newVocabulary(merged), nil
	}
	if err != nil {
		return Vocabulary{}, fmt.Errorf("read %s: %w", ProjectVocabularyFile, err)
	}
	project, err := parseVocabulary(extra, ProjectVocabularyFile)
	if err != nil {
		return Vocabulary{}, err
	}
	return newVocabulary(append(merged, project...)), nil
}

func parseVocabulary(raw []byte, name string) ([]Tag, error) {
	var parsed vocabularyFile
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse %s: %w", name, err)
	}
	for _, tag := range parsed.Tags {
		if tag.Name == "" {
			return nil, fmt.Errorf("%s has a tag with no name", name)
		}
	}
	return parsed.Tags, nil
}

// newVocabulary keeps one entry per name; a later description wins.
func newVocabulary(tags []Tag) Vocabulary {
	byName := map[string]Tag{}
	for _, tag := range tags {
		byName[tag.Name] = tag
	}
	merged := make([]Tag, 0, len(byName))
	for _, tag := range byName {
		merged = append(merged, tag)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Name < merged[j].Name })
	return Vocabulary{tags: merged}
}

// Tags returns every allowed tag, sorted by name.
func (v Vocabulary) Tags() []Tag { return v.tags }

// Contains reports whether a tag is allowed.
func (v Vocabulary) Contains(name string) bool {
	for _, tag := range v.tags {
		if tag.Name == name {
			return true
		}
	}
	return false
}

// NewVocabularyForTest builds a vocabulary of the named tags, for tests in
// other packages.
func NewVocabularyForTest(names ...string) Vocabulary {
	tags := make([]Tag, 0, len(names))
	for _, name := range names {
		tags = append(tags, Tag{Name: name, Description: name})
	}
	return newVocabulary(tags)
}
