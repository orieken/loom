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
	"github.com/orieken/loom/internal/factsheet"
	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/claude"
	"github.com/orieken/loom/internal/repodoc"
	"github.com/orieken/loom/internal/repograph"
	"github.com/orieken/loom/internal/telemetry"
	"github.com/spf13/cobra"
)

type documentFlags struct {
	root      string
	path      string
	limit     int
	dryRun    bool
	maxRounds int
	noAudit   bool
	// noTelemetry stops the run recording its OTLP/JSON trace beside the run.
	noTelemetry bool
}

var documentArgs documentFlags

var errDocumentFailed = errors.New("some directories could not be documented or verified")

// maxPlanLines caps how many directories a dry run lists.
const maxPlanLines = 25

var documentCmd = &cobra.Command{
	Use:   "document",
	Short: "Document directories whose facts changed, with the repo-documenter agent",
	Long: `Runs the repo-documenter agent over every directory that is undocumented or whose
fact-sheet hash has changed since it was documented, children before parents.
Each invocation gets one fact sheet and its children's summaries — never raw
source — and returns typed JSON.

Every result is then checked: first by a deterministic verifier that confirms each
name, tag and citation exists in the fact sheet, then by the repo-doc-auditor
agent, which judges whether the cited facts support the claims. A problem sends
the directory back to the documenter with typed findings, up to --max-rounds. A
directory still failing then is reported "unverified" and nothing is accepted.

Verified results are candidates, written to .loom/runs/<run>/. Nothing is added
to the index or to your repository. Use --dry-run to see what would be sent, and
--limit or --path to bound the cost: a directory is at least one model call, and
up to two per round.

Run 'loom index' first so the fact sheets are current.`,
	Args: cobra.NoArgs,
	RunE: runDocument,
}

func init() {
	documentCmd.Flags().StringVar(&documentArgs.root, "root", ".", "repository root")
	documentCmd.Flags().StringVar(&documentArgs.path, "path", "", "only document this directory and those below it")
	documentCmd.Flags().IntVar(&documentArgs.limit, "limit", 0, "document at most this many directories, deepest first (0 = no limit)")
	documentCmd.Flags().IntVar(&documentArgs.maxRounds, "max-rounds", repodoc.DefaultMaxRounds, "attempts per directory before it is reported unverified")
	documentCmd.Flags().BoolVar(&documentArgs.noAudit, "no-audit", false, "skip the semantic auditor and run only the deterministic verifier (cheaper, weaker)")
	documentCmd.Flags().BoolVar(&documentArgs.noTelemetry, "no-telemetry", false, "do not write the run's OTLP/JSON trace (.loom/runs/<run>/traces.jsonl)")
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
	return claude.New(claude.Config{AgentsDir: agentsDir, IsolateProjectContext: true}), nil
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
	runner := repodoc.Runner{Recorded: recorded, Vocabulary: vocabulary, Now: time.Now, MaxRounds: flags.maxRounds, SkipAudit: flags.noAudit,
		CurrentHashes: currentHashes(sheets)}
	return executeDocument(ctx, out, flags, runner, tasks)
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

func executeDocument(ctx context.Context, out io.Writer, flags documentFlags, runner repodoc.Runner, tasks []repodoc.Task) error {
	root := flags.root
	provider, err := documentProvider(root)
	if err != nil {
		return err
	}
	runner.Provider, runner.Root = provider, root
	started := runner.Now()
	runner.RunID, runner.Now = repodoc.RunIDAt(started), func() time.Time { return started }
	tracer, stop := startToolTelemetry(out, traceFileForDocument(flags, root, runner.RunID))
	defer stop()
	runner.Tracer = tracer
	fmt.Fprintf(out, "documenting %d directories with %s, verified by a Go check%s\n", len(tasks), repodoc.AgentName, auditorClause(runner))
	record, err := runner.Run(ctx, tasks)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, summarizeDocumentRun(record, root))
	if record.Failed()+record.Unverified() > 0 {
		return fmt.Errorf("%w: %d failed, %d unverified of %d", errDocumentFailed, record.Failed(), record.Unverified(), len(record.Outcomes))
	}
	return nil
}

func auditorClause(runner repodoc.Runner) string {
	if runner.SkipAudit {
		return " only (--no-audit)"
	}
	return " and the " + repodoc.AuditorName
}

func summarizeDocumentRun(record repodoc.RunRecord, root string) string {
	line := fmt.Sprintf("%d verified, %d unverified, %d failed: candidates in %s",
		record.Count(repodoc.StatusVerified), record.Unverified(), record.Failed(), repodoc.RunDir(root, record.RunID))
	if record.Interrupted {
		line += " (interrupted)"
	}
	if record.StoppedBecause != "" {
		line += "\nstopped early, the rest are undocumented — run `loom document` again once this clears: " + record.StoppedBecause
	}
	if record.TotalUsage != nil {
		line += fmt.Sprintf(" — reported cost $%.4f", record.TotalUsage.CostUSD)
	}
	return line
}

// traceFileForDocument is where the run's trace goes: beside the run's own
// record, unless telemetry is off.
func traceFileForDocument(flags documentFlags, root, runID string) string {
	if flags.noTelemetry {
		return ""
	}
	return telemetry.TraceFileFor(repodoc.RunDir(root, runID))
}

// currentHashes maps each directory to its present fact-sheet hash.
func currentHashes(sheets []factsheet.FactSheet) map[string]string {
	hashes := make(map[string]string, len(sheets))
	for _, sheet := range sheets {
		hashes[sheet.Path] = sheet.InputHash
	}
	return hashes
}
