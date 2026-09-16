package mock

// Typed stages (roadmap L2.9) return validated state documents rather than
// markdown, so the mock needs a valid document per typed stage to script a
// deterministic run.

import (
	"encoding/json"

	"github.com/orieken/loom/internal/state"
)

// TypedScript returns a scripted payload for a state kind, and false for a
// stage that still exchanges markdown.
func TypedScript(kind string) (Script, bool) {
	payload, typed := typedPayload(state.Kind(kind))
	if !typed {
		return Script{}, false
	}
	return Script{Payload: payload}, true
}

// typedPayloads maps a state kind to the scripted document the mock
// returns for it. A table rather than a switch: it is data, and it gains an
// entry with every stage that becomes typed.
func typedPayloads() map[state.Kind]func() interface{} {
	return map[state.Kind]func() interface{}{
		state.KindContext:        func() interface{} { return sampleContext() },
		state.KindAnalysis:       func() interface{} { return sampleAnalysis() },
		state.KindArchitecture:   func() interface{} { return sampleArchitecture() },
		state.KindReview:         func() interface{} { return SampleReview(state.VerdictApproved) },
		state.KindImplementation: func() interface{} { return sampleImplementation() },
		state.KindSecurity:       func() interface{} { return sampleSecurity() },
		state.KindQA:             func() interface{} { return sampleQA() },
	}
}

func typedPayload(kind state.Kind) ([]byte, bool) {
	build, scripted := typedPayloads()[kind]
	if !scripted {
		return nil, false
	}
	return mustEncode(build()), true
}

// mustEncode panics only on a programming error: these values are compiled
// in, so a failure means the structs and this file disagree, which a test
// catches immediately.
func mustEncode(value interface{}) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		panic("mock: cannot encode scripted state: " + err.Error())
	}
	return raw
}

// SampleReview builds a scripted review with the given verdict. A
// changes-requested review carries a blocking finding, because the schema
// requires one — a rejection with nothing actionable would spin the loop.
func SampleReview(verdict state.Verdict) state.ReviewState {
	review := state.ReviewState{
		SchemaVersion:   state.SchemaVersion,
		Feature:         "mock-feature",
		Verdict:         verdict,
		DesignNarrative: "Scripted review produced by the mock provider.",
		DesignScore:     state.DesignScore{Clarity: 4, Cohesion: 4, Coupling: 4, Craft: 4},
		// Required, and deliberately phrased as the "no tests here" answer
		// rather than a YES: the mock reviews a scripted implementation that
		// adds no tests, and a fixture claiming to have judged tests that do
		// not exist is the vacuous assertion this field exists to catch.
		TestDesignReview: []string{"No tests added or modified in this diff."},
	}
	if verdict == state.VerdictChangesRequested {
		review.DesignScore.Cohesion = 2
		review.Findings = []state.Finding{{
			Operation: "Extract Function", File: "internal/mock/thing.go",
			Smell: "does two things", Instruction: "split it", Blocking: true,
		}}
	}
	return review
}

func sampleImplementation() state.ImplementationState {
	return state.ImplementationState{
		SchemaVersion: state.SchemaVersion,
		Feature:       "mock-feature",
		FilesCreated:  []string{"internal/mock/thing.go"},
		SelfReview:    []state.ChecklistItem{{Item: "scripted self-review", Checked: true}},
		SimpleDesign:  []state.ChecklistItem{{Item: "passes the tests", Checked: true}},
	}
}

// sampleSecurity assesses every STRIDE category, because the schema
// requires all six — the mock has to produce a document that validates.
func sampleSecurity() state.SecurityState {
	categories := []state.StrideCategory{
		state.StrideSpoofing, state.StrideTampering, state.StrideRepudiation,
		state.StrideInformationDisclosure, state.StrideDenialOfService, state.StrideElevationOfPrivilege,
	}
	stride := make([]state.StrideAnalysis, 0, len(categories))
	for _, category := range categories {
		stride = append(stride, state.StrideAnalysis{Category: category, Assessed: "scripted: not applicable"})
	}
	return state.SecurityState{
		SchemaVersion:      state.SchemaVersion,
		Feature:            "mock-feature",
		ThreatModelSummary: "Scripted threat model produced by the mock provider.",
		Stride:             stride,
	}
}

func sampleQA() state.QAState {
	return state.QAState{
		SchemaVersion:    state.SchemaVersion,
		Feature:          "mock-feature",
		TestFilesCreated: []string{"internal/mock/thing_test.go"},
		Coverage: state.CoverageSummary{AcceptanceCriteriaCovered: 1, AcceptanceCriteriaTotal: 1, NewTests: 1,
			Statements: []state.PackageCoverage{{Unit: "internal/mock", Percent: 90}}},
		TestResults: state.TestResults{Passed: 1},
	}
}

// sampleContext pins a file the mock cannot promise exists. That is
// deliberate: the executor measures pinned files from disk, and a scripted
// manifest whose file is absent exercises the unmeasurable path rather than
// pretending measurement always succeeds.
func sampleContext() state.ContextState {
	return state.ContextState{
		SchemaVersion: state.SchemaVersion,
		Feature:       "mock-feature",
		Tier:          "analyst",
		TargetStage:   "analyst",
		PinnedFiles:   []state.PinnedFile{{Path: "internal/mock/thing.go", Reason: "scripted"}},
	}
}

func sampleAnalysis() state.AnalysisState {
	return state.AnalysisState{
		SchemaVersion:      state.SchemaVersion,
		Feature:            "mock-feature",
		Summary:            "Scripted analysis produced by the mock provider.",
		AcceptanceCriteria: []state.AcceptanceCriterion{{Statement: "the mocked behaviour happens"}},
		BoundedContext:     state.BoundedContext{Owning: "mock"},
		AffectedComponents: []state.AffectedComponent{{Path: "internal/mock/thing.go", Reason: "scripted"}},
		Tasks:              state.TaskList{Developer: []string{"implement the mocked behaviour"}},
		DefinitionOfDone:   []string{"tests green"},
	}
}

func sampleArchitecture() state.ArchitectureState {
	fitness := state.FitnessFunction{Property: "mock property", Verification: "go test ./..."}
	return state.ArchitectureState{
		SchemaVersion: state.SchemaVersion,
		Feature:       "mock-feature",
		StructuralDecisions: []state.StructuralDecision{
			{Decision: "keep the mock in the adapter layer", Rationale: "scripted", Fitness: &fitness},
		},
		BoundedContext:   state.BoundedContext{Owning: "mock"},
		FitnessFunctions: []state.FitnessFunction{fitness},
	}
}
