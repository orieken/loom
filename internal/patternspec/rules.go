package patternspec

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// ReferenceResolver answers whether a verify reference names something that
// exists. loom health supplies one that reads the installed skills and the
// MCP tool registry.
type ReferenceResolver interface {
	HasSkill(name string) bool
	HasTool(name string) bool
}

type contractRule func(Spec, ReferenceResolver) []Issue

// contractRules are the rules shared/contracts/pattern-spec-contract.md
// adds to the schema, in the order their issues are reported.
var contractRules = []contractRule{
	uniqueStepIDs,
	humanStepReasons,
	consumedValues,
	uniqueInputNames,
	enumInputValues,
	inputDefaults,
	verifyReferences,
	minedEvidence,
	unusedInputs,
}

// Validate applies the contract rules to a spec that parsed cleanly.
func Validate(spec Spec, references ReferenceResolver) []Issue {
	var issues []Issue
	for _, rule := range contractRules {
		issues = append(issues, rule(spec, references)...)
	}
	return issues
}

func stepField(index int, field string) string { return fmt.Sprintf("steps[%d].%s", index, field) }

func inputField(index int, field string) string { return fmt.Sprintf("inputs[%d].%s", index, field) }

func uniqueStepIDs(spec Spec, _ ReferenceResolver) []Issue {
	var issues []Issue
	seen := map[string]bool{}
	for index, step := range spec.Steps {
		if seen[step.ID] {
			issues = append(issues, failure(stepField(index, "id"), fmt.Sprintf("duplicate step id %q", step.ID)))
		}
		seen[step.ID] = true
	}
	return issues
}

func humanStepReasons(spec Spec, _ ReferenceResolver) []Issue {
	var issues []Issue
	for index, step := range spec.Steps {
		issues = append(issues, reasonIssues(index, step)...)
	}
	return issues
}

func reasonIssues(index int, step Step) []Issue {
	label := fmt.Sprintf(" (step %q)", step.ID)
	if step.IsHuman() && !step.HasReason() {
		return []Issue{failure(stepField(index, "reason"), "a human step MUST give a reason, one of: "+joinQuoted(stringsOf(HumanReasons()))+label)}
	}
	if !step.IsHuman() && step.HasReason() {
		return []Issue{failure(stepField(index, "reason"), "only a human step gives a reason"+label)}
	}
	return nil
}

// consumedValues requires every consumed name to be an input or the output
// of an earlier step. Steps are a linear list, so "earlier" is list order.
func consumedValues(spec Spec, _ ReferenceResolver) []Issue {
	available := map[string]bool{}
	for _, input := range spec.Inputs {
		available[input.Name] = true
	}
	var issues []Issue
	for index, step := range spec.Steps {
		issues = append(issues, unavailableValues(index, step, available)...)
		for _, produced := range step.Produces {
			available[produced] = true
		}
	}
	return issues
}

func unavailableValues(index int, step Step, available map[string]bool) []Issue {
	var issues []Issue
	for position, consumed := range step.Consumes {
		if !available[consumed] {
			field := fmt.Sprintf("steps[%d].consumes[%d]", index, position)
			issues = append(issues, failure(field, fmt.Sprintf("%q is not an input or the output of an earlier step (step %q)", consumed, step.ID)))
		}
	}
	return issues
}

func uniqueInputNames(spec Spec, _ ReferenceResolver) []Issue {
	var issues []Issue
	seen := map[string]bool{}
	for index, input := range spec.Inputs {
		if seen[input.Name] {
			issues = append(issues, failure(inputField(index, "name"), fmt.Sprintf("duplicate input name %q", input.Name)))
		}
		seen[input.Name] = true
	}
	return issues
}

func enumInputValues(spec Spec, _ ReferenceResolver) []Issue {
	var issues []Issue
	for index, input := range spec.Inputs {
		if input.IsEnum() && len(input.Values) == 0 {
			issues = append(issues, failure(inputField(index, "values"), "an enum input MUST list its values"))
		}
		if !input.IsEnum() && len(input.Values) > 0 {
			issues = append(issues, failure(inputField(index, "values"), "only an enum input lists values"))
		}
	}
	return issues
}

