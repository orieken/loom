package docstyle

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// dictionaryFileName is the fixed, current-working-directory-relative path
// Check reads for AC5's "Synonyms to AVOID" table — the same convention
// every other loom DOMAIN_DICTIONARY.md consumer uses for a project's root.
const dictionaryFileName = "DOMAIN_DICTIONARY.md"

// Report is every finding Check produced, plus how many a
// `<!-- docstyle-ignore: <rule> -->` directive suppressed.
type Report struct {
	Findings []Finding
	Skipped  int
}

// Check walks every path. A file is checked directly; a directory is
// walked for every *.md file under it, skipping dot-directories. Every
// rule runs against each file's prose, and findings are sorted by file
// then line so output is deterministic across runs and directory-walk
// orderings.
func Check(paths []string) (Report, error) {
	files, err := markdownFiles(paths)
	if err != nil {
		return Report{}, err
	}
	entries, err := loadDictionary(dictionaryFileName)
	if err != nil {
		return Report{}, err
	}
	var findings []Finding
	skipped := 0
	for _, file := range files {
		fileFindings, fileSkipped, err := checkFile(file, entries)
		if err != nil {
			return Report{}, err
		}
		findings = append(findings, fileFindings...)
		skipped += fileSkipped
	}
	sortFindings(findings)
	return Report{Findings: findings, Skipped: skipped}, nil
}

func checkFile(file string, entries []synonymEntry) ([]Finding, int, error) {
	source, err := os.ReadFile(file)
	if err != nil {
		return nil, 0, err
	}
	body, firstLine := StripFrontmatter(string(source))
	lines := Prose(body, firstLine)
	var findings []Finding
	findings = append(findings, CheckSentenceLength(file, lines)...)
	findings = append(findings, CheckPlainVocabulary(file, lines)...)
	findings = append(findings, CheckModalConflict(file, lines)...)
	findings = append(findings, CheckPassiveVoice(file, lines)...)
	findings = append(findings, CheckUbiquitousLanguage(file, lines, entries)...)
	kept, skipped := Suppress(findings, Directives(body, firstLine))
	return kept, skipped, nil
}

// markdownFiles expands each path into the *.md files it names: itself, if
// a file, or every *.md file under it, if a directory.
func markdownFiles(paths []string) ([]string, error) {
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			files = append(files, path)
			continue
		}
		walked, err := walkMarkdown(path)
		if err != nil {
			return nil, err
		}
		files = append(files, walked...)
	}
	return files, nil
}

func walkMarkdown(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func sortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})
}
