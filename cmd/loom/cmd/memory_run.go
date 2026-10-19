package cmd

// The `loom memory` subcommand bodies.

import (
	"fmt"

	"github.com/orieken/loom/internal/memory"
	"github.com/spf13/cobra"
)

func runMemoryIngest(cmd *cobra.Command, _ []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	sources, err := collectIngestSources(newDaySessionSource(sessionLayout()))
	if err != nil {
		return err
	}
	if sources.isEmpty() {
		cmd.Printf("No archived runs found under %s — nothing to ingest.\n", featuresDir())
		return nil
	}
	return ingestSources(cmd, store, sources)
}

// ingestSourceSet holds the two independent things that `loom memory ingest`
// can rebuild from: archived runs and session day files.
type ingestSourceSet struct {
	runDirs     []string
	hasDayFiles bool
}

func (s ingestSourceSet) isEmpty() bool { return len(s.runDirs) == 0 && !s.hasDayFiles }

// collectIngestSources is either the one directory asked for, or every
// archived run plus the day files. Rebuilding from the archive is the
// reason those records are committed. A project with day files and no
// archived run still has something to ingest. With --dir, sessions are
// skipped.
func collectIngestSources(sessions memory.SessionSource) (ingestSourceSet, error) {
	if memoryArgs.dir != "" {
		return ingestSourceSet{runDirs: []string{memoryArgs.dir}}, nil
	}
	dirs, err := memory.ArchivedRunDirs(featuresDir())
	if err != nil {
		return ingestSourceSet{}, err
	}
	hasDayFiles, err := sessions.HasDayFiles()
	if err != nil {
		return ingestSourceSet{}, fmt.Errorf("look for session day files: %w", err)
	}
	return ingestSourceSet{runDirs: dirs, hasDayFiles: hasDayFiles}, nil
}

func ingestSources(cmd *cobra.Command, store *memory.Store, sources ingestSourceSet) error {
	if len(sources.runDirs) > 0 {
		if err := ingestAll(cmd, store, sources.runDirs); err != nil {
			return err
		}
	}
	if sources.hasDayFiles {
		return ingestSessions(cmd, store)
	}
	return nil
}

// ingestAll reports each failure and continues. One unreadable archive
// should not stop the other twenty from being imported.
func ingestAll(cmd *cobra.Command, store *memory.Store, dirs []string) error {
	ingested := 0
	for _, dir := range dirs {
		records, err := memory.ReadRecords(dir)
		if err != nil {
			cmd.PrintErrf("skipped %s: %v\n", dir, err)
			continue
		}
		run, err := store.Ingest(records.State, records.Events)
		if err != nil {
			cmd.PrintErrf("skipped %s: %v\n", dir, err)
			continue
		}
		ingested++
		cmd.Printf("ingested %-28s %d stages, %d events%s\n", run.Feature, run.Stages, run.Events, completeSuffix(run))
	}
	cmd.Printf("%d of %d archived runs ingested.\n", ingested, len(dirs))
	return nil
}

func completeSuffix(run memory.Run) string {
	if run.Complete {
		return ""
	}
	return " (halted)"
}

func runMemoryRuns(cmd *cobra.Command, _ []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	runs, err := store.Runs(memoryArgs.limit)
	if err != nil {
		return err
	}
	if memoryArgs.asJSON {
		return emit(cmd, runs)
	}
	printRuns(cmd, store, runs)
	return nil
}

func printRuns(cmd *cobra.Command, store *memory.Store, runs []memory.RunSummary) {
	if len(runs) == 0 {
		cmd.Println("No runs recorded yet.")
		return
	}
	cmd.Printf("%-22s %-20s %-10s %10s %10s %6s\n", "FEATURE", "STARTED", "STATE", "TOKENS", "COST", "FIXES")
	for _, run := range runs {
		cmd.Printf("%-22s %-20s %-10s %10d %10s %6d\n",
			run.Feature, shortTime(run.StartedAt), runState(run),
			run.TotalTokens(), costText(run.CostUSD), run.Corrections)
	}
	noteUnreportedCost(cmd, store)
}

