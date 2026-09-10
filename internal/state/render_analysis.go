package state

import (
	"fmt"
	"strings"
)

// renderAnalysis writes analysis.md with the exact headings
// analysis-contract.md requires, in contract order.
func renderAnalysis(analysis AnalysisState) string {
	doc := &document{}
	doc.frontmatter(analysis.frontmatter())
	doc.title("Feature Analysis: " + analysis.Feature)
	doc.section("## Summary", []string{analysis.Summary})
	doc.section("### Acceptance Criteria", criteriaLines(analysis.AcceptanceCriteria))
	doc.section("### Non-Functional Requirements", requirementLines(analysis.NonFunctionalRequirements))
	doc.section("## Proposed Fitness Functions", fitnessLines(analysis.ProposedFitnessFunctions))
	doc.section("## Out of Scope", bullets(analysis.OutOfScope))
	doc.section("## Technical Breakdown", nil)
	renderBreakdown(doc, analysis)
	doc.section("## Task List", nil)
	renderTasks(doc, analysis.Tasks)
	doc.section("## Edge Cases and Risks", edgeCaseLines(analysis.EdgeCases))
	doc.section("## Definition of Done", bullets(analysis.DefinitionOfDone))
	return doc.String()
}

func renderBreakdown(doc *document, analysis AnalysisState) {
	doc.section("### Bounded Context", contextLines(analysis.BoundedContext))
	doc.section("### Domain Events (Event Storming Lite)", eventLines(analysis.DomainEvents))
	doc.section("### Affected Components", componentLines(analysis.AffectedComponents))
	doc.section("### Data Model Changes", dataModelLines(analysis.DataModelChanges))
	doc.section("### API Changes", apiLines(analysis.APIChanges))
	doc.section("### New Dependencies", bullets(analysis.NewDependencies))
}

func renderTasks(doc *document, tasks TaskList) {
	doc.section("### Developer Tasks", numbered(tasks.Developer))
	doc.section("### QA Tasks", numbered(tasks.QA))
	doc.section("### Tech Writer Tasks", numbered(tasks.TechWriter))
	doc.section("### DevOps Tasks", numbered(tasks.DevOps))
}

func criteriaLines(criteria []AcceptanceCriterion) []string {
	lines := make([]string, 0, len(criteria))
	for _, criterion := range criteria {
		lines = append(lines, "- "+criterion.Statement)
		for _, example := range criterion.Examples {
			lines = append(lines, "  - Example: "+example)
		}
	}
	return lines
}

func requirementLines(requirements []NonFunctionalRequirement) []string {
	lines := make([]string, 0, len(requirements))
	for _, requirement := range requirements {
		lines = append(lines, fmt.Sprintf("- **%s**: %s%s",
			requirement.Category, requirement.Requirement, thresholdSuffix(requirement.Threshold)))
	}
	return lines
}

func thresholdSuffix(threshold *Threshold) string {
	if !threshold.IsMeasurable() {
		return ""
	}
	return " (" + threshold.String() + ")"
}

func fitnessLines(functions []FitnessFunction) []string {
	lines := make([]string, 0, len(functions)*3)
	for _, function := range functions {
		lines = append(lines,
			fmt.Sprintf("- **%s**", function.Property),
			"  - **Verification**: "+function.Verification)
		if function.Owner != "" {
			lines = append(lines, "  - **Owner**: "+function.Owner)
		}
	}
	return lines
}

func contextLines(context BoundedContext) []string {
	lines := []string{"- **Owning Context**: " + context.Owning}
	if len(context.Crossings) == 0 {
		return append(lines, "- **Context Crossings**: None")
	}
	return append(lines, "- **Context Crossings**: "+joinOr(context.Crossings))
}

func eventLines(events DomainEvents) []string {
	pairs := []struct {
		label  string
		values []string
	}{
		{"Commands", events.Commands},
		{"Events Produced", events.EventsProduced},
		{"Owning Aggregates", events.OwningAggregates},
		{"Read Models / Projections", events.ReadModels},
	}
	lines := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		lines = append(lines, fmt.Sprintf("- **%s**: %s", pair.label, joinOr(pair.values)))
	}
	return lines
}

func componentLines(components []AffectedComponent) []string {
	lines := make([]string, 0, len(components))
	for _, component := range components {
		lines = append(lines, fmt.Sprintf("- `%s` — %s", component.Path, component.Reason))
	}
	return lines
}

func dataModelLines(changes []DataModelChange) []string {
	lines := make([]string, 0, len(changes))
	for _, change := range changes {
		lines = append(lines, fmt.Sprintf("- %s (**%s** phase)", change.Description, change.Phase))
	}
	return lines
}

func apiLines(changes []APIChange) []string {
	lines := make([]string, 0, len(changes))
	for _, change := range changes {
		lines = append(lines, fmt.Sprintf("- `%s` — %s", change.Endpoint, change.Change))
	}
	return lines
}

func edgeCaseLines(cases []EdgeCase) []string {
	lines := make([]string, 0, len(cases))
	for _, edge := range cases {
		lines = append(lines, fmt.Sprintf("- %s: %s", edge.Case, edge.Handling))
	}
	return lines
}

// joinOr renders a list inline, or the explicit "None" the contracts ask
// for when it is empty.
func joinOr(values []string) string {
	if len(values) == 0 {
		return "None"
	}
	return joinComma(values)
}

func joinComma(values []string) string {
	return strings.Join(values, ", ")
}

// render satisfies the renderable interface.
func (a AnalysisState) render() string { return renderAnalysis(a) }
