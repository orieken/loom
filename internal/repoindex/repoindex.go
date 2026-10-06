// Package repoindex runs the whole of `loom index` in one call: the repo graph,
// the per-directory fact sheets and the context index (ADR-010). It exists so
// the command and the tests that need real index artifacts share one
// implementation instead of each repeating the three steps.
package repoindex

import (
	"time"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/factsheet"
	"github.com/orieken/loom/internal/repograph"
)

// Result is everything one indexing run produced.
type Result struct {
	Graph        repograph.Result
	Sheets       []factsheet.FactSheet
	Facts        factsheet.Outcome
	Index        contextindex.Index
	IndexWritten bool
}

// Run indexes the repository at root. Each artifact is written only if its
// content changed, so a rerun over an unchanged repo writes nothing.
func Run(root string, lister repograph.Lister, now func() time.Time) (Result, error) {
	graph, err := repograph.Index(root, lister, repograph.DefaultExtractors(), now)
	if err != nil {
		return Result{}, err
	}
	sheets := factsheet.Build(graph.Repo, graph.Graph)
	facts, err := factsheet.Write(root, sheets)
	if err != nil {
		return Result{}, err
	}
	index, written, err := contextindex.Refresh(root, sheets)
	if err != nil {
		return Result{}, err
	}
	return Result{Graph: graph, Sheets: sheets, Facts: facts, Index: index, IndexWritten: written}, nil
}
