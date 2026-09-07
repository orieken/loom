package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/orieken/loom/internal/policy"
)

// Executor runs a Plan's stages in order against a Provider, persisting
// state after every transition (stage started, completed, failed,
// interrupted) so a run can always resume from its last checkpoint.
type Executor struct {
	provider Provider
	store    *StateStore
	timeline *Timeline
	onStale  func([]StaleStage)
	onReset  func(*StaleApprovalError)
	onRoute  func(RouteSummary)
	onLoop   func(LoopRound)
	tracer   Tracer
	// onBaselineError reports a failure to retain what a human was shown at
	// a gate (roadmap L4.5). Retention is best-effort: it observes a human's
	// action rather than controlling the run, so a failure is reported and
	// never propagated.
	onBaselineError func(error)
	// policies are the loaded delivery policies evaluated at each gate
	// (roadmap L2.16). Empty means no policy file exists, which is the
	// default and the safe one.
	policies []policy.Policy
	onPolicy func(policy.Decision)
}

// NewExecutor wires a provider and a state store into an executor.
func NewExecutor(provider Provider, store *StateStore) *Executor {
	return &Executor{
		provider: provider,
		store:    store,
		timeline: NewTimeline(store.Path()),
		tracer:   noopTracer{},
	}
}

// emit appends one event to the run's timeline. Timestamps come from the
// clock here, not from anything a provider reported.
func (e *Executor) emit(event Event) error {
	return e.timeline.Append(event)
}

// OnStale registers a callback invoked once per Run, before any stage
// executes, with the stages that digest verification demoted. It is how the
// CLI tells the human which completed work is being redone and why.
func (e *Executor) OnStale(report func([]StaleStage)) {
	e.onStale = report
}

// OnApprovalReset registers a callback invoked when an edit resets a gate's
// approval. It is how the CLI explains a halt that a human's own edit
// caused, rather than leaving them to work it out from the state file.
func (e *Executor) OnApprovalReset(report func(*StaleApprovalError)) {
	e.onReset = report
}

// OnRoute registers a callback invoked once, when the router decides which
// stages run. It is how the CLI tells a human what is about to happen
// before the design gate asks them to approve it.
func (e *Executor) OnRoute(report func(RouteSummary)) {
	e.onRoute = report
}

// WithPolicies registers the policies evaluated at each gate. They are
// recorded and reported; nothing in this build acts on them.
func (e *Executor) WithPolicies(policies []policy.Policy) {
	e.policies = policies
}

// OnPolicyDecision registers a callback invoked with each gate's policy
// decision, so the CLI can tell a human what a policy would have done
// before anyone trusts it to do it.
func (e *Executor) OnPolicyDecision(report func(policy.Decision)) {
	e.onPolicy = report
}

// OnBaselineError registers a callback invoked when the executor cannot
// retain the artifact state a human is being shown at a gate. The run
// continues either way — losing the ability to describe a later edit is a
// lost observation, not a broken delivery.
func (e *Executor) OnBaselineError(report func(error)) {
	e.onBaselineError = report
}

// OnLoopRound registers a callback invoked when a loop sends its span round
// again. It is how the CLI says which round is starting, so a run that
// takes four attempts does not look like a run that is stuck.
func (e *Executor) OnLoopRound(report func(LoopRound)) {
	e.onLoop = report
}

// Run executes every stage of the plan that is not already COMPLETED in
// persisted state, stopping on the first failure. Cancelling ctx (SIGINT)
// persists a clean INTERRUPTED checkpoint for the in-flight stage before
// returning, so a later Run resumes by re-running that stage.
func (e *Executor) Run(ctx context.Context, plan Plan, input StageInput) error {
	state, err := e.prepareState(plan, input)
	if err != nil {
		return err
	}
	if err := e.emit(Event{Kind: EventRunStarted}); err != nil {
		return err
	}
	// Detect corrections before anything else touches the workspace. For an
	// artifact that is still markdown, the tracked file IS the file a human
	// edits, so verification is about to demote the stage and re-run it —
	// and the agent's second attempt would then look like the human's
	// correction (roadmap L4.5).
	if err := e.collectCorrections(state); err != nil {
		return err
	}
	ctx, span := e.tracer.StartRun(ctx, e.runSpanFor(plan, input))
	err = e.runStages(ctx, plan, input, state)
	span.End(runOutcome(err))
	if err != nil {
		return err
	}
	return e.emit(Event{Kind: EventRunCompleted})
}

