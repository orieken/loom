package repodoc

import (
	"context"
	"fmt"
	"time"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/state"
)

// AgentName is the agent every invocation runs.
const AgentName = "repo-documenter"

// DefaultTimeout bounds one directory's invocation.
const DefaultTimeout = 5 * time.Minute

// Runner documents directories through a Provider.
type Runner struct {
	Provider   orchestrator.Provider
	Root       string
	Vocabulary Vocabulary
	// Recorded is the index as last written; its summaries stand in for
	// children this run does not re-document.
	Recorded contextindex.Index
	Now      func() time.Time
	Timeout  time.Duration
}

// Run documents each task in the order given — which Plan makes bottom-up —
// and records a candidate per directory plus one run record. A directory that
// fails is recorded and skipped: its parent is told the child is undocumented
// rather than being given nothing or being blocked. Run stops early, and says
// so in the record, when ctx is cancelled.
func (r Runner) Run(ctx context.Context, tasks []Task) (RunRecord, error) {
	started := r.Now().UTC()
	record := RunRecord{RunID: started.Format("20060102T150405Z"), StartedAt: started.Format(time.RFC3339), Agent: AgentName, Outcomes: []Outcome{}}
	runDir := RunDir(r.Root, record.RunID)
	known := summaries{thisRun: map[string]state.RepoDocState{}, recorded: r.Recorded}
	var total orchestrator.Usage
	reported := false
	for _, task := range tasks {
		if ctx.Err() != nil {
			record.Interrupted = true
			break
		}
		outcome, doc := r.document(ctx, task, known, runDir)
		record.Outcomes = append(record.Outcomes, outcome)
		if doc != nil {
			known.thisRun[task.Path] = *doc
		}
		if outcome.Usage != nil {
			total.Add(outcome.Usage)
			reported = true
		}
	}
	if reported {
		record.TotalUsage = &total
	}
	if err := writeRecord(runDir, record); err != nil {
		return record, err
	}
	return record, nil
}

// document runs the agent for one directory and validates what comes back.
func (r Runner) document(ctx context.Context, task Task, known summaries, runDir string) (Outcome, *state.RepoDocState) {
	outcome := Outcome{Path: task.Path, InputHash: task.Sheet.InputHash, Status: StatusFailed}
	input, err := buildInput(task, known, r.Vocabulary)
	if err != nil {
		outcome.Error = err.Error()
		return outcome, nil
	}
	timeout := r.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	stage := orchestrator.Stage{ID: stageID(task.Path), Agent: AgentName, StateKind: string(state.KindRepoDoc), Timeout: timeout}
	stageCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	output, err := r.Provider.Invoke(stageCtx, stage, orchestrator.StageInput{
		SpecPath: ".loom/facts/" + task.Path, WorkspaceDir: runDir, ProjectRoot: r.Root, UpstreamState: input,
	})
	outcome.Usage = output.Usage
	if err != nil {
		outcome.Error = err.Error()
		return outcome, nil
	}
	doc, err := accept(task, output.Payload)
	if err != nil {
		outcome.Error = err.Error()
		return outcome, nil
	}
	if err := writeCandidate(runDir, task.Path, output.Payload); err != nil {
		outcome.Error = err.Error()
		return outcome, nil
	}
	outcome.Status, outcome.FilesRead = StatusDocumented, len(doc.FilesRead)
	return outcome, doc
}

// accept decodes a payload against the schema and checks it documents the
// directory — and the version of it — that was sent. An agent that documents
// a different path, or a stale hash, has answered a different question.
func accept(task Task, payload []byte) (*state.RepoDocState, error) {
	decoded, err := state.Decode(state.KindRepoDoc, payload)
	if err != nil {
		return nil, err
	}
	doc, ok := decoded.(*state.RepoDocState)
	if !ok {
		return nil, fmt.Errorf("decoded %T, want a repo-doc document", decoded)
	}
	if doc.Path != task.Path {
		return nil, fmt.Errorf("documents %q but %q was sent", doc.Path, task.Path)
	}
	if doc.InputHash != task.Sheet.InputHash {
		return nil, fmt.Errorf("documents input hash %q but the fact sheet's is %q", doc.InputHash, task.Sheet.InputHash)
	}
	return doc, nil
}
