// Package docstyle mechanically checks a Markdown file against the
// deterministic parts of shared/rules/writing-style.md (Simplified
// Technical English): sentence length, plain vocabulary, modal conflicts,
// an estimated passive-voice share, and DOMAIN_DICTIONARY.md's "Synonyms to
// AVOID" column. It cannot judge whether a doc is clear — it is a report,
// not a gate, so its output names candidates for a person or agent to fix.
package docstyle

import "fmt"

// Rule names, one per mechanized check. A `<!-- docstyle-ignore: <rule> -->`
// directive names one of these to suppress the finding it precedes.
const (
	RuleSentenceLength     = "sentence-length"
	RulePlainVocabulary    = "plain-vocabulary"
	RuleModalConflict      = "modal-conflict"
	RulePassiveVoice       = "passive-voice"
	RuleUbiquitousLanguage = "ubiquitous-language"
)

// Finding is one place a Markdown file breaks a mechanized writing-style
// rule.
type Finding struct {
	File    string
	Line    int
	Rule    string
	Excerpt string
}

// String renders "file:line: rule: excerpt".
func (finding Finding) String() string {
	return fmt.Sprintf("%s:%d: %s: %s", finding.File, finding.Line, finding.Rule, finding.Excerpt)
}
