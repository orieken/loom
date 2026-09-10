// Package orchestrator is the minimal pipeline executor decided by ADR-006
// (loom executes pipelines) and roadmap item M0.4. It owns the run loop:
// load a plan, execute stages in order via a Provider, persist durable state,
// halt at approval gates (L2.13), route around stages the analysis does not
// call for (L3.0), and stop. Parallelism and policy evaluation are later
// roadmap items (L3.3, L2.16) that plug into this skeleton.
package orchestrator

import (
	"fmt"
	"strings"
	"time"

	"github.com/orieken/loom/internal/state"
)

// Stage is one step of a Plan. Stages are identified by stable string IDs
// (agent names), never ordinals — hand-numbered step lists are a defect the
// roadmap (L2.15) explicitly retires.
type Stage struct {
	ID    string
	Agent string
	// Gate names the approval barrier guarding entry to this stage. Empty
	// means ungated; a named gate means the executor refuses to start the
	// stage until run state records a human approval for that name.
	Gate string
	// StateKind names the typed state document this stage produces
	// (roadmap L2.9), empty for stages that still write markdown. Typedness
	// is plan data for the same reason gates are: the plan, not an agent's
	// name, decides how a stage exchanges data.
	StateKind string
	// Consumes names the stages whose typed state is projected into this
	// stage's input. Empty when the stage reads no upstream state; more
	// than one when its contract says it reads more than one.
	Consumes []string
	// Skippable marks a stage the router may route around (roadmap L3.0).
	// The review stages are deliberately not skippable: an unnecessary
	// devops run wastes an invocation, a skipped security review does not
	// fail so cheaply.
	Skippable bool
	// Internal marks a stage the executor runs itself rather than invoking
	// a provider — the router is the only one today. An internal stage is
	// still a stage: it has a record, a sequence, a digest, and is bound by
	// the next gate like any other.
	Internal bool
	Timeout  time.Duration
}

// Plan is an ordered list of stages the executor runs sequentially, plus
// any bounded loops over spans of them (roadmap L2.17).
type Plan struct {
	Name   string
	Stages []Stage
	Loops  []Loop
}

// Validate rejects plans the executor cannot run safely: empty names,
// stages without IDs, or duplicate stage IDs (state is keyed by stage ID,
// so duplicates would silently overwrite each other's records).
func (p Plan) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("plan has no name")
	}
	if len(p.Stages) == 0 {
		return fmt.Errorf("plan %q has no stages", p.Name)
	}
	if err := p.validateStageIDs(); err != nil {
		return err
	}
	return p.validateGateNames()
}

func (p Plan) validateGateNames() error {
	for _, stage := range p.Stages {
		if stage.Gate != "" && strings.TrimSpace(stage.Gate) == "" {
			return fmt.Errorf("plan %q stage %q has a blank gate name", p.Name, stage.ID)
		}
	}
	return nil
}

func (p Plan) validateStageIDs() error {
	seen := make(map[string]bool, len(p.Stages))
	for _, stage := range p.Stages {
		if stage.ID == "" {
			return fmt.Errorf("plan %q has a stage with an empty ID", p.Name)
		}
		if seen[stage.ID] {
			return fmt.Errorf("plan %q has duplicate stage ID %q", p.Name, stage.ID)
		}
		seen[stage.ID] = true
	}
	return nil
}

// Gate names of the built-in plan, mirroring the human PAUSE checkpoints in
// shared/skills/deliver-feature/SKILL.md (steps 11, 13, 25 / Phase 4).
const (
	GateConfirmDesign   = "confirm-design"
	GateConfirmSecurity = "confirm-security"
	GateConfirmShip     = "confirm-ship"
	// GateConfirmUnresolvedReview halts a run whose review loop hit its
	// bound with changes still requested. It has no prose counterpart: the
	// markdown pipeline's loop has no bound to exhaust.
	GateConfirmUnresolvedReview = "confirm-unresolved-review"
)

// defaultPlanGates maps the three gated stages of the built-in plan to their
// gate names: design is confirmed before code is written, security before QA
// signs off, and the ship gate before anything touches infrastructure.
func defaultPlanGates() map[string]string {
	return map[string]string{
		"developer":       GateConfirmDesign,
		"qa-engineer":     GateConfirmSecurity,
		"devops-engineer": GateConfirmShip,
	}
}

// RouterStageID names the executor-internal stage that computes the route
// from the analysis (roadmap L3.0). It sits after the analyst — the
// earliest point the facts exist — and before the design gate, so the human
// approves the route along with the design.
const RouterStageID = "router"

// defaultSkippableStages declares which stages the router may route around.
// Everything absent from this map always runs. The review stages
// (code-reviewer, security-reviewer) are absent deliberately: an unnecessary
// review wastes an invocation, a skipped one does not fail so cheaply.
//
// visual-qa-engineer and sre-engineer joined the list in L3.24. Being
// unskippable was never a property either had earned — it was where they
// happened to land — and it cost $1.18 on a three-line array filter, in a
// run where accessibility-engineer was correctly skipped for having no UI.
// A stage that can only review a surface should not run when the surface is
// absent, whichever stage it is.
func defaultSkippableStages() map[string]bool {
	return map[string]bool{
		"architect":              true,
		"performance-engineer":   true,
		"data-engineer":          true,
		"accessibility-engineer": true,
		"visual-qa-engineer":     true,
		"sre-engineer":           true,
		"devops-engineer":        true,
	}
}

