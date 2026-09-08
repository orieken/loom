package state_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/state"
)

// routableStages mirrors the built-in plan's routable set: the review
// stages are present but never skippable.
func routableStages() []state.RoutableStage {
	return []state.RoutableStage{
		{ID: "context-engineer"},
		{ID: "analyst"},
		{ID: "architect", Skippable: true},
		{ID: "performance-engineer", Skippable: true},
		{ID: "data-engineer", Skippable: true},
		{ID: "developer"},
		{ID: "code-reviewer"},
		{ID: "accessibility-engineer", Skippable: true},
		{ID: "security-reviewer"},
		{ID: "qa-engineer"},
		{ID: "visual-qa-engineer", Skippable: true},
		{ID: "sre-engineer", Skippable: true},
		{ID: "tech-writer"},
		{ID: "devops-engineer", Skippable: true},
	}
}

// minimalAnalysis is a feature that needs none of the optional stages: no
// crossing, no migration, no dependency, no threshold, no a11y requirement,
// no devops tasks.
func minimalAnalysis() state.AnalysisState {
	analysis := validAnalysis()
	analysis.BoundedContext.Crossings = nil
	analysis.DataModelChanges = nil
	analysis.NonFunctionalRequirements = nil
	analysis.NewDependencies = nil
	analysis.ArchitecturalFlags = nil
	analysis.Tasks = state.TaskList{Developer: []string{"do the thing"}}
	analysis.Surfaces = state.Surfaces{}
	analysis.APIChanges = nil
	return analysis
}

func TestRouteSkipsWhatTheAnalysisDoesNotAskFor(t *testing.T) {
	route := state.RouteFor(minimalAnalysis(), routableStages())

	for _, stage := range []string{"architect", "performance-engineer", "data-engineer",
		"accessibility-engineer", "visual-qa-engineer", "sre-engineer", "devops-engineer"} {
		if route.Includes(stage) {
			t.Errorf("stage %q was included for a feature that asks for none of it", stage)
		}
		if route.ReasonFor(stage) == "" {
			t.Errorf("stage %q was skipped with no recorded reason", stage)
		}
	}
	if err := route.Validate(); err != nil {
		t.Errorf("route invalid: %v", err)
	}
}

// TestReviewStagesAreNeverSkipped is the safety property: automation gets
// the cheap half of the asymmetry, never the expensive half.
func TestReviewStagesAreNeverSkipped(t *testing.T) {
	route := state.RouteFor(minimalAnalysis(), routableStages())

	// sre-engineer left this list in L3.24. It is not a review stage in the
	// asymmetry sense — skipping it risks no unreviewed code — and being
	// unskippable was where it happened to land, not a property it earned.
	for _, stage := range []string{"code-reviewer", "security-reviewer", "qa-engineer", "developer", "tech-writer"} {
		if !route.Includes(stage) {
			t.Errorf("stage %q was skipped; it is not skippable by routing", stage)
		}
	}
}

// TestVisualQAStillRunsForAUIFeature keeps the ADR-007 boundary honest while
// L3.24 narrows the stage.
//
// ADR-007 says visual-qa-engineer's real precondition is "a UI evidence
// bundle for this version is available" — an environmental fact, and one the
// ADR itself records as not implemented. L3.24 does not implement it. It
// applies the necessary condition that IS knowable from the analysis: a
// feature with no UI surface can never produce a bundle, so the stage cannot
// have anything to do. That is strictly narrower than "always runs" and
// strictly wider than the bundle check, so it cannot skip a run the bundle
// check would have included.
func TestVisualQAStillRunsForAUIFeature(t *testing.T) {
	analysis := minimalAnalysis()
	analysis.Surfaces.UI = true

	route := state.RouteFor(analysis, routableStages())

	if !route.Includes("visual-qa-engineer") {
		t.Error("visual-qa-engineer was routed out of a feature that declares a UI surface")
	}
}

