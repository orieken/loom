package patternspec

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

const schemaLocation = "mem:///pattern-spec.schema.json"

// kebabName is the pattern of a spec name and a step id. A step id is
// echoed in a message only when it matches, so an invalid id is never
// repeated back.
var kebabName = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// compiledSchema compiles the generated schema once. The validator reads
// the same bytes cmd/gen-schemas commits, so the editor and loom health
// judge structure by one schema.
var compiledSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	raw, err := GenerateSchema()
	if err != nil {
		return nil, err
	}
	return CompileSchema(raw)
})

// CompileSchema compiles a pattern spec JSON Schema, with date formats
// asserted. Tests use it to check the committed schema file directly.
func CompileSchema(raw []byte) (*jsonschema.Schema, error) {
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("pattern spec schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource(schemaLocation, document); err != nil {
		return nil, fmt.Errorf("pattern spec schema: %w", err)
	}
	return compiler.Compile(schemaLocation)
}

// structuralIssues validates an instance against the generated schema.
func structuralIssues(instance any) []Issue {
	schema, err := compiledSchema()
	if err != nil {
		return []Issue{failure(rootField, "the pattern spec schema does not compile: "+err.Error())}
	}
	return SchemaIssues(schema, instance)
}

// SchemaIssues reports every leaf violation of instance against schema, in
// a stable order, as field paths with loom's own messages.
func SchemaIssues(schema *jsonschema.Schema, instance any) []Issue {
	var validationErr *jsonschema.ValidationError
	if !errors.As(schema.Validate(instance), &validationErr) {
		return nil
	}
	var issues []Issue
	for _, unit := range validationErr.BasicOutput().Errors {
		issues = append(issues, issuesFromUnit(unit, instance)...)
	}
	// The validator walks maps, so its order changes from run to run.
	slices.SortStableFunc(issues, func(left, right Issue) int {
		return cmp.Or(strings.Compare(left.Field, right.Field), strings.Compare(left.Message, right.Message))
	})
	return issues
}

// skippedKinds are combinators and wrappers: the leaves under them say what
// is wrong.
var skippedKinds = map[reflect.Type]bool{
	reflect.TypeFor[*kind.Schema]():    true,
	reflect.TypeFor[*kind.Group]():     true,
	reflect.TypeFor[*kind.Reference](): true,
	reflect.TypeFor[*kind.AllOf]():     true,
	reflect.TypeFor[*kind.AnyOf]():     true,
	reflect.TypeFor[*kind.OneOf]():     true,
}

func issuesFromUnit(unit jsonschema.OutputUnit, instance any) []Issue {
	if unit.Error == nil || skippedKinds[reflect.TypeOf(unit.Error.Kind)] {
		return nil
	}
	location := splitPointer(unit.InstanceLocation)
	if names := namedFields(unit.Error.Kind); names != nil {
		return namedIssues(location, names, instance)
	}
	return []Issue{failure(fieldPath(location), messageFor(unit.Error.Kind)+stepLabel(location, instance))}
}

// namedFields returns the property names a Required or AdditionalProperties
// error is about, each reported as its own field.
func namedFields(errorKind jsonschema.ErrorKind) []namedField {
	switch typed := errorKind.(type) {
	case *kind.Required:
		return namesWith(typed.Missing, "is required")
	case *kind.AdditionalProperties:
		return namesWith(typed.Properties, "is not a pattern spec field")
	}
	return nil
}

type namedField struct{ name, message string }

func namesWith(names []string, message string) []namedField {
	fields := make([]namedField, 0, len(names))
	for _, name := range names {
		fields = append(fields, namedField{name: name, message: message})
	}
	return fields
}

func namedIssues(location []string, fields []namedField, instance any) []Issue {
	issues := make([]Issue, 0, len(fields))
	for _, field := range fields {
		path := append(append([]string{}, location...), field.name)
		issues = append(issues, failure(fieldPath(path), field.message+stepLabel(location, instance)))
	}
	return issues
}

