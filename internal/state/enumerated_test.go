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
