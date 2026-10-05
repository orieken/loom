package cmd

// `loom index` — build the repo graph (ADR-010, Phase 1). It extracts import
// edges deterministically and writes .loom/graph.json; no model is consulted
// and nothing outside .loom/ is touched.

import (
	"fmt"
	"time"

	"github.com/orieken/loom/internal/factsheet"
	"github.com/orieken/loom/internal/repograph"
	"github.com/spf13/cobra"
)

var indexRoot string

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Build the repo graph and per-directory fact sheets of a git repository into .loom/",
	Long: `Extracts module-level import edges from the tracked files of a git repository
and writes .loom/graph.json: directories as nodes, imports as edges, each edge
labelled exact or approximate. Imports that cannot be resolved are listed, not
guessed. Go, TypeScript/JavaScript, Python and Java are read.

Extraction is local and offline. Files come from git ls-files, minus anything
matched by .loomignore. A rerun on an unchanged repo writes nothing.

It also writes one fact sheet per directory to .loom/facts/: files, exported
symbols, imports, dependents, entrypoints, tests, children and an input hash
that changes when the directory or anything beneath it changes.`,
	Args: cobra.NoArgs,
	RunE: runIndex,
}

func init() {
	indexCmd.Flags().StringVar(&indexRoot, "root", ".", "repository root to index")
	rootCmd.AddCommand(indexCmd)
}

func runIndex(cmd *cobra.Command, _ []string) error {
	result, err := repograph.Index(indexRoot, repograph.GitLister{}, repograph.DefaultExtractors(), time.Now)
	if err != nil {
		return fmt.Errorf("index %s: %w", indexRoot, err)
	}
	sheets := factsheet.Build(result.Repo, result.Graph)
	outcome, err := factsheet.Write(indexRoot, sheets)
	if err != nil {
		return fmt.Errorf("write fact sheets: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), summarizeIndex(result))
	fmt.Fprintf(cmd.OutOrStdout(), "%d fact sheets: %d written, %d removed\n", len(sheets), outcome.Written, outcome.Removed)
	return nil
}

func summarizeIndex(result repograph.Result) string {
	graph := result.Graph
	exact := 0
	for _, edge := range graph.Edges {
		if edge.Confidence == repograph.Exact {
			exact++
		}
	}
	state := "unchanged, nothing written"
	if result.Written {
		state = "wrote " + repograph.OutputDir + "/" + repograph.GraphFile
	}
	return fmt.Sprintf("%d nodes, %d edges (%d exact, %d approximate), %d unresolved: %s",
		len(graph.Nodes), len(graph.Edges), exact, len(graph.Edges)-exact, len(graph.Unresolved), state)
}
