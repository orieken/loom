package state

// A Route is the decision about which stages a run will execute, computed
// once from the typed analysis (roadmap L3.0) rather than re-derived by a
// model at each stage. It is a state document like any other, so it is
// digest-recorded, rendered for humans, and — because it completes before
// the design gate — bound to that gate's approval: a human approves the
// route along with the design, and editing the route resets the approval.

import "fmt"

// RouteDecision is one stage's fate and the reason for it. The reason is
// not decoration: "why isn't devops running?" is the question this whole
// item exists to answer, and it must be answerable from the artifact.
type RouteDecision struct {
	Stage    string `json:"stage" jsonschema:"required"`
	Included bool   `json:"included"`
	Reason   string `json:"reason" jsonschema:"required"`
}

// Route is the typed form of the routing decision for one run.
type Route struct {
	SchemaVersion int             `json:"schemaVersion" jsonschema:"required"`
	Feature       string          `json:"feature" jsonschema:"required"`
	Decisions     []RouteDecision `json:"decisions" jsonschema:"required"`
}

// Validate enforces what a consumer of the route cannot work without.
func (r Route) Validate() error {
	return firstError(
		requireSchemaVersion(r.SchemaVersion),
		requireText("feature", r.Feature),
		requireItems("decisions", len(r.Decisions)),
		validateDecisionReasons(r.Decisions),
	)
}

func validateDecisionReasons(decisions []RouteDecision) error {
	for index, decision := range decisions {
		if decision.Stage == "" {
			return &ValidationError{Field: fmt.Sprintf("decisions[%d].stage", index), Reason: "is required and empty"}
		}
		if decision.Reason == "" {
			return &ValidationError{Field: fmt.Sprintf("decisions[%d].reason", index),
				Reason: "is required — a routing decision nobody can explain is not auditable"}
		}
	}
	return nil
}

// Includes reports whether a stage runs under this route. A stage the route
// does not mention runs: an unrouted stage is not a skipped one.
func (r Route) Includes(stageID string) bool {
	for _, decision := range r.Decisions {
		if decision.Stage == stageID {
			return decision.Included
		}
	}
	return true
}

// ReasonFor returns the recorded reason for a stage's fate.
func (r Route) ReasonFor(stageID string) string {
	for _, decision := range r.Decisions {
		if decision.Stage == stageID {
			return decision.Reason
		}
	}
	return ""
}

// RoutableStage is what the router needs to know about one stage of a plan:
// its ID, and whether it may be skipped at all.
type RoutableStage struct {
	ID        string
	Skippable bool
}

// RouteFor computes the route for a plan's stages from the analysis. A
// stage that is not skippable is always included, whatever the predicates
// say — the review stages are excluded from automation because the costs
// are asymmetric: an unnecessary devops run wastes an invocation, a skipped
// security review does not fail so cheaply.
func RouteFor(analysis AnalysisState, stages []RoutableStage) Route {
	decisions := make([]RouteDecision, 0, len(stages))
	for _, stage := range stages {
		decisions = append(decisions, decideStage(analysis, stage))
	}
	return Route{SchemaVersion: SchemaVersion, Feature: analysis.Feature, Decisions: decisions}
}

func decideStage(analysis AnalysisState, stage RoutableStage) RouteDecision {
	if !stage.Skippable {
		return RouteDecision{Stage: stage.ID, Included: true, Reason: "always runs; not skippable by routing"}
	}
	predicate, routable := stagePredicates()[stage.ID]
	if !routable {
		return RouteDecision{Stage: stage.ID, Included: true, Reason: "no routing predicate; runs by default"}
	}
	return predicate(analysis, stage.ID)
}

// stagePredicates maps a stage to the question that decides it. Every
// predicate reads only fields the analysis contract promises.
func stagePredicates() map[string]func(AnalysisState, string) RouteDecision {
	return map[string]func(AnalysisState, string) RouteDecision{
		// The architect's reason names the disjunct that fired, not the
		// disjunction (roadmap L3.34). A route file whose reason is the
		// whole condition cannot tell a reader which fact decided it, and
		// run 4 produced one that was provably false against its own input.
		"architect": func(a AnalysisState, id string) RouteDecision {
			if reason := a.architectReason(); reason != "" {
				return RouteDecision{Stage: id, Included: true, Reason: "structural work: " + reason}
			}
			return RouteDecision{Stage: id, Included: false,
				Reason: "no context crossing, data-model change, new dependency, performance threshold, or architectural flag"}
		},
		"performance-engineer": func(a AnalysisState, id string) RouteDecision {
			return decide(id, a.RequiresPerformanceEngineer(),
				"a performance requirement carries a measurable threshold",
				"no performance requirement carries a threshold with a number and a unit")
		},
		"data-engineer": func(a AnalysisState, id string) RouteDecision {
			return decide(id, a.RequiresDataEngineer(),
				"the analysis declares an expand or contract data-model change",
				"no data-model change to sequence")
		},
		"accessibility-engineer": func(a AnalysisState, id string) RouteDecision {
			return decide(id, a.RequiresAccessibilityEngineer(),
				"the analysis declares a UI surface",
				"the analysis declares no UI surface")
		},
		// Same question as accessibility-engineer, so deliberately the same
		// answer: two stages reviewing the same surface disagreeing about
		// whether it exists is the defect L3.24 records.
		"visual-qa-engineer": func(a AnalysisState, id string) RouteDecision {
			return decide(id, a.RequiresVisualQAEngineer(),
				"the analysis declares a UI surface",
				"the analysis declares no UI surface, so there is nothing to look at")
		},
		"sre-engineer": func(a AnalysisState, id string) RouteDecision {
			return decide(id, a.RequiresSREEngineer(),
				"the analysis declares a served runtime surface",
				"the analysis declares no served runtime surface, so no availability or latency SLI applies")
		},
		"devops-engineer": func(a AnalysisState, id string) RouteDecision {
			return decide(id, a.RequiresDevOpsEngineer(),
				"the analysis lists DevOps tasks",
				"the analysis lists no DevOps tasks")
		},
	}
}

func decide(stageID string, included bool, includedReason, skippedReason string) RouteDecision {
	if included {
		return RouteDecision{Stage: stageID, Included: true, Reason: includedReason}
	}
	return RouteDecision{Stage: stageID, Included: false, Reason: skippedReason}
}
