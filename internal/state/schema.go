package state

// JSON Schema is generated from the Go structs above, never hand-written:
// two hand-maintained copies of one shape drift, and the drift is silent.
// The generated files live in shared/schemas/pipeline/ because agents (and
// humans) read them from the installed framework content.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/invopop/jsonschema"
)

// SchemaDir is where generated pipeline schemas are committed, relative to
// the repo root.
const SchemaDir = "shared/schemas/pipeline"

// Kind names a state document type. A plan stage declares the kind it
// produces, so typedness is plan data — like gates — rather than a global
// property of an agent's name.
type Kind string

// The kinds typed today. L2.9's first cut types one hop; the rest of the
// pipeline still exchanges markdown, and this list is where later epics
// grow.
const (
	KindAnalysis       Kind = "analysis"
	KindArchitecture   Kind = "architecture"
	KindRoute          Kind = "route"
	KindReview         Kind = "review"
	KindImplementation Kind = "implementation"
	KindSecurity       Kind = "security"
	KindQA             Kind = "qa"
	KindScenarios      Kind = "scenarios"
	KindContext        Kind = "context"
)

// StageSchema names one state document kind and the type behind it.
type StageSchema struct {
	Kind Kind
	// FileName is the committed schema file, e.g. analysis.schema.json.
	FileName string
	subject  interface{}
}

// Subject returns the Go value the schema is generated from, so a fitness
// function can compare the generated document against the types behind it
// (roadmap L2.25).
func (s StageSchema) Subject() interface{} { return s.subject }

// StageSchemas returns every typed state document.
func StageSchemas() []StageSchema {
	return []StageSchema{
		{Kind: KindAnalysis, FileName: "analysis.schema.json", subject: &AnalysisState{}},
		{Kind: KindArchitecture, FileName: "architecture.schema.json", subject: &ArchitectureState{}},
		{Kind: KindRoute, FileName: "route.schema.json", subject: &Route{}},
		{Kind: KindReview, FileName: "review.schema.json", subject: &ReviewState{}},
		{Kind: KindImplementation, FileName: "implementation.schema.json", subject: &ImplementationState{}},
		{Kind: KindSecurity, FileName: "security.schema.json", subject: &SecurityState{}},
		{Kind: KindQA, FileName: "qa.schema.json", subject: &QAState{}},
		{Kind: KindScenarios, FileName: "scenarios.schema.json", subject: &ScenariosState{}},
		{Kind: KindContext, FileName: "context.schema.json", subject: &ContextState{}},
	}
}

// SchemaForKind returns the generated JSON Schema for a state kind, or
// false when the kind is unknown.
func SchemaForKind(kind Kind) ([]byte, bool) {
	for _, schema := range StageSchemas() {
		if schema.Kind != kind {
			continue
		}
		raw, err := schema.Generate()
		if err != nil {
			return nil, false
		}
		return raw, true
	}
	return nil, false
}

// Generate renders the JSON Schema for one stage. Output is deterministic:
// the same structs always produce byte-identical schemas, which is what
// makes the committed copies checkable.
func (s StageSchema) Generate() ([]byte, error) {
	reflector := &jsonschema.Reflector{ExpandedStruct: true, DoNotReference: true}
	schema := reflector.Reflect(s.subject)
	pinSchemaVersion(schema)
	applyConditionalRequirements(s.Kind, schema)
	var rendered bytes.Buffer
	encoder := json.NewEncoder(&rendered)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(schema); err != nil {
		return nil, fmt.Errorf("render schema for kind %q: %w", s.Kind, err)
	}
	return rendered.Bytes(), nil
}

// TypedStateDir is the workspace subdirectory holding one JSON document per
// typed stage. The document IS the stage's artifact, so L2.12's digest
// recording and staleness cascade cover typed state with no new mechanism.
const TypedStateDir = "state"

// documentFactories maps a kind to an empty document of that type. A table
// rather than a switch, for the same reason viewFileNames is one.
func documentFactories() map[Kind]func() Validatable {
	return map[Kind]func() Validatable{
		KindAnalysis:       func() Validatable { return &AnalysisState{} },
		KindArchitecture:   func() Validatable { return &ArchitectureState{} },
		KindRoute:          func() Validatable { return &Route{} },
		KindReview:         func() Validatable { return &ReviewState{} },
		KindImplementation: func() Validatable { return &ImplementationState{} },
		KindSecurity:       func() Validatable { return &SecurityState{} },
		KindQA:             func() Validatable { return &QAState{} },
		KindScenarios:      func() Validatable { return &ScenariosState{} },
		KindContext:        func() Validatable { return &ContextState{} },
	}
}

// Decode parses and validates a payload of the given kind. Unknown fields
// are rejected: the schema is generated from these same structs, so a field
// the struct does not have is a field the agent invented.
func Decode(kind Kind, payload []byte) (Validatable, error) {
	newDocument, known := documentFactories()[kind]
	if !known {
		return nil, fmt.Errorf("unknown state kind %q", kind)
	}
	return decodeInto(payload, newDocument())
}

func decodeInto(payload []byte, target Validatable) (Validatable, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return nil, fmt.Errorf("payload does not conform to the stage schema: %w", err)
	}
	if err := target.Validate(); err != nil {
		return nil, err
	}
	return target, nil
}

// pinSchemaVersion tells the agent the one value schemaVersion may take.
//
// It reflected as a bare {"type": "integer"} while requireSchemaVersion
// refuses anything but the current constant — the agent was asked for "an
// integer" and validated against an equality check. That was harmless only
// while the constant was 1, which is what a model writes unprompted; the bump
// to 2 in bf302c8 made it fatal, and run 4 died on its first stage twice for
// $2.09 with every stage after it a coin-flip behind.
//
// This is the L2.25 discipline applied to the field L2.25 did not cover: a
// value the validator fixes is derived into the schema from the same
// constant, never left for the author or the model to guess.
func pinSchemaVersion(schema *jsonschema.Schema) {
	if schema == nil || schema.Properties == nil {
		return
	}
	property, found := schema.Properties.Get("schemaVersion")
	if !found || property == nil {
		return
	}
	property.Const = SchemaVersion
	property.Description = "Always " + strconv.Itoa(SchemaVersion) + " — the schema version this build accepts."
}