// kindMessages words each leaf error without the value that caused it.
var kindMessages = map[reflect.Type]func(jsonschema.ErrorKind) string{
	reflect.TypeFor[*kind.Enum](): func(errorKind jsonschema.ErrorKind) string {
		return "must be one of: " + joinQuoted(stringValues(errorKind.(*kind.Enum).Want))
	},
	reflect.TypeFor[*kind.Type](): func(errorKind jsonschema.ErrorKind) string {
		return "must be " + strings.Join(errorKind.(*kind.Type).Want, " or ")
	},
	reflect.TypeFor[*kind.Pattern](): func(errorKind jsonschema.ErrorKind) string {
		return "does not match " + errorKind.(*kind.Pattern).Want
	},
	reflect.TypeFor[*kind.MinItems](): func(errorKind jsonschema.ErrorKind) string {
		return fmt.Sprintf("must have at least %d item(s)", errorKind.(*kind.MinItems).Want)
	},
	reflect.TypeFor[*kind.MinLength](): func(jsonschema.ErrorKind) string { return "must not be empty" },
	reflect.TypeFor[*kind.Minimum](): func(errorKind jsonschema.ErrorKind) string {
		return "must be at least " + errorKind.(*kind.Minimum).Want.RatString()
	},
	reflect.TypeFor[*kind.Format](): func(errorKind jsonschema.ErrorKind) string {
		return "must be a " + errorKind.(*kind.Format).Want + " (YYYY-MM-DD)"
	},
	reflect.TypeFor[*kind.Const](): func(errorKind jsonschema.ErrorKind) string {
		return fmt.Sprintf("must be %v", errorKind.(*kind.Const).Want)
	},
}

func messageFor(errorKind jsonschema.ErrorKind) string {
	if describe, found := kindMessages[reflect.TypeOf(errorKind)]; found {
		return describe(errorKind)
	}
	return "is invalid (" + strings.Join(errorKind.KeywordPath(), "/") + ")"
}

func stringValues(values []any) []string {
	rendered := make([]string, 0, len(values))
	for _, value := range values {
		rendered = append(rendered, fmt.Sprint(value))
	}
	return rendered
}

// splitPointer turns a JSON pointer ("/steps/1/executor") into its tokens.
func splitPointer(pointer string) []string {
	if pointer == "" {
		return nil
	}
	tokens := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	for index, token := range tokens {
		tokens[index] = strings.NewReplacer("~1", "/", "~0", "~").Replace(token)
	}
	return tokens
}

// fieldPath renders tokens as "steps[1].executor", and the root as "(spec)".
func fieldPath(tokens []string) string {
	if len(tokens) == 0 {
		return rootField
	}
	var path strings.Builder
	for _, token := range tokens {
		writeToken(&path, token)
	}
	return path.String()
}

func writeToken(path *strings.Builder, token string) {
	if _, err := strconv.Atoi(token); err == nil {
		path.WriteString("[" + token + "]")
		return
	}
	if path.Len() > 0 {
		path.WriteString(".")
	}
	path.WriteString(token)
}

// stepLabel names the step an issue sits in, by its id, when the id is
// readable: ` (step "run-index")`.
func stepLabel(location []string, instance any) string {
	if len(location) < 2 || location[0] != "steps" {
		return ""
	}
	index, err := strconv.Atoi(location[1])
	if err != nil {
		return ""
	}
	if id := stepIDAt(instance, index); kebabName.MatchString(id) {
		return fmt.Sprintf(" (step %q)", id)
	}
	return ""
}

func stepIDAt(instance any, index int) string {
	document, _ := instance.(map[string]any)
	steps, _ := document["steps"].([]any)
	if index < 0 || index >= len(steps) {
		return ""
	}
	step, _ := steps[index].(map[string]any)
	id, _ := step["id"].(string)
	return id
}
