package cmd

// `loom memory policies` — the policy evidence corpus (roadmap L2.19).
//
// L2.19 halts on a judgement: do real runs show the evaluator deciding what
// a human would? Nothing here answers that. What it does is make the
// question answerable from data rather than from memory — the decisions were
// already being recorded and nothing could read them back.

import (
	"strings"

	"github.com/orieken/loom/internal/memory"
	"github.com/spf13/cobra"
)

func runMemoryPolicies(cmd *cobra.Command, _ []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	evidence, err := store.Policies(memoryArgs.limit)
	if err != nil {
		return err
	}
	if memoryArgs.asJSON {
		return emit(cmd, evidence)
	}
	printPolicies(cmd, evidence)
	return nil
}

func printPolicies(cmd *cobra.Command, evidence memory.PolicyEvidence) {
	if len(evidence.Decisions) == 0 {
		printNoPolicyEvidence(cmd)
		return
	}
	cmd.Printf("%-22s %-18s %-14s %-10s %s\n", "FEATURE", "GATE", "EFFECT", "AGREEMENT", "POLICIES")
	for _, decision := range evidence.Decisions {
		cmd.Printf("%-22s %-18s %-14s %-10s %s\n",
			decision.Feature, decision.Gate, decision.Effect,
			string(decision.Agreement), policySummary(decision))
	}
	printPolicyTotals(cmd, evidence)
}

// printNoPolicyEvidence says which of the two reasons applies, because they
// call for opposite actions: write a policy, or run something.
func printNoPolicyEvidence(cmd *cobra.Command) {
	cmd.Println("No policy decisions recorded.")
	cmd.Println("\nA decision is recorded per gate a run halts at, when a policy watches that gate.")
	cmd.Println("If you have written policies and still see nothing, check they watch a gate")
	cmd.Println("`loom run` actually halts at — confirm-design, confirm-security, confirm-ship,")
	cmd.Println("or confirm-unresolved-review. Four of the six shipped examples do not.")
}

// policySummary names each policy and marks the ones that could not see a
// fact they needed. A blind UNKNOWN and a considered UNKNOWN look identical
// without it, and they are not the same evidence.
func policySummary(decision memory.PolicyDecisionRow) string {
	if len(decision.Policies) == 0 {
		return "(none matched)"
	}
	parts := make([]string, 0, len(decision.Policies))
	for _, policy := range decision.Policies {
		parts = append(parts, policy.Name+"="+policy.Outcome+blindMark(policy))
	}
	return strings.Join(parts, " ")
}

func blindMark(policy memory.PolicyOutcomeRow) string {
	if !policy.IsBlind() {
		return ""
	}
	return " (blind to " + strings.Join(policy.Missing, ",") + ")"
}

// printPolicyTotals states the corpus size and what it does not establish.
// The stop condition is a judgement about evidence, and a count presented
// without its limits invites reading it as a score.
func printPolicyTotals(cmd *cobra.Command, evidence memory.PolicyEvidence) {
	cmd.Printf("\n%d decisions across %d runs — %d agreed, %d diverged, %d no-opinion.\n",
		len(evidence.Decisions), evidence.Runs,
		evidence.Agreed, evidence.Diverged, evidence.NoOpinion)
	cmd.Printf("Honoured: %d. The executor halts at every gate until roadmap L2.19 ships,\n", evidence.Honoured)
	cmd.Println("so a non-zero figure there is a defect rather than progress.")
	cmd.Println("\n\"Agreed\" means a policy asked to auto-approve and a human then approved the")
	cmd.Println("same gate. That is concurrence, not an independent second opinion: the human")
	cmd.Println("could see the decision. Treat it as the weakest kind of evidence it is.")
}
