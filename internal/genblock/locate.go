package genblock

import "strings"

// span is where a block sits in a file, as line indexes.
type span struct {
	found      bool
	hash       string
	start, end int
}

// locate finds the first block. A start marker with no end after it is
// reported as an error rather than guessed at.
func locate(text string) (span, string) {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		hash, isStart := startHash(line)
		if !isStart {
			continue
		}
		for later := index + 1; later < len(lines); later++ {
			if strings.Contains(lines[later], endToken) {
				return span{found: true, hash: hash, start: index, end: later}, ""
			}
		}
		return span{}, "a generated-block start marker has no end marker"
	}
	return span{}, ""
}

// startHash reads the hash out of a start-marker line.
func startHash(line string) (string, bool) {
	_, after, found := strings.Cut(line, startToken)
	if !found {
		return "", false
	}
	fields := strings.Fields(after)
	if len(fields) == 0 {
		return "", false
	}
	return fields[0], true
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
