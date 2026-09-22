package cmd

// `loom memory` — the episodic store's command surface (roadmap L3.5).
//
// Every query is a named subcommand backed by a parameterized statement.
// There is deliberately no free-form SQL: it keeps the schema private so
// later phases can change it, and it means everything the store can answer
// is discoverable through --help rather than by writing SQL against a shape
// nobody promised to keep.

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/orieken/loom/internal/memory"
	"github.com/spf13/cobra"
)

type memoryFlags struct {
	asJSON   bool
	agent    string
	moreThan int
	limit    int
	dir      string
}

var memoryArgs memoryFlags

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Query what past runs actually did",
	Long: `Query the episodic store: what runs did, what they cost, and what a human
had to correct.

The store lives at .claude/memory/episodes.db and is populated automatically
at the end of every "loom run". It is a projection of records that ARE
committed — run-state.json, run-events.jsonl and the typed stage documents
under state/ are archived into docs/features/<name>/ — so deleting it loses
nothing that "loom memory ingest" cannot rebuild.`,
	Args: cobra.NoArgs,
}

var memoryIngestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Rebuild the store from archived runs, or ingest one directory",
	Long: `With no --dir, walks docs/features/*/ and ingests every run whose records
were archived — which is how a deleted store is rebuilt, and how deliveries
that predate the store are imported. Ingesting is idempotent, so running it
repeatedly is safe.`,
	Args: cobra.NoArgs,
	RunE: runMemoryIngest,
}

var memoryRunsCmd = &cobra.Command{
	Use:   "runs",
	Short: "List ingested runs, newest first",
	Args:  cobra.NoArgs,
	RunE:  runMemoryRuns,
}

var memoryRetriesCmd = &cobra.Command{
	Use:   "retries",
	Short: "Show stages that needed more than N attempts",
	Args:  cobra.NoArgs,
	RunE:  runMemoryRetries,
}

var memoryPoliciesCmd = &cobra.Command{
	Use:   "policies",
	Short: "Show every recorded policy decision, and what the human did at that gate",
	Long: `Reads the corpus roadmap L2.19 is waiting on.

L2.16 evaluates the policies watching a gate and records what they decided,
then halts for a human anyway. L2.19 — honouring an auto-approve decision —
is deliberately not built until real runs show the evaluator deciding what a
human would. This command is how that question gets looked at: it lists each
decision beside the approval that followed it, and marks policies that could
not see a fact they needed.

It honours nothing and changes nothing.`,
	Args: cobra.NoArgs,
	RunE: runMemoryPolicies,
}

var memoryCorrectionsCmd = &cobra.Command{
	Use:   "corrections",
	Short: "Rank agents by how often a human corrected their output",
	Args:  cobra.NoArgs,
	RunE:  runMemoryCorrections,
}

func init() {
	rootCmd.AddCommand(memoryCmd)
	memoryCmd.AddCommand(memoryIngestCmd, memoryRunsCmd, memoryRetriesCmd, memoryCorrectionsCmd, memoryPoliciesCmd)
	for _, command := range []*cobra.Command{memoryRunsCmd, memoryRetriesCmd, memoryCorrectionsCmd, memoryPoliciesCmd} {
		command.Flags().BoolVar(&memoryArgs.asJSON, "json", false, "emit JSON instead of a table")
	}
	memoryIngestCmd.Flags().StringVar(&memoryArgs.dir, "dir", "",
		"ingest one directory holding run-state.json (default: every archived run)")
	memoryRetriesCmd.Flags().StringVar(&memoryArgs.agent, "agent", "",
		"limit to one agent (default: every agent)")
	memoryRetriesCmd.Flags().IntVar(&memoryArgs.moreThan, "more-than", 2,
		"report stages whose iteration count exceeds this")
	memoryRunsCmd.Flags().IntVar(&memoryArgs.limit, "limit", 20, "how many runs to list")
	memoryPoliciesCmd.Flags().IntVar(&memoryArgs.limit, "limit", 100, "how many decisions to list")
}

// openStore opens the project-local store. A missing store is not an error
// worth a stack trace: it means no run has been recorded yet, and the
// message says how to fix that.
func openStore() (*memory.Store, error) {
	store, err := memory.Open(memory.DefaultPath("."))
	if err != nil {
		return nil, fmt.Errorf("%w — run a delivery, or `loom memory ingest` to rebuild from docs/features/", err)
	}
	return store, nil
}

func emit(cmd *cobra.Command, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode result: %w", err)
	}
	cmd.Println(string(raw))
	return nil
}

func featuresDir() string { return filepath.Join("docs", "features") }
