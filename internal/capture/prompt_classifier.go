package capture

import (
	"strings"
	"unicode"
)

// correctiveOpenings are the prompt openings that mark a corrective prompt.
// The list is lexical on purpose. It misses many corrections and flags some
// non-corrections ("no, problem, continue"). The precision review decides
// whether the signal ships on (spec; analysis criterion 23).
var correctiveOpenings = []string{"no,", "don't"}

const curlyApostrophe = "’"

// PromptClassifier maps prompt text to a class label. It keeps no text.
type PromptClassifier struct{}

// Classify returns the class of a prompt: it opens with "no," or "don't"
// (after leading space, in any case, with a straight or curly apostrophe),
// or it is other. The caller discards the text after this call.
func (PromptClassifier) Classify(prompt string) PromptClass {
	opening := strings.TrimLeftFunc(prompt, unicode.IsSpace)
	opening = strings.ToLower(strings.ReplaceAll(opening, curlyApostrophe, "'"))
	for _, corrective := range correctiveOpenings {
		if strings.HasPrefix(opening, corrective) {
			return PromptCorrective
		}
	}
	return PromptOther
}
