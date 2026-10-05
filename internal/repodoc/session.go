package repodoc

import (
	"context"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/state"
)

// session is the bounded loop over one directory: document, verify, audit,
// and on a problem go round again with the findings. The executor counts the
// rounds and reads the verdicts; the agents do neither (roadmap L2.17).
type session struct {
	runner  Runner
	task    Task
	known   summaries
	runDir  string
	outcome *Outcome
	tally   usageTally
	last    *state.RepoDocState
}

// document runs the loop and returns the outcome plus the verified document,
// or nil when the directory did not verify.
func (r Runner) document(ctx context.Context, task Task, known summaries, runDir string) (Outcome, *state.RepoDocState) {
	outcome := Outcome{Path: task.Path, InputHash: task.Sheet.InputHash, Status: StatusFailed}
	s := &session{runner: r, task: task, known: known, runDir: runDir, outcome: &outcome}
	verified := s.run(ctx)
	outcome.Usage = s.tally.result()
	return outcome, verified
}

func (s *session) run(ctx context.Context) *state.RepoDocState {
	var feedback *Feedback
	for round := 1; round <= s.runner.rounds(); round++ {
		result := s.attempt(ctx, round, feedback)
		if result.fatal {
			return nil
		}
		s.outcome.Rounds = append(s.outcome.Rounds, result.record)
		if len(result.record.Violations) == 0 {
			return s.accept(result.doc)
		}
		feedback = &Feedback{Round: round + 1, Previous: result.doc, Findings: result.record.Violations}
	}
	return s.giveUp()
}

type attemptResult struct {
	doc    *state.RepoDocState
	record RoundRecord
	// fatal means the provider failed, so there is nothing to revise.
	fatal bool
}

// attempt is one round: ask the documenter, then check what came back.
func (s *session) attempt(ctx context.Context, round int, feedback *Feedback) attemptResult {
	record := RoundRecord{Round: round, Violations: []Violation{}}
	doc, rejection, fatal := s.askDocumenter(ctx, round, feedback)
	if fatal {
		return attemptResult{fatal: true}
	}
	if rejection != "" {
		record.Violations = []Violation{{Source: SourceVerifier, Code: CodeInvalidOutput, Where: "document", Message: rejection}}
		return attemptResult{record: record}
	}
	s.last = doc
	record.Violations = Verify(s.task.Sheet, *doc, s.runner.Vocabulary)
	if len(record.Violations) == 0 && !s.runner.SkipAudit {
		return s.auditAttempt(ctx, doc, record)
	}
	return attemptResult{doc: doc, record: record}
}

func (s *session) auditAttempt(ctx context.Context, doc *state.RepoDocState, record RoundRecord) attemptResult {
	violations, verdict, usage, err := s.runner.audit(ctx, s.task, s.known, s.runDir, *doc)
	s.tally.add(usage)
	if err != nil {
		s.outcome.Error = "audit: " + err.Error()
		return attemptResult{fatal: true}
	}
	record.Audited, record.Verdict, record.Violations = true, verdict, violations
	return attemptResult{doc: doc, record: record}
}

// askDocumenter invokes the agent. A provider failure is fatal; a reply that
// fails validation is not — it is fed back as a typed problem, because the
// agent can fix a malformed reply and cannot fix a network error.
func (s *session) askDocumenter(ctx context.Context, round int, feedback *Feedback) (*state.RepoDocState, string, bool) {
	input, err := buildInput(s.task, s.known, s.runner.Vocabulary, feedback)
	if err != nil {
		s.outcome.Error = err.Error()
		return nil, "", true
	}
	stage := orchestrator.Stage{ID: stageID(s.task.Path), Agent: AgentName, StateKind: string(state.KindRepoDoc), Timeout: s.runner.timeout()}
	output, err := s.runner.invoke(ctx, stage, s.task, s.runDir, input)
	s.tally.add(output.Usage)
	if err != nil {
		s.outcome.Error = err.Error()
		return nil, "", true
	}
	doc, err := accept(s.task, output.Payload)
	if err != nil {
		return nil, err.Error(), false
	}
	_ = writePayload(RoundPath(s.runDir, s.task.Path, round), output.Payload)
	return doc, "", false
}

// accept records a verified document as a candidate.
func (s *session) accept(doc *state.RepoDocState) *state.RepoDocState {
	if err := writeDocument(CandidatePath(s.runDir, s.task.Path), *doc); err != nil {
		s.outcome.Error = err.Error()
		return nil
	}
	s.outcome.Status, s.outcome.FilesRead = StatusVerified, len(doc.FilesRead)
	return doc
}

// giveUp ends a directory that ran out of rounds. The last attempt is kept for
// a human to read, apart from the candidates, and nothing is accepted.
func (s *session) giveUp() *state.RepoDocState {
	s.outcome.Status = StatusUnverified
	if s.last != nil {
		_ = writeDocument(UnverifiedPath(s.runDir, s.task.Path), *s.last)
	}
	return nil
}
