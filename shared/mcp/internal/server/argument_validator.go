package server

// Server-side argument validation (roadmap L2.1). A tool's InputSchema used
// to exist only to describe arguments to the model; enforcement was an
// unchecked type assertion, so `projectPath: 42` became "" and the model got a
// generic error it could only retry blindly. Every call is now validated
// against the tool's own schema before Execute runs, and a failure comes back
// as a list of field-level violations a model can repair from.
//
// The validator lives here, in the adapter layer, because it is a third-party
// library: internal/domain stays stdlib-only (M0.3).

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"

	"github.com/orieken/loom/shared/mcp/internal/domain"
)

// argumentValidator checks one tool's arguments against its InputSchema.
type argumentValidator struct {
	tool   string
	schema *jsonschema.Schema
}

// compileArgumentValidator compiles a tool's InputSchema. A schema that does
// not compile is a programmer error, reported when tools are registered
// rather than on the first call.
func compileArgumentValidator(tool domain.Tool) (*argumentValidator, error) {
	raw := tool.InputSchema()
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("tool %q declares no input schema; MCP requires one, and the server validates every call against it", tool.Name())
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("tool %q input schema: %w", tool.Name(), err)
	}
	location := "mem:///" + tool.Name() + ".json"
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(location, document); err != nil {
		return nil, fmt.Errorf("tool %q input schema: %w", tool.Name(), err)
	}
	schema, err := compiler.Compile(location)
	if err != nil {
		return nil, fmt.Errorf("tool %q input schema: %w", tool.Name(), err)
	}
	return &argumentValidator{tool: tool.Name(), schema: schema}, nil
}

// violations returns every field-level problem with args, or none. A nil
// map validates as an empty object — TestACallWithNoArgumentsIsValidatedAsEmpty
// pins that, and a defensive default here was dead code (mutant V4 survived).
func (v *argumentValidator) violations(args map[string]any) []domain.FieldViolation {
	err := v.schema.Validate(args)
	var validationErr *jsonschema.ValidationError
	if !errors.As(err, &validationErr) {
		return nil
	}
	var found []domain.FieldViolation
	for _, unit := range validationErr.BasicOutput().Errors {
		found = append(found, violationsFrom(unit)...)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Field < found[j].Field })
	return found
}

// violationsFrom turns one output unit into violations. A missing or unknown
// argument is reported against that argument, not the object holding it; the
// combinators (anyOf and friends) are left to the leaves under them.
func violationsFrom(unit jsonschema.OutputUnit) []domain.FieldViolation {
	if unit.Error == nil {
		return nil
	}
	switch errorKind := unit.Error.Kind.(type) {
	case *kind.Required:
		return namedViolations(errorKind.Missing, "is required")
	case *kind.AdditionalProperties:
		return namedViolations(errorKind.Properties, "is not an argument this tool accepts")
	case *kind.Schema, *kind.Group, *kind.Reference, *kind.AnyOf, *kind.AllOf, *kind.OneOf:
		return nil // the leaves under a combinator say what is wrong
	}
	return []domain.FieldViolation{{Field: fieldName(unit.InstanceLocation), Problem: unit.Error.String()}}
}

// namedViolations reports one problem against each named argument.
func namedViolations(names []string, problem string) []domain.FieldViolation {
	violations := make([]domain.FieldViolation, 0, len(names))
	for _, name := range names {
		violations = append(violations, domain.FieldViolation{Field: name, Problem: problem})
	}
	return violations
}

// fieldName turns a JSON pointer ("/tags/0") into a field path ("tags/0"),
// and the root into "(arguments)".
func fieldName(pointer string) string {
	field := strings.TrimPrefix(pointer, "/")
	if field == "" {
		return "(arguments)"
	}
	return field
}

// invalidArgumentsResult is the error result a rejected call returns: a
// validation failure (L2.5) whose violations name each field to repair.
func invalidArgumentsResult(tool string, violations []domain.FieldViolation) *domain.ToolResult {
	failure := domain.NewToolError(domain.ErrorValidation, "invalid arguments to "+tool)
	failure.Violations = violations
	return domain.NewToolErrorResult(failure)
}
