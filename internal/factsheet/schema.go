package factsheet

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
)

// GenerateSchema renders the JSON Schema for a fact sheet from the Go type.
func GenerateSchema() ([]byte, error) {
	reflector := &jsonschema.Reflector{ExpandedStruct: true, DoNotReference: true}
	var rendered bytes.Buffer
	encoder := json.NewEncoder(&rendered)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(reflector.Reflect(&FactSheet{})); err != nil {
		return nil, fmt.Errorf("render fact sheet schema: %w", err)
	}
	return rendered.Bytes(), nil
}
