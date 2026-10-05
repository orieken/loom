package repograph

import (
	"path"
	"strings"
)

// FileEdge is one resolved import before nodes exist: a source file depends on
// a directory.
type FileEdge struct {
	FromFile   string
	ToDir      string
	Confidence Confidence
}

// Extraction is everything one Extractor found.
type Extraction struct {
	Edges      []FileEdge
	Unresolved []Unresolved
}

// Extractor reads one language's imports. Adding a language is one
// implementation and a fixture repo (ADR-010).
type Extractor interface {
	Language() string
	// NodeKind is "package" or "module".
	NodeKind() string
	// Handles reports whether a path is a source file of this language.
	Handles(path string) bool
	Extract(repo Repo) (Extraction, error)
}

// DefaultExtractors returns the extractors `loom index` runs.
func DefaultExtractors() []Extractor {
	return []Extractor{goExtractor{}, typeScriptExtractor{}, pythonExtractor{}, javaExtractor{}}
}

// handledFiles returns the repo's files an extractor owns.
func handledFiles(repo Repo, extractor Extractor) []string {
	var files []string
	for _, file := range repo.Files() {
		if extractor.Handles(file) {
			files = append(files, file)
		}
	}
	return files
}

func fileSet(files []string) map[string]bool {
	set := make(map[string]bool, len(files))
	for _, file := range files {
		set[file] = true
	}
	return set
}

// dirOf is path.Dir with "." for the repo root, which is already what
// path.Dir returns; the helper exists to name the intent.
func dirOf(file string) string { return path.Dir(file) }

func hasExtension(file string, extensions ...string) bool {
	for _, extension := range extensions {
		if strings.HasSuffix(file, extension) {
			return true
		}
	}
	return false
}
