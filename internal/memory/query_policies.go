package memory

// The policy evidence query (roadmap L2.19).
//
// L2.19 will not be built until real runs show the evaluator deciding what a
// human would. That condition is a judgement about accumulated evidence, and
// until this query existed the evidence was write-only: decisions went into
// the store at ingest and nothing read them back, so "have we seen enough
// yet?" could only be answered by hand-reading run archives.
//
// This makes the condition checkable. It does not answer it, and nothing
// here honours a gate.

import (
	"database/sql"
	"fmt"
	"strings"
)

// Agreement is what a recorded decision and the human at the same gate did
// relative to each other.
type Agreement string

const (
	// AgreementAgreed means the policy asked to auto-approve and a human
	// went on to approve the same gate. It is weak evidence, deliberately
	// named rather than scored: the human was not blind to the policy's
	// decision, so this is concurrence, not an independent second opinion.
	AgreementAgreed Agreement = "agreed"
	// AgreementDiverged means the policy asked to auto-approve and no valid
	// approval followed — the human declined, or the run ended there.
	AgreementDiverged Agreement = "diverged"
	// AgreementNoOpinion covers every decision that asked for nothing an
	// approval could confirm: UNKNOWN, require-human, or no matching policy.
	AgreementNoOpinion Agreement = "no-opinion"
)

// PolicyDecisionRow is one recorded decision, with the human's action at
// that same gate beside it.
type PolicyDecisionRow struct {
	RunID    string   `json:"runId"`
	Feature  string   `json:"feature"`
	Gate     string   `json:"gate"`
	Effect   string   `json:"effect"`
	Honoured bool     `json:"honoured"`
	Conflict []string `json:"conflict,omitempty"`
	At       string   `json:"at,omitempty"`
	// ApprovalMethod is how the human approved this gate, empty when they
	// did not. Invalidated marks an approval reset by a later edit.
	ApprovalMethod string             `json:"approvalMethod,omitempty"`
	Invalidated    bool               `json:"invalidated,omitempty"`
	Agreement      Agreement          `json:"agreement"`
	Policies       []PolicyOutcomeRow `json:"policies"`
}

// PolicyOutcomeRow is one policy's result within a decision.
type PolicyOutcomeRow struct {
	Name    string `json:"name"`
	Action  string `json:"action"`
	Outcome string `json:"outcome"`
	// Missing names the condition fields this policy could not answer. It is
	// the field that separates "looked and did not match" from "was blind",
	// which the decision's effect alone cannot express.
	Missing []string `json:"missing,omitempty"`
	Source  string   `json:"source,omitempty"`
}

// IsBlind reports a policy that could not see at least one fact it needed.
func (p PolicyOutcomeRow) IsBlind() bool { return len(p.Missing) > 0 }

// PolicyEvidence summarises the corpus L2.19's stop condition is about.
type PolicyEvidence struct {
	Decisions []PolicyDecisionRow `json:"decisions"`
	Runs      int                 `json:"runs"`
	Agreed    int                 `json:"agreed"`
	Diverged  int                 `json:"diverged"`
	NoOpinion int                 `json:"noOpinion"`
	// Honoured counts decisions the executor acted on. It is expected to be
	// zero until L2.19 ships; a non-zero here in a build that has not
	// shipped it is a defect, not a milestone.
	Honoured int `json:"honoured"`
}

// Policies returns every recorded policy decision, newest run first.
func (s *Store) Policies(limit int) (PolicyEvidence, error) {
	rows, err := s.db.Query(`
		SELECT p.run_id, r.feature, p.gate, p.effect, p.honoured,
		       COALESCE(p.conflict, ''), COALESCE(p.at, ''),
		       COALESCE(a.method, ''), COALESCE(a.invalidated, 0)
		FROM policy_decisions p
		JOIN runs r ON r.run_id = p.run_id
		LEFT JOIN gate_approvals a ON a.run_id = p.run_id AND a.gate = p.gate
		ORDER BY r.started_at DESC, p.seq ASC LIMIT ?`, limit)
	if err != nil {
		return PolicyEvidence{}, fmt.Errorf("query policy decisions: %w", err)
	}
	decisions, err := scanPolicyDecisions(rows)
	if err != nil {
		return PolicyEvidence{}, err
	}
	if err := s.attachOutcomes(decisions); err != nil {
		return PolicyEvidence{}, err
	}
	return summarise(decisions), nil
}

