package repograph

import (
	"regexp"
	"strings"
)

// IgnoreFileName is the per-repo file listing tracked paths the index skips —
// vendored and generated code. gitignore-style: `#` comments, `*`, `**`, `?`,
// a pattern holding a slash is anchored at the repo root, `!` re-includes.
// A trailing slash is accepted but not distinguished from the bare name.
const IgnoreFileName = ".loomignore"

type ignoreRule struct {
	matcher *regexp.Regexp
	negate  bool
}

// Ignore decides which repo-relative paths are excluded.
type Ignore struct {
	rules []ignoreRule
}

// ParseIgnore reads the text of a .loomignore.
func ParseIgnore(text string) *Ignore {
	ignore := &Ignore{}
	for _, line := range strings.Split(text, "\n") {
		if rule, ok := parseIgnoreLine(line); ok {
			ignore.rules = append(ignore.rules, rule)
		}
	}
	return ignore
}

func parseIgnoreLine(line string) (ignoreRule, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ignoreRule{}, false
	}
	negate := strings.HasPrefix(line, "!")
	pattern := strings.TrimSuffix(strings.TrimPrefix(line, "!"), "/")
	matcher, err := regexp.Compile(globToRegexp(pattern))
	if err != nil {
		return ignoreRule{}, false
	}
	return ignoreRule{matcher: matcher, negate: negate}, true
}

// globToRegexp matches the path itself or anything beneath it.
func globToRegexp(pattern string) string {
	anchored := strings.Contains(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")
	var out strings.Builder
	out.WriteString("^")
	if !anchored {
		out.WriteString("(?:.*/)?")
	}
	out.WriteString(translateGlob(pattern))
	out.WriteString("(?:/.*)?$")
	return out.String()
}

func translateGlob(pattern string) string {
	var out strings.Builder
	for i := 0; i < len(pattern); i++ {
		switch {
		case strings.HasPrefix(pattern[i:], "**"):
			out.WriteString(".*")
			i++
		case pattern[i] == '*':
			out.WriteString("[^/]*")
		case pattern[i] == '?':
			out.WriteString("[^/]")
		default:
			out.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	return out.String()
}

// Matches reports whether path is excluded. The last matching rule wins.
func (i *Ignore) Matches(path string) bool {
	ignored := false
	for _, rule := range i.rules {
		if rule.matcher.MatchString(path) {
			ignored = !rule.negate
		}
	}
	return ignored
}
