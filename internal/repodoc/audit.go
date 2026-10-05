package repodoc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/state"
)

// AuditorName is the read-only agent that judges whether cited facts support
// the claims made on them.
const AuditorName = "repo-doc-auditor"

// auditStageID names the auditor's stage for a directory.
func auditStageID(dir string) string { return "repo-doc-audit-" + dirKey(dir) }

// auditInput is what the auditor is given: the sheet, the children and the
// candidate. It is told nothing about earlier rounds, so each audit is fresh.
func auditInput(task Task, known summaries, vocabulary Vocabulary, candidate state.RepoDocState) (map[string][]byte, error) {
	input, err := buildInput(task, known, vocabulary, nil)
	if err != nil {
		return nil, err
	}
	raw, err := json.MarshalIndent(candidate, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode candidate %s: %w", task.Path, err)
	}
	input[inputCandidate] = raw
	delete(input, inputAllowedTags)
	return input, nil
}

// audit runs the auditor on a candidate that already passed the verifier.
// Only blocking findings become violations: a suggestion does not cost a round.
func (r Runner) audit(ctx context.Context, task Task, known summaries, runDir string, candidate state.RepoDocState) ([]Violation, string, *orchestrator.Usage, error) {
	input, err := auditInput(task, known, r.Vocabulary, candidate)
	if err != nil {
		return nil, "", nil, err
	}
	stage := orchestrator.Stage{ID: auditStageID(task.Path), Agent: AuditorName, StateKind: string(state.KindRepoDocAudit), Timeout: r.timeout()}
	output, err := r.invoke(ctx, stage, task, runDir, input)
	if err != nil {
		return nil, "", output.Usage, err
	}
	verdict, err := acceptAudit(task, output.Payload)
	if err != nil {
		return nil, "", output.Usage, err
	}
	return blockingViolations(verdict), string(verdict.Verdict), output.Usage, nil
}

// acceptAudit decodes the verdict and checks it audited what was sent.
func acceptAudit(task Task, payload []byte) (*state.RepoDocAuditState, error) {
	decoded, err := state.Decode(state.KindRepoDocAudit, payload)
	if err != nil {
		return nil, err
	}
	verdict, ok := decoded.(*state.RepoDocAuditState)
	if !ok {
		return nil, fmt.Errorf("decoded %T, want a repo-doc-audit document", decoded)
	}
	if verdict.Path != task.Path || verdict.InputHash != task.Sheet.InputHash {
		return nil, fmt.Errorf("audited %q at %q but %q at %q was sent", verdict.Path, verdict.InputHash, task.Path, task.Sheet.InputHash)
	}
	return verdict, nil
}

func blockingViolations(verdict *state.RepoDocAuditState) []Violation {
	var violations []Violation
	for _, finding := range verdict.Findings {
		if finding.Blocking {
			violations = append(violations, Violation{Source: SourceAuditor, Code: CodeAuditorBlockingFix, Where: finding.Claim,
				Message: finding.Problem + " " + finding.Instruction})
		}
	}
	return violations
}
