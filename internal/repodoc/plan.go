package repodoc

import (
	"sort"
	"strings"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/factsheet"
)

// Task is one directory to document.
type Task struct {
	Path   string
	Sheet  factsheet.FactSheet
	Status contextindex.Status
}

// Options narrow a plan.
type Options struct {
	// PathPrefix keeps only directories at or below it. Empty keeps all.
	PathPrefix string
	// Limit caps the number of directories. Zero means no cap.
	Limit int
}

// Plan returns the directories to document, children before parents.
//
// Only a directory whose recorded input hash differs from its current one is
// sent: undocumented ones, and stale ones. A fresh directory costs nothing.
// Order is deepest first and then by path, so it is stable, and a parent never
// precedes a child — the agent is shown its children's summaries and they must
// exist. A limit therefore keeps the leaves, never a parent without them.
func Plan(index contextindex.Index, sheets []factsheet.FactSheet, options Options) []Task {
	var tasks []Task
	for _, sheet := range sheets {
		status := contextindex.StatusOf(index.Entries[sheet.Path].InputHash, sheet.InputHash)
		if status != contextindex.StatusFresh && inScope(sheet.Path, options.PathPrefix) {
			tasks = append(tasks, Task{Path: sheet.Path, Sheet: sheet, Status: status})
		}
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		if depth(tasks[i].Path) != depth(tasks[j].Path) {
			return depth(tasks[i].Path) > depth(tasks[j].Path)
		}
		return tasks[i].Path < tasks[j].Path
	})
	if options.Limit > 0 && len(tasks) > options.Limit {
		tasks = tasks[:options.Limit]
	}
	return tasks
}

func inScope(path, prefix string) bool {
	if prefix == "" || prefix == factsheet.RootPath {
		return true
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func depth(path string) int {
	if path == factsheet.RootPath {
		return 0
	}
	return strings.Count(path, "/") + 1
}
