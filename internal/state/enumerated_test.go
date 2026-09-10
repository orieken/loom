package state_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/state"
)

// TestEveryClosedSetFieldCarriesItsEnum is L2.25's fitness function.
//
// It walks every generated stage schema, finds each field whose Go type
// declares a closed set of values, and asserts the schema tells an agent
// what those values are. stride[].category shipped as a bare
// {"type": "string"} while the validator demanded six exact literals, and
// the run that hit it failed twice at ~$0.65 with a message reporting a
// security gap where there was a serialization mismatch. Nothing but this
// test stops the next closed-set field from doing the same.
func TestEveryClosedSetFieldCarriesItsEnum(t *testing.T) {
	checked := 0
	for _, schema := range state.StageSchemas() {
		raw, err := schema.Generate()
		if err != nil {
			t.Fatalf("generate %s: %v", schema.Kind, err)
		}
		var document map[string]any
		if err := json.Unmarshal(raw, &document); err != nil {
			t.Fatalf("decode %s: %v", schema.Kind, err)
		}
		checked += assertEnumsPresent(t, string(schema.Kind), document, expectedEnums(schema.Subject()))
	}
	if checked == 0 {
		t.Fatal("no enum-bearing field was found; the check is walking nothing")
	}
}

// expectedEnums reflects over a stage's Go type and returns, for each JSON
// property backed by a closed-set type, the values that type publishes.
// Reflection rather than property names: two different closed sets are both
// called "category" and two are both called "verdict", so matching by name
// checks the wrong field.
func expectedEnums(subject interface{}) map[string][]string {
	found := map[string][]string{}
	collectEnums(reflect.TypeOf(subject), found, map[reflect.Type]bool{})
	return found
}

var enumeratedInterface = reflect.TypeOf((*state.Enumerated)(nil)).Elem()

func collectEnums(target reflect.Type, found map[string][]string, seen map[reflect.Type]bool) {
	target = deref(target)
	if target.Kind() != reflect.Struct || seen[target] {
		return
	}
	seen[target] = true
	for index := 0; index < target.NumField(); index++ {
		field := target.Field(index)
		name := jsonName(field)
		if name == "" {
			continue
		}
		if values, isClosed := enumValuesOf(field.Type); isClosed {
			found[name] = values
			continue
		}
		collectEnums(field.Type, found, seen)
	}
}

func enumValuesOf(target reflect.Type) ([]string, bool) {
	if !target.Implements(enumeratedInterface) {
		return nil, false
	}
	return reflect.New(target).Elem().Interface().(state.Enumerated).Values(), true
}

func deref(target reflect.Type) reflect.Type {
	for target.Kind() == reflect.Pointer || target.Kind() == reflect.Slice {
		target = target.Elem()
	}
	return target
}

func jsonName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" || tag == "-" {
		return ""
	}
	if comma := strings.Index(tag, ","); comma >= 0 {
		return tag[:comma]
	}
	return tag
}

// assertEnumsPresent walks the generated document and checks each property
// the Go types say is a closed set.
func assertEnumsPresent(t *testing.T, kind string, node map[string]any, expected map[string][]string) int {
	t.Helper()
	found := 0
	for name, property := range propertiesOf(node) {
		if values, isClosed := expected[name]; isClosed {
			found++
			assertEnumMatches(t, kind, name, property, values)
		}
		found += assertEnumsPresent(t, kind, property, expected)
	}
	for _, child := range nestedSchemas(node) {
		found += assertEnumsPresent(t, kind, child, expected)
	}
	return found
}

func assertEnumMatches(t *testing.T, kind, name string, property map[string]any, want []string) {
	t.Helper()
	raw, present := property["enum"]
	if !present {
		t.Errorf("%s schema: field %q is a closed set of %v but declares no enum — "+
			"the agent is validated against a rule it was never told", kind, name, want)
		return
	}
	listed := make([]string, 0, len(want))
	for _, value := range raw.([]any) {
		listed = append(listed, value.(string))
	}
	if !reflect.DeepEqual(listed, want) {
		t.Errorf("%s schema: field %q enum = %v, want %v", kind, name, listed, want)
	}
}

