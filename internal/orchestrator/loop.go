package orchestrator

// A bounded loop (roadmap L2.17) is plan data: a span of stages, a named
// condition, and a maximum number of iterations. The executor evaluates the
// condition and counts the rounds; the model does neither.
//
// The prose this replaces — deliver-feature steps 18–21 — asks an LLM to
// notice CHANGES REQUESTED in its own output, copy files to .history/, and
// repeat "until APPROVED" with no stated bound.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/orieken/loom/internal/state"
)

// IterationsDirName holds the retained artifact of every round. Keeping
// them is what makes "why did this take four rounds?" answerable, and each
// is digested like any other artifact.
const IterationsDirName = ".iterations"

// Loop declares a span of stages that repeats until a condition holds.
type Loop struct {
	// ID names the loop for messages and state.
	ID string
	// From and To bound the span, inclusive. The condition is evaluated
	// after To completes.
	From string
	To   string
	// Condition names a predicate resolved in Go. Deliberately not an
	// expression language: an evaluator that is a prompt is the L2.16
	// defect, and one that is CEL is L2.16's solution.
	Condition string
	// Gate halts the run when MaxIterations is reached with the condition
	// still false — exhaustion is where a human has to look, not a crash.
	Gate          string
	MaxIterations int
}

// ReviewApprovedCondition is the only named condition today.
const ReviewApprovedCondition = "review-approved"

// conditions maps a name to the predicate that decides it.
func conditions() map[string]func(Loop, StageInput) (bool, error) {
	return map[string]func(Loop, StageInput) (bool, error){
		ReviewApprovedCondition: reviewApproved,
	}
}

// reviewApproved reads the verdict field of the loop's final stage. It is a
// field lookup, never a parse of prose.
func reviewApproved(loop Loop, input StageInput) (bool, error) {
	raw, err := os.ReadFile(typedStatePath(input.WorkspaceDir, loop.To))
	if err != nil {
		return false, fmt.Errorf("condition %q needs %s's typed state: %w", loop.Condition, loop.To, err)
	}
	decoded, err := state.Decode(state.KindReview, raw)
	if err != nil {
		return false, fmt.Errorf("condition %q cannot read the review: %w", loop.Condition, err)
	}
	review, ok := decoded.(*state.ReviewState)
	if !ok {
		return false, fmt.Errorf("condition %q read something that is not a review", loop.Condition)
	}
	return review.IsApproved(), nil
}

// loopEndingAt finds the loop whose span closes at this stage.
func (p Plan) loopEndingAt(stageID string) (Loop, bool) {
	for _, loop := range p.Loops {
		if loop.To == stageID {
			return loop, true
		}
	}
	return Loop{}, false
}

// stageIndex returns a stage's position in the plan.
func (p Plan) stageIndex(stageID string) (int, bool) {
	for index, stage := range p.Stages {
		if stage.ID == stageID {
			return index, true
		}
	}
	return 0, false
}

// span lists the stage IDs a loop repeats, inclusive of both ends.
func (p Plan) span(loop Loop) []string {
	from, fromFound := p.stageIndex(loop.From)
	to, toFound := p.stageIndex(loop.To)
	if !fromFound || !toFound || to < from {
		return nil
	}
	ids := make([]string, 0, to-from+1)
	for _, stage := range p.Stages[from : to+1] {
		ids = append(ids, stage.ID)
	}
	return ids
}

// IterationArtifact is one round's retained output.
type IterationArtifact struct {
	Iteration int       `json:"iteration"`
	Path      string    `json:"path"`
	SHA256    string    `json:"sha256"`
	At        time.Time `json:"at"`
}

// retainIteration copies a completed artifact into the iterations
// directory and records it with its own digest, so an earlier round is
// verifiable rather than merely remembered.
func retainIteration(input StageInput, stageID string, record StageRecord) (IterationArtifact, error) {
	if record.ArtifactPath == "" {
		return IterationArtifact{}, nil
	}
	target := iterationPath(input.WorkspaceDir, stageID, record.Iteration, record.ArtifactPath)
	if err := copyFile(record.ArtifactPath, target); err != nil {
		return IterationArtifact{}, err
	}
	sum, err := ArtifactSHA256(target)
	if err != nil {
		return IterationArtifact{}, fmt.Errorf("digest retained iteration: %w", err)
	}
	return IterationArtifact{Iteration: record.Iteration, Path: target, SHA256: sum, At: time.Now().UTC()}, nil
}

func iterationPath(workspaceDir, stageID string, iteration int, artifactPath string) string {
	extension := filepath.Ext(artifactPath)
	name := fmt.Sprintf("%s.%d%s", stageID, iteration, extension)
	return filepath.Join(workspaceDir, IterationsDirName, name)
}

