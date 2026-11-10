package docstyle

import (
	"fmt"
	"strings"
)

// Render renders a Report as one `file:line: rule: excerpt` line per
// finding, plus a trailing summary of how many findings a
// `<!-- docstyle-ignore: <rule> -->` directive suppressed.
func Render(report Report) string {
	var out strings.Builder
	for _, finding := range report.Findings {
		out.WriteString(finding.String())
		out.WriteByte('\n')
	}
	fmt.Fprintf(&out, "%d finding(s) skipped via docstyle-ignore\n", report.Skipped)
	return out.String()
}