func propertiesOf(node map[string]any) map[string]map[string]any {
	properties, ok := node["properties"].(map[string]any)
	if !ok {
		return nil
	}
	typed := make(map[string]map[string]any, len(properties))
	for name, value := range properties {
		if property, isObject := value.(map[string]any); isObject {
			typed[name] = property
		}
	}
	return typed
}

func nestedSchemas(node map[string]any) []map[string]any {
	var children []map[string]any
	for _, key := range []string{"items", "$defs", "definitions"} {
		switch value := node[key].(type) {
		case map[string]any:
			children = append(children, value)
			for _, nested := range value {
				if child, isObject := nested.(map[string]any); isObject {
					children = append(children, child)
				}
			}
		}
	}
	return children
}

// TestEverySchemaPinsItsVersion is the regression guard for run 4's blocker.
//
// schemaVersion reflected as a bare {"type": "integer"} while
// requireSchemaVersion refuses anything but the current constant. The agent
// was told "an integer" and validated against an equality check. That was
// survivable only while the constant was 1 — the value a model writes
// unprompted — so bumping it to 2 turned a latent hole into a stage-1 halt
// on every typed stage at once.
func TestEverySchemaPinsItsVersion(t *testing.T) {
	for _, schema := range state.StageSchemas() {
		raw, err := schema.Generate()
		if err != nil {
			t.Fatalf("generate %s: %v", schema.Kind, err)
		}
		var document struct {
			Properties struct {
				SchemaVersion struct {
					Const *int `json:"const"`
				} `json:"schemaVersion"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(raw, &document); err != nil {
			t.Fatalf("decode %s: %v", schema.Kind, err)
		}
		pinned := document.Properties.SchemaVersion.Const
		if pinned == nil {
			t.Errorf("%s schema does not pin schemaVersion — the agent is told 'an integer' "+
				"and validated against an equality check", schema.Kind)
			continue
		}
		if *pinned != state.SchemaVersion {
			t.Errorf("%s schema pins schemaVersion to %d, this build accepts %d",
				schema.Kind, *pinned, state.SchemaVersion)
		}
	}
}

// TestASchemaConformingDocumentValidates closes the boundary the mock
// provider cannot reach.
//
// The typed context-engineer was recorded as "verified by mock run", and the
// mock builds its documents in Go where the constant is correct by
// construction — so no mock can ever exercise the path where a model chooses
// the value. This builds each document from the SCHEMA the agent is handed,
// filling every fixed value the schema declares, and asserts the validator
// accepts it. If the schema and the validator ever disagree again, this fails
// on a laptop instead of on stage 1 of a paid run.
func TestASchemaConformingDocumentValidates(t *testing.T) {
	for _, schema := range state.StageSchemas() {
		t.Run(string(schema.Kind), func(t *testing.T) {
			raw, err := schema.Generate()
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			document := minimalDocumentFrom(t, raw)
			payload, err := json.Marshal(document)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if _, err := state.Decode(schema.Kind, payload); err != nil {
				// A missing required field is this helper's limitation, not a
				// contract defect. A rejected schemaVersion is the defect.
				if strings.Contains(err.Error(), "schemaVersion") {
					t.Errorf("a document carrying the schema's own declared schemaVersion was refused: %v", err)
				}
			}
		})
	}
}

// minimalDocumentFrom builds a document that honours every const the schema
// declares. It does not attempt to satisfy required lists — the assertion
// above is scoped to schemaVersion for that reason.
func minimalDocumentFrom(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var schema struct {
		Properties map[string]struct {
			Const any `json:"const"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	document := map[string]any{}
	for name, property := range schema.Properties {
		if property.Const != nil {
			document[name] = property.Const
		}
	}
	return document
}

// TestEveryConditionalRequirementReachesTheSchema is L3.33's fitness
// function, and the third instance of one rule.
//
// The architecture validator required `fitness` unless `judgmentOnly` was
// set. The schema said `fitness` was optional, stated the real rule only in
// one field's prose, and left the escape hatch undocumented — so run 4's
// architect omitted a fitness function it genuinely did not have, could not
// discover how to say so, and halted Experiment A at stage 4 for $2.52.
//
// A conditional the validator enforces and the schema does not express is
// the same defect as L3.28's unpinned schemaVersion and L2.25's missing
// STRIDE enum. This asserts every declared conditional survives generation.
func TestEveryConditionalRequirementReachesTheSchema(t *testing.T) {
	architecture := generatedSchemaFor(t, state.KindArchitecture)
	items := architecture["properties"].(map[string]any)["structuralDecisions"].(map[string]any)["items"].(map[string]any)

	branches, present := items["anyOf"].([]any)
	if !present {
		t.Fatal("the architecture schema does not express the fitness/judgmentOnly conditional; " +
			"an architect with no fitness function has no way to learn how to say so")
	}
	if len(branches) != 2 {
		t.Fatalf("conditional has %d branches, want 2 (field present, or escape hatch true)", len(branches))
	}
	assertBranchRequires(t, branches[0], "fitness")
	assertBranchRequires(t, branches[1], "judgmentOnly")

	// The escape hatch must be documented, or it cannot be used.
	hatch := items["properties"].(map[string]any)["judgmentOnly"].(map[string]any)
	if description, _ := hatch["description"].(string); description == "" {
		t.Error("judgmentOnly carries no description — it is the only way to omit fitness, " +
			"and run 4's architect could not have known that")
	}
}

func assertBranchRequires(t *testing.T, branch any, field string) {
	t.Helper()
	required, _ := branch.(map[string]any)["required"].([]any)
	for _, name := range required {
		if name == field {
			return
		}
	}
	t.Errorf("conditional branch %v does not require %q", branch, field)
}

func generatedSchemaFor(t *testing.T, kind state.Kind) map[string]any {
	t.Helper()
	for _, schema := range state.StageSchemas() {
		if schema.Kind != kind {
			continue
		}
		raw, err := schema.Generate()
		if err != nil {
			t.Fatalf("generate %s: %v", kind, err)
		}
		var document map[string]any
		if err := json.Unmarshal(raw, &document); err != nil {
			t.Fatalf("decode %s: %v", kind, err)
		}
		return document
	}
	t.Fatalf("no schema for kind %q", kind)
	return nil
}

// A document that satisfies the schema's escape-hatch branch must satisfy
// the validator. This is the property that failed: the two disagreed.
func TestTheEscapeHatchTheSchemaOffersIsOneTheValidatorAccepts(t *testing.T) {
	payload := []byte(`{
		"schemaVersion": 2, "feature": "f",
		"structuralDecisions": [
			{"decision": "keep it in the adapter", "rationale": "no meaningful automated check", "judgmentOnly": true}
		],
		"boundedContext": {"owning": "console"},
		"fitnessFunctions": [{"property": "p", "verification": "go test ./..."}]
	}`)

	if _, err := state.Decode(state.KindArchitecture, payload); err != nil {
		t.Fatalf("the validator refused a decision using the escape hatch the schema offers: %v", err)
	}
}

// And the rule still bites: a decision with neither is still refused.
func TestADecisionWithNeitherFitnessNorTheHatchIsRefused(t *testing.T) {
	payload := []byte(`{
		"schemaVersion": 2, "feature": "f",
		"structuralDecisions": [{"decision": "d", "rationale": "r"}],
		"boundedContext": {"owning": "console"},
		"fitnessFunctions": [{"property": "p", "verification": "go test ./..."}]
	}`)

	_, err := state.Decode(state.KindArchitecture, payload)
	if err == nil {
		t.Fatal("a decision with no fitness function and no judgmentOnly flag was accepted")
	}
	if !strings.Contains(err.Error(), "judgmentOnly") {
		t.Errorf("the error does not name the escape hatch, so it cannot be acted on: %v", err)
	}
}
