package capture

import "bytes"

const newlineByte = '\n'

// HunkLocator finds the lines an edit changed, in the file as it is after
// the write, and hashes them. It never guesses: an edit that matches more
// than once, or in no way, gives no hunk (architecture-notes.md decision 3).
type HunkLocator struct {
	hasher *Hasher
}

// NewHunkLocator builds a locator that hashes with the given Hasher.
func NewHunkLocator(hasher *Hasher) HunkLocator {
	return HunkLocator{hasher: hasher}
}

// LocateAll returns the hunk facts of every edit that can be placed. The
// content and its split lines describe the file after the write.
func (l HunkLocator) LocateAll(content []byte, lines [][]byte, text EditText) []HunkFacts {
	var found []HunkFacts
	for _, pair := range text.edits {
		if facts, isPlaced := l.locate(content, lines, pair); isPlaced {
			found = append(found, facts)
		}
	}
	return found
}

// WholeFile gives the facts of a file that an agent wrote in full: all
// lines, with no pre-image.
func (l HunkLocator) WholeFile(lines [][]byte) (HunkFacts, bool) {
	post, isInside := l.hasher.HashSplitLines(lines, 1, len(lines))
	return HunkFacts{RangeStart: 1, RangeEnd: len(lines), PostHash: post}, isInside
}

func (l HunkLocator) locate(content []byte, lines [][]byte, pair editPair) (HunkFacts, bool) {
	if pair.newText == "" {
		return HunkFacts{}, false
	}
	offset, count := occurrences(content, []byte(pair.newText))
	switch count {
	case 1:
		return l.placeAtOffset(content, lines, offset, pair)
	case 0:
		return l.placeByLines(lines, pair)
	}
	return HunkFacts{}, false
}

// placeAtOffset widens the match to whole lines. The pre-image is the same
// span of lines with the new text swapped for the old text.
func (l HunkLocator) placeAtOffset(content []byte, lines [][]byte, offset int, pair editPair) (HunkFacts, bool) {
	matchEnd := offset + len(pair.newText)
	lineStart := bytes.LastIndexByte(content[:offset], newlineByte) + 1
	lineEnd := endOfLine(content, matchEnd-1)
	start := bytes.Count(content[:offset], []byte{newlineByte}) + 1
	end := start + len(SplitLines(content[lineStart:lineEnd])) - 1
	pre := joinBytes(content[lineStart:offset], []byte(pair.oldText), content[matchEnd:lineEnd])
	return l.factsFor(lines, start, end, pre)
}

// placeByLines is the fallback when the exact text is not in the file, for
// example after a line-ending change. It compares line by line and accepts
// one match only.
func (l HunkLocator) placeByLines(lines [][]byte, pair editPair) (HunkFacts, bool) {
	wanted := SplitLines([]byte(pair.newText))
	first, count := matchingRuns(lines, wanted)
	if count != 1 {
		return HunkFacts{}, false
	}
	return l.factsFor(lines, first+1, first+len(wanted), []byte(pair.oldText))
}

func (l HunkLocator) factsFor(lines [][]byte, start, end int, preImage []byte) (HunkFacts, bool) {
	post, isInside := l.hasher.HashSplitLines(lines, start, end)
	if !isInside {
		return HunkFacts{}, false
	}
	preLines := SplitLines(preImage)
	pre, _ := l.hasher.HashSplitLines(preLines, 1, len(preLines))
	return HunkFacts{RangeStart: start, RangeEnd: end, PreHash: pre, PostHash: post}, true
}

// occurrences finds the first match and tells whether a second one exists.
// The count is 0, 1, or 2 for "two or more".
func occurrences(content, needle []byte) (offset, count int) {
	offset = bytes.Index(content, needle)
	if offset < 0 {
		return 0, 0
	}
	if bytes.Contains(content[offset+1:], needle) {
		return offset, 2
	}
	return offset, 1
}

// endOfLine is the offset after the newline that ends the line holding the
// byte at last, or the content length when that line has no newline.
func endOfLine(content []byte, last int) int {
	next := bytes.IndexByte(content[last:], newlineByte)
	if next < 0 {
		return len(content)
	}
	return last + next + 1
}

func matchingRuns(lines, wanted [][]byte) (first, count int) {
	for i := 0; i+len(wanted) <= len(lines) && count < 2; i++ {
		if !runEquals(lines[i:i+len(wanted)], wanted) {
			continue
		}
		if count == 0 {
			first = i
		}
		count++
	}
	return first, count
}

func runEquals(run, wanted [][]byte) bool {
	for i := range wanted {
		if !bytes.Equal(run[i], wanted[i]) {
			return false
		}
	}
	return true
}

func joinBytes(parts ...[]byte) []byte {
	return bytes.Join(parts, nil)
}
