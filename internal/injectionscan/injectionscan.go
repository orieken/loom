// Package injectionscan flags text that addresses a model instead of
// informing one (roadmap L3.7). Knowledge Items synced from an org repo
// (ADR-003) are read into agent context, and their bodies were validated for
// frontmatter only; memory-trust-boundary.md's defence was a prompt asking
// the model to treat other prompt text as data — the defence in the same
// channel as the attack. This is the deterministic control in front of it: a
// body that matches is flagged before any model sees it.
//
// It catches naive injection, not every injection. A pattern list cannot
// read intent; what it guarantees is that the phrasing every published
// injection starts from does not pass silently. Prompt-level caution stays as
// defence in depth behind it.
package injectionscan

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Finding is one line that matched a rule.
type Finding struct {
	Line int
	Rule string
	// Excerpt is the matched text, bounded — enough for a person to judge.
	Excerpt string
}

// rule is one pattern and the name a finding carries.
type rule struct {
	name    string
	pattern *regexp.Regexp
}

// rules are matched case-insensitively against each line. Each targets text
// directed at an agent: a Knowledge Item describes a pattern or a decision,
// and has no reason to tell its reader to drop its instructions.
var rules = []rule{
	{"override-instructions", regexp.MustCompile(`(?i)\b(ignore|disregard|forget|override)\b[^.\n]{0,40}\b(previous|prior|above|earlier|preceding|all|your|any|the|system)\b[^.\n]{0,20}\b(instructions?|rules|prompts?|directives|guidelines|guardrails)\b`)},
	{"new-instructions", regexp.MustCompile(`(?i)\b(new|updated|real|actual)\s+(instructions|task|directive)s?\s*:`)},
	{"role-reassignment", regexp.MustCompile(`(?i)\b(you are now|from now on,? you|pretend (that )?you are|act as if you have no)\b`)},
	{"bypass-control", regexp.MustCompile(`(?i)\b(bypass|skip|disable|circumvent|turn off)\b[^.\n]{0,30}\b(approval gates?|approval|guardrails?|safety checks?|security checks?|the gate|code review|csrf protection)\b`)},
	{"prompt-disclosure", regexp.MustCompile(`(?i)\b(reveal|print|show|repeat|output)\b[^.\n]{0,20}\b(your|the)\s+(system prompt|hidden instructions|initial instructions)\b`)},
	{"conceal-from-user", regexp.MustCompile(`(?i)\b(do not|don't|never)\s+(tell|inform|mention (this )?to|alert)\s+the\s+(user|human|operator)\b`)},
	{"role-marker", regexp.MustCompile(`(?i)(^\s*(system|assistant)\s*:|<\|im_(start|end)\|>|\[/?INST\]|</?system>)`)},
	// Invisible characters carry text a reviewer cannot see: zero-width
	// characters, and bidirectional overrides that reorder what is shown.
	{"invisible-characters", regexp.MustCompile(invisibleClass())},
}

// excerptLimit bounds a finding's excerpt, in runes.
const excerptLimit = 80

// Scan returns every rule each line of text matches, in line order.
func Scan(text string) []Finding {
	var findings []Finding
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for line := 1; scanner.Scan(); line++ {
		findings = append(findings, scanLine(line, scanner.Text())...)
	}
	return findings
}

func scanLine(line int, text string) []Finding {
	var findings []Finding
	for _, candidate := range rules {
		if match := candidate.pattern.FindString(text); match != "" {
			findings = append(findings, Finding{Line: line, Rule: candidate.name, Excerpt: excerpt(match)})
		}
	}
	return findings
}

// excerpt bounds a match and makes invisible characters visible, so the
// report shows what a reader of the file could not.
func excerpt(match string) string {
	var builder strings.Builder
	for count, r := range []rune(match) {
		if count == excerptLimit {
			builder.WriteString("...")
			break
		}
		builder.WriteString(visible(r))
	}
	return builder.String()
}

func visible(r rune) string {
	if isInvisible(r) {
		return fmt.Sprintf("<U+%04X>", r)
	}
	return string(r)
}

// invisible is every character the invisible-characters rule matches: one
// table, so the rule and the report cannot disagree about what is invisible.
var invisible = &unicode.RangeTable{R16: []unicode.Range16{
	{Lo: 0x200B, Hi: 0x200F, Stride: 1}, // zero-width space, joiners, direction marks
	{Lo: 0x202A, Hi: 0x202E, Stride: 1}, // bidirectional embeddings and overrides
	{Lo: 0x2060, Hi: 0x2060, Stride: 1}, // word joiner
	{Lo: 0x2066, Hi: 0x2069, Stride: 1}, // bidirectional isolates
	{Lo: 0xFEFF, Hi: 0xFEFF, Stride: 1}, // zero-width no-break space
}}

func isInvisible(r rune) bool {
	return unicode.Is(invisible, r)
}

// invisibleClass is the table as a regexp character class.
func invisibleClass() string {
	var class strings.Builder
	class.WriteString("[")
	for _, span := range invisible.R16 {
		fmt.Fprintf(&class, `\x{%04X}-\x{%04X}`, span.Lo, span.Hi)
	}
	class.WriteString("]")
	return class.String()
}

// Rules names every rule, for documentation and reports.
func Rules() []string {
	names := make([]string, len(rules))
	for index, candidate := range rules {
		names[index] = candidate.name
	}
	return names
}
