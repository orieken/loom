package cmd

import (
	"strings"
	"testing"

	"github.com/orieken/loom/internal/memory"
	"github.com/spf13/cobra"
)

func agentRows() []memory.AgentMetrics {
	p50 := int64(4000)
	return []memory.AgentMetrics{
		{Agent: "analyst", Runs: 2, Stages: 4, Retried: 1, Corrections: 2, DurationP50Ms: &p50},
		{Agent: "developer", Runs: 2, Stages: 2},
	}
}

func renderAgents(t *testing.T, rows []memory.AgentMetrics) string {
	t.Helper()
	return renderMemory(t, func(command *cobra.Command) { printAgents(command, rows) })
}

// An agent no stage of which finished has no latency. A 0 in that column
// would read as the fastest agent in the table.
func TestAnAgentWithNoFinishedStageShowsNoLatency(t *testing.T) {
	output := renderAgents(t, agentRows())

	line := lineFor(t, output, "developer")
	if strings.Contains(line, "0.0s") {
		t.Errorf("an unmeasured latency rendered as 0.0s:\n%s", line)
	}
	if !strings.Contains(line, "—") {
		t.Errorf("developer row does not mark its latency absent:\n%s", line)
	}
}

// L4.5's meaning has to survive into the table, or a reader takes the
// corrections column for defects fixed.
func TestTheAgentTableSaysCorrectionsWereNotAdopted(t *testing.T) {
	output := renderAgents(t, agentRows())

	if !strings.Contains(output, "recorded, not adopted") {
		t.Errorf("corrections column is presented without its meaning:\n%s", output)
	}
}

// A corpus of mock runs reports no usage. A cost table of zeroes that did
// not say so would read as a pipeline that costs nothing.
func TestAnUnreportedCostCorpusSaysItIsUnmeasured(t *testing.T) {
	output := renderAgents(t, agentRows())

	if !strings.Contains(output, "unmeasured here,") {
		t.Errorf("no-usage corpus does not distinguish unmeasured from zero:\n%s", output)
	}
}

func TestACorpusWithRealCostOmitsTheUnmeasuredNote(t *testing.T) {
	rows := agentRows()
	rows[0].CostReported = true
	rows[0].CostUSD = 1.25

	output := renderAgents(t, rows)

	if strings.Contains(output, "unmeasured here,") {
		t.Errorf("a corpus with real figures still claims to be unmeasured:\n%s", output)
	}
}

func lineFor(t *testing.T, output, agent string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, agent) {
			return line
		}
	}
	t.Fatalf("no row for %q in:\n%s", agent, output)
	return ""
}
