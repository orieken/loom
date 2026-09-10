package state

// Conditional requirements the validator enforces, declared once so the
// schema an agent is handed can state them too (roadmap L3.33).
//
// `StructuralDecision.fitness` is required unless the decision is flagged
// `judgmentOnly`. The generated schema said none of that: `fitness` was
// simply optional, `required` listed only `decision` and `rationale`, the
// real rule appeared solely in one field's prose description, and
// `judgmentOnly` — the escape hatch — carried no description at all.
//
// Run 4's Experiment A died on it. That feature adds one method to one class,
// so the architect reasonably had a structural decision with no meaningful
// fitness function and omitted it, with no way to learn that `judgmentOnly:
// true` was how to say so. $2.52 spent, run abandoned at stage 4 of 12.
//
// This is L3.28's shape for the second time: a constraint the validator
// enforces and the schema does not communicate, invisible to mocks because
// mocks build valid state in Go. L3.28 pinned `schemaVersion` and did not
// audit the other constraints for the same gap. So the rule is declared here
// as data, the validator reads its message from this declaration, and the
// generator emits it into the schema — one statement, two consumers, no room
// to drift.

import "github.com/invopop/jsonschema"

// conditionalRequirement is a field required on the items of an array
// property unless an escape-hatch boolean is set on the same item.
type conditionalRequirement struct {
	// Kind and Property locate the array whose items carry the rule.
	Kind     Kind
	Property string
	// Field is required unless Unless is true.
	Field  string
	Unless string
	// Reason is what a human is told when the rule is broken. The validator
	// uses it verbatim, so the error and the schema describe one rule.
	Reason string
}

// structuralDecisionFitness is the one conditional the pipeline has today.
var structuralDecisionFitness = conditionalRequirement{
	Kind:     KindArchitecture,
	Property: "structuralDecisions",
	Field:    "fitness",
	Unless:   "judgmentOnly",
	Reason:   "is required unless the decision is flagged judgmentOnly (architecture-guardrails.md #7)",
}

// conditionalRequirements returns every declared conditional. A rule absent
// from this list is a rule the agent is not told.
func conditionalRequirements() []conditionalRequirement {
	return []conditionalRequirement{structuralDecisionFitness}
}

// apply writes the rule into the generated schema as an `anyOf`: either the
// required field is present, or the escape hatch is present and true. That
// reads as the rule itself, which `if`/`then` over an absent-means-false
// boolean does not.
func (c conditionalRequirement) apply(schema *jsonschema.Schema) {
	items := arrayItemsOf(schema, c.Property)
	if items == nil {
		return
	}
	items.AnyOf = []*jsonschema.Schema{{Required: []string{c.Field}}, c.escapeHatchBranch()}
	describeEscapeHatch(items, c)
}

// describeEscapeHatch documents the flag that excuses the requirement.
// Run 4's architect could not have used `judgmentOnly` because nothing told
// it the field existed for that.
func describeEscapeHatch(items *jsonschema.Schema, c conditionalRequirement) {
	property, found := items.Properties.Get(c.Unless)
	if !found || property == nil {
		return
	}
	property.Description = "Set true when no meaningful " + c.Field +
		" exists for this decision, which is the only way to omit it. The rationale must then carry the justification."
}

func arrayItemsOf(schema *jsonschema.Schema, property string) *jsonschema.Schema {
	if schema == nil || schema.Properties == nil {
		return nil
	}
	array, found := schema.Properties.Get(property)
	if !found || array == nil {
		return nil
	}
	return array.Items
}

// escapeHatchBranch is the alternative that excuses the requirement: the
// flag present and explicitly true. Requiring it present matters — absent
// would mean false, which is precisely the case the other branch covers.
func (c conditionalRequirement) escapeHatchBranch() *jsonschema.Schema {
	branch := &jsonschema.Schema{Required: []string{c.Unless}, Properties: jsonschema.NewProperties()}
	branch.Properties.Set(c.Unless, &jsonschema.Schema{Type: "boolean", Const: true})
	return branch
}

// applyConditionalRequirements writes every conditional declared for a kind
// into its generated schema.
func applyConditionalRequirements(kind Kind, schema *jsonschema.Schema) {
	for _, requirement := range conditionalRequirements() {
		if requirement.Kind == kind {
			requirement.apply(schema)
		}
	}
}