func inputDefaults(spec Spec, _ ReferenceResolver) []Issue {
	var issues []Issue
	for index, input := range spec.Inputs {
		issues = append(issues, defaultIssues(index, input)...)
	}
	return issues
}

func defaultIssues(index int, input Input) []Issue {
	if !input.HasDefault() {
		return nil
	}
	if input.Required {
		return []Issue{failure(inputField(index, "default"), "a required input has no default")}
	}
	if !defaultMatchesType(input) {
		return []Issue{failure(inputField(index, "default"), "does not match the input type "+string(input.Type))}
	}
	return nil
}

// defaultMatchers narrow the one heterogeneous field, Input.Default, by the
// input's declared type. Defaults arrive through encoding/json, so a number
// is a float64 and a list is []any.
var defaultMatchers = map[InputType]func(Input) bool{
	InputString:  func(input Input) bool { return isString(input.Default) },
	InputPath:    func(input Input) bool { return isString(input.Default) },
	InputNumber:  func(input Input) bool { _, isNumber := input.Default.(float64); return isNumber },
	InputBoolean: func(input Input) bool { _, isBool := input.Default.(bool); return isBool },
	InputList:    func(input Input) bool { return isListOfStrings(input.Default) },
	InputEnum: func(input Input) bool {
		value, isText := input.Default.(string)
		return isText && slices.Contains(input.Values, value)
	},
}

func defaultMatchesType(input Input) bool {
	matches, found := defaultMatchers[input.Type]
	return found && matches(input)
}

func isString(value any) bool {
	_, isText := value.(string)
	return isText
}

func isListOfStrings(value any) bool {
	list, isList := value.([]any)
	if !isList {
		return false
	}
	return !slices.ContainsFunc(list, func(element any) bool { return !isString(element) })
}

func verifyReferences(spec Spec, references ReferenceResolver) []Issue {
	var issues []Issue
	for index, reference := range spec.Verify {
		if !referenceExists(reference, references) {
			field := fmt.Sprintf("verify[%d].name", index)
			issues = append(issues, failure(field, fmt.Sprintf("no %s named %q exists", reference.Kind, reference.Name)))
		}
	}
	return issues
}

func referenceExists(reference VerifyReference, references ReferenceResolver) bool {
	if references == nil {
		return false
	}
	if reference.Kind == VerifySkill {
		return references.HasSkill(reference.Name)
	}
	return references.HasTool(reference.Name)
}

func minedEvidence(spec Spec, _ ReferenceResolver) []Issue {
	evidence := spec.Provenance.Evidence
	if spec.Provenance.IsMined() && evidence == nil {
		return []Issue{failure("provenance.evidence", "a mined spec MUST carry its evidence")}
	}
	if evidence == nil {
		return nil
	}
	return evidenceDateIssues(*evidence)
}

func evidenceDateIssues(evidence Evidence) []Issue {
	first, firstErr := time.Parse(dateLayout, evidence.FirstSeen)
	last, lastErr := time.Parse(dateLayout, evidence.LastSeen)
	if firstErr != nil || lastErr != nil || !last.Before(first) {
		return nil
	}
	return []Issue{failure("provenance.evidence.lastSeen", "is before firstSeen")}
}

// unusedInputs warns about an input no step consumes. Mined specs can carry
// one, and a reviewer SHOULD see it.
func unusedInputs(spec Spec, _ ReferenceResolver) []Issue {
	consumed := map[string]bool{}
	for _, step := range spec.Steps {
		for _, name := range step.Consumes {
			consumed[name] = true
		}
	}
	var issues []Issue
	for index, input := range spec.Inputs {
		if !consumed[input.Name] {
			issues = append(issues, warning(inputField(index, "name"), fmt.Sprintf("no step consumes input %q", input.Name)))
		}
	}
	return issues
}

// joinQuoted renders allowed values as "a", "b", "c".
func joinQuoted(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return strings.Join(quoted, ", ")
}
