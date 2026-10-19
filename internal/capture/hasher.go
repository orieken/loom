package capture

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
)

// HashKind separates hash domains, so equal strings of different kinds
// never give equal hashes.
type HashKind string

// The hash kinds (architecture-notes.md decision 5).
const (
	HashPath    HashKind = "path"
	HashLines   HashKind = "lines"
	HashSession HashKind = "session"
)

// HashBytes is the number of digest bytes kept (16 bytes, 32 hex digits).
const HashBytes = 16

const kindSeparator = 0x00

// Hasher computes HMAC-SHA256(salt, kind || 0x00 || value), truncated to
// HashBytes and written as hex (RFC, status Accepted). It keeps one keyed
// HMAC instance and resets it for each call, so it is NOT safe for
// concurrent use. A hook process is single-threaded.
type Hasher struct {
	mac hash.Hash
}

// NewHasher keys a Hasher with the Capture salt.
func NewHasher(salt Salt) *Hasher {
	return &Hasher{mac: hmac.New(sha256.New, salt.key)}
}

// Hash returns the salted hash of one value.
func (h *Hasher) Hash(kind HashKind, value string) string {
	h.mac.Reset()
	_, _ = io.WriteString(h.mac, string(kind))
	_, _ = h.mac.Write([]byte{kindSeparator})
	_, _ = io.WriteString(h.mac, value)
	return hex.EncodeToString(h.mac.Sum(nil)[:HashBytes])
}

// HashLineRange hashes lines start..end (1-based, inclusive) of a file's
// content. Each line loses a trailing "\r", nothing else is trimmed, so a
// formatter change counts as a change. It reports false when the range does
// not lie inside the content.
func (h *Hasher) HashLineRange(content []byte, start, end int) (string, bool) {
	return h.HashSplitLines(SplitLines(content), start, end)
}

// HashSplitLines is HashLineRange for content that SplitLines already
// split. A caller that hashes several ranges of one file splits it once.
func (h *Hasher) HashSplitLines(lines [][]byte, start, end int) (string, bool) {
	if start < 1 || end < start || end > len(lines) {
		return "", false
	}
	joined := bytes.Join(lines[start-1:end], []byte{'\n'})
	return h.Hash(HashLines, string(joined)), true
}

// SplitLines splits content into lines. One final newline does not start an
// extra empty line, and each line loses a trailing "\r".
func SplitLines(content []byte) [][]byte {
	return splitLines(content)
}

func splitLines(content []byte) [][]byte {
	content = bytes.TrimSuffix(content, []byte{'\n'})
	lines := bytes.Split(content, []byte{'\n'})
	for i, line := range lines {
		lines[i] = bytes.TrimSuffix(line, []byte{'\r'})
	}
	return lines
}

// String never shows the keyed state.
func (*Hasher) String() string { return "Hasher" + redactedText }

// GoString never shows the keyed state.
func (*Hasher) GoString() string { return "Hasher" + redactedText }

// Format never shows the keyed state, whatever the verb.
func (*Hasher) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "Hasher"+redactedText)
}
