package cmd

// Session capture in `loom memory`: ingest of the day files and the Session
// Corrections section of `loom memory corrections`.

import (
	"context"
	"fmt"
	"time"

	"github.com/orieken/loom/internal/capture"
	"github.com/orieken/loom/internal/capture/localfs"
	"github.com/orieken/loom/internal/memory"
	"github.com/spf13/cobra"
)

// sessionLayout is the capture layout of the project in the working
// directory, the same root that openStore uses.
func sessionLayout() capture.Layout { return capture.NewLayout(".") }

func loadCaptureConfig() (capture.Config, error) {
	return localfs.NewConfigFile(sessionLayout().ConfigPath()).Load()
}

// ingestSessions rebuilds the session table and prints the stable counts.
func ingestSessions(cmd *cobra.Command, store *memory.Store) error {
	cfg, err := loadCaptureConfig()
	if err != nil {
		return err
	}
	result, err := store.IngestSessions(context.Background(), newDaySessionSource(sessionLayout()), cfg)
	if err != nil {
		return err
	}
	printSessionIngest(cmd, result)
	return nil
}

func printSessionIngest(cmd *cobra.Command, result memory.SessionIngest) {
	cmd.Printf("%d session corrections ingested from day files.\n", result.Corrections)
	cmd.Printf("torn lines skipped: %d\n", result.Stats.TornLines)
	cmd.Printf("unknown-version lines skipped: %d\n", result.Stats.UnknownVersionLines)
	cmd.Printf("clock anomalies: %d\n", result.Stats.ClockAnomalies)
	cmd.Printf("unattributed corrections (not counted): %d\n", result.Unattributed)
}

// sessionFilterFromFlags reads --since, --tool and --glob.
func sessionFilterFromFlags() (memory.SessionFilter, error) {
	filter := memory.SessionFilter{Tool: memoryArgs.tool, Glob: memoryArgs.glob}
	if memoryArgs.since == "" {
		return filter, nil
	}
	since, err := parseSince(memoryArgs.since)
	if err != nil {
		return memory.SessionFilter{}, err
	}
	filter.Since = since
	return filter, nil
}

// parseSince accepts a full RFC 3339 time or a plain UTC date.
func parseSince(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02"} {
		if at, err := time.Parse(layout, value); err == nil {
			return at, nil
		}
	}
	return time.Time{}, fmt.Errorf("--since %q is not a date (2006-01-02) or an RFC 3339 time", value)
}

func hasSessionFilterFlags() bool {
	return memoryArgs.since != "" || memoryArgs.glob != "" || memoryArgs.tool != ""
}

// buildSessionReport reads the filters and the capture configuration, then
// asks the store for the report.
func buildSessionReport(store *memory.Store) (memory.SessionReport, capture.Config, error) {
	filter, err := sessionFilterFromFlags()
	if err != nil {
		return memory.SessionReport{}, capture.Config{}, err
	}
	cfg, err := loadCaptureConfig()
	if err != nil {
		return memory.SessionReport{}, capture.Config{}, err
	}
	report, err := store.SessionReport(context.Background(), newDaySessionSource(sessionLayout()), cfg, filter)
	return report, cfg, err
}

// printSessionCorrections prints the Session Corrections section. It is a
// separate section: gate-level corrections and session corrections never
// share a total.
func printSessionCorrections(cmd *cobra.Command, report memory.SessionReport, cfg capture.Config) {
	cmd.Println("\nSession Corrections")
	switch report.State {
	case memory.SessionStateNoData:
		cmd.Println(memory.SessionNoDataMessage)
		return
	case memory.SessionStateNeedsIngest:
		cmd.Println(memory.SessionNeedsIngestMessage)
		return
	}
	cmd.Printf("Method: %s\n", report.Method)
	cmd.Printf("Newest recorded event: %s\n", report.NewestEventAt)
	cmd.Println(weightsLine(cfg.Weights))
	cmd.Println("Filters --since, --glob and --tool apply to Session Corrections only.")
	printSessionRows(cmd, report)
}

func weightsLine(weights capture.Weights) string {
	return fmt.Sprintf("Weights (minutes per correction, estimate): hunk-edit %d, revert %d, denied-call %d, corrective-prompt %d",
		weights.HunkEdit, weights.Revert, weights.DeniedCall, weights.CorrectivePrompt)
}

func printSessionRows(cmd *cobra.Command, report memory.SessionReport) {
	if len(report.Kinds) == 0 {
		cmd.Println("No session corrections match.")
	} else {
		cmd.Printf("%-20s %8s %12s\n", "KIND", "COUNT", "EST. MINUTES")
		for _, row := range report.Kinds {
			cmd.Printf("%-20s %8d %12d\n", row.Kind, row.Count, row.EstimatedMinutes)
		}
		cmd.Printf("Total estimated minutes: %d\n", report.TotalEstimatedMinutes)
	}
	cmd.Printf("unattributed (not counted): %d\n", report.Unattributed)
}
