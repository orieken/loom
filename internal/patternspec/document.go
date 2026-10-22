package patternspec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	yaml "go.yaml.in/yaml/v4"
)

const dateLayout = "2006-01-02"

var yamlLinePattern = regexp.MustCompile(`line (\d+)`)

// Parse reads one pattern spec. It checks the schema version, then the
// structure against the generated JSON Schema, then decodes the typed Spec.
// It does not apply the contract rules: Validate does. A spec with any
// issue comes back as the zero Spec.
//
// The source is untrusted: Parse never runs, includes or fetches anything,
// and its messages never echo the source.
func Parse(source []byte) (Spec, []Issue) {
	instance, issue, isParsed := decodeDocument(source)
	if !isParsed {
		return Spec{}, []Issue{issue}
	}
	if issues := versionIssues(instance); len(issues) > 0 {
		return Spec{}, issues
	}
	if issues := structuralIssues(instance); len(issues) > 0 {
		return Spec{}, issues
	}
	return decodeSpec(instance)
}

// decodeDocument parses YAML into the JSON data model the schema validator
// reads: string keys, json.Number numbers, dates as YYYY-MM-DD strings.
func decodeDocument(source []byte) (any, Issue, bool) {
	if len(bytes.TrimSpace(source)) == 0 {
		return nil, failure(rootField, "the file holds no spec"), false
	}
	var raw any
	if err := yaml.Unmarshal(source, &raw); err != nil {
		return nil, failure(rootField, "is not valid YAML"+yamlLine(err)), false
	}
	if raw == nil {
		return nil, failure(rootField, "the file holds no spec"), false
	}
	return toJSONModel(raw)
}

// yamlLine keeps only the line number of a YAML error; the rest of the
// message can quote the source.
func yamlLine(err error) string {
	match := yamlLinePattern.FindStringSubmatch(err.Error())
	if match == nil {
		return ""
	}
	return " (line " + match[1] + ")"
}

func toJSONModel(raw any) (any, Issue, bool) {
	normalized, isNormalized := normalizeYAML(raw)
	if !isNormalized {
		return nil, failure(rootField, "every mapping key MUST be a string"), false
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, failure(rootField, "holds a value that has no JSON form"), false
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		return nil, failure(rootField, "holds a value that has no JSON form"), false
	}
	return instance, Issue{}, true
}

// normalizeYAML converts what the YAML decoder produces into values JSON
// can hold. It reports false for a mapping key that is not a string.
func normalizeYAML(value any) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return normalizeMap(typed)
	case map[any]any:
		return normalizeAnyKeyMap(typed)
	case []any:
		return normalizeList(typed)
	case time.Time:
		return formatTime(typed), true
	}
	return value, true
}

func normalizeMap(mapping map[string]any) (any, bool) {
	normalized := make(map[string]any, len(mapping))
	for key, value := range mapping {
		converted, isConverted := normalizeYAML(value)
		if !isConverted {
			return nil, false
		}
		normalized[key] = converted
	}
	return normalized, true
}

func normalizeAnyKeyMap(mapping map[any]any) (any, bool) {
	stringKeyed := make(map[string]any, len(mapping))
	for key, value := range mapping {
		name, isString := key.(string)
		if !isString {
			return nil, false
		}
		stringKeyed[name] = value
	}
	return normalizeMap(stringKeyed)
}

func normalizeList(list []any) (any, bool) {
	normalized := make([]any, 0, len(list))
	for _, value := range list {
		converted, isConverted := normalizeYAML(value)
		if !isConverted {
			return nil, false
		}
		normalized = append(normalized, converted)
	}
	return normalized, true
}

// formatTime renders a YAML timestamp the way the spec wrote a date: a
// plain date when it has no time of day.
func formatTime(instant time.Time) string {
	if instant.Equal(instant.Truncate(24 * time.Hour)) {
		return instant.Format(dateLayout)
	}
	return instant.Format(time.RFC3339)
}

// versionIssues refuses a schema version this build does not read before
// anything else is checked, so a newer spec gets one clear message rather
// than a field-by-field list.
func versionIssues(instance any) []Issue {
	document, isObject := instance.(map[string]any)
	if !isObject {
		return nil
	}
	number, isNumber := document["schemaVersion"].(json.Number)
	if !isNumber || number.String() == fmt.Sprint(SchemaVersion) {
		return nil
	}
	return []Issue{failure("schemaVersion", fmt.Sprintf("this version is not supported; supported versions: %d", SchemaVersion))}
}

func decodeSpec(instance any) (Spec, []Issue) {
	encoded, err := json.Marshal(instance)
	if err != nil {
		return Spec{}, []Issue{failure(rootField, "holds a value that has no JSON form")}
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, []Issue{failure(rootField, "does not match the pattern spec types")}
	}
	return spec, nil
}
