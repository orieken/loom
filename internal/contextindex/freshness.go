package contextindex

import "github.com/orieken/loom/internal/factsheet"

// Report is what a freshness check found, each list sorted.
type Report struct {
	// Stale: documented, and the directory has since changed.
	Stale []string
	// Undocumented: indexed but never documented.
	Undocumented []string
	// Unindexed: a directory with no entry — the index predates it.
	Unindexed []string
	// Orphaned: an entry for a directory that no longer exists.
	Orphaned []string
	// Fresh counts documented entries that are still true.
	Fresh int
}

// NeedsReindex says the index itself is out of step with the tree, as opposed
// to merely undocumented.
func (report Report) NeedsReindex() bool {
	return len(report.Stale) > 0 || len(report.Unindexed) > 0 || len(report.Orphaned) > 0
}

// Documented counts entries that have ever been documented.
func (report Report) Documented() int { return report.Fresh + len(report.Stale) }

// Check compares a stored index with the working tree's current sheets. It
// trusts the stored input hashes, never the stored status: status is a
// snapshot, and this is the check that says whether it still holds.
func Check(stored Index, sheets []factsheet.FactSheet) Report {
	var report Report
	current := map[string]bool{}
	for _, sheet := range sheets {
		current[sheet.Path] = true
		classify(&report, stored.Entries, sheet)
	}
	for path := range stored.Entries {
		if !current[path] {
			report.Orphaned = append(report.Orphaned, path)
		}
	}
	report.Stale, report.Undocumented = sorted(report.Stale), sorted(report.Undocumented)
	report.Unindexed, report.Orphaned = sorted(report.Unindexed), sorted(report.Orphaned)
	return report
}

func classify(report *Report, entries map[string]Entry, sheet factsheet.FactSheet) {
	entry, indexed := entries[sheet.Path]
	if !indexed {
		report.Unindexed = append(report.Unindexed, sheet.Path)
		return
	}
	switch StatusOf(entry.InputHash, sheet.InputHash) {
	case StatusStale:
		report.Stale = append(report.Stale, sheet.Path)
	case StatusUndocumented:
		report.Undocumented = append(report.Undocumented, sheet.Path)
	default:
		report.Fresh++
	}
}
