// Package ownership decides whether loom may write to a destination
// (roadmap L3.26).
//
// The rule the whole package exists to enforce: loom writes only to paths it
// installed itself. Anything else in the target belongs to the project — a
// team's own agent, a skill someone wrote last quarter — and install must
// leave it exactly as it found it.
//
// Before L3.26 the unit of installation was the directory, so installing
// `shared/agents` moved whatever occupied `.claude/agents` aside wholesale.
// Ownership is recorded per path instead, because that is the granularity at
// which a foreign file can be distinguished from one of ours.
package ownership

import (
	"crypto/sha256"
	"encoding/hex"
)

// Decision is what install should do with one destination path.
type Decision int

const (
	// Write installs the path: nothing occupies it.
	Write Decision = iota
	// Update rewrites a path loom installed and nobody has edited since.
	Update
	// Skip leaves a path alone because loom does not own it.
	Skip
	// SkipModified leaves a path alone because loom owns it but the project
	// has edited it. Overwriting would discard someone's work silently.
	SkipModified
)

// Ledger is the set of paths a prior install recorded as loom's own, mapped
// to the digest of the content loom wrote there.
type Ledger map[string]string

// State is what the filesystem reports about one destination.
type State struct {
	// Exists is false when nothing occupies the path.
	Exists bool
	// Digest is of the content currently at the path, resolved through any
	// symlink. Empty when the path does not exist or could not be read.
	Digest string
}

// Decide resolves one destination against the ledger. The four cases are
// exhaustive and each has exactly one honest answer, so this is a lookup
// rather than a judgement.
func (ledger Ledger) Decide(destination string, state State) Decision {
	if !state.Exists {
		return Write
	}
	installed, isOwned := ledger[destination]
	if !isOwned {
		return Skip
	}
	if installed == state.Digest {
		return Update
	}
	return SkipModified
}

// Digest hashes content the way the ledger records it.
func Digest(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// Reason explains a decision in the words the install output uses. A skip
// nobody can account for is the defect L3.26 was filed for.
func Reason(decision Decision) string {
	switch decision {
	case Skip:
		return "not installed by loom — left untouched"
	case SkipModified:
		return "edited since loom installed it — left untouched (--force overwrites)"
	default:
		return ""
	}
}

// IsSkip reports whether a decision leaves the destination alone.
func IsSkip(decision Decision) bool {
	return decision == Skip || decision == SkipModified
}
