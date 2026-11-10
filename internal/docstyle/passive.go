package docstyle

import (
	"fmt"
	"regexp"
	"strings"
)

// passiveThreshold is the share of a file's prose sentences that must match
// the "to be" + past-participle heuristic before it is worth a finding — a
// file at or below it reports nothing.
const passiveThreshold = 0.10

var toBeRe = regexp.MustCompile(`(?i)^(is|are|was|were|be|being|been|am)$`)

// irregularParticiples covers common past participles that do not end in
// "-ed", so the suffix check in isPastParticiple does not miss them.
var irregularParticiples = map[string]bool{
	"done": true, "gone": true, "written": true, "seen": true, "given": true,
	"taken": true, "known": true, "shown": true, "made": true, "built": true,
	"sent": true, "held": true, "found": true, "kept": true, "left": true,
	"told": true, "brought": true, "bought": true,
}

// CheckPassiveVoice reports one file-level finding, rule "passive-voice",
// labelled an estimate, when more than 10% of a file's prose sentences
// match a "to be" form immediately followed by a likely past participle.
func CheckPassiveVoice(file string, lines []Line) []Finding {
	total, passive := countPassiveSentences(lines)
	if total == 0 {
		return nil
	}
	share := float64(passive) / float64(total)
	if share <= passiveThreshold {
		return nil
	}
	return []Finding{{File: file, Line: 1, Rule: RulePassiveVoice, Excerpt: fmt.Sprintf("estimate: ~%.0f%% passive", share*100)}}
}

func countPassiveSentences(lines []Line) (total, passive int) {
	for _, line := range lines {
		for _, sentence := range splitSentences(line.Text) {
			total++
			if isLikelyPassive(sentence) {
				passive++
			}
		}
	}
	return total, passive
}

func isLikelyPassive(sentence string) bool {
	words := strings.Fields(stripEmphasis(sentence))
	for i, word := range words {
		if !toBeRe.MatchString(word) {
			continue
		}
		if i+1 < len(words) && isPastParticiple(words[i+1]) {
			return true
		}
	}
	return false
}

func isPastParticiple(word string) bool {
	clean := strings.ToLower(strings.Trim(word, ".,;:!?\"'()"))
	return irregularParticiples[clean] || (strings.HasSuffix(clean, "ed") && len(clean) > 2)
}
