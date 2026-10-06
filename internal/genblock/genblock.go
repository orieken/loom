// Package genblock reads and writes generated blocks: text a tool owns inside
// a file a human also owns (ADR-010). A block is fenced by a start marker that
// records a hash of its content and an end marker, in the host language's
// comment syntax. Regeneration replaces only what is between the markers and
// never touches the rest; a block whose content no longer matches its recorded
// hash was edited by hand and is reported, not overwritten.
package genblock

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const (
	startToken = "loom:generated:start hash="
	endToken   = "loom:generated:end"
)

// Syntax wraps a block's lines for one language's comment form.
type Syntax struct {
	// Start returns the full start-marker line for a content hash.
	Start func(hash string) string
	// End is the full end-marker line.
	End string
	// LinePrefix is put before every body line, e.g. "// " or " * ".
	LinePrefix string
}

// Markdown is the syntax for markdown files.
var Markdown = Syntax{
	Start: func(hash string) string { return "<!-- " + startToken + hash + " -->" },
	End:   "<!-- " + endToken + " -->",
}

// LineComment builds the syntax for languages with a line-comment prefix such
// as "//" (Go).
func LineComment(prefix string) Syntax {
	return Syntax{
		Start:      func(hash string) string { return prefix + " " + startToken + hash },
		End:        prefix + " " + endToken,
		LinePrefix: prefix + " ",
	}
}

// BlockComment builds the syntax for a body held inside a block comment whose
// lines start with a star, as JSDoc and Javadoc do.
func BlockComment() Syntax {
	return Syntax{
		Start:      func(hash string) string { return " * " + startToken + hash },
		End:        " * " + endToken,
		LinePrefix: " * ",
	}
}

// Docstring builds the syntax for a Python docstring: the markers are plain
// lines inside the string.
func Docstring() Syntax {
	return Syntax{
		Start: func(hash string) string { return startToken + hash },
		End:   endToken,
	}
}

// Hash is the digest recorded in a start marker: of the literal lines between
// the markers, joined by newlines, prefixes included.
func Hash(literal string) string {
	sum := sha256.Sum256([]byte(literal))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// literal is the exact text of a body as it sits between the markers.
func (s Syntax) literal(body string) string {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	for index, line := range lines {
		lines[index] = strings.TrimRight(s.LinePrefix+line, " ")
	}
	return strings.Join(lines, "\n")
}

// Render returns a complete block for a body, without a trailing newline.
func (s Syntax) Render(body string) string {
	literal := s.literal(body)
	return s.Start(Hash(literal)) + "\n" + literal + "\n" + s.End
}

// Status says what Upsert did.
type Status string

// What Upsert can report.
const (
	// Created: there was no file, so the result is a new one holding the block.
	Created Status = "created"
	// Appended: the file had no block, so one was added after the human's text.
	Appended Status = "appended"
	// Replaced: the block's content changed and was replaced.
	Replaced Status = "replaced"
	// Unchanged: the block already holds this content.
	Unchanged Status = "unchanged"
	// Conflict: the block was edited by hand or its markers are damaged, so
	// nothing was changed.
	Conflict Status = "conflict"
)

// Upsert puts a generated body into a file's text. exists says whether the
// file is there at all. It returns the new text and what happened; on
// Conflict the text returned is the input, untouched, and the reason says why.
func (s Syntax) Upsert(existing string, exists bool, body string) (string, Status, string) {
	if !exists {
		return s.Render(body) + "\n", Created, ""
	}
	// A file with CRLF line endings (Windows, or core.autocrlf) is read as LF so
	// its hash matches the one recorded when the block was written, and written
	// back with CRLF so the file keeps its line endings.
	crlf := strings.Contains(existing, "\r\n")
	text, status, reason := s.upsertNormalized(normalizeLineEndings(existing), body)
	if crlf && status != Conflict && status != Unchanged {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	if status == Conflict || status == Unchanged {
		return existing, status, reason
	}
	return text, status, reason
}

func normalizeLineEndings(text string) string { return strings.ReplaceAll(text, "\r\n", "\n") }

func (s Syntax) upsertNormalized(existing, body string) (string, Status, string) {
	located, err := locate(existing)
	if err != "" {
		return existing, Conflict, err
	}
	if !located.found {
		return appendBlock(existing, s.Render(body)), Appended, ""
	}
	current := located.literal(existing)
	if Hash(current) != located.hash {
		return existing, Conflict, "the generated block was edited by hand (its content no longer matches its recorded hash)"
	}
	if current == s.literal(body) {
		return existing, Unchanged, ""
	}
	return located.replace(existing, s.Render(body)), Replaced, ""
}

// Extract returns the literal text between the markers and the hash recorded
// for it, for callers that need to compare a block without changing it.
func Extract(existing string) (literal, recorded string, found bool) {
	existing = normalizeLineEndings(existing)
	located, err := locate(existing)
	if err != "" || !located.found {
		return "", "", false
	}
	return located.literal(existing), located.hash, true
}

func appendBlock(existing, block string) string {
	if strings.TrimSpace(existing) == "" {
		return block + "\n"
	}
	return strings.TrimRight(existing, "\n") + "\n\n" + block + "\n"
}
