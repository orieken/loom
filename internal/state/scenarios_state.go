package state

// ScenariosState is the acceptance-scenarios stage's output: Gherkin
// scenarios written from the analysis's acceptance criteria before any
// implementation exists (roadmap L3.62, ADR-009).
//
// The independence ADR-009 keeps is that acceptance tests come from what was
// asked for, not from what was built. qa-engineer used to write them after
// the developer, reading implementation notes and a working tree that already
// held the code — so its scenarios could describe the build rather than the
// requirement. This stage runs first and reads only the analysis; blindness
// is a property of order, which no projection can take back afterwards.

import "fmt"

// AcceptanceScenario is one Gherkin scenario, traced to the criterion it
// verifies by that criterion's statement.
type AcceptanceScenario struct {
	Criterion string   `json:"criterion" jsonschema:"required,description=The acceptance criterion's statement this scenario verifies, copied from the analysis"`
	Name      string   `json:"name" jsonschema:"required,description=Scenario name in business language; it IS the acceptance criterion as a test"`
	Given     []string `json:"given,omitempty" jsonschema:"description=Preconditions, in business language"`
	When      []string `json:"when" jsonschema:"required,minItems=1,description=The action, in business language — never a click path or an API call"`
	Then      []string `json:"then" jsonschema:"required,minItems=1,description=The observable outcome that decides pass or fail"`
}

// ScenariosState is the whole stage document.
type ScenariosState struct {
	SchemaVersion int                  `json:"schemaVersion" jsonschema:"required"`
	Feature       string               `json:"feature" jsonschema:"required"`
	Scenarios     []AcceptanceScenario `json:"scenarios" jsonschema:"required,minItems=1"`
	Retrieval     Retrieval            `json:"retrieval,omitempty"`
}

// Validate requires at least one scenario, and every scenario to name its
// criterion and to have an action and an outcome: a scenario without a Then
// cannot fail, which is ADR-008's first objection to any test.
func (s ScenariosState) Validate() error {
	return firstError(
		requireSchemaVersion(s.SchemaVersion),
		requireText("feature", s.Feature),
		requireItems("scenarios", len(s.Scenarios)),
		requireCompleteScenarios(s.Scenarios),
	)
}

func requireCompleteScenarios(scenarios []AcceptanceScenario) error {
	for index, scenario := range scenarios {
		field := fmt.Sprintf("scenarios[%d]", index)
		if err := firstError(
			requireText(field+".criterion", scenario.Criterion),
			requireText(field+".name", scenario.Name),
			requireItems(field+".when", len(scenario.When)),
			requireItems(field+".then", len(scenario.Then)),
		); err != nil {
			return err
		}
	}
	return nil
}
