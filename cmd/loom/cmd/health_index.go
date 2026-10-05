package cmd

import (
	"fmt"
	"strings"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/repograph"
)

// maxListedPaths caps how many paths one health line names.
const maxListedPaths = 5

// verifyContextIndex compares the stored context index with the working tree
// (ADR-010). It is silent when the project has no index: indexing is opt-in.
// Everything it finds is a warning — a stale index is out of date, not broken.
func (check *healthCheck) verifyContextIndex() {
	stored, found, err := contextindex.Load(check.request.target)
	if err != nil {
		check.warn("context index unreadable: " + err.Error())
		return
	}
	if !found {
		return
	}
	sheets, err := contextindex.CurrentSheets(check.request.target, repograph.GitLister{})
	if err != nil {
		check.warn("context index freshness not checked: " + err.Error())
		return
	}
	check.reportContextIndex(contextindex.Check(stored, sheets), len(sheets))
}

func (check *healthCheck) reportContextIndex(report contextindex.Report, total int) {
	check.warnAboutPaths(report.Stale, "stale — changed since documented")
	check.warnAboutPaths(report.Unindexed, "not in the index — run loom index")
	check.warnAboutPaths(report.Orphaned, "indexed but no longer exist — run loom index")
	if report.Documented() > 0 {
		check.warnAboutPaths(report.Undocumented, "undocumented")
	}
	if report.NeedsReindex() || (report.Documented() > 0 && len(report.Undocumented) > 0) {
		return
	}
	check.output.success(contextIndexSummary(report, total))
}

func contextIndexSummary(report contextindex.Report, total int) string {
	if report.Documented() == 0 {
		return fmt.Sprintf("context index current (%d paths indexed, none documented yet)", total)
	}
	return fmt.Sprintf("context index fresh (%d paths)", total)
}

// warnAboutPaths emits one warning naming the first few paths.
func (check *healthCheck) warnAboutPaths(paths []string, what string) {
	if len(paths) == 0 {
		return
	}
	listed := paths
	if len(listed) > maxListedPaths {
		listed = listed[:maxListedPaths]
	}
	message := fmt.Sprintf("context index: %d path(s) %s: %s", len(paths), what, strings.Join(listed, ", "))
	if len(paths) > len(listed) {
		message += fmt.Sprintf(" (+%d more)", len(paths)-len(listed))
	}
	check.warn(message)
}
