package memory_test

// Roadmap L2.19's evidence corpus.
//
// The decisions were being recorded and nothing read them back, so "have we
// seen enough for the stop condition?" could only be answered by hand. Worse,
// ingest was lossy in exactly the dimension the question needs: a policy's
// own outcome and the facts it could not see were dropped, and the store held
// no record of what the human did at the same gate.

import (
	"testing"
	"time"

	"github.com/orieken/loom/internal/memory"
	"github.com/orieken/loom/internal/orchestrator"
)

// policyRun is a run whose gate drew two policies: one that decided on facts
// it had, one blind to a fact it needed. A human then approved the gate.
func policyRun() (*orchestrator.RunState, []orchestrator.Event) {
	state, events := fixtureRun()
	at := state.StartedAt.Add(time.Minute)
	state.PolicyDecisions = []orchestrator.PolicyRecord{{
		Gate: "confirm-security", Effect: "auto-approve", At: at, Honoured: false,
		Policies: []orchestrator.PolicyOutcome{
			{Name: "auto-approve-refactor", Action: "auto-approve", Outcome: "TRUE", Source: "run-state"},
			{Name: "require-human-on-criticals", Action: "require-human", Outcome: "UNKNOWN",
				Missing: []string{"dryRunPass", "fitnessFunction.allPass"}},
		},
	}}
	state.Approvals = map[string]orchestrator.Approval{
		"confirm-security": {ApprovedAt: at.Add(time.Minute), Method: orchestrator.ApprovalMethodTTY, Approver: "human"},
	}
	return state, events
}

func ingestPolicyRun(t *testing.T) *memory.Store {
	t.Helper()
	store := openStore(t)
	state, events := policyRun()
	if _, err := store.Ingest(state, events); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	return store
}

func onlyDecision(t *testing.T, store *memory.Store) memory.PolicyDecisionRow {
	t.Helper()
	evidence, err := store.Policies(100)
	if err != nil {
		t.Fatalf("Policies: %v", err)
	}
	if len(evidence.Decisions) != 1 {
		t.Fatalf("got %d decisions, want 1", len(evidence.Decisions))
	}
	return evidence.Decisions[0]
}

// TestEachPolicysOwnOutcomeSurvivesIngest is the loss that mattered most. A
// decision's effect cannot distinguish a policy that looked and did not match
// from one that never saw the facts, and only the second is a reason to fix
// the plumbing rather than the policy.
func TestEachPolicysOwnOutcomeSurvivesIngest(t *testing.T) {
	decision := onlyDecision(t, ingestPolicyRun(t))

	if len(decision.Policies) != 2 {
		t.Fatalf("got %d policy outcomes, want 2 — the per-policy detail was dropped", len(decision.Policies))
	}
	if decision.Policies[0].Name != "auto-approve-refactor" || decision.Policies[0].Outcome != "TRUE" {
		t.Errorf("first outcome = %+v, want the deciding policy", decision.Policies[0])
	}
	blind := decision.Policies[1]
	if !blind.IsBlind() {
		t.Error("a policy that reported two missing fields is not marked blind")
	}
	if len(blind.Missing) != 2 {
		t.Errorf("missing fields = %v, want both recorded", blind.Missing)
	}
}

// TestTheHumansActionAtTheGateIsRecorded is the comparison's other half. The
// store held no record of it at all, so a decision could never be read
// against what actually happened.
func TestTheHumansActionAtTheGateIsRecorded(t *testing.T) {
	decision := onlyDecision(t, ingestPolicyRun(t))

	if decision.ApprovalMethod != string(orchestrator.ApprovalMethodTTY) {
		t.Errorf("approval method = %q, want the human's channel", decision.ApprovalMethod)
	}
	if decision.Agreement != memory.AgreementAgreed {
		t.Errorf("agreement = %q, want agreed — auto-approve followed by an approval", decision.Agreement)
	}
}

// TestAnAutoApproveNobodyApprovedIsDivergence guards the direction that
// would otherwise flatter the corpus: absence of an approval must not read
// as agreement.
func TestAnAutoApproveNobodyApprovedIsDivergence(t *testing.T) {
	store := openStore(t)
	state, events := policyRun()
	state.Approvals = map[string]orchestrator.Approval{}
	if _, err := store.Ingest(state, events); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	if got := onlyDecision(t, store).Agreement; got != memory.AgreementDiverged {
		t.Errorf("agreement = %q, want diverged — nobody approved", got)
	}
}

// TestAnInvalidatedApprovalIsNotAgreement: an approval reset by a later edit
// (L2.14) is not a human standing behind the state the policy decided on.
func TestAnInvalidatedApprovalIsNotAgreement(t *testing.T) {
	store := openStore(t)
	state, events := policyRun()
	reset := state.StartedAt.Add(2 * time.Hour)
	approval := state.Approvals["confirm-security"]
	approval.InvalidatedAt = &reset
	approval.InvalidatedBy = "analysis.md"
	state.Approvals["confirm-security"] = approval
	if _, err := store.Ingest(state, events); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	if got := onlyDecision(t, store).Agreement; got != memory.AgreementDiverged {
		t.Errorf("agreement = %q, want diverged — the approval was invalidated", got)
	}
}

// TestARequireHumanDecisionClaimsNothing keeps the counts honest. Only an
// auto-approve decision makes a claim an approval can confirm; counting the
// rest as agreement would make the corpus look stronger the less it said.
func TestARequireHumanDecisionClaimsNothing(t *testing.T) {
	store := openStore(t)
	state, events := fixtureRun()
	state.Approvals = map[string]orchestrator.Approval{
		"confirm-design": {ApprovedAt: state.StartedAt, Method: orchestrator.ApprovalMethodTTY},
	}
	if _, err := store.Ingest(state, events); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	evidence, err := store.Policies(100)
	if err != nil {
		t.Fatalf("Policies: %v", err)
	}
	if evidence.Agreed != 0 {
		t.Errorf("agreed = %d, want 0 — require-human asked for nothing an approval confirms", evidence.Agreed)
	}
	if evidence.NoOpinion != 1 {
		t.Errorf("noOpinion = %d, want 1", evidence.NoOpinion)
	}
}

// TestNothingIsHonouredYet is the invariant this build must keep: L2.19 has
// not shipped, so the executor halts at every gate. A non-zero honoured
// count reaching the corpus means something skipped a barrier.
func TestNothingIsHonouredYet(t *testing.T) {
	evidence, err := ingestPolicyRun(t).Policies(100)
	if err != nil {
		t.Fatalf("Policies: %v", err)
	}
	if evidence.Honoured != 0 {
		t.Errorf("honoured = %d, want 0 — no gate may be skipped before L2.19 ships", evidence.Honoured)
	}
}

// TestAnEmptyCorpusIsNotAnError: no policies written yet is the expected
// state, not a failure.
func TestAnEmptyCorpusIsNotAnError(t *testing.T) {
	evidence, err := openStore(t).Policies(100)
	if err != nil {
		t.Fatalf("Policies on an empty store: %v", err)
	}
	if len(evidence.Decisions) != 0 || evidence.Runs != 0 {
		t.Errorf("empty store reported %+v", evidence)
	}
}
