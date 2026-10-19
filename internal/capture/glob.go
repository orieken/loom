package capture

import (
	"fmt"
	"path"
	"strings"
)

const (
	segmentSeparator = "/"
	doubleStar       = "**"
)

// MatchGlob reports whether a "/"-separated name matches a pattern. The
// segment "**" matches zero or more whole segments. Every other segment uses
// path.Match, so "*" and "?" never cross a "/". A pattern with no "/"
// matches the last segment of the name, as a .gitignore line does
// (architecture-notes.md decision 14).
//
// A malformed pattern matches nothing. Use ValidateGlob to reject it early.
func MatchGlob(pattern, name string) bool {
	names := splitSegments(name)
	if !strings.Contains(pattern, segmentSeparator) {
		return len(names) > 0 && matchSegments([]string{pattern}, names[len(names)-1:])
	}
	return matchSegments(splitSegments(pattern), names)
}

// ValidateGlob returns an error when a segment of the pattern is malformed.
func ValidateGlob(pattern string) error {
	for _, segment := range splitSegments(pattern) {
		if segment == doubleStar {
			continue
		}
		if _, err := path.Match(segment, ""); err != nil {
			return fmt.Errorf("capture: bad glob %q: %w", pattern, err)
		}
	}
	return nil
}

func splitSegments(value string) []string {
	value = strings.TrimPrefix(value, segmentSeparator)
	if value == "" {
		return nil
	}
	return strings.Split(value, segmentSeparator)
}

func matchSegments(pattern, names []string) bool {
	if len(pattern) == 0 {
		return len(names) == 0
	}
	if pattern[0] == doubleStar {
		return matchDoubleStar(pattern[1:], names)
	}
	if len(names) == 0 {
		return false
	}
	matched, err := path.Match(pattern[0], names[0])
	return err == nil && matched && matchSegments(pattern[1:], names[1:])
}

func matchDoubleStar(rest, names []string) bool {
	for skipped := 0; skipped <= len(names); skipped++ {
		if matchSegments(rest, names[skipped:]) {
			return true
		}
	}
	return false
}
