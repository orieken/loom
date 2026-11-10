package docstyle

import "regexp"

var directiveRe = regexp.MustCompile(`^\s*<!--\s*docstyle-ignore:\s*([\w-]+)\s*-->\s*$`)

// Directives maps the line number of a finding a `<!-- docstyle-ignore:
// <rule> -->` directive would suppress — the line right after it — to the
// rule name it named. A directive line with nothing on the next line
// matters only if Suppress is later asked about that line; it is otherwise
// never counted on its own.
func Directives(body string, firstLine int) map[int]string {
	directives := map[int]string{}
	for i, line := range splitLines(body) {
		if match := directiveRe.FindStringSubmatch(line); match != nil {
			directives[firstLine+i+1] = match[1]
		}
	}
	return directives
}

// Suppress removes a finding whose line and rule match a directive, and
// reports how many were removed. A directive naming a different rule than
// the finding actually on that line does not suppress it.
func Suppress(findings []Finding, directives map[int]string) ([]Finding, int) {
	var kept []Finding
	skipped := 0
	for _, finding := range findings {
		if directives[finding.Line] == finding.Rule {
			skipped++
			continue
		}
		kept = append(kept, finding)
	}
	return kept, skipped
}
