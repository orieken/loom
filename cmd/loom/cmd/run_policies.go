package cmd

// Policy wiring for `loom run` (roadmap L2.16).
//
// Policies are loaded, evaluated at each gate, and reported. Nothing acts
// on them: the run halts at every gate exactly as it did before. What
// changes is that a human at the barrier is told what a policy would have
// decided, and the decision is recorded — which is the audit trail
// approval-gates.md has asserted since v3.3 without ever producing one.

import (
	"fmt"
	"strings"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/policy"
	"github.com/spf13/cobra"
)

// loadPolicies validates delivery-policy.yaml, then reads the project's
// policy directory. Both failures are fatal on purpose: a policy that does
// not parse is not a policy that silently does nothing, it is a file whose
// author believes it is working — and since ADR-010 the same holds for a
// delivery-policy.yaml still setting `mode` or `autoProceed*`.
func loadPolicies(cmd *cobra.Command, dir string) ([]policy.Policy, error) {
	config, err := policy.LoadConfig(policy.ConfigFile)
	if err != nil {
		return nil, err
	}
	if !config.PoliciesEvaluated() {
		cmd.Println(policy.DisabledNotice(policy.ConfigFile))
		return nil, nil
	}
	policies, err := policy.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("policies in %s: %w", dir, err)
	}
	return policies, nil
}

// reportPolicyDecision tells a human what the policies said, and is
// explicit that nothing was skipped because of them.
func reportPolicyDecision(cmd *cobra.Command, decision policy.Decision) {
	cmd.Printf("Policy at gate %q: %s\n", decision.Gate, decision.Summary())
	for _, result := range decision.Policies {
		cmd.Printf("  %-32s %-8s %s%s\n", result.Name, result.Outcome, result.Action, missingNote(result))
	}
	if len(decision.Conflict) > 0 {
		cmd.Printf("  conflict between %s — require-human wins\n", strings.Join(decision.Conflict, ", "))
	}
}

// missingNote names the facts a policy asked about that this run cannot
// answer. Without it "no match" is indistinguishable from "asked about
// something nobody measures", and five of the nine declared fields are
// currently in the second category.
func missingNote(result policy.Result) string {
	if len(result.Missing) == 0 {
		return ""
	}
	names := make([]string, 0, len(result.Missing))
	for _, field := range result.Missing {
		names = append(names, string(field))
	}
	return "  (unknown: " + strings.Join(names, ", ") + ")"
}

// runDryRunPolicies evaluates policies against a completed run's recorded
// state and prints what each gate would have decided, mutating nothing.
func runDryRunPolicies(cmd *cobra.Command, store *orchestrator.StateStore, policies []policy.Policy) error {
	state, err := store.Load()
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("no run state at %s — dry-run evaluates policies against a run that already happened", store.Path())
	}
	if len(policies) == 0 {
		cmd.Println("No policies found — nothing to dry-run.")
		return nil
	}
	printDryRun(cmd, store, state, policies)
	return nil
}

func printDryRun(cmd *cobra.Command, store *orchestrator.StateStore, state *orchestrator.RunState, policies []policy.Policy) {
	cmd.Printf("Dry-run against %s (%d loaded, no state is modified)\n", state.FeatureName, len(policies))
	for _, gate := range policy.EligibleGates() {
		decision := policy.Decide(policies, orchestrator.PolicyContextFor(store, state, gate))
		if len(decision.Policies) == 0 {
			continue
		}
		reportPolicyDecision(cmd, decision)
	}
}
