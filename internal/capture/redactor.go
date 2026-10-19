package capture

import (
	"path/filepath"
	"strings"
)

const homePrefix = "~/"

// defaultRedactionGlobs is the built-in list (spec; analysis task 4).
func defaultRedactionGlobs() []string {
	return []string{
		".env*",
		"**/secrets/**",
		"*.pem",
		"*.key",
		"**/credentials*",
		"~/.ssh/**",
	}
}

// Redactor decides whether a path is on the redaction list: the default list
// plus the globs the user configured. A redacted path stores only a salted
// hash, no line range and no path.
type Redactor struct {
	relative []string
	absolute []string
}

// NewRedactor builds a Redactor. The home directory replaces a leading "~/"
// once, here, so the domain never asks the operating system. A bad glob is
// an error: an invalid list must fail closed (decision 4).
func NewRedactor(extra []string, home string) (Redactor, error) {
	var redactor Redactor
	for _, pattern := range append(defaultRedactionGlobs(), extra...) {
		expanded, ok := expandHome(pattern, home)
		if !ok {
			continue
		}
		if err := ValidateGlob(expanded); err != nil {
			return Redactor{}, err
		}
		redactor.add(expanded)
	}
	return redactor, nil
}

func expandHome(pattern, home string) (string, bool) {
	if !strings.HasPrefix(pattern, homePrefix) {
		return pattern, true
	}
	if home == "" {
		return "", false
	}
	return filepath.ToSlash(filepath.Clean(home)) + segmentSeparator + pattern[len(homePrefix):], true
}

func (r *Redactor) add(pattern string) {
	if strings.HasPrefix(pattern, segmentSeparator) {
		r.absolute = append(r.absolute, pattern)
		return
	}
	r.relative = append(r.relative, pattern)
}

// IsRedacted checks a project-relative path (empty when the file lies
// outside the project) and the cleaned absolute path. Both use "/".
func (r Redactor) IsRedacted(relativePath, absolutePath string) bool {
	for _, pattern := range r.relative {
		if matchesAnywhere(pattern, relativePath) {
			return true
		}
	}
	for _, pattern := range r.absolute {
		if MatchGlob(pattern, absolutePath) {
			return true
		}
	}
	return false
}

// matchesAnywhere is stricter than MatchGlob for a pattern with no "/": it
// tests every segment, so ".env*" also hides ".env.d/key". Hiding too much
// is the safe error for a redaction list.
func matchesAnywhere(pattern, name string) bool {
	if strings.Contains(pattern, segmentSeparator) {
		return MatchGlob(pattern, name)
	}
	for _, segment := range splitSegments(name) {
		if MatchGlob(pattern, segment) {
			return true
		}
	}
	return false
}
