package cmd

// The policy evidence table exists to be read by someone deciding whether
// roadmap L2.19's stop condition is met. Every assertion here is about the
// output not overstating what it holds.

import (
	"strings"
	"testing"

	"github.com/orieken/loom/internal/memory"
	"github.com/spf13/cobra"
)

func samplePolicyEvidence() memory.PolicyEvidence {
	return memory.PolicyEvidence{
		Decisions: []memory.PolicyDecisionRow{{
			RunID: "abc", Feature: "user-auth", Gate: "confirm-security",
			Effect: "auto-approve", Agreement: memory.AgreementAgreed,
			ApprovalMethod: "tty",
			Policies: []memory.PolicyOutcomeRow{
				{Name: "auto-approve-refactor", Action: "auto-approve", Outcome: "TRUE"},
				{Name: "require-human-on-criticals", Action: "require-human", Outcome: "UNKNOWN",
					Missing: []string{"dryRunPass"}},
			},
		}},
		Runs: 1, Agreed: 1,
	}
}

func renderPolicies(t *testing.T, evidence memory.PolicyEvidence) string {
	t.Helper()
	return renderMemory(t, func(command *cobra.Command) { printPolicies(command, evidence) })
}

// A policy that could not see a fact it needed is not the same evidence as
// one that looked and did not match. The table has to say which.
func TestThePolicyTableMarksABlindPolicy(t *testing.T) {
	output := renderPolicies(t, samplePolicyEvidence())

	if !strings.Contains(output, "blind to dryRunPass") {
		t.Errorf("table does not mark the policy that could not see its fact:\n%s", output)
	}
	if !strings.Contains(output, "auto-approve-refactor=TRUE") {
		t.Errorf("table missing the deciding policy's outcome:\n%s", output)
	}
}

// "Agreed" is concurrence, not an independent second opinion — the human
// could see the decision. A reader counting agreements as validation would
// draw exactly the conclusion L2.16 stopped short to avoid.
func TestThePolicyTableQualifiesWhatAgreementMeans(t *testing.T) {
	output := renderPolicies(t, samplePolicyEvidence())

	if !strings.Contains(output, "concurrence, not an independent second opinion") {
		t.Errorf("agreement is presented without its limit:\n%s", output)
	}
}

// A non-zero honoured count in a build where L2.19 has not shipped means a
// gate was skipped. The output must not let that read as progress.
func TestThePolicyTableSaysHonouredShouldBeZero(t *testing.T) {
	output := renderPolicies(t, samplePolicyEvidence())

	if !strings.Contains(output, "defect rather than progress") {
		t.Errorf("honoured count is presented without its meaning:\n%s", output)
	}
}

// An empty corpus has two causes calling for opposite actions: no policies
// written, or policies watching gates the executor never halts at.
func TestAnEmptyPolicyTableNamesTheGatesThatWork(t *testing.T) {
	output := renderPolicies(t, memory.PolicyEvidence{})

	if !strings.Contains(output, "No policy decisions recorded") {
		t.Errorf("empty output does not say so:\n%s", output)
	}
	if !strings.Contains(output, "confirm-security") {
		t.Errorf("empty output does not name a gate that actually halts:\n%s", output)
	}
}