func scanPolicyDecisions(rows *sql.Rows) ([]PolicyDecisionRow, error) {
	defer func() { _ = rows.Close() }()
	decisions := make([]PolicyDecisionRow, 0)
	for rows.Next() {
		var row PolicyDecisionRow
		var conflict string
		var invalidated int
		if err := rows.Scan(&row.RunID, &row.Feature, &row.Gate, &row.Effect, &row.Honoured,
			&conflict, &row.At, &row.ApprovalMethod, &invalidated); err != nil {
			return nil, fmt.Errorf("scan policy decision: %w", err)
		}
		row.Conflict = split(conflict)
		row.Invalidated = invalidated != 0
		row.Agreement = agreementOf(row)
		decisions = append(decisions, row)
	}
	return decisions, rows.Err()
}

// agreementOf reads the pair without inflating it. Only an auto-approve
// decision makes a claim an approval can confirm or contradict; everything
// else is recorded as no-opinion rather than counted as agreement by
// default, which would make the corpus look stronger the less it said.
func agreementOf(row PolicyDecisionRow) Agreement {
	if row.Effect != "auto-approve" {
		return AgreementNoOpinion
	}
	if row.ApprovalMethod != "" && !row.Invalidated {
		return AgreementAgreed
	}
	return AgreementDiverged
}

func (s *Store) attachOutcomes(decisions []PolicyDecisionRow) error {
	for index := range decisions {
		outcomes, err := s.outcomesFor(decisions[index].RunID, decisions[index].Gate)
		if err != nil {
			return err
		}
		decisions[index].Policies = outcomes
	}
	return nil
}

func (s *Store) outcomesFor(runID, gate string) ([]PolicyOutcomeRow, error) {
	rows, err := s.db.Query(`
		SELECT o.name, o.action, o.outcome, COALESCE(o.missing, ''), COALESCE(o.source, '')
		FROM policy_outcomes o
		JOIN policy_decisions p ON p.run_id = o.run_id AND p.seq = o.seq
		WHERE o.run_id = ? AND p.gate = ? ORDER BY o.seq ASC, o.idx ASC`, runID, gate)
	if err != nil {
		return nil, fmt.Errorf("query policy outcomes: %w", err)
	}
	return scanPolicyOutcomes(rows)
}

func scanPolicyOutcomes(rows *sql.Rows) ([]PolicyOutcomeRow, error) {
	defer func() { _ = rows.Close() }()
	outcomes := make([]PolicyOutcomeRow, 0)
	for rows.Next() {
		var row PolicyOutcomeRow
		var missing string
		if err := rows.Scan(&row.Name, &row.Action, &row.Outcome, &missing, &row.Source); err != nil {
			return nil, fmt.Errorf("scan policy outcome: %w", err)
		}
		row.Missing = split(missing)
		outcomes = append(outcomes, row)
	}
	return outcomes, rows.Err()
}

func summarise(decisions []PolicyDecisionRow) PolicyEvidence {
	evidence := PolicyEvidence{Decisions: decisions}
	runs := map[string]struct{}{}
	for _, decision := range decisions {
		runs[decision.RunID] = struct{}{}
		evidence.count(decision)
	}
	evidence.Runs = len(runs)
	return evidence
}

func (e *PolicyEvidence) count(decision PolicyDecisionRow) {
	if decision.Honoured {
		e.Honoured++
	}
	switch decision.Agreement {
	case AgreementAgreed:
		e.Agreed++
	case AgreementDiverged:
		e.Diverged++
	case AgreementNoOpinion:
		e.NoOpinion++
	}
}

func split(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}
