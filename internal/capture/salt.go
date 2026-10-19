package capture

import (
	"errors"
	"fmt"
	"io"
	"slices"
)

// SaltBytes is the length of the Capture salt.
const SaltBytes = 32

const redactedText = "[redacted]"

// ErrBadSalt marks a salt that is not SaltBytes long.
var ErrBadSalt = errors.New("capture: salt must be 32 bytes")

// Salt is the Capture salt. Every way to print it, with any fmt verb, a
// %#v, JSON or text marshalling, gives "[redacted]". The key bytes leave the
// type only through the Hasher in this package.
type Salt struct {
	key []byte
}

// NewSalt checks the length and copies the bytes.
func NewSalt(raw []byte) (Salt, error) {
	if len(raw) != SaltBytes {
		return Salt{}, ErrBadSalt
	}
	return Salt{key: slices.Clone(raw)}, nil
}

// String prints a fixed marker, never the key.
func (Salt) String() string { return redactedText }

// GoString prints a fixed marker for %#v.
func (Salt) GoString() string { return redactedText }

// Format prints a fixed marker for every verb.
func (Salt) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, redactedText)
}

// MarshalText keeps the key out of JSON and other text encoders.
func (Salt) MarshalText() ([]byte, error) { return []byte(redactedText), nil }
