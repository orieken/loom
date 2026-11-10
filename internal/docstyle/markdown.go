package docstyle

import (
	"regexp"
	"strings"
)

// Line is one line of a Markdown file's body left after every ignored
// region — frontmatter, fenced and inline code, a table row, a heading, a
// link's target, and an HTML comment — is stripped out. Number is the
// line's position in the original file (1-based), so a Finding can point
// back to it.
type Line struct {
	Number int
	Text   string
}

const fenceMarker = "---"

var (
	headingRe         = regexp.MustCompile(`^\s*#{1,6}\s`)
	tableRowRe        = regexp.MustCompile(`^\s*\|`)
	fullLineCommentRe = regexp.MustCompile(`^\s*<!--.*-->\s*$`)
	codeFenceRe       = regexp.MustCompile("^\\s*(```+|~~~+)")
	inlineCodeRe      = regexp.MustCompile("`[^`]*`")
	linkTargetRe      = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	inlineCommentRe   = regexp.MustCompile(`<!--.*?-->`)
)

// StripFrontmatter removes a leading `---` ... `---` fence and reports the
// body and the 1-based line the body starts on. A file with no opening
// fence, or an unclosed one, is returned whole: this is a report, not a
// build gate, so a malformed file degrades to "scan it all as prose"
// instead of failing the check.
func StripFrontmatter(source string) (string, int) {
	lines := splitLines(source)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != fenceMarker {
		return source, 1
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == fenceMarker {
			return strings.Join(lines[i+1:], "\n"), i + 2
		}
	}
	return source, 1
}

// Prose returns every line of body left after stripping ignored regions,
// numbered from the original file starting at firstLine (StripFrontmatter's
// second result). A line that is entirely ignored, or empty once its
// ignored spans are removed, is dropped — an ignored region is a hard
// sentence boundary, so prose either side of it is never joined.
func Prose(body string, firstLine int) []Line {
	var prose []Line
	state := &regionState{}
	for i, raw := range splitLines(body) {
		if state.consume(raw) {
			continue
		}
		if isFullyIgnoredLine(raw) {
			continue
		}
		text := strings.TrimSpace(stripInlineIgnored(raw))
		if text == "" {
			continue
		}
		prose = append(prose, Line{Number: firstLine + i, Text: text})
	}
	return prose
}

// regionState tracks a fenced code block or a multi-line HTML comment
// across lines — both are hard boundaries no single-line regex can see.
type regionState struct {
	fence     string
	inComment bool
}

// consume reports whether line belongs to an open fence or comment region,
// updating the state if that region closes on this line.
func (state *regionState) consume(line string) bool {
	if state.inComment {
		state.closeCommentIfEnded(line)
		return true
	}
	if state.fence != "" {
		state.closeFenceIfEnded(line)
		return true
	}
	return state.openRegionIfStarting(line)
}

func (state *regionState) closeCommentIfEnded(line string) {
	if strings.Contains(line, "-->") {
		state.inComment = false
	}
}

func (state *regionState) closeFenceIfEnded(line string) {
	if strings.TrimSpace(line) == state.fence {
		state.fence = ""
	}
}

func (state *regionState) openRegionIfStarting(line string) bool {
	if fence := codeFenceRe.FindString(line); fence != "" {
		state.fence = strings.TrimSpace(fence)
		return true
	}
	if opensUnclosedComment(line) {
		state.inComment = true
		return true
	}
	return false
}

// opensUnclosedComment reports whether line opens an HTML comment that does
// not also close on the same line.
func opensUnclosedComment(line string) bool {
	start := strings.Index(line, "<!--")
	if start == -1 {
		return false
	}
	return !strings.Contains(line[start:], "-->")
}

// isFullyIgnoredLine reports a line that contributes nothing to prose on
// its own: a heading, a table row, or a comment that opens and closes on
// this one line.
func isFullyIgnoredLine(line string) bool {
	return headingRe.MatchString(line) || tableRowRe.MatchString(line) || fullLineCommentRe.MatchString(line)
}

// stripInlineIgnored removes an inline code span and an inline HTML
// comment, and keeps a link's visible text while dropping its target.
func stripInlineIgnored(line string) string {
	line = inlineCodeRe.ReplaceAllString(line, " ")
	line = linkTargetRe.ReplaceAllString(line, "$1")
	line = inlineCommentRe.ReplaceAllString(line, " ")
	return line
}

func splitLines(body string) []string {
	return strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
}
