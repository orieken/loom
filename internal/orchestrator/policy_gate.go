package orchestrator

// Policy evaluation at a gate (roadmap L2.16).
//
// The executor evaluates every policy watching a gate, records what they
// collectively decided, and then halts anyway. That is the whole shape of
// this cut: `loom run` has never auto-approved, and the first run that
// skips a barrier should not also be the first evidence the evaluator is
// correct. What this buys now is the audit trail approval-gates.md has
// promised since v3.3 and never had — every decision recorded, with the
// facts it could not see named.
//
// Honouring a decision is a separate, later change. It should be a small
// one, and it should land only once these records show the evaluator
// deciding what a human would have.

import (
	"os"

	"github.com/orieken/loom/internal/policy"
	"github.com/orieken/loom/internal/state"
)

// evaluatePolicies records what the policies watching this gate decided.
// Failure to evaluate is never fatal: this observes, it does not control.
func (e *Executor) evaluatePolicies(runState *RunState, stage Stage) {
	if len(e.policies) == 0 || stage.Gate == "" {
		return
	}
	decision := policy.Decide(e.policies, e.gateContext(runState, stage))
	if len(decision.Policies) == 0 {
		return
	}
	runState.PolicyDecisions = append(runState.PolicyDecisions, newPolicyRecord(decision))
	if e.onPolicy != nil {
		e.onPolicy(decision)
	}
	_ = e.emit(policyEvent(decision))
}

// PolicyContextFor builds a gate context from persisted state alone, for
// dry-running policies against a run that already finished. It reads the
// same facts a live gate would, so a dry-run and a real evaluation cannot
// disagree about what was visible.
func PolicyContextFor(store *StateStore, runState *RunState, gate policy.GateID) policy.GateContext {
	executor := &Executor{store: store}
	return executor.gateContext(runState, Stage{Gate: string(gate)})
}

// gateContext assembles the facts the executor can actually see. Five of
// the nine declared condition fields have no source in run state — diff
// size and type, dry-run status, fitness-function results, and whether the
// review reported a behaviour change — so they are simply absent, and a
// check against them resolves to UNKNOWN rather than to a guess.
func (e *Executor) gateContext(runState *RunState, stage Stage) policy.GateContext {
	context := policy.GateContext{Gate: policy.GateID(stage.Gate)}
	e.addReviewFacts(runState, &context)
	e.addSecurityFacts(runState, &context)
	e.addQAFacts(runState, &context)
	e.addPathFacts(runState, &context)
	return context
}

func (e *Executor) addReviewFacts(runState *RunState, context *policy.GateContext) {
	review, ok := readStageState[*state.ReviewState](e, runState, state.KindReview)
	if !ok {
		return
	}
	verdict := string(review.Verdict)
	context.ReviewVerdict = &verdict
	// Copied rather than aliased: the decoded document is short-lived and a
	// pointer into it would outlive what it describes.
	if review.BehaviorChange != nil {
		changed := *review.BehaviorChange
		context.ReviewBehaviorChange = &changed
	}
}

func (e *Executor) addSecurityFacts(runState *RunState, context *policy.GateContext) {
	security, ok := readStageState[*state.SecurityState](e, runState, state.KindSecurity)
	if !ok {
		return
	}
	criticals := 0
	for _, finding := range security.Findings {
		if finding.Severity == state.SeverityCritical {
			criticals++
		}
	}
	context.SecurityCriticals = &criticals
}

func (e *Executor) addQAFacts(runState *RunState, context *policy.GateContext) {
	qa, ok := readStageState[*state.QAState](e, runState, state.KindQA)
	if !ok {
		return
	}
	passing := qa.TestResults.Failed == 0
	context.TestsPass = &passing
}

// addPathFacts sets PathsKnown separately from the slice, because "changed
// no files" and "no implementation state exists" are different facts and an
// allMatch over an empty set is vacuously true.
func (e *Executor) addPathFacts(runState *RunState, context *policy.GateContext) {
	implementation, ok := readStageState[*state.ImplementationState](e, runState, state.KindImplementation)
	if !ok {
		return
	}
	context.PathsKnown = true
	context.ChangedPaths = append(append([]string{},
		implementation.FilesCreated...), implementation.FilesModified...)
}

// readStageState loads the typed document a completed stage produced, by
// kind. It returns false whenever the fact simply is not there — the stage
// was routed out, has not run, or its file is unreadable — because a policy
// deciding on state it could not read is exactly the failure mode UNKNOWN
// exists to prevent.
func readStageState[T state.Validatable](e *Executor, runState *RunState, kind state.Kind) (T, bool) {
	var zero T
	stageID, ok := completedStageOfKind(runState, kind)
	if !ok {
		return zero, false
	}
	raw, err := os.ReadFile(typedStatePath(e.workspaceDir(), stageID))
	if err != nil {
		return zero, false
	}
	document, err := state.Decode(kind, raw)
	if err != nil {
		return zero, false
	}
	typed, ok := document.(T)
	return typed, ok
}

// completedStageOfKind finds which completed stage produced a kind. The
// plan is not in hand at a gate, so this reads the record the executor
// wrote — StateKind is recorded there for exactly this reason.
func completedStageOfKind(runState *RunState, kind state.Kind) (string, bool) {
	for _, stageID := range runState.StagesInSequence() {
		record := runState.Stages[stageID]
		if record.Status == StageStatusCompleted && record.StateKind == string(kind) {
			return stageID, true
		}
	}
	return "", false
}
