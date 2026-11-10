package docstyle

import "regexp"

var (
	// rfc2119Re is deliberately case-sensitive: writing-style.md reserves
	// MUST/SHOULD/MAY for requirements, written upper-case. Lower-case "may"
	// is an ordinary word, not the keyword this rule is about.
	rfc2119Re   = regexp.MustCompile(`\b(MUST|SHOULD|MAY)\b`)
	weakModalRe = regexp.MustCompile(`(?i)\b(can|might|ought to)\b`)
)

// CheckModalConflict reports "can", "might" or "ought to" in a sentence
// that also contains MUST, SHOULD or MAY, as rule "modal-conflict".
func CheckModalConflict(file string, lines []Line) []Finding {
	var findings []Finding
	for _, line := range lines {
		for _, sentence := range splitSentences(line.Text) {
			if rfc2119Re.MatchString(sentence) && weakModalRe.MatchString(sentence) {
				findings = append(findings, Finding{File: file, Line: line.Number, Rule: RuleModalConflict, Excerpt: excerpt(sentence)})
			}
		}
	}
	return findings
}
