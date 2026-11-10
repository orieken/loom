package docstyle

import (
	"regexp"
	"strings"
)

// Word-count thresholds from shared/rules/writing-style.md's sentence-length
// rule: a numbered-list step stays under 20 words, a descriptive sentence
// (including a bulleted list item) stays under 25.
const (
	listStepWordLimit    = 20
	descriptiveWordLimit = 25
)

var (
	orderedListRe   = regexp.MustCompile(`^\s*\d+[.)]\s+`)
	bulletedListRe  = regexp.MustCompile(`^\s*[-*+]\s+`)
	emphasisRe      = regexp.MustCompile(`[*_]{1,2}`)
	sentenceSplitRe = regexp.MustCompile(`[.!?]+(\s+|$)`)
)

// CheckSentenceLength reports a descriptive sentence over 25 words, or a
// numbered-list step over 20 words, as rule "sentence-length". A bulleted
// list item is judged against the descriptive threshold, not the list-step
// one — per the spec's own open-question assumption, only a numbered item
// is a "list step".
func CheckSentenceLength(file string, lines []Line) []Finding {
	var findings []Finding
	for _, line := range lines {
		limit, text := lineLimitAndText(line.Text)
		for _, sentence := range splitSentences(text) {
			if countWords(stripEmphasis(sentence)) > limit {
				findings = append(findings, Finding{File: file, Line: line.Number, Rule: RuleSentenceLength, Excerpt: excerpt(sentence)})
			}
		}
	}
	return findings
}

// lineLimitAndText strips a list marker, if present, and reports which
// word limit the remaining text is judged against.
func lineLimitAndText(text string) (int, string) {
	if marker := orderedListRe.FindString(text); marker != "" {
		return listStepWordLimit, text[len(marker):]
	}
	if marker := bulletedListRe.FindString(text); marker != "" {
		return descriptiveWordLimit, text[len(marker):]
	}
	return descriptiveWordLimit, text
}

// splitSentences splits text on sentence-ending punctuation. Text with no
// terminator at all — the common case for a markdown line wrapped mid-
// sentence — is returned as its own, single "sentence": prose lines are
// never joined across an ignored region, so a sentence split across two
// surviving lines is judged line by line rather than merged.
func splitSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var sentences []string
	for _, part := range sentenceSplitRe.Split(text, -1) {
		if part = strings.TrimSpace(part); part != "" {
			sentences = append(sentences, part)
		}
	}
	return sentences
}

func stripEmphasis(text string) string { return emphasisRe.ReplaceAllString(text, "") }

func countWords(text string) int { return len(strings.Fields(text)) }

// excerpt trims a sentence used as a Finding's excerpt to a readable length.
func excerpt(text string) string {
	const maxLen = 80
	if len(text) <= maxLen {
		return text
	}
	return strings.TrimSpace(text[:maxLen]) + "..."
}
