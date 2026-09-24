package state

import "strings"

// renderScenarios writes acceptance-scenarios.md: one Gherkin block per
// scenario, each preceded by the criterion it verifies, so a reviewer can
// check coverage of the analysis without opening it.
func renderScenarios(scenarios ScenariosState) string {
	doc := &document{}
	doc.frontmatter(newFrontmatter(scenarios.Feature, "", scenarios.Retrieval, nil))
	doc.title("Acceptance Scenarios: " + scenarios.Feature)
	for _, scenario := range scenarios.Scenarios {
		doc.section("## "+scenario.Name, gherkinLines(scenario))
	}
	return doc.String()
}

func gherkinLines(scenario AcceptanceScenario) []string {
	lines := []string{"Verifies: " + scenario.Criterion, "", "```gherkin", "Scenario: " + scenario.Name}
	lines = append(lines, steps("Given", scenario.Given)...)
	lines = append(lines, steps("When", scenario.When)...)
	lines = append(lines, steps("Then", scenario.Then)...)
	return append(lines, "```")
}

// steps renders a keyword once and continues with And, as Gherkin reads.
func steps(keyword string, clauses []string) []string {
	lines := make([]string, 0, len(clauses))
	for index, clause := range clauses {
		lead := keyword
		if index > 0 {
			lead = "And"
		}
		lines = append(lines, "  "+lead+" "+strings.TrimSpace(clause))
	}
	return lines
}

// render satisfies the renderable interface.
func (s ScenariosState) render() string { return renderScenarios(s) }
