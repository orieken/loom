// Package references finds live text that points at an agent, skill or
// workflow which does not exist (roadmap L3.61, ADR-009).
//
// Two kinds of reference are checked:
//
//   - a path — shared/agents/<name>.md, shared/skills/<name>/SKILL.md or
//     shared/workflows/<name>.md — that no longer resolves, and
//   - the bare name of something deliberately removed, listed in Rules.Retired.
//     A path check alone cannot catch "invoke test-driven-developer": prose
//     names agents far more often than it links them.
//
// Nothing failed on either before this package: a removed agent could stay
// cited in thirty files and every check stayed green.
package references

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// pathReference matches a repository path to an agent, skill or workflow and
// captures the name inside it.
var pathReference = regexp.MustCompile(
	`shared/(?:agents/([a-z0-9-]+)\.md|skills/([a-z0-9-]+)/SKILL\.md|workflows/([a-z0-9-]+)\.md)`)

// Finding is one dangling reference.
type Finding struct {
	File      string
	Line      int
	Reference string
	Problem   string
}

// Rules says what to look for and where not to.
type Rules struct {
	// Retired maps a removed name to why it was removed. Any mention in a
	// scanned file is a finding.
	Retired map[string]string
	// Placeholders are names used only as illustrations — "shared/agents/foo.md"
	// in an agent's example output — and never resolved.
	Placeholders map[string]bool
	// Historical lists files that record the past and are not rewritten when
	// something is removed: an entry ending in "/" is a directory prefix, one
	// starting with "*" a suffix, anything else an exact path.
	Historical []string
}

// Scan checks files (repository-relative, slash-separated) under root.
func Scan(root string, files []string, rules Rules) ([]Finding, error) {
	retired := retiredPatterns(rules.Retired)
	var findings []Finding
	for _, file := range files {
		if isHistorical(file, rules.Historical) {
			continue
		}
		found, err := scanFile(root, file, rules, retired)
		if err != nil {
			return nil, err
		}
		findings = append(findings, found...)
	}
	return findings, nil
}

func scanFile(root, file string, rules Rules, retired map[string]*regexp.Regexp) ([]Finding, error) {
	path := filepath.Join(root, filepath.FromSlash(file))
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, nil // its target is scanned in its own right, if tracked
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if bytes.IndexByte(content, 0) >= 0 {
		return nil, nil // binary
	}
	var findings []Finding
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for line := 1; scanner.Scan(); line++ {
		text := scanner.Text()
		findings = append(findings, missingPaths(root, file, line, text, rules.Placeholders)...)
		findings = append(findings, retiredNames(file, line, text, rules.Retired, retired)...)
	}
	return findings, scanner.Err()
}

func missingPaths(root, file string, line int, text string, placeholders map[string]bool) []Finding {
	var findings []Finding
	for _, match := range pathReference.FindAllStringSubmatch(text, -1) {
		if placeholders[referencedName(match)] {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(match[0]))); err == nil {
			continue
		}
		findings = append(findings, Finding{File: file, Line: line, Reference: match[0], Problem: "does not exist"})
	}
	return findings
}

func referencedName(match []string) string {
	for _, group := range match[1:] {
		if group != "" {
			return group
		}
	}
	return ""
}

func retiredNames(file string, line int, text string, reasons map[string]string, patterns map[string]*regexp.Regexp) []Finding {
	var findings []Finding
	for _, name := range sortedNames(patterns) {
		if patterns[name].MatchString(text) {
			findings = append(findings, Finding{File: file, Line: line, Reference: name,
				Problem: fmt.Sprintf("retired: %s", reasons[name])})
		}
	}
	return findings
}

// retiredPatterns matches a name as a whole token: "developer" must not match
// inside "test-driven-developer", nor "tdd" inside "tdd-state".
func retiredPatterns(retired map[string]string) map[string]*regexp.Regexp {
	patterns := map[string]*regexp.Regexp{}
	for name := range retired {
		patterns[name] = regexp.MustCompile(`(?:^|[^A-Za-z0-9_-])` + regexp.QuoteMeta(name) + `(?:[^A-Za-z0-9_-]|$)`)
	}
	return patterns
}

func isHistorical(file string, historical []string) bool {
	for _, entry := range historical {
		if matchesEntry(file, entry) {
			return true
		}
	}
	return false
}

func matchesEntry(file, entry string) bool {
	switch {
	case strings.HasSuffix(entry, "/"):
		return strings.HasPrefix(file, entry)
	case strings.HasPrefix(entry, "*"):
		return strings.HasSuffix(file, entry[1:])
	default:
		return file == entry
	}
}

func sortedNames(patterns map[string]*regexp.Regexp) []string {
	names := make([]string, 0, len(patterns))
	for name := range patterns {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
