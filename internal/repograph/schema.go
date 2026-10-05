package repograph

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
)

// GenerateSchema renders the JSON Schema for graph.json from the Go types, so
// the committed copy cannot drift from what Index writes. Output is
// deterministic.
func GenerateSchema() ([]byte, error) {
	reflector := &jsonschema.Reflector{ExpandedStruct: true, DoNotReference: true}
	var rendered bytes.Buffer
	encoder := json.NewEncoder(&rendered)
	encoder.SetIndent("", "  ")
	schema := reflector.Reflect(&Graph{})
	pinSchemaVersion(schema)
	if err := encoder.Encode(schema); err != nil {
		return nil, fmt.Errorf("render graph schema: %w", err)
	}
	return rendered.Bytes(), nil
}

// pinSchemaVersion derives the one accepted value from the constant, never
// from a second hand-written copy (the L2.25 discipline).
func pinSchemaVersion(schema *jsonschema.Schema) {
	if property, found := schema.Properties.Get("schema_version"); found {
		property.Const = SchemaVersion
	}
}
