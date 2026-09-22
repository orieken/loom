package cmd

// `loom memory agents` — what each agent's execution actually shows
// (roadmap L3.13).
//
// The numbers here are counted from records. The judgement about what they
// mean is not in this command, and `agent-scorecard` is where it belongs:
// the split this item exists to make is that a model writes the narrative
// and telemetry supplies the figures, rather than a model deriving figures
// from prose it also interprets.

import (
	"fmt"

	"github.com/orieken/loom/internal/memory"
	"github.com/spf13/cobra"
)

func runMemoryAgents(cmd *cobra.Command, _ []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	metrics, err := store.Agents()
	if err != nil {
		return err
	}
	if memoryArgs.asJSON {
		return emit(cmd, metrics)
	}
	printAgents(cmd, metrics)
	return nil
}

func printAgents(cmd *cobra.Command, metrics []memory.AgentMetrics) {
	if len(metrics) == 0 {
		cmd.Println("No agent runs recorded yet. Run `loom memory ingest` if deliveries are archived.")
		return
	}
	cmd.Printf("%-24s %6s %6s %8s %8s %8s %10s %10s\n",
		"AGENT", "RUNS", "STAGES", "RETRIED", "FAILED", "FIXES", "P50", "P95")
	for _, agent := range metrics {
		cmd.Printf("%-24s %6d %6d %8s %8s %8d %10s %10s\n",
			agent.Agent, agent.Runs, agent.Stages,
			rateText(agent.RetryRate()), rateText(agent.FailureRate()), agent.Corrections,
			durationText(agent.DurationP50Ms), durationText(agent.DurationP95Ms))
	}
	printAgentCaveats(cmd, metrics)
}

// rateText renders a rate, or an em dash when there was nothing to take a
// rate of. "0%" and "no stages ran" are different facts.
func rateText(rate *float64) string {
	if rate == nil {
		return "—"
	}
	return fmt.Sprintf("%.0f%%", *rate*100)
}

// durationText renders milliseconds as seconds, or an em dash when no stage
// of this agent finished. A stage that never finished has no duration, and
// printing 0s would read as the fastest agent in the table.
func durationText(ms *int64) string {
	if ms == nil {
		return "—"
	}
	return fmt.Sprintf("%.1fs", float64(*ms)/1000)
}

// printAgentCaveats states what the columns do not mean. Two of them are
// routinely misread, and a scorecard built on the misreading produces a
// confident wrong number.
func printAgentCaveats(cmd *cobra.Command, metrics []memory.AgentMetrics) {
	cmd.Println("\nFIXES counts human corrections, which were recorded, not adopted — evidence an")
	cmd.Println("agent's output needed fixing, not that anything shipped fixed.")
	cmd.Println("P50/P95 are nearest-rank over stages that finished; an em dash means none did.")
	if anyCostReported(metrics) {
		return
	}
	cmd.Println("No agent has reported usage, so cost and tokens are omitted: unmeasured here,")
	cmd.Println("not zero. Only `--provider claude` runs carry real figures.")
}

func anyCostReported(metrics []memory.AgentMetrics) bool {
	for _, agent := range metrics {
		if agent.CostReported {
			return true
		}
	}
	return false
}
