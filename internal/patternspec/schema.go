package patternspec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/invopop/jsonschema"
)

// Where the generated schema is committed, relative to the repository root.
const (
	SchemaDir  = "shared/schemas"
	SchemaFile = "pattern-spec.schema.json"
)

const schemaID = "https://github.com/orieken/loom/shared/schemas/pattern-spec.schema.json"

// enumValues publishes each closed set once, from the constants above, so
// the schema cannot hold a second hand-written copy (the L2.25 discipline).
var enumValues = map[reflect.Type][]string{
	reflect.TypeFor[Executor]():    stringsOf(Executors()),
	reflect.TypeFor[HumanReason](): stringsOf(HumanReasons()),
	reflect.TypeFor[InputType]():   stringsOf(InputTypes()),
	reflect.TypeFor[Source]():      stringsOf(Sources()),
	reflect.TypeFor[Status]():      stringsOf(Statuses()),
	reflect.TypeFor[VerifyKind]():  stringsOf(VerifyKinds()),
}

// GenerateSchema renders the JSON Schema for a pattern spec from the Go
// types. Output is deterministic; a test fails when the committed copy
// drifts from it.
func GenerateSchema() ([]byte, error) {
	reflector := &jsonschema.Reflector{ExpandedStruct: true, DoNotReference: true, Mapper: enumSchema}
	schema := reflector.Reflect(&Spec{})
	schema.ID = schemaID
	schema.Title = "Pattern Spec"
	schema.Description = "One repeatable workflow (pattern mining F2). Rules a JSON Schema cannot express: shared/contracts/pattern-spec-contract.md."
	pinSchemaVersion(schema)
	var rendered bytes.Buffer
	encoder := json.NewEncoder(&rendered)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(schema); err != nil {
		return nil, fmt.Errorf("render pattern spec schema: %w", err)
	}
	return rendered.Bytes(), nil
}

func enumSchema(goType reflect.Type) *jsonschema.Schema {
	values, found := enumValues[goType]
	if !found {
		return nil
	}
	enum := make([]any, 0, len(values))
	for _, value := range values {
		enum = append(enum, value)
	}
	return &jsonschema.Schema{Type: "string", Enum: enum}
}

func pinSchemaVersion(schema *jsonschema.Schema) {
	if property, found := schema.Properties.Get("schemaVersion"); found {
		property.Const = SchemaVersion
	}
}

func stringsOf[T ~string](values []T) []string {
	converted := make([]string, 0, len(values))
	for _, value := range values {
		converted = append(converted, string(value))
	}
	return converted
}
