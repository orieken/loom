package state

// Closed-set field types derive their JSON Schema enum from their own
// constants (roadmap L2.25).
//
// security_state.go requires stride[].category to be one of six exact
// literals — SPOOFING, INFORMATION_DISCLOSURE, and so on — while the schema
// handed to the agent declared it as a bare {"type": "string"}: no enum, no
// list of legal values. The agent was validated against a rule it was never
// told. It emitted all six categories, correctly and completely assessed, in
// the title case the agent definition itself uses, and the second real
// end-to-end run failed twice at ~$0.65 each with
//
//	field "stride" does not assess SPOOFING — the contract requires all six
//
// which is not what happened: spoofing WAS assessed. The message reported a
// security gap where there was a serialization mismatch.
//
// findings[].severity in the same struct carried a hand-written enum tag, so
// the generator could always express this — the author of each type simply
// had to remember. Deriving it from the constants removes the remembering,
// and enumeratedFieldsCarryTheirEnum asserts no closed-set field ships
// without one.

import "github.com/invopop/jsonschema"

// Enumerated is a field type whose legal values are a closed set. Anything
// implementing it publishes those values once, and both the validator and
// the generated schema read them from there.
type Enumerated interface {
	// Values returns every legal value, in the order the schema lists them.
	Values() []string
}

// enumSchema builds the schema for a closed-set string type.
func enumSchema(values []string) *jsonschema.Schema {
	schema := &jsonschema.Schema{Type: "string"}
	for _, value := range values {
		schema.Enum = append(schema.Enum, value)
	}
	return schema
}

// Values lists the STRIDE categories. Uppercase with underscores is the
// serialization; the agent definition writes them in title case, which is
// exactly why the schema has to say so.
func (StrideCategory) Values() []string {
	return []string{
		string(StrideSpoofing), string(StrideTampering), string(StrideRepudiation),
		string(StrideInformationDisclosure), string(StrideDenialOfService),
		string(StrideElevationOfPrivilege),
	}
}

// JSONSchema makes the generated schema carry the categories the validator
// enforces.
func (c StrideCategory) JSONSchema() *jsonschema.Schema { return enumSchema(c.Values()) }

// Values lists the severity ladder.
func (Severity) Values() []string {
	return []string{
		string(SeverityCritical), string(SeverityHigh), string(SeverityMedium),
		string(SeverityLow), string(SeverityInfo),
	}
}

// JSONSchema derives severity's enum from the same constants valid() checks.
func (s Severity) JSONSchema() *jsonschema.Schema { return enumSchema(s.Values()) }

// Values lists the review verdicts.
func (Verdict) Values() []string {
	return []string{string(VerdictApproved), string(VerdictChangesRequested)}
}

// JSONSchema derives the review verdict's enum.
func (v Verdict) JSONSchema() *jsonschema.Schema { return enumSchema(v.Values()) }

// Values lists the fitness-check verdicts.
func (CheckVerdict) Values() []string {
	return []string{string(VerdictPass), string(VerdictFail), string(VerdictNotApplicable)}
}

// JSONSchema derives the check verdict's enum.
func (v CheckVerdict) JSONSchema() *jsonschema.Schema { return enumSchema(v.Values()) }

// Values lists the migration phases.
func (MigrationPhase) Values() []string {
	return []string{
		string(MigrationPhaseNone), string(MigrationPhaseExpand), string(MigrationPhaseContract),
	}
}

// JSONSchema derives the migration phase's enum.
func (p MigrationPhase) JSONSchema() *jsonschema.Schema { return enumSchema(p.Values()) }
