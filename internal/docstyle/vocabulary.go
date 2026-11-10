package docstyle

import (
	"fmt"
	"regexp"
)

// vocabularyEntry is one word or phrase shared/rules/writing-style.md's
// plain-vocabulary rule replaces, and the plain word to use instead.
type vocabularyEntry struct {
	phrase, replacement string
	pattern             *regexp.Regexp
}

var plainVocabulary = []vocabularyEntry{
	newVocabularyEntry("utilize", "use"),
	newVocabularyEntry("prior to", "before"),
	newVocabularyEntry("with regard to", "about"),
}

func newVocabularyEntry(phrase, replacement string) vocabularyEntry {
	return vocabularyEntry{
		phrase:      phrase,
		replacement: replacement,
		pattern:     regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(phrase) + `\b`),
	}
}

// CheckPlainVocabulary reports a case-insensitive, word-boundary match of a
// word or phrase the rule replaces, naming the plain replacement, as rule
// "plain-vocabulary".
func CheckPlainVocabulary(file string, lines []Line) []Finding {
	var findings []Finding
	for _, line := range lines {
		for _, entry := range plainVocabulary {
			if entry.pattern.MatchString(line.Text) {
				findings = append(findings, Finding{
					File: file, Line: line.Number, Rule: RulePlainVocabulary,
					Excerpt: fmt.Sprintf("%q → %q", entry.phrase, entry.replacement),
				})
			}
		}
	}
	return findings
}
