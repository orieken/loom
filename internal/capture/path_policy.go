package capture

import (
	"path/filepath"
	"strings"
)

// PathDecision is the outcome of the path rules for one file path.
type PathDecision struct {
	// IsRedacted is true for a path on the redaction list, outside the
	// project, or longer than MaxPathBytes.
	IsRedacted bool
	// HashInput is the text the path hash is computed from.
	HashInput string
	// Display is the project-relative path with "/" separators. It is nil
	// when the path is redacted.
	Display *string
}

// PathPolicy applies the path rules of architecture-notes.md decision 4.
type PathPolicy struct {
	root     string
	redactor Redactor
}

// NewPathPolicy builds the policy of one project root.
func NewPathPolicy(root string, redactor Redactor) PathPolicy {
	return PathPolicy{root: filepath.Clean(root), redactor: redactor}
}

// Decide cleans the path and decides how to store it. It does no file
// system access. A "not a regular file" check belongs to the adapter.
func (p PathPolicy) Decide(rawPath string) PathDecision {
	cleaned := filepath.ToSlash(filepath.Clean(rawPath))
	relative, isInside := p.relativeToRoot(cleaned)
	if !isInside {
		return PathDecision{IsRedacted: true, HashInput: cleaned}
	}
	if len(relative) > MaxPathBytes || p.redactor.IsRedacted(relative, cleaned) {
		return PathDecision{IsRedacted: true, HashInput: relative}
	}
	return PathDecision{HashInput: relative, Display: &relative}
}

func (p PathPolicy) relativeToRoot(cleaned string) (string, bool) {
	relative := cleaned
	if filepath.IsAbs(filepath.FromSlash(cleaned)) {
		var err error
		relative, err = filepath.Rel(p.root, filepath.FromSlash(cleaned))
		if err != nil {
			return "", false
		}
		relative = filepath.ToSlash(relative)
	}
	isOutside := relative == "." || relative == ".." || strings.HasPrefix(relative, "../")
	return relative, !isOutside
}
