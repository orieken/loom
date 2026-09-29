package tools

import (
	"strings"
	"unicode/utf8"
)

// A document is embedded as sections, not whole: one vector for a long
// document averages its topics into a point near none of them. Sections split
// at "## " headings and, past maxChunkRunes, at the last whitespace before the
// limit — about 500 tokens, well inside embedding models' context windows.
const maxChunkRunes = 2000

// docChunk is one embeddable section of a document.
type docChunk struct {
	heading string
	text    string
}

// chunkDocument splits body into sections, dropping frontmatter and empty ones.
func chunkDocument(body string) []docChunk {
	var chunks []docChunk
	for _, section := range splitSections(stripFrontmatter(body)) {
		for _, piece := range splitLong(section.text) {
			if strings.TrimSpace(piece) != "" {
				chunks = append(chunks, docChunk{heading: section.heading, text: piece})
			}
		}
	}
	return chunks
}

// stripFrontmatter drops a leading "---" block: metadata, not prose. The
// closing marker is searched for from the opening line's own newline, so an
// empty block ("---\n---") is found too — searching from after it missed one,
// and the block was embedded as text.
func stripFrontmatter(body string) string {
	if !strings.HasPrefix(body, "---\n") {
		return body
	}
	end := strings.Index(body[3:], "\n---")
	if end < 0 {
		return body
	}
	return strings.TrimPrefix(body[3+end+4:], "\n")
}

// splitSections cuts at each "## " heading; text before the first belongs to
// a section headed by nothing.
func splitSections(body string) []docChunk {
	sections := []docChunk{{}}
	for _, line := range strings.SplitAfter(body, "\n") {
		if strings.HasPrefix(line, "## ") {
			sections = append(sections, docChunk{heading: strings.TrimSpace(strings.TrimPrefix(line, "## "))})
		}
		sections[len(sections)-1].text += line
	}
	return sections
}

// splitLong cuts text into pieces of at most maxChunkRunes, at whitespace
// where there is any.
func splitLong(text string) []string {
	var pieces []string
	for utf8.RuneCountInString(text) > maxChunkRunes {
		cut := cutPoint(text)
		pieces = append(pieces, text[:cut])
		text = text[cut:]
	}
	return append(pieces, text)
}

// cutPoint is the byte offset of the last whitespace within the limit, or the
// limit itself when a run of text has none.
func cutPoint(text string) int {
	limit := byteOffsetOfRune(text, maxChunkRunes)
	if space := strings.LastIndexAny(text[:limit], " \n\t"); space > 0 {
		return space + 1
	}
	return limit
}

func byteOffsetOfRune(text string, runes int) int {
	offset := 0
	for count := 0; count < runes && offset < len(text); count++ {
		_, size := utf8.DecodeRuneInString(text[offset:])
		offset += size
	}
	return offset
}
