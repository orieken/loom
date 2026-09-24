package tools

import "encoding/json"

// pathNote is appended to every path argument's description: since L2.3 a
// path is resolved against the server's workspace root, so "absolute path"
// was no longer the whole truth.
const pathNote = " — absolute, or relative to the server's workspace root; it must resolve inside that root"

func projectPathProperty() map[string]any {
	return map[string]any{
		"type":        "string",
		"minLength":   1,
		"description": "Path to the project root" + pathNote,
	}
}

// objectSchema builds a raw JSON Schema for an object with the given required
// keys and properties. Arguments it does not declare are rejected
// (additionalProperties: false): the server validates every call against this
// schema before the tool runs (roadmap L2.1), and a hallucinated argument name
// is exactly the malformed call that should come back as a field-level error
// rather than be silently ignored.
func objectSchema(required []string, properties map[string]any) json.RawMessage {
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return mustMarshalSchema(schema)
}

// mustMarshalSchema panics on failure: schemas are static literals, so a
// marshal error is a programmer error surfaced at registration, not a runtime
// condition to handle.
func mustMarshalSchema(schema map[string]any) json.RawMessage {
	body, err := json.Marshal(schema)
	if err != nil {
		panic("tools: static schema failed to marshal: " + err.Error())
	}
	return body
}

func projectPathOnlySchema() json.RawMessage {
	return objectSchema([]string{"projectPath"}, map[string]any{
		"projectPath": projectPathProperty(),
	})
}

// eitherOfSchema is objectSchema for a tool that needs at least one of
// several arguments.
func eitherOfSchema(alternatives []string, properties map[string]any) json.RawMessage {
	anyOf := make([]map[string]any, 0, len(alternatives))
	for _, name := range alternatives {
		anyOf = append(anyOf, map[string]any{"required": []string{name}})
	}
	return mustMarshalSchema(map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
		"anyOf":                anyOf,
	})
}
