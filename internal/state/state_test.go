package state_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/state"
)

func validAnalysis() state.AnalysisState {
	return state.AnalysisState{
		SchemaVersion: state.SchemaVersion,
		Feature:       "user-auth",
		Summary:       "Let a returning user sign in with email and password.",
		AcceptanceCriteria: []state.AcceptanceCriterion{
			{Statement: "a registered user signs in with valid credentials", Examples: []string{"ada@example.com / correct-horse"}},
		},
		NonFunctionalRequirements: []state.NonFunctionalRequirement{
			{Category: "performance", Requirement: "sign-in responds quickly", Threshold: &state.Threshold{Metric: "p99 sign-in latency", Value: 200, Unit: "ms"}},
		},
		BoundedContext:     state.BoundedContext{Owning: "identity", Crossings: []string{"notifications"}},
		AffectedComponents: []state.AffectedComponent{{Path: "internal/identity/session.go", Reason: "issues the session"}},
		DataModelChanges:   []state.DataModelChange{{Description: "add sessions table", Phase: state.MigrationPhaseExpand}},
		Tasks:              state.TaskList{Developer: []string{"issue a session on valid credentials"}, QA: []string{"cover lockout"}},
		DefinitionOfDone:   []string{"acceptance criteria met", "tests green"},
	}
}

func validArchitecture() state.ArchitectureState {
	return state.ArchitectureState{
		SchemaVersion: state.SchemaVersion,
		Feature:       "user-auth",
		StructuralDecisions: []state.StructuralDecision{{
			Decision:  "session issuing lives in the use-case layer",
			Rationale: "keeps the domain free of transport concerns",
			Fitness:   &state.FitnessFunction{Property: "domain imports no adapters", Verification: "go test ./tools -run TestDeps"},
		}},
		BoundedContext:   state.BoundedContext{Owning: "identity"},
		FitnessFunctions: []state.FitnessFunction{{Property: "domain imports no adapters", Verification: "go test ./tools -run TestDeps"}},
	}
}

func TestValidAnalysisPassesValidation(t *testing.T) {
	if err := validAnalysis().Validate(); err != nil {
		t.Fatalf("valid analysis rejected: %v", err)
	}
}

func TestAnalysisValidationNamesTheOffendingField(t *testing.T) {
	cases := map[string]func(*state.AnalysisState){
		"feature":            func(a *state.AnalysisState) { a.Feature = "" },
		"summary":            func(a *state.AnalysisState) { a.Summary = "" },
		"acceptanceCriteria": func(a *state.AnalysisState) { a.AcceptanceCriteria = nil },
		"affectedComponents": func(a *state.AnalysisState) { a.AffectedComponents = nil },
		"definitionOfDone":   func(a *state.AnalysisState) { a.DefinitionOfDone = nil },
		"schemaVersion":      func(a *state.AnalysisState) { a.SchemaVersion = 99 },
	}
	for field, breakField := range cases {
		t.Run(field, func(t *testing.T) {
			analysis := validAnalysis()
			breakField(&analysis)
			assertValidationNames(t, analysis.Validate(), field)
		})
	}
}

func assertValidationNames(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatalf("validation passed with %q broken", field)
	}
	if !strings.Contains(err.Error(), field) {
		t.Errorf("error %q does not name the offending field %q", err, field)
	}
}

func TestArchitectureRequiresFitnessFunctionOrJudgmentOnly(t *testing.T) {
	architecture := validArchitecture()
	architecture.StructuralDecisions[0].Fitness = nil

	assertValidationNames(t, architecture.Validate(), "fitness")

	// A decision that cannot be machine-verified is legitimate when it says
	// so explicitly — that is the documented exception, not a loophole.
	architecture.StructuralDecisions[0].JudgmentOnly = true
	if err := architecture.Validate(); err != nil {
		t.Errorf("judgment-only decision rejected: %v", err)
	}
}

func TestArchitectureValidationNamesTheOffendingField(t *testing.T) {
	cases := map[string]func(*state.ArchitectureState){
		"feature":             func(a *state.ArchitectureState) { a.Feature = "" },
		"structuralDecisions": func(a *state.ArchitectureState) { a.StructuralDecisions = nil },
		"fitnessFunctions":    func(a *state.ArchitectureState) { a.FitnessFunctions = nil },
		"boundedContext":      func(a *state.ArchitectureState) { a.BoundedContext.Owning = "" },
	}
	for field, breakField := range cases {
		t.Run(field, func(t *testing.T) {
			architecture := validArchitecture()
			breakField(&architecture)
			assertValidationNames(t, architecture.Validate(), field)
		})
	}
}

func TestAnalysisRoundTripsThroughJSON(t *testing.T) {
	original := validAnalysis()

	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded state.AnalysisState
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("round trip lost data:\n got %+v\nwant %+v", decoded, original)
	}
}

func TestArchitectureRoundTripsThroughJSON(t *testing.T) {
	original := validArchitecture()

	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded state.ArchitectureState
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("round trip lost data:\n got %+v\nwant %+v", decoded, original)
	}
}

