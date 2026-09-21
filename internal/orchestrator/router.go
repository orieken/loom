package orchestrator

// The router (roadmap L3.0) is an executor-internal stage: it reads the
// analyst's typed state and decides which of the remaining stages run,
// recording one reason per decision. It runs after the analyst — the
// earliest point those facts exist — and before the design gate, so a human
// approving the design also approves the route, and editing the route
// resets that approval through the ordinary L2.14 binding.

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/orieken/loom/internal/state"
)

// runInternalStage dispatches a stage the executor runs itself. Only the
// router exists today; a second one should make this a registry rather than
// a switch.
func (e *Executor) runInternalStage(stage Stage, plan Plan, input StageInput) (StageOutput, error) {
	if stage.ID != RouterStageID {
		return StageOutput{}, fmt.Errorf("stage %q is marked internal but the executor has no routine for it", stage.ID)
	}
	return computeRoute(plan, input)
}

// computeRoute reads the analysis and returns the route as a typed payload,
// which the ordinary typed-stage path then validates, digests, and renders.
func computeRoute(plan Plan, input StageInput) (StageOutput, error) {
	analysis, err := readAnalysis(input)
	if err != nil {
		return StageOutput{}, err
	}
	route := state.RouteFor(*analysis, routableStagesOf(plan))
	// The route describes THIS RUN, so it is titled from the run's feature
	// rather than the analyst's payload (roadmap L3.20, papercut 2). Under
	// the mock the analysis says `mock-feature`, and a route document for
	// `health-endpoint` was headed "Delivery Route: mock-feature" — a
	// document naming a feature the run is not about.
	if feature := FeatureNameFromSpec(input.SpecPath); feature != "" {
		route.Feature = feature
	}
	payload, err := json.Marshal(route)
	if err != nil {
		return StageOutput{}, fmt.Errorf("encode route: %w", err)
	}
	return StageOutput{Payload: payload}, nil
}

func readAnalysis(input StageInput) (*state.AnalysisState, error) {
	raw, err := os.ReadFile(typedStatePath(input.WorkspaceDir, "analyst"))
	if err != nil {
		return nil, fmt.Errorf("router needs the analyst's state: %w", err)
	}
	decoded, err := state.Decode(state.KindAnalysis, raw)
	if err != nil {
		return nil, fmt.Errorf("router cannot read the analysis: %w", err)
	}
	analysis, ok := decoded.(*state.AnalysisState)
	if !ok {
		return nil, fmt.Errorf("router read something that is not an analysis")
	}
	return analysis, nil
}

// routableStagesOf lists the stages the router decides on: everything after
// the router itself. Stages before it have already run.
func routableStagesOf(plan Plan) []state.RoutableStage {
	stages := make([]state.RoutableStage, 0, len(plan.Stages))
	seenRouter := false
	for _, stage := range plan.Stages {
		if stage.ID == RouterStageID {
			seenRouter = true
			continue
		}
		if seenRouter {
			stages = append(stages, state.RoutableStage{ID: stage.ID, Skippable: stage.Skippable})
		}
	}
	return stages
}

// applyRoute records every skipped stage immediately, so the shape of the
// run is visible before the developer stage starts rather than discovered
// one stage at a time.
//
// A stage that still counts as completed stays completed even when a
// recomputed route would skip it: the work happened and its artifact is
// real. In practice this rarely fires — a route only changes because the
// analysis changed, and that demotes everything recorded after it — but
// the rule is stated here rather than left to the cascade's timing.
//
// The reverse also has to hold: a stage the new route now includes must
// lose an earlier SKIPPED record, or a reroute could never bring work back.
func (e *Executor) applyRoute(plan Plan, input StageInput, runState *RunState) error {
	route, err := readRoute(input)
	if err != nil {
		return err
	}
	for _, stage := range plan.Stages {
		if err := e.applyRouteToStage(stage, *route, runState); err != nil {
			return err
		}
	}
	if e.onRoute != nil {
		e.onRoute(summarize(*route))
	}
	return nil
}

// RouteSummary is the one-line view of a routing decision: how much of the
// plan runs, and what was routed out. The reasons live in the rendered
// route document rather than in the terminal, so a halt message is not
// pushed off screen by a table that mostly says "runs as usual".
type RouteSummary struct {
	Included int
	Total    int
	Skipped  []string
}

func summarize(route state.Route) RouteSummary {
	summary := RouteSummary{Total: len(route.Decisions)}
	for _, decision := range route.Decisions {
		if decision.Included {
			summary.Included++
			continue
		}
		summary.Skipped = append(summary.Skipped, decision.Stage)
	}
	return summary
}

func (e *Executor) applyRouteToStage(stage Stage, route state.Route, runState *RunState) error {
	if route.Includes(stage.ID) {
		return e.unskip(stage.ID, runState)
	}
	if runState.Stages[stage.ID].Status == StageStatusCompleted {
		return nil
	}
	now := time.Now().UTC()
	record := runState.Stages[stage.ID]
	record.Status = StageStatusSkipped
	record.SkipReason = route.ReasonFor(stage.ID)
	record.FinishedAt = &now
	if err := e.persistStatus(runState, stage.ID, record); err != nil {
		return err
	}
	return e.emit(Event{Kind: EventStageSkipped, Stage: stage.ID,
		Sequence: runState.Stages[stage.ID].Sequence, Reason: record.SkipReason})
}

// unskip clears a SKIPPED record so a stage the recomputed route now
// includes actually runs. Anything other than SKIPPED is left alone.
func (e *Executor) unskip(stageID string, runState *RunState) error {
	record := runState.Stages[stageID]
	if record.Status != StageStatusSkipped {
		return nil
	}
	record.Status = ""
	record.SkipReason = ""
	record.FinishedAt = nil
	return e.persistStatus(runState, stageID, record)
}

func readRoute(input StageInput) (*state.Route, error) {
	raw, err := os.ReadFile(typedStatePath(input.WorkspaceDir, RouterStageID))
	if err != nil {
		return nil, fmt.Errorf("read route: %w", err)
	}
	decoded, err := state.Decode(state.KindRoute, raw)
	if err != nil {
		return nil, fmt.Errorf("read route: %w", err)
	}
	route, ok := decoded.(*state.Route)
	if !ok {
		return nil, fmt.Errorf("routed state is not a route")
	}
	return route, nil
}
