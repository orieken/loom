package contextindex

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
)

// SchemaDir and SchemaFile locate the generated schema beside the graph's.
const (
	SchemaDir  = "shared/schemas/repograph"
	SchemaFile = "index.schema.json"
)

// GenerateSchema renders the JSON Schema for index.json from the Go types.
func GenerateSchema() ([]byte, error) {
	reflector := &jsonschema.Reflector{ExpandedStruct: true, DoNotReference: true}
	schema := reflector.Reflect(&Index{})
	if property, found := schema.Properties.Get("schema_version"); found {
		property.Const = SchemaVersion
	}
	var rendered bytes.Buffer
	encoder := json.NewEncoder(&rendered)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(schema); err != nil {
		return nil, fmt.Errorf("render index schema: %w", err)
	}
	return rendered.Bytes(), nil
}