// TestCommittedSchemasMatchTheStructs is the drift check: a generated file
// that no longer matches its source is worse than no generated file, so
// this fails the build rather than waiting for someone to notice.
func TestCommittedSchemasMatchTheStructs(t *testing.T) {
	for _, schema := range state.StageSchemas() {
		t.Run(string(schema.Kind), func(t *testing.T) {
			generated, err := schema.Generate()
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			committed, err := os.ReadFile(filepath.Join(repoRoot(t), state.SchemaDir, schema.FileName))
			if err != nil {
				t.Fatalf("read committed schema: %v", err)
			}
			if string(generated) != string(committed) {
				t.Errorf("%s is stale — run: go run ./cmd/gen-schemas", schema.FileName)
			}
		})
	}
}

// repoRoot walks up from the package directory to the module root, so the
// test does not depend on where `go test` was invoked from.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for range 10 {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not find the module root")
	return ""
}

func TestSchemaForKindCoversTypedKinds(t *testing.T) {
	for _, kind := range []state.Kind{
		state.KindAnalysis, state.KindArchitecture, state.KindRoute,
		state.KindReview, state.KindImplementation, state.KindSecurity, state.KindQA,
	} {
		if _, ok := state.SchemaForKind(kind); !ok {
			t.Errorf("kind %q has no generated schema", kind)
		}
	}
	// The end-of-pipeline reports are still markdown; nothing evaluates a
	// condition over them yet.
	if _, ok := state.SchemaForKind(state.Kind("devops")); ok {
		t.Error("devops reported a schema; it is not typed in this cut")
	}
}

func TestGeneratedSchemaDeclaresRequiredFields(t *testing.T) {
	raw, ok := state.SchemaForKind(state.KindAnalysis)
	if !ok {
		t.Fatal("no analyst schema")
	}
	var schema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("generated schema does not parse: %v", err)
	}
	for _, want := range []string{"schemaVersion", "feature", "summary", "acceptanceCriteria"} {
		if !contains(schema.Required, want) {
			t.Errorf("generated schema does not require %q: %v", want, schema.Required)
		}
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestRequiresArchitectDerivesFromStructuralFacts(t *testing.T) {
	cases := []struct {
		name  string
		shape func(*state.AnalysisState)
		want  bool
	}{
		{"plain feature needs no architect", func(a *state.AnalysisState) {
			a.BoundedContext.Crossings = nil
			a.DataModelChanges = nil
			a.NonFunctionalRequirements = nil
		}, false},
		{"context crossing is an integration decision", func(a *state.AnalysisState) {
			a.DataModelChanges = nil
			a.NonFunctionalRequirements = nil
			a.BoundedContext.Crossings = []string{"billing"}
		}, true},
		{"expand migration constrains the deploy", func(a *state.AnalysisState) {
			a.BoundedContext.Crossings = nil
			a.NonFunctionalRequirements = nil
			a.DataModelChanges = []state.DataModelChange{{Description: "add table", Phase: state.MigrationPhaseExpand}}
		}, true},
		{"a documented absence of migration does not summon an architect", func(a *state.AnalysisState) {
			a.BoundedContext.Crossings = nil
			a.NonFunctionalRequirements = nil
			a.DataModelChanges = []state.DataModelChange{{Description: "none", Phase: state.MigrationPhaseNone}}
		}, false},
		{"new dependency is an adapter decision", func(a *state.AnalysisState) {
			a.BoundedContext.Crossings = nil
			a.DataModelChanges = nil
			a.NonFunctionalRequirements = nil
			a.NewDependencies = []string{"github.com/some/queue"}
		}, true},
		{"performance threshold forces stability patterns", func(a *state.AnalysisState) {
			a.BoundedContext.Crossings = nil
			a.DataModelChanges = nil
			a.NonFunctionalRequirements = []state.NonFunctionalRequirement{
				{Category: "performance", Requirement: "fast", Threshold: &state.Threshold{Metric: "p99 sign-in latency", Value: 200, Unit: "ms"}},
			}
		}, true},
		{"performance prose without a threshold is not structural", func(a *state.AnalysisState) {
			a.BoundedContext.Crossings = nil
			a.DataModelChanges = nil
			a.NonFunctionalRequirements = []state.NonFunctionalRequirement{
				{Category: "performance", Requirement: "should feel snappy"},
			}
		}, false},
		// The derivation cannot see "this introduces a new base class" or
		// "this reverses ADR-004". The explicit flag is how those reach the
		// architect, and why it is OR'd with the derived signals.
		{"explicit flag reaches an architect the derivation cannot see", func(a *state.AnalysisState) {
			a.BoundedContext.Crossings = nil
			a.DataModelChanges = nil
			a.NonFunctionalRequirements = nil
			a.ArchitecturalFlags = []string{"introduces a new base class for all report exporters"}
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			analysis := validAnalysis()
			analysis.NewDependencies = nil
			tc.shape(&analysis)
			if got := analysis.RequiresArchitect(); got != tc.want {
				t.Errorf("RequiresArchitect() = %v, want %v", got, tc.want)
			}
		})
	}
}
