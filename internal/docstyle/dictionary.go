package docstyle

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// skipListedSynonyms are ordinary-English words that happen to be listed in
// some project's DOMAIN_DICTIONARY.md as a "Synonym to AVOID" but are
// common enough that enforcing them would be noise, not signal (AC5's
// explicit, tested skip-list).
var skipListedSynonyms = map[string]bool{
	"data": true, "info": true, "handler": true, "manager": true,
	"process": true, "result": true,
}

var (
	dictionaryTableRowRe = regexp.MustCompile(`^\|\s*\*\*([^*]+)\*\*\s*\|(.+)$`)
	backtickTermRe       = regexp.MustCompile("`([^`]+)`")
)

// synonymEntry is one DOMAIN_DICTIONARY.md "Synonym to AVOID" ready to match
// against prose in its exact listed surface form.
type synonymEntry struct {
	synonym, canonical string
	pattern            *regexp.Regexp
}

func newSynonymEntry(synonym, canonical string) synonymEntry {
	return synonymEntry{synonym: synonym, canonical: canonical, pattern: regexp.MustCompile(`\b` + regexp.QuoteMeta(synonym) + `\b`)}
}

// loadDictionary parses a DOMAIN_DICTIONARY.md's "Synonyms to AVOID" table
// column into synonym entries, dropping any entry on the ordinary-English
// skip-list. A missing file is not an error: AC5 only applies "given a
// project with a DOMAIN_DICTIONARY.md" — a project without one simply gets
// no entries, and every other rule still runs.
//
// This reimplements, rather than imports, the table-row/backtick/comma
// parsing shape shared/mcp/internal/analyzers/ubiquitous_language_analyzer.go
// already proved against this exact table format: that package sits under a
// different internal/ visibility root, so Go's compiler refuses the import.
func loadDictionary(path string) ([]synonymEntry, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []synonymEntry
	for _, line := range strings.Split(string(data), "\n") {
		canonical, synonyms := parseDictionaryRow(line)
		for _, synonym := range synonyms {
			if skipListedSynonyms[strings.ToLower(synonym)] {
				continue
			}
			entries = append(entries, newSynonymEntry(synonym, canonical))
		}
	}
	return entries, nil
}

// parseDictionaryRow extracts the canonical term and its listed synonyms
// from one `| **Term** | ... | Synonyms to AVOID |` table row. The
// "Synonyms to AVOID" column is always the row's last one, whatever the
// number of columns between it and the term — matching DOMAIN_DICTIONARY.md
// tables that carry a Domain column and ones that do not.
func parseDictionaryRow(line string) (string, []string) {
	match := dictionaryTableRowRe.FindStringSubmatch(line)
	if match == nil {
		return "", nil
	}
	canonical := strings.TrimSpace(match[1])
	columns := strings.Split(match[2], "|")
	for len(columns) > 0 && strings.TrimSpace(columns[len(columns)-1]) == "" {
		columns = columns[:len(columns)-1]
	}
	if len(columns) < 2 {
		return "", nil
	}
	return canonical, splitSynonyms(columns[len(columns)-1])
}

// splitSynonyms prefers backtick-quoted terms when the cell has any, else
// splits on commas, and always strips a trailing parenthetical note.
func splitSynonyms(cell string) []string {
	if matches := backtickTermRe.FindAllStringSubmatch(cell, -1); len(matches) > 0 {
		terms := make([]string, 0, len(matches))
		for _, match := range matches {
			terms = append(terms, cleanSynonym(match[1]))
		}
		return terms
	}
	var terms []string
	for _, part := range strings.Split(cell, ",") {
		if term := cleanSynonym(part); term != "" {
			terms = append(terms, term)
		}
	}
	return terms
}

func cleanSynonym(term string) string {
	term = strings.TrimSpace(term)
	if idx := strings.Index(term, "("); idx > 0 {
		term = strings.TrimSpace(term[:idx])
	}
	return strings.Trim(term, "`*_ ")
}

// CheckUbiquitousLanguage reports a synonym-to-AVOID term used in prose in
// its exact listed surface form, naming the canonical term, as rule
// "ubiquitous-language".
func CheckUbiquitousLanguage(file string, lines []Line, entries []synonymEntry) []Finding {
	var findings []Finding
	for _, line := range lines {
		for _, entry := range entries {
			if entry.pattern.MatchString(line.Text) {
				findings = append(findings, Finding{
					File: file, Line: line.Number, Rule: RuleUbiquitousLanguage,
					Excerpt: fmt.Sprintf("%q → %q", entry.synonym, entry.canonical),
				})
			}
		}
	}
	return findings
}