// noteUnreportedCost keeps a column of zeroes from reading as "these runs
// were free". A provider that reported nothing is a different fact from one
// that reported nothing spent.
func noteUnreportedCost(cmd *cobra.Command, store *memory.Store) {
	reported, err := store.CostReported()
	if err != nil || reported {
		return
	}
	cmd.Println("\nNo run has reported usage. Cost is unmeasured here, not zero — " +
		"the mock provider reports nothing, and only `--provider claude` runs carry real figures.")
}

func runState(run memory.RunSummary) string {
	if run.Complete {
		return "complete"
	}
	if run.WaitingGate != "" {
		return "at gate"
	}
	return "unfinished"
}

func costText(cost float64) string {
	if cost == 0 {
		return "—"
	}
	return fmt.Sprintf("$%.4f", cost)
}

func shortTime(timestamp string) string {
	if len(timestamp) < 16 {
		return timestamp
	}
	return timestamp[:16]
}

func runMemoryRetries(cmd *cobra.Command, _ []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	rows, err := store.Retries(memoryArgs.agent, memoryArgs.moreThan)
	if err != nil {
		return err
	}
	if memoryArgs.asJSON {
		return emit(cmd, rows)
	}
	printRetries(cmd, rows)
	return nil
}

// printRetries states the threshold it applied. "Retried more than twice"
// has two defensible readings — three attempts, or three retries after the
// first — so the header says which one produced these rows rather than
// leaving a reader to infer it.
func printRetries(cmd *cobra.Command, rows []memory.RetryRow) {
	cmd.Printf("Stages with more than %d iterations (an iteration count of %d means %d attempts):\n\n",
		memoryArgs.moreThan, memoryArgs.moreThan+1, memoryArgs.moreThan+1)
	if len(rows) == 0 {
		cmd.Println("None recorded.")
		return
	}
	cmd.Printf("%-22s %-20s %-20s %10s\n", "FEATURE", "STARTED", "STAGE", "ATTEMPTS")
	for _, row := range rows {
		cmd.Printf("%-22s %-20s %-20s %10d\n", row.Feature, shortTime(row.StartedAt), row.Stage, row.Iterations)
	}
}

func runMemoryCorrections(cmd *cobra.Command, _ []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	if memoryArgs.asJSON {
		return emitCorrections(cmd, store)
	}
	return printCorrectionReport(cmd, store)
}

// emitCorrections keeps the bare-array shape that the agent-scorecard skill
// reads. Session Corrections have their own object, behind --sessions.
func emitCorrections(cmd *cobra.Command, store *memory.Store) error {
	if memoryArgs.sessions {
		report, _, err := buildSessionReport(store)
		if err != nil {
			return err
		}
		return emit(cmd, report)
	}
	if hasSessionFilterFlags() {
		return fmt.Errorf("--since, --glob and --tool filter Session Corrections: add --sessions to use them with --json")
	}
	rows, err := store.Corrections()
	if err != nil {
		return err
	}
	return emit(cmd, rows)
}

func printCorrectionReport(cmd *cobra.Command, store *memory.Store) error {
	rows, err := store.Corrections()
	if err != nil {
		return err
	}
	printAgentCorrections(cmd, rows)
	report, cfg, err := buildSessionReport(store)
	if err != nil {
		return err
	}
	printSessionCorrections(cmd, report, cfg)
	return nil
}

// printAgentCorrections is careful about what a correction means: epic 85
// records the edit and the pipeline does not adopt it, so a high count is
// evidence an agent needed fixing, not that anything was fixed.
func printAgentCorrections(cmd *cobra.Command, rows []memory.AgentCorrections) {
	if len(rows) == 0 {
		cmd.Println("No human corrections recorded.")
		return
	}
	cmd.Printf("%-24s %8s %8s %8s %8s\n", "AGENT", "FIXES", "RUNS", "+LINES", "-LINES")
	for _, row := range rows {
		cmd.Printf("%-24s %8d %8d %8d %8d\n", row.Agent, row.Corrections, row.Runs, row.LinesAdded, row.LinesCut)
	}
	cmd.Println("\nA correction was recorded, not adopted: the pipeline did not take the human's text " +
		"(roadmap L4.5). A high count means that agent's output needed fixing.")
}