func TestRouteIncludesWhatTheAnalysisAsksFor(t *testing.T) {
	cases := map[string]struct {
		stage string
		shape func(*state.AnalysisState)
	}{
		"crossing summons the architect": {"architect", func(a *state.AnalysisState) {
			a.BoundedContext.Crossings = []string{"billing"}
		}},
		"threshold summons performance": {"performance-engineer", func(a *state.AnalysisState) {
			a.NonFunctionalRequirements = []state.NonFunctionalRequirement{
				{Category: "performance", Requirement: "fast", Threshold: &state.Threshold{Metric: "p99 sign-in latency", Value: 200, Unit: "ms"}},
			}
		}},
		"migration summons data": {"data-engineer", func(a *state.AnalysisState) {
			a.DataModelChanges = []state.DataModelChange{{Description: "add table", Phase: state.MigrationPhaseExpand}}
		}},
		"a11y requirement summons accessibility": {"accessibility-engineer", func(a *state.AnalysisState) {
			a.NonFunctionalRequirements = []state.NonFunctionalRequirement{
				{Category: "accessibility", Requirement: "keyboard navigable"},
			}
		}},
		"devops tasks summon devops": {"devops-engineer", func(a *state.AnalysisState) {
			a.Tasks.DevOps = []string{"add a CI job"}
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			analysis := minimalAnalysis()
			tc.shape(&analysis)
			route := state.RouteFor(analysis, routableStages())
			if !route.Includes(tc.stage) {
				t.Errorf("stage %q was skipped: %s", tc.stage, route.ReasonFor(tc.stage))
			}
		})
	}
}

func TestRouteIsDeterministic(t *testing.T) {
	first := state.RouteFor(minimalAnalysis(), routableStages())
	second := state.RouteFor(minimalAnalysis(), routableStages())

	if len(first.Decisions) != len(second.Decisions) {
		t.Fatalf("route length differs between runs")
	}
	for i := range first.Decisions {
		if first.Decisions[i] != second.Decisions[i] {
			t.Fatalf("decision %d differs between runs: %+v vs %+v", i, first.Decisions[i], second.Decisions[i])
		}
	}
}

func TestUnroutedStageRuns(t *testing.T) {
	route := state.RouteFor(minimalAnalysis(), routableStages())

	if !route.Includes("some-stage-nobody-routed") {
		t.Error("a stage the route does not mention should run; an unrouted stage is not a skipped one")
	}
}

func TestRouteValidationRequiresAReasonForEveryDecision(t *testing.T) {
	route := state.Route{
		SchemaVersion: state.SchemaVersion, Feature: "user-auth",
		Decisions: []state.RouteDecision{{Stage: "devops-engineer", Included: false}},
	}

	err := route.Validate()

	if err == nil || !strings.Contains(err.Error(), "reason") {
		t.Errorf("error = %v, want a refusal naming the missing reason", err)
	}
}

func TestRouteRendersTheDecisionAndItsConsequence(t *testing.T) {
	route := state.RouteFor(minimalAnalysis(), routableStages())
	raw, err := json.Marshal(route)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	name, body, err := state.RenderView(state.KindRoute, raw)
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}

	if name != "route.md" {
		t.Errorf("rendered to %q, want route.md", name)
	}
	for _, want := range []string{"devops-engineer", "no DevOps tasks", "resets the design gate's approval"} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered route missing %q:\n%s", want, body)
		}
	}
}

