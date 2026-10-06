package genblock

import (
	"regexp"
	"strings"
)

// span is where a block sits in a file, as line indexes.
type span struct {
	found      bool
	hash       string
	start, end int
}

// A marker is a whole line: an optional comment opener, the token, and nothing
// after it but an optional comment closer. Anchoring matters: a README that
// documents the marker syntax mid-sentence or in a quoted line must not be
// mistaken for a block.
var (
	startLine = regexp.MustCompile(`^\s*(?:<!--|//|/\*|\*|#)?\s*` + regexp.QuoteMeta(startToken) + `(\S*?)\s*(?:-->|\*/)?\s*$`)
	endLine   = regexp.MustCompile(`^\s*(?:<!--|//|/\*|\*|#)?\s*` + regexp.QuoteMeta(endToken) + `\s*(?:-->|\*/)?\s*$`)
)

// locate finds the first block. A start marker with no end after it is
// reported as an error rather than guessed at. Lines inside a markdown code
// fence are never markers: they are examples of one.
func locate(text string) (span, string) {
	lines := strings.Split(text, "\n")
	fenced := fenceMask(lines)
	for index, line := range lines {
		if fenced[index] {
			continue
		}
		hash, isStart := startHash(line)
		if !isStart {
			continue
		}
		for later := index + 1; later < len(lines); later++ {
			if !fenced[later] && endLine.MatchString(lines[later]) {
				return span{found: true, hash: hash, start: index, end: later}, ""
			}
		}
		return span{}, "a generated-block start marker has no end marker"
	}
	return span{}, ""
}

// fenceMask marks the lines inside (and including) ``` code fences.
func fenceMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	inside := false
	for index, line := range lines {
		isFence := strings.HasPrefix(strings.TrimSpace(line), "```")
		if isFence || inside {
			mask[index] = true
		}
		if isFence {
			inside = !inside
		}
	}
	return mask
}

// startHash reads the hash out of a start-marker line.
func startHash(line string) (string, bool) {
	match := startLine.FindStringSubmatch(line)
	if match == nil {
		return "", false
	}
	// An anchored start line with no readable hash is a damaged marker, not
	// prose: it is returned with an empty hash so it surfaces as a conflict.
	return match[1], true
}

// literal returns the lines strictly between the markers.
func (s span) literal(text string) string {
	lines := strings.Split(text, "\n")
	return strings.Join(lines[s.start+1:s.end], "\n")
}

// replace swaps the whole block, marker lines included, leaving every other
// line exactly as it was.
func (s span) replace(text, block string) string {
	lines := strings.Split(text, "\n")
	replaced := append([]string{}, lines[:s.start]...)
	replaced = append(replaced, strings.Split(block, "\n")...)
	replaced = append(replaced, lines[s.end+1:]...)
	return strings.Join(replaced, "\n")
}
