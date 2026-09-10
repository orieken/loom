package cmd

// Reporting a suspicious file claim during a run (roadmap L2.24).

import "github.com/spf13/cobra"

// reportClaimWarning surfaces a claim that is suspicious rather than false —
// a path a stage listed that exists but did not change while it ran. It
// prints rather than fails: a stage can legitimately name a file it read and
// left alone, so this is a signal for a human, not a barrier.
func reportClaimWarning(cmd *cobra.Command) func(error) {
	return func(err error) { cmd.PrintErrf("warning: %v\n", err) }
}