func TestRouteRoundTripsThroughItsSchema(t *testing.T) {
	raw, err := json.Marshal(state.RouteFor(minimalAnalysis(), routableStages()))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	decoded, err := state.Decode(state.KindRoute, raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	route, ok := decoded.(*state.Route)
	if !ok || len(route.Decisions) != len(routableStages()) {
		t.Errorf("decoded route = %+v", decoded)
	}
}

// consoleLogFilteringAnalysis is the third real end-to-end run's feature: a
// three-line synchronous array filter added to an existing class. No UI, no
// I/O, no network, and one boilerplate NFR sentence the analyst writes every
// time. Four stages ran on it and reported having nothing to do, for $2.63.
func consoleLogFilteringAnalysis() state.AnalysisState {
	analysis := minimalAnalysis()
	// The run put its prose IN the threshold field — "O(n) over the captured
	// logs array... no I/O" — and a non-empty test read that as a measurable
	// target. The typed form cannot hold a sentence, so the faithful
	// equivalent is a threshold naming a metric with no number and no unit.
	analysis.NonFunctionalRequirements = []state.NonFunctionalRequirement{
		{Category: "performance", Requirement: "filtering is linear in the captured logs array",
			Threshold: &state.Threshold{Metric: "O(n) over the captured logs array; no I/O"}},
	}
	analysis.Tasks = state.TaskList{
		Developer: []string{"add getLogsByType to ConsoleLogger"},
		DevOps:    []string{"None required by this spec — no CI or deployment config changes requested."},
	}
	return analysis
}

// The L3.24 done-when, stated as the run that produced it.
func TestTheThirdRunsFeatureRoutesInNoStageWithNothingToDo(t *testing.T) {
	route := state.RouteFor(consoleLogFilteringAnalysis(), routableStages())

	for _, stage := range []string{"architect", "performance-engineer", "visual-qa-engineer", "sre-engineer"} {
		if route.Includes(stage) {
			t.Errorf("stage %q ran on a three-line array filter: %s", stage, route.ReasonFor(stage))
		}
	}
}

// The L3.18 done-when. A prose "none" is indistinguishable from work under an
// arity test, and models write prose "none" constantly. The second real run
// spent $0.64 on this exact sentence.
func TestAProseNoneIsNotADevOpsTask(t *testing.T) {
	route := state.RouteFor(consoleLogFilteringAnalysis(), routableStages())

	if route.Includes("devops-engineer") {
		t.Error(`devops-engineer ran on a task list whose only entry says "None required by this spec"`)
	}
}

// The other half of the L3.24 done-when: narrowing must not blind the router
// to work that is really there.
func TestARealThresholdStillSummonsThePerformanceEngineer(t *testing.T) {
	analysis := minimalAnalysis()
	analysis.NonFunctionalRequirements = []state.NonFunctionalRequirement{
		{Category: "performance", Requirement: "search responds quickly",
			Threshold: &state.Threshold{Metric: "p99 search latency", Value: 200, Unit: "ms"}},
	}

	route := state.RouteFor(analysis, routableStages())

	for _, stage := range []string{"performance-engineer", "architect"} {
		if !route.Includes(stage) {
			t.Errorf("stage %q was skipped despite a real p99 200ms budget", stage)
		}
	}
}

// A threshold missing its number or its unit is prose wearing a struct, and
// must not route two stages in on the strength of being non-empty.
func TestAThresholdWithoutANumberIsNotMeasurable(t *testing.T) {
	cases := map[string]*state.Threshold{
		"no value":  {Metric: "latency", Unit: "ms"},
		"no unit":   {Metric: "latency", Value: 200},
		"no metric": {Value: 200, Unit: "ms"},
	}
	for name, threshold := range cases {
		t.Run(name, func(t *testing.T) {
			analysis := minimalAnalysis()
			analysis.NonFunctionalRequirements = []state.NonFunctionalRequirement{
				{Category: "performance", Requirement: "fast", Threshold: threshold},
			}
			if state.RouteFor(analysis, routableStages()).Includes("performance-engineer") {
				t.Errorf("an unmeasurable threshold (%s) routed the performance-engineer in", name)
			}
		})
	}
}

// The two stages that answer "is there a UI?" must not disagree — one being
// skipped while the other could not be is the inconsistency L3.24 records.
func TestTheTwoUIStagesAlwaysAgree(t *testing.T) {
	for _, hasUI := range []bool{false, true} {
		analysis := minimalAnalysis()
		analysis.Surfaces.UI = hasUI
		route := state.RouteFor(analysis, routableStages())

		if route.Includes("accessibility-engineer") != route.Includes("visual-qa-engineer") {
			t.Errorf("UI surface %v: accessibility-engineer=%v but visual-qa-engineer=%v",
				hasUI, route.Includes("accessibility-engineer"), route.Includes("visual-qa-engineer"))
		}
	}
}

func TestASurfaceSummonsItsReviewer(t *testing.T) {
	cases := map[string]struct {
		stage string
		shape func(*state.AnalysisState)
	}{
		"a UI surface summons visual QA": {"visual-qa-engineer", func(a *state.AnalysisState) {
			a.Surfaces.UI = true
		}},
		"an accessibility requirement still summons visual QA": {"visual-qa-engineer", func(a *state.AnalysisState) {
			a.NonFunctionalRequirements = []state.NonFunctionalRequirement{
				{Category: "accessibility", Requirement: "keyboard navigable"},
			}
		}},
		"a served runtime summons the SRE": {"sre-engineer", func(a *state.AnalysisState) {
			a.Surfaces.Runtime = true
		}},
		"a runtime surface summons the SRE even for a library API": {"sre-engineer", func(a *state.AnalysisState) {
			a.Surfaces.Runtime = true
			a.APIChanges = []state.APIChange{{Endpoint: "POST /sessions", Change: "new"}}
		}},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			analysis := minimalAnalysis()
			testCase.shape(&analysis)
			if !state.RouteFor(analysis, routableStages()).Includes(testCase.stage) {
				t.Errorf("stage %q was skipped for a feature that needs it", testCase.stage)
			}
		})
	}
}

