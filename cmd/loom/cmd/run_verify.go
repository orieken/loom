package cmd

// Wiring the measurement verifier into a run (roadmap L2.24).

import (
	"context"

	"github.com/orieken/loom/internal/verify"
	"github.com/spf13/cobra"
)

// testVerifier reproduces a stage's claim that the suite passes by running
// the command the PROJECT declares — never one the stage named.
type testVerifier struct {
	config verify.Config
}

func (v testVerifier) VerifyTests(ctx context.Context, root string) (bool, bool, string) {
	outcome := verify.Tests(ctx, root, v.config)
	return outcome.Verified, outcome.Unverifiable, outcome.Reason
}

// newTestVerifier loads the project's verification config. A project that
// has declared no test command still gets a verifier: it reports every claim
// as unverified, which is the honest state and is louder than silence.
func newTestVerifier(cmd *cobra.Command, root string) testVerifier {
	config, err := verify.LoadConfig(root)
	if err != nil {
		cmd.PrintErrf("warning: %v — test claims will be recorded unverified\n", err)
	}
	return testVerifier{config: config}
}

// reportClaimWarning surfaces a claim that is unverified or suspicious. It
// prints rather than fails: an unverified claim is the absence of evidence,
// and treating that as a failure would break every project that has not
// configured a test command.
func reportClaimWarning(cmd *cobra.Command) func(error) {
	return func(err error) { cmd.PrintErrf("warning: %v\n", err) }
}