// runStages is the loop itself, split out so Run can close the root span on
// every exit path — including the gate halt, which is a normal outcome that
// still has to flush what it recorded.
func (e *Executor) runStages(ctx context.Context, plan Plan, input StageInput, state *RunState) error {
	for index := 0; index < len(plan.Stages); index++ {
		if err := e.advance(ctx, plan.Stages[index], plan, input, state); err != nil {
			return err
		}
		back, err := e.closeLoop(plan, plan.Stages[index], input, state)
		if err != nil {
			return err
		}
		if back >= 0 {
			index = back - 1
		}
	}
	return nil
}

func (e *Executor) runSpanFor(plan Plan, input StageInput) RunSpan {
	return RunSpan{
		Plan:      plan.Name,
		Feature:   FeatureNameFromSpec(input.SpecPath),
		SpecPath:  input.SpecPath,
		StateFile: e.store.Path(),
	}
}

// runOutcome maps how Run ended onto the executor's own status vocabulary,
// so a trace and run state never describe the same run differently. A halt
// at a gate is not a failure: the run is waiting on a human, which is the
// outcome the design intends.
func runOutcome(err error) SpanOutcome {
	switch {
	case err == nil:
		return SpanOutcome{Status: StageStatusCompleted}
	case errors.Is(err, ErrWaitingApproval):
		return SpanOutcome{Status: StageStatusWaitingApproval, Err: err}
	case errors.Is(err, context.Canceled):
		return SpanOutcome{Status: StageStatusInterrupted, Err: err}
	default:
		return SpanOutcome{Status: StageStatusFailed, Err: err}
	}
}

// advance moves one stage forward: clear its gate, then run it unless it is
// already settled.
//
// The gate is checked before the settled check on purpose. A gate is a
// barrier on reaching this point in the run, not a property of the stage
// behind it, so routing a gated stage out must not silently delete the
// human checkpoint that guarded it (roadmap L3.0).
func (e *Executor) advance(ctx context.Context, stage Stage, plan Plan, input StageInput, state *RunState) error {
	if err := e.checkGate(stage, state); err != nil {
		return err
	}
	if err := e.restoreStatusAfterGate(state, stage.ID); err != nil {
		return err
	}
	if state.IsStageSettled(stage.ID) {
		return nil
	}
	return e.runStage(ctx, stage, plan, input, state)
}

func (e *Executor) prepareState(plan Plan, input StageInput) (*RunState, error) {
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	state, err := e.store.Load()
	if err != nil {
		return nil, err
	}
	if state == nil {
		return newRunFor(plan, input), nil
	}
	if err := state.CheckCreatedBy(CreatedByExecutor); err != nil {
		return nil, err
	}
	if err := checkStateBelongsToRun(state, plan, input, e.store.Path()); err != nil {
		return nil, err
	}
	return e.verifyResumedState(state)
}

func (e *Executor) emitStale(state *RunState, stale []StaleStage) error {
	for _, item := range stale {
		event := Event{Kind: EventStageStale, Stage: item.StageID, StaleReason: item.Reason,
			Sequence: state.Stages[item.StageID].Sequence}
		if err := e.emit(event); err != nil {
			return err
		}
	}
	return nil
}

func newRunFor(plan Plan, input StageInput) *RunState {
	state := NewRunState(plan.Name, CreatedByExecutor)
	state.SpecPath = input.SpecPath
	state.FeatureName = FeatureNameFromSpec(input.SpecPath)
	return state
}

// checkStateBelongsToRun refuses state from a different plan or a different
// spec: resuming one feature's run against another's spec would replay the
// wrong work against the right state.
func checkStateBelongsToRun(state *RunState, plan Plan, input StageInput, path string) error {
	if state.PlanName != plan.Name {
		return fmt.Errorf("run state %s belongs to plan %q, not %q — refusing to mix runs",
			path, state.PlanName, plan.Name)
	}
	if state.SpecPath != "" && input.SpecPath != "" && state.SpecPath != input.SpecPath {
		return fmt.Errorf("run state %s belongs to spec %q, not %q — refusing to mix runs",
			path, state.SpecPath, input.SpecPath)
	}
	return nil
}