// defaultTypedStages declares which stages of the built-in plan exchange
// typed state and where each reads its input from. L2.9's first cut types
// the analyst -> architect hop; every other stage still writes markdown.
func defaultTypedStages() (kinds map[string]string, consumes map[string][]string) {
	return map[string]string{
		"context-engineer":  string(state.KindContext),
		"analyst":           string(state.KindAnalysis),
		RouterStageID:       string(state.KindRoute),
		"architect":         string(state.KindArchitecture),
		"developer":         string(state.KindImplementation),
		"code-reviewer":     string(state.KindReview),
		"security-reviewer": string(state.KindSecurity),
		"qa-engineer":       string(state.KindQA),
	}, map[string][]string{
		// The router deliberately has no projection: projections exist to
		// narrow what a *model* is shown, and the router is the executor
		// reading its own state. It loads the analysis in full itself.
		"architect": {"analyst"},
		// On a second round the developer reads the reviewer's findings.
		"developer":         {"code-reviewer"},
		"security-reviewer": {"developer"},
		// Three upstreams, per qa-contract.md's "Consumed by" line: what
		// was built, what security found, and the criteria to test against.
		"qa-engineer": {"developer", "security-reviewer", "analyst"},
		// tech-writer produces markdown in this cut but still reads typed
		// state — what a stage reads and what it writes vary separately.
		"tech-writer": {"qa-engineer", "analyst"},
	}
}

// BuiltInStages returns every stage the framework ships, keyed by ID, fully
// configured — gate, typed state kind, upstream reads, skippability and
// timeout.
//
// It is the catalogue a plan file selects from (roadmap L3.27). Plans pick
// and order these; they do not redefine them. That is the whole safety
// property: a plan cannot drop a stage's gate, un-type its contract, or make
// a routed stage unconditional, because it never states any of those things.
// L3.24 measured what an always-runs stage costs on a feature it cannot
// serve, and a format that let each project re-declare skippability would
// hand that bill back to every project that wrote a plan.
func BuiltInStages() map[string]Stage {
	catalogue := make(map[string]Stage)
	for _, stage := range DefaultDeliverFeaturePlan().Stages {
		catalogue[stage.ID] = stage
	}
	return catalogue
}

// BuiltInLoops returns the loops of the built-in plan, keyed by ID, so a
// plan file can name one rather than restating its bound. L2.17 put a number
// on the review loop because prose said "repeat until APPROVED"; a format
// that let a project write its own bound would hand that back too.
func BuiltInLoops() map[string]Loop {
	loops := make(map[string]Loop)
	for _, loop := range DefaultDeliverFeaturePlan().Loops {
		loops[loop.ID] = loop
	}
	return loops
}

// DefaultDeliverFeaturePlanName names the built-in plan.
const DefaultDeliverFeaturePlanName = "deliver-feature"

// defaultStageTimeout bounds each stage invocation. Every network or
// subprocess call must have an explicit timeout (architecture-guardrails.md
// #5); per-stage overrides belong in the plan, not in provider internals.
const defaultStageTimeout = 30 * time.Minute

// DefaultDeliverFeaturePlan returns the built-in plan: the agent sequence
// from shared/skills/deliver-feature/SKILL.md in invocation order, plus the
// executor-internal router after the analyst. Which of the conditional
// stages actually run is decided by the router from typed analysis (roadmap
// L3.0), not by the plan — the plan only declares which stages *may* be
// routed around.
func DefaultDeliverFeaturePlan() Plan {
	gates := defaultPlanGates()
	kinds, consumes := defaultTypedStages()
	skippable := defaultSkippableStages()
	agents := []string{
		"context-engineer",
		"analyst",
		RouterStageID,
		"architect",
		"performance-engineer",
		"data-engineer",
		"developer",
		"code-reviewer",
		"accessibility-engineer",
		"security-reviewer",
		"qa-engineer",
		"visual-qa-engineer",
		"sre-engineer",
		"tech-writer",
		"devops-engineer",
	}
	stages := make([]Stage, 0, len(agents))
	for _, agent := range agents {
		stages = append(stages, Stage{ID: agent, Agent: agent, Gate: gates[agent],
			StateKind: kinds[agent], Consumes: consumes[agent], Skippable: skippable[agent],
			Internal: agent == RouterStageID, Timeout: defaultStageTimeout})
	}
	return Plan{Name: DefaultDeliverFeaturePlanName, Stages: stages, Loops: defaultPlanLoops()}
}

// defaultPlanLoops encodes deliver-feature steps 18–21: the code-reviewer
// sends the developer back until it approves. The prose states no bound —
// "repeat until APPROVED" — so this puts a number on it, because a loop the
// executor cannot bound is one it cannot safely run.
func defaultPlanLoops() []Loop {
	return []Loop{{
		ID: "review", From: "developer", To: "code-reviewer",
		Condition: ReviewApprovedCondition, Gate: GateConfirmUnresolvedReview,
		MaxIterations: defaultReviewIterations,
	}}
}

// defaultReviewIterations bounds the review loop. Three rounds matches the
// Tier B contract-retry default in deliver-feature's own policy block.
const defaultReviewIterations = 3
