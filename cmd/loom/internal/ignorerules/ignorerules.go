// Package ignorerules answers one question about a project: does the root
// .gitignore ignore a given directory?
//
// The answer is deliberately conservative. The package reads only the
// .gitignore at the project root and accepts only plain patterns that name
// the directory or one of its parents. A pattern it cannot judge (a
// wildcard in the middle, a negation that touches the path) gives "no".
// A false "no" costs the user one line in .gitignore. A false "yes" could
// put captured session data into a commit.
package ignorerules

import (
	"bufio"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	filename     = ".gitignore"
	negationMark = "!"
)

// IgnoresDirectory reports whether the .gitignore in root ignores the
// directory. The directory is a slash-separated path relative to root, for
// example ".claude/memory". A missing .gitignore ignores nothing.
func IgnoresDirectory(root, directory string) (bool, error) {
	file, err := os.Open(filepath.Join(root, filename))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer func() { _ = file.Close() }()
	lines := bufio.NewScanner(file)
	isIgnored := scan(lines, path.Clean(directory))
	return isIgnored && lines.Err() == nil, lines.Err()
}

// scan walks the lines in order. The last matching line wins, as in git:
// a plain pattern that covers the directory ignores it, and a negation
// that touches the directory un-ignores it.
func scan(lines *bufio.Scanner, directory string) bool {
	isIgnored := false
	for lines.Scan() {
		line := strings.TrimSpace(lines.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if pattern, isNegation := strings.CutPrefix(line, negationMark); isNegation {
			if touches(pattern, directory) {
				isIgnored = false
			}
			continue
		}
		if covers(line, directory) {
			isIgnored = true
		}
	}
	return isIgnored
}

// covers reports whether a plain pattern ignores the directory: the
// directory itself or one of its parents, with an optional leading slash,
// an optional trailing slash, or a trailing /* or /**.
func covers(pattern, directory string) bool {
	normal := normalise(pattern)
	for current := directory; current != "." && current != "/"; current = path.Dir(current) {
		if normal == current {
			return true
		}
	}
	return false
}

// touches reports whether a negated pattern could un-ignore the directory.
// Any negation that names the directory, a parent, or something below it
// counts, so the package stays on the safe side.
func touches(pattern, directory string) bool {
	normal := normalise(pattern)
	return covers(pattern, directory) || normal == directory || strings.HasPrefix(normal, directory+"/") || strings.ContainsAny(normal, "*?[")
}

// normalise strips the decoration that does not change what a plain
// pattern names.
func normalise(pattern string) string {
	normal := strings.TrimSuffix(strings.TrimSuffix(pattern, "**"), "*")
	normal = strings.TrimSuffix(normal, "/")
	return strings.TrimPrefix(normal, "/")
}