// verifyResumedState is the L2.12 integrity check: every completed stage's
// artifact is re-hashed in Go before the run loop trusts it.
func (e *Executor) verifyResumedState(state *RunState) (*RunState, error) {
	stale := VerifyCompletedStages(state)
	if len(stale) == 0 {
		return state, nil
	}
	if err := e.store.Save(state); err != nil {
		return nil, fmt.Errorf("persist verification result: %w", err)
	}
	if err := e.emitStale(state, stale); err != nil {
		return nil, err
	}
	if err := e.invalidateApprovalsFor(state, stale); err != nil {
		return nil, err
	}
	if err := e.store.Save(state); err != nil {
		return nil, fmt.Errorf("persist approval invalidation: %w", err)
	}
	if e.onStale != nil {
		e.onStale(stale)
	}
	return state, nil
}

// sequenceFor keeps a stage's original position when it is re-recorded: a
// re-run after an interrupt or a review loop is the same step of the run,
// not a new one.
func sequenceFor(state *RunState, stageID string) int {
	if existing := state.Stages[stageID].Sequence; existing > 0 {
		return existing
	}
	return state.NextSequence()
}

// checkGate is the process interrupt: a gated stage cannot start until run
// state records an approval for its gate. Nothing a provider returns can
// create that approval — only the CLI approval channels write it (roadmap
// L2.13). The halt is persisted so the run resumes exactly here.
func (e *Executor) checkGate(stage Stage, state *RunState) error {
	if stage.Gate == "" {
		return nil
	}
	if state.IsGateApproved(stage.Gate) {
		invalidated, err := e.enforceApprovalBinding(state, stage)
		if err != nil {
			return err
		}
		if !invalidated {
			return nil
		}
	}
	// Evaluate before the halt is announced, so the decision is recorded
	// with the state the human is about to see. It does not change whether
	// the halt happens (roadmap L2.16).
	e.evaluatePolicies(state, stage)
	// Capture before the halt is announced: this is the moment the state is
	// presented to a human, and it is the baseline any edit they make is
	// measured against (roadmap L4.5).
	e.captureBaseline(state, stage.Gate, BaselinePresented)
	if err := e.persistStatus(state, stage.ID, waitingRecord(state, stage)); err != nil {
		return err
	}
	if err := e.emit(Event{Kind: EventGateWaiting, Stage: stage.ID, Gate: stage.Gate}); err != nil {
		return err
	}
	return &WaitingApprovalError{Gate: stage.Gate, Stage: stage.ID}
}

// waitingRecord marks a stage as halted at its gate without discarding what
// the record already said. A gate is a barrier on reaching a point in the
// run, so a stage that was already settled — routed out, or completed
// before a loop exhausted on it — must come back settled once a human
// approves, rather than being resurrected by the halt.
func waitingRecord(state *RunState, stage Stage) StageRecord {
	record := state.Stages[stage.ID]
	if record.Status == StageStatusSkipped || record.Status == StageStatusCompleted {
		record.PreviousStatus = record.Status
	}
	record.Status = StageStatusWaitingApproval
	record.Gate = stage.Gate
	record.StartedAt = time.Now().UTC()
	return record
}

// restoreStatusAfterGate puts a stage back to what it was before the halt,
// so approving a checkpoint does not re-run work that was already settled.
func (e *Executor) restoreStatusAfterGate(state *RunState, stageID string) error {
	record := state.Stages[stageID]
	if record.Status != StageStatusWaitingApproval || record.PreviousStatus == "" {
		return nil
	}
	record.Status = record.PreviousStatus
	record.PreviousStatus = ""
	return e.persistStatus(state, stageID, record)
}

