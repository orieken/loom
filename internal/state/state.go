// Package state is the typed pipeline state graph decided by ADR-006 and
// roadmap item L2.9. Agents used to hand each other whole markdown
// documents; these types are the transport instead, and markdown becomes a
// rendered view of them.
//
// This is L2.9's first cut: it types the analyst -> architect hop. Every
// other stage still exchanges markdown, so this package models two stages,
// not eighteen.
package state

import (
	"errors"
	"fmt"
)

// SchemaVersion identifies the shape of every state document in this
// package. A document carrying a different version is refused at load
// rather than migrated — nothing is deployed anywhere yet.
//
// 2 (roadmap L3.24, L3.25): NonFunctionalRequirement.Threshold became a
// typed {metric, value, unit} rather than free text, AnalysisState gained
// Surfaces, and ContextState was added. The first two exist so a routing
// predicate reads a fact a model cannot satisfy by writing a sentence, which
// means an analysis written against v1 cannot be routed correctly and must
// not be loaded as though it could.
const SchemaVersion = 2

// ValidationError names the field that failed and why, so a failing stage
// reports something a human can act on.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("field %q %s", e.Field, e.Reason)
}

// Validatable is implemented by every stage's state document.
type Validatable interface {
	Validate() error
}

func requireSchemaVersion(version int) error {
	if version != SchemaVersion {
		return &ValidationError{Field: "schemaVersion",
			Reason: fmt.Sprintf("is %d, this build supports %d", version, SchemaVersion)}
	}
	return nil
}

func requireText(field, value string) error {
	if value == "" {
		return &ValidationError{Field: field, Reason: "is required and empty"}
	}
	return nil
}

// requireItems enforces that a list a downstream stage reads is not empty.
// "None" is expressed as an explicit entry saying so, never as an absent
// list — an empty list and "we considered it and there are none" are
// different facts, and downstream stages need to tell them apart.
func requireItems(field string, count int) error {
	if count == 0 {
		return &ValidationError{Field: field, Reason: "must have at least one entry"}
	}
	return nil
}

// firstError returns the first failure, keeping Validate implementations
// flat instead of a stack of ifs.
func firstError(checks ...error) error {
	return errors.Join(firstNonNil(checks))
}

func firstNonNil(checks []error) error {
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	return nil
}

// fieldPath names a field inside a list entry, e.g.
// structuralDecisions[2].fitness, so a validation error points at the exact
// item that failed.
func fieldPath(index int, field string) string {
	return fmt.Sprintf("structuralDecisions[%d].%s", index, field)
}