func copyFile(source, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create iterations directory: %w", err)
	}
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("read artifact to retain: %w", err)
	}
	defer func() { _ = in.Close() }()
	return writeCopy(in, target)
}

func writeCopy(in io.Reader, target string) error {
	out, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("create retained iteration: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("write retained iteration: %w", err)
	}
	return out.Close()
}

// closeLoop runs at the end of a loop's span. It returns the plan index to
// re-enter at, or -1 to carry on. The condition and the count are both
// evaluated here, in Go: this is the whole point of L2.17.
func (e *Executor) closeLoop(plan Plan, stage Stage, input StageInput, runState *RunState, outcomes map[string]string) (int, error) {
	loop, closes := plan.loopEndingAt(stage.ID)
	if !closes {
		return -1, nil
	}
	if runState.IsGateApproved(loop.Gate) {
		outcomes[loop.ID] = LoopGateApproved
		return -1, nil
	}
	satisfied, err := evaluateCondition(loop, input)
	if err != nil {
		outcomes[loop.ID] = LoopConditionError
		return -1, err
	}
	if satisfied {
		outcomes[loop.ID] = LoopConverged
		return -1, nil
	}
	return e.iterateOrExhaust(plan, loop, input, runState, outcomes)
}

func evaluateCondition(loop Loop, input StageInput) (bool, error) {
	condition, known := conditions()[loop.Condition]
	if !known {
		return false, fmt.Errorf("loop %q names condition %q, which does not exist", loop.ID, loop.Condition)
	}
	return condition(loop, input)
}

// iterateOrExhaust sends the span round again, or halts at the loop's gate
// when the bound is reached. Exhaustion is not a failure: it is the point
// where automation has done what it can and a human has to decide.
func (e *Executor) iterateOrExhaust(plan Plan, loop Loop, input StageInput, runState *RunState, outcomes map[string]string) (int, error) {
	next := runState.Stages[loop.To].Iteration + 1
	if next > loop.MaxIterations {
		outcomes[loop.ID] = LoopRoundLimit
		return -1, e.exhaust(loop, runState)
	}
	if err := e.reenter(plan, loop, input, runState, next); err != nil {
		return -1, err
	}
	index, _ := plan.stageIndex(loop.From)
	return index, nil
}

// reenter retains each stage's artifact for the round just finished, then
// clears the span so it runs again with the iteration incremented.
func (e *Executor) reenter(plan Plan, loop Loop, input StageInput, runState *RunState, iteration int) error {
	for _, stageID := range plan.span(loop) {
		if err := e.retainAndReset(input, runState, stageID, iteration); err != nil {
			return err
		}
	}
	if e.onLoop != nil {
		e.onLoop(LoopRound{Loop: loop.ID, From: loop.From, Iteration: iteration, Max: loop.MaxIterations})
	}
	return e.emit(Event{Kind: EventLoopIterated, Loop: loop.ID, Stage: loop.From, Iteration: iteration})
}

// LoopRound describes a loop about to repeat: which loop, where it
// re-enters, and how many rounds are left.
type LoopRound struct {
	Loop      string
	From      string
	Iteration int
	Max       int
}

func (e *Executor) retainAndReset(input StageInput, runState *RunState, stageID string, iteration int) error {
	record := runState.Stages[stageID]
	retained, err := retainIteration(input, stageID, record)
	if err != nil {
		return err
	}
	if retained.Path != "" {
		record.IterationArtifacts = append(record.IterationArtifacts, retained)
	}
	record.Iteration = iteration
	record.Status = ""
	record.FinishedAt = nil
	return e.persistStatus(runState, stageID, record)
}

// exhaust halts at the loop's gate, leaving the run resumable: a human who
// approves is accepting the outstanding findings, and that approval binds
// the artifacts it was given like any other.
func (e *Executor) exhaust(loop Loop, runState *RunState) error {
	record := runState.Stages[loop.To]
	record.PreviousStatus = record.Status
	record.Status = StageStatusWaitingApproval
	record.Gate = loop.Gate
	if err := e.persistStatus(runState, loop.To, record); err != nil {
		return err
	}
	if err := e.emit(Event{Kind: EventLoopExhausted, Loop: loop.ID, Stage: loop.To,
		Iteration: record.Iteration}); err != nil {
		return err
	}
	if err := e.emit(Event{Kind: EventGateWaiting, Stage: loop.To, Gate: loop.Gate}); err != nil {
		return err
	}
	return &WaitingApprovalError{Gate: loop.Gate, Stage: loop.To}
}