func (e *Executor) runStage(ctx context.Context, stage Stage, plan Plan, input StageInput, state *RunState) error {
	if err := e.persistStatus(state, stage.ID, runningRecord(state, stage.ID)); err != nil {
		return err
	}
	if err := e.emit(Event{Kind: EventStageStarted, Stage: stage.ID, Sequence: state.Stages[stage.ID].Sequence}); err != nil {
		return err
	}
	ctx, span := e.tracer.StartStage(ctx, stageSpanFor(stage, state))
	err := e.executeStage(ctx, stage, plan, input, state)
	span.End(stageOutcome(state, stage.ID, err))
	return err
}

// executeStage is the stage body: project what it may read, run it, and
// persist whichever terminal state resulted.
func (e *Executor) executeStage(ctx context.Context, stage Stage, plan Plan, input StageInput, state *RunState) error {
	input, projectErr := projectUpstream(stage, plan, input)
	if projectErr != nil {
		return e.persistFailure(ctx, state, stageFailure{stage: stage, err: projectErr})
	}
	output, invokeErr := e.runOrInvoke(ctx, stage, plan, input)
	if invokeErr != nil {
		return e.persistFailure(ctx, state, stageFailure{stage: stage, err: invokeErr, usage: output.Usage})
	}
	return e.persistCompletion(state, stage, plan, input, output)
}

func stageSpanFor(stage Stage, state *RunState) StageSpan {
	record := state.Stages[stage.ID]
	return StageSpan{
		ID:        stage.ID,
		Agent:     stage.Agent,
		Sequence:  record.Sequence,
		Iteration: record.Iteration,
		Gate:      stage.Gate,
		Internal:  stage.Internal,
	}
}

// stageOutcome reads the status the executor just persisted rather than
// inferring one from the error, so the span reports what state records. A
// stage that failed has already been written as FAILED by the time the span
// closes.
func stageOutcome(state *RunState, stageID string, err error) SpanOutcome {
	record := state.Stages[stageID]
	return SpanOutcome{Status: record.Status, Reason: record.SkipReason, Err: err}
}

// runningRecord marks a stage in flight without discarding what the record
// already carries. A fresh struct would drop the iteration count, which for
// a looping stage means the loop never advances and never terminates — the
// first thing the loop tests caught.
func runningRecord(state *RunState, stageID string) StageRecord {
	record := state.Stages[stageID]
	record.Status = StageStatusRunning
	record.StartedAt = time.Now().UTC()
	record.FinishedAt = nil
	record.Error = ""
	return record
}

// runOrInvoke executes an internal stage here, or hands any other stage to
// the provider.
func (e *Executor) runOrInvoke(ctx context.Context, stage Stage, plan Plan, input StageInput) (StageOutput, error) {
	if stage.Internal {
		return e.runInternalStage(stage, plan, input)
	}
	return e.invoke(ctx, stage, input)
}

// invoke calls the provider under the stage's timeout, inside its own span.
// A zero timeout means no per-stage deadline beyond the parent context.
func (e *Executor) invoke(ctx context.Context, stage Stage, input StageInput) (StageOutput, error) {
	ctx, span := e.tracer.StartProvider(ctx, ProviderSpan{
		Stage: stage.ID, Agent: stage.Agent, Operation: GenAIOperationName,
	})
	output, err := e.callProvider(ctx, stage, input)
	span.End(invokeOutcome(output, err))
	return output, err
}

func (e *Executor) callProvider(ctx context.Context, stage Stage, input StageInput) (StageOutput, error) {
	if stage.Timeout <= 0 {
		return e.provider.Invoke(ctx, stage, input)
	}
	stageCtx, cancel := context.WithTimeout(ctx, stage.Timeout)
	defer cancel()
	return e.provider.Invoke(stageCtx, stage, input)
}

// GenAIOperationName is the GenAI semantic convention's operation name for
// what a stage does: one prompt, one completion.
const GenAIOperationName = "generate_content"

// invokeOutcome carries whatever usage the provider reported, including on
// failure — a call that produced tokens and then failed still cost money,
// and a trace that dropped those numbers would understate the run.
func invokeOutcome(output StageOutput, err error) SpanOutcome {
	if err != nil {
		return SpanOutcome{Status: StageStatusFailed, Err: err, Usage: output.Usage}
	}
	return SpanOutcome{Status: StageStatusCompleted, Usage: output.Usage}
}

