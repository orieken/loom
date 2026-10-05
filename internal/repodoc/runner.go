package repodoc

import (
	"context"
	"fmt"
	"time"

	"github.com/orieken/loom/internal/contextindex"
	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/state"
)

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
	Now      func() time.Time
	Timeout  time.Duration
	// MaxRounds caps attempts per directory; zero means DefaultMaxRounds.
	MaxRounds int
	// SkipAudit runs only the deterministic verifier.
	SkipAudit bool
}

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
// record, when ctx is cancelled.
func (r Runner) Run(ctx context.Context, tasks []Task) (RunRecord, error) {
	started := r.Now().UTC()
	record := RunRecord{RunID: started.Format("20060102T150405Z"), StartedAt: started.Format(time.RFC3339), Agent: AgentName, Outcomes: []Outcome{}}
	runDir := RunDir(r.Root, record.RunID)
	known := summaries{thisRun: map[string]state.RepoDocState{}, recorded: r.Recorded}
	var tally usageTally
	for _, task := range tasks {
		if ctx.Err() != nil {
			record.Interrupted = true
			break
		}
		outcome, verified := r.document(ctx, task, known, runDir)
		record.Outcomes = append(record.Outcomes, outcome)
		if verified != nil {
			known.thisRun[task.Path] = *verified
		}
		tally.add(outcome.Usage)
	}
	record.TotalUsage = tally.result()
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

// invoke calls the provider under the stage's own deadline.
func (r Runner) invoke(ctx context.Context, stage orchestrator.Stage, task Task, runDir string, input map[string][]byte) (orchestrator.StageOutput, error) {
	stageCtx, cancel := context.WithTimeout(ctx, stage.Timeout)
	defer cancel()
	return r.Provider.Invoke(stageCtx, stage, orchestrator.StageInput{
		SpecPath: ".loom/facts/" + task.Path, WorkspaceDir: runDir, ProjectRoot: r.Root, UpstreamState: input,
	})
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