// A real DevOps task must still route the stage in.
func TestARealDevOpsTaskStillSummonsDevOps(t *testing.T) {
	analysis := minimalAnalysis()
	analysis.Tasks.DevOps = []string{"Add a nightly workflow that publishes the bundle"}

	if !state.RouteFor(analysis, routableStages()).Includes("devops-engineer") {
		t.Error("devops-engineer was skipped despite a real task")
	}
}

// Run 4's counter-example. The SRE predicate used to OR the declared runtime
// surface with "the analysis changes an API", which read as a reasonable
// proxy and is not one: the analyst recorded a TypeScript class method as an
// API change on a package with no served surface, and the route file then
// reported a runtime surface the same document denied.
func TestALibraryMethodDoesNotSummonTheSRE(t *testing.T) {
	analysis := minimalAnalysis()
	analysis.Surfaces.Runtime = false
	analysis.APIChanges = []state.APIChange{{
		Endpoint: "ConsoleLogger.getLogsByType(type: string)",
		Change:   "New public method added to the ConsoleLogger class's public surface.",
	}}

	route := state.RouteFor(analysis, routableStages())

	if route.Includes("sre-engineer") {
		t.Errorf("the SRE was summoned to a package declaring no runtime surface: %s",
			route.ReasonFor("sre-engineer"))
	}
}

// Run 4's other counter-example, and the reason L3.18's fix was incomplete.
// The analyst wrote its "no architect needed" conclusion INTO the flag list,
// and len() > 0 read it as a flag demanding one. The architect then ran on a
// three-line method, failed, and halted Experiment A at stage 4.
func TestAProseNoneIsNotAnArchitecturalFlag(t *testing.T) {
	analysis := minimalAnalysis()
	analysis.ArchitecturalFlags = []string{
		"None — purely additive method on an existing class following the established " +
			"getLogs()/clear() pattern; no new package, base class, layer boundary change, or " +
			"cross-cutting concern introduced. Architect step can be skipped.",
	}

	route := state.RouteFor(analysis, routableStages())

	if route.Includes("architect") {
		t.Errorf("the architect was summoned by a flag that says it can be skipped: %s",
			route.ReasonFor("architect"))
	}
}

// A prose "none" in the dependency list must not summon an architect either.
// Every list the router counts goes through the same filter now.
func TestAProseNoneIsNotADependency(t *testing.T) {
	analysis := minimalAnalysis()
	analysis.NewDependencies = []string{"None required"}

	if state.RouteFor(analysis, routableStages()).Includes("architect") {
		t.Error("the architect was summoned by a dependency list that says there are none")
	}
}

// A real architectural flag still summons the architect, and the route file
// names the fact that fired rather than the whole disjunction (L3.34).
func TestTheRouteNamesTheFactThatSummonedTheArchitect(t *testing.T) {
	cases := map[string]struct {
		shape func(*state.AnalysisState)
		want  string
	}{
		"flag":       {func(a *state.AnalysisState) { a.ArchitecturalFlags = []string{"reverses ADR-004"} }, "architectural flag"},
		"crossing":   {func(a *state.AnalysisState) { a.BoundedContext.Crossings = []string{"billing"} }, "bounded context"},
		"dependency": {func(a *state.AnalysisState) { a.NewDependencies = []string{"github.com/some/queue"} }, "dependency"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			analysis := minimalAnalysis()
			testCase.shape(&analysis)
			route := state.RouteFor(analysis, routableStages())

			if !route.Includes("architect") {
				t.Fatal("the architect was skipped for real structural work")
			}
			reason := route.ReasonFor("architect")
			if !strings.Contains(reason, testCase.want) {
				t.Errorf("reason %q does not name the fact that fired (%q)", reason, testCase.want)
			}
			if strings.Contains(reason, " or ") {
				t.Errorf("reason %q is a disjunction; it must name the disjunct that fired", reason)
			}
		})
	}
}
