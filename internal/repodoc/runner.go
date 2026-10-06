package repodoc

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/state"
)

// PlanName is the plan name recorded on a documentation run's root span.
const PlanName = "repo-context-index"

// AgentName is the agent that writes each directory's document.
const AgentName = "repo-documenter"

// DefaultTimeout bounds one invocation.
const DefaultTimeout = 5 * time.Minute

// DefaultMaxRounds is how many times a directory may be attempted. Like the
// review loop's bound (roadmap L2.17), exhaustion is where a human looks, not
// a crash and not an unbounded spend.
const DefaultMaxRounds = 3

// Runner documents directories through a Provider and verifies each result.
type Runner struct {
	Provider   orchestrator.Provider
	Root       string
	Vocabulary Vocabulary
	// Recorded is the index as last written; its summaries stand in for
	// children this run does not re-document.
	Recorded contextindex.Index
	// CurrentHashes maps every directory to its present fact-sheet hash, so a
	// recorded child summary that predates the child's latest change is
	// recognised as stale rather than handed to its parent as current.
	CurrentHashes map[string]string
	Now           func() time.Time
	Timeout       time.Duration
	// MaxRounds caps attempts per directory; zero means DefaultMaxRounds.
	MaxRounds int
	// SkipAudit runs only the deterministic verifier.
	SkipAudit bool
	// Tracer receives spans for the run, each attempt and each model call, with
	// the usage the provider reported (roadmap L3.8). Nil traces nothing.
	Tracer orchestrator.Tracer
	// RunID names the run. Empty derives it from Now, which is what a caller
	// that needs the id before the run starts (to place a trace file) overrides.
	RunID string
}

// RunIDAt is the run id for a start time. Ids are timestamps, so the lexically
// greatest is the newest.
func RunIDAt(started time.Time) string { return started.UTC().Format("20060102T150405Z") }

func (r Runner) timeout() time.Duration {
	if r.Timeout == 0 {
		return DefaultTimeout
	}
	return r.Timeout
}

func (r Runner) rounds() int {
	if r.MaxRounds <= 0 {
		return DefaultMaxRounds
	}
	return r.MaxRounds
}

// Run documents each task in the order given — which Plan makes bottom-up —
// and records a run. A directory that fails or stays unverified is recorded
// and skipped: its parent is told the child is undocumented rather than being
// blocked or being handed unverified text. Run stops early, and says so in the
// record, when ctx is cancelled. It also stops when the provider is unavailable
// (a usage limit): every remaining call would fail the same way, so the
// directory that hit it and those after it stay undocumented for a later run.
func (r Runner) Run(ctx context.Context, tasks []Task) (RunRecord, error) {
	started := r.Now().UTC()
	runID := r.RunID
	if runID == "" {
		runID = RunIDAt(started)
	}
	record := RunRecord{RunID: runID, StartedAt: started.Format(time.RFC3339), Agent: AgentName, Outcomes: []Outcome{}}
	runDir := RunDir(r.Root, record.RunID)
	ctx, runSpan := orchestrator.OrNoop(r.Tracer).StartRun(ctx, orchestrator.RunSpan{
		Plan: PlanName, SpecPath: r.Root, StateFile: filepath.Join(runDir, "run.json")})
	known := summaries{thisRun: map[string]state.RepoDocState{}, recorded: r.Recorded, current: r.CurrentHashes}
	var tally usageTally
	for index, task := range tasks {
		if ctx.Err() != nil {
			record.Interrupted = true
			break
		}
		outcome, verified, halt := r.document(ctx, task, known, runDir, index+1)
		tally.add(outcome.Usage)
		if halt != nil {
			record.StoppedBecause = halt.Error()
			break
		}
		record.Outcomes = append(record.Outcomes, outcome)
		if verified != nil {
			known.thisRun[task.Path] = *verified
		}
	}
	record.TotalUsage = tally.result()
	runSpan.End(runOutcome(record, tally.result()))
	return record, writeRecord(runDir, record)
}

// usageTally totals what providers reported. Nothing reported stays nil:
// unmeasured is not zero.
type usageTally struct {
	total    orchestrator.Usage
	reported bool
}

func (t *usageTally) add(usage *orchestrator.Usage) {
	if usage == nil {
		return
	}
	t.total.Add(usage)
	t.reported = true
}

func (t *usageTally) result() *orchestrator.Usage {
	if !t.reported {
		return nil
	}
	total := t.total
	return &total
}

// invoke calls the provider under the stage's own deadline, inside a stage span
// and a provider span: the stage is one attempt (Iteration is the round), the
// provider span is the model call and carries the usage it reported.
func (r Runner) invoke(ctx context.Context, stage orchestrator.Stage, task Task, runDir string, input map[string][]byte, at position) (orchestrator.StageOutput, error) {
	tracer := orchestrator.OrNoop(r.Tracer)
	ctx, stageSpan := tracer.StartStage(ctx, orchestrator.StageSpan{ID: stage.ID, Agent: stage.Agent, Sequence: at.sequence, Iteration: at.round})
	ctx, providerSpan := tracer.StartProvider(ctx, orchestrator.ProviderSpan{Stage: stage.ID, Agent: stage.Agent, Operation: orchestrator.GenAIOperationName})
	stageCtx, cancel := context.WithTimeout(ctx, stage.Timeout)
	defer cancel()
	output, err := r.Provider.Invoke(stageCtx, stage, orchestrator.StageInput{
		SpecPath: ".loom/facts/" + task.Path, WorkspaceDir: runDir, ProjectRoot: r.Root, UpstreamState: input,
	})
	outcome := callOutcome(output, err)
	providerSpan.End(outcome)
	stageSpan.End(outcome)
	return output, err
}

// position says where a call sits in the run: which directory it is, and which
// attempt at it.
type position struct {
	sequence int
	round    int
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
