package cmd

// `loom document` — run the repo-documenter over the directories whose
// fact-sheet hash changed (ADR-010, Phase 4). The executor chooses the
// directories and their order; the agent only interprets one fact sheet at a
// time. Results are candidates under .loom/runs/: nothing reaches the index or
// the target repository until a later phase verifies and approves it.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/claude"
	"github.com/orieken/loom/internal/repodoc"
	"github.com/orieken/loom/internal/repograph"
	"github.com/spf13/cobra"
)

type documentFlags struct {
	root   string
	path   string
	limit  int
	dryRun bool
}

var documentArgs documentFlags

var errDocumentFailed = errors.New("some directories could not be documented")

// maxPlanLines caps how many directories a dry run lists.
const maxPlanLines = 25

var documentCmd = &cobra.Command{
	Use:   "document",
	Short: "Document directories whose facts changed, with the repo-documenter agent",
	Long: `Runs the repo-documenter agent over every directory that is undocumented or whose
fact-sheet hash has changed since it was documented, children before parents.
Each invocation gets one fact sheet and its children's summaries — never raw
source — and returns typed JSON.

Results are candidates, written to .loom/runs/<run>/. Nothing is added to the
index or to your repository. Use --dry-run to see what would be sent, and
--limit or --path to bound the cost: every directory is one model call.

Run 'loom index' first so the fact sheets are current.`,
	Args: cobra.NoArgs,
	RunE: runDocument,
}

func init() {
	documentCmd.Flags().StringVar(&documentArgs.root, "root", ".", "repository root")
	documentCmd.Flags().StringVar(&documentArgs.path, "path", "", "only document this directory and those below it")
	documentCmd.Flags().IntVar(&documentArgs.limit, "limit", 0, "document at most this many directories, deepest first (0 = no limit)")
	documentCmd.Flags().BoolVar(&documentArgs.dryRun, "dry-run", false, "list what would be documented and make no model calls")
	rootCmd.AddCommand(documentCmd)
}

// documentProvider builds the provider for a run. A variable so tests can
// substitute the mock.
var documentProvider = func(root string) (orchestrator.Provider, error) {
	agentsDir, err := findAgentsDirIn(root)
	if err != nil {
		return nil, err
	}
	return claude.New(claude.Config{AgentsDir: agentsDir}), nil
}

func runDocument(cmd *cobra.Command, _ []string) error {
	ctx, stop := signalContext(cmd.Context())
	defer stop()
	return documentRepo(ctx, cmd.OutOrStdout(), documentArgs)
}

// signalContext cancels on Ctrl-C, so an interrupted run still writes its
// record of what was done.
func signalContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return signal.NotifyContext(parent, os.Interrupt)
}

func documentRepo(ctx context.Context, out io.Writer, flags documentFlags) error {
	recorded, _, err := contextindex.Load(flags.root)
	if err != nil {
		return err
	}
	sheets, err := contextindex.CurrentSheets(flags.root, repograph.GitLister{})
	if err != nil {
		return err
	}
	vocabulary, err := repodoc.LoadVocabulary(frameworkFS, flags.root)
	if err != nil {
		return err
	}
	tasks := repodoc.Plan(recorded, sheets, repodoc.Options{PathPrefix: flags.path, Limit: flags.limit})
	if len(tasks) == 0 {
		fmt.Fprintln(out, "nothing to document: every directory in scope is fresh")
		return nil
	}
	if flags.dryRun {
		printDocumentPlan(out, tasks)
		return nil
	}
	return executeDocument(ctx, out, flags.root, repodoc.Runner{Recorded: recorded, Vocabulary: vocabulary, Now: time.Now}, tasks)
}

func printDocumentPlan(out io.Writer, tasks []repodoc.Task) {
	fmt.Fprintf(out, "%d directories to document, children first (dry run: no model calls made):\n", len(tasks))
	for index, task := range tasks {
		if index == maxPlanLines {
			fmt.Fprintf(out, "  … and %d more\n", len(tasks)-maxPlanLines)
			return
		}
		fmt.Fprintf(out, "  %s (%s)\n", task.Path, task.Status)
	}
}

func executeDocument(ctx context.Context, out io.Writer, root string, runner repodoc.Runner, tasks []repodoc.Task) error {
	provider, err := documentProvider(root)
	if err != nil {
		return err
	}
	runner.Provider, runner.Root = provider, root
	fmt.Fprintf(out, "documenting %d directories with %s (one model call each)\n", len(tasks), repodoc.AgentName)
	record, err := runner.Run(ctx, tasks)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, summarizeDocumentRun(record, root))
	if record.Failed() > 0 {
		return fmt.Errorf("%w: %d of %d failed", errDocumentFailed, record.Failed(), len(record.Outcomes))
	}
	return nil
}

func summarizeDocumentRun(record repodoc.RunRecord, root string) string {
	line := fmt.Sprintf("%d documented, %d failed: candidates in %s", len(record.Outcomes)-record.Failed(), record.Failed(), repodoc.RunDir(root, record.RunID))
	if record.Interrupted {
		line += " (interrupted)"
	}
	if record.TotalUsage != nil {
		line += fmt.Sprintf(" — reported cost $%.4f", record.TotalUsage.CostUSD)
	}
	return line
}