// artifactFor resolves what this stage's artifact is: a typed stage's
// validated state document, written here, or the markdown file a provider
// wrote itself.
func artifactFor(stage Stage, input StageInput, output StageOutput) (string, error) {
	if stage.StateKind == "" {
		return output.ArtifactPath, nil
	}
	return persistTypedOutput(stage, input, output)
}

// persistFailure distinguishes parent cancellation (SIGINT — checkpoint as
// INTERRUPTED, resumable) from a stage failure or timeout (FAILED, stops
// the run).
// stageFailure is what a stage failed with, and what it consumed before it
// did. The usage is carried here rather than dropped because a call that
// produced tokens and then failed still cost money — a run whose total
// omitted its failures would understate itself, and the failures are often
// the expensive part.
type stageFailure struct {
	stage Stage
	err   error
	usage *Usage
}

func (e *Executor) persistFailure(ctx context.Context, state *RunState, failure stageFailure) error {
	record := state.Stages[failure.stage.ID]
	now := time.Now().UTC()
	record.FinishedAt = &now
	record.Error = failure.err.Error()
	record.Usage = accumulateUsage(record.Usage, failure.usage)
	if errors.Is(ctx.Err(), context.Canceled) {
		record.Status = StageStatusInterrupted
	} else {
		record.Status = StageStatusFailed
	}
	if err := e.persistStatus(state, failure.stage.ID, record); err != nil {
		return errors.Join(failure.err, err)
	}
	if err := e.emitStageEnd(state, failure.stage.ID, record); err != nil {
		return errors.Join(failure.err, err)
	}
	return fmt.Errorf("stage %q: %w", failure.stage.ID, failure.err)
}

func (e *Executor) persistCompletion(state *RunState, stage Stage, plan Plan, input StageInput, output StageOutput) error {
	artifactPath, err := artifactFor(stage, input, output)
	if err != nil {
		return e.persistFailure(context.Background(), state, stageFailure{stage: stage, err: err, usage: output.Usage})
	}
	record := state.Stages[stage.ID]
	now := time.Now().UTC()
	record.FinishedAt = &now
	record.Status = StageStatusCompleted
	record.ArtifactPath = artifactPath
	record.ViewPath = viewPathFor(stage, input)
	record.Agent = stage.Agent
	record.StateKind = stage.StateKind
	record.Usage = accumulateUsage(record.Usage, output.Usage)
	if artifactPath != "" {
		sum, err := ArtifactSHA256(artifactPath)
		if err != nil {
			return fmt.Errorf("stage %q completed but its artifact is unreadable: %w", stage.ID, err)
		}
		record.ArtifactSHA256 = sum
	}
	if err := e.persistStatus(state, stage.ID, record); err != nil {
		return err
	}
	// The executor just wrote this stage's output, so any baseline holding
	// it is stale in a way no human caused (roadmap L4.5).
	e.refreshBaselinesFor(state, stage.ID)
	if err := e.emitStageEnd(state, stage.ID, record); err != nil {
		return err
	}
	if stage.ID == RouterStageID {
		return e.applyRoute(plan, input, state)
	}
	return nil
}

// emitStageEnd records how a stage finished, using the same record the
// state file holds so the two never disagree.
func (e *Executor) emitStageEnd(state *RunState, stageID string, record StageRecord) error {
	kinds := map[StageStatus]EventKind{
		StageStatusCompleted:   EventStageCompleted,
		StageStatusFailed:      EventStageFailed,
		StageStatusInterrupted: EventStageInterrupted,
	}
	kind, ok := kinds[record.Status]
	if !ok {
		return nil
	}
	return e.emit(Event{Kind: kind, Stage: stageID, Sequence: state.Stages[stageID].Sequence, Error: record.Error})
}

func (e *Executor) persistStatus(state *RunState, stageID string, record StageRecord) error {
	if record.Sequence == 0 {
		record.Sequence = sequenceFor(state, stageID)
	}
	if record.Iteration == 0 {
		record.Iteration = 1
	}
	state.Stages[stageID] = record
	if err := e.store.Save(state); err != nil {
		return fmt.Errorf("persist state for stage %q: %w", stageID, err)
	}
	return nil
}
