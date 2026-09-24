package state_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/state"
)

func validScenarios() state.ScenariosState {
	return state.ScenariosState{
		SchemaVersion: state.SchemaVersion,
		Feature:       "password reset",
		Scenarios: []state.AcceptanceScenario{{
			Criterion: "A user can reset a forgotten password by email",
			Name:      "reset link arrives for a registered address",
			Given:     []string{"a registered user", "who has forgotten their password"},
			When:      []string{"they ask for a reset link"},
			Then:      []string{"a single-use link is emailed to them", "it expires after one hour"},
		}},
	}
}

func TestACompleteScenarioSetValidates(t *testing.T) {
	if err := validScenarios().Validate(); err != nil {
		t.Errorf("Validate() = %v, want a complete scenario set accepted", err)
	}
}

// A scenario without a Then cannot fail — ADR-008's first objection to any
// test — and one without a criterion cannot be traced to what was asked.
func TestAnIncompleteScenarioIsRejectedByField(t *testing.T) {
	for field, breakIt := range map[string]func(*state.ScenariosState){
		"schemaVersion":          func(s *state.ScenariosState) { s.SchemaVersion = 0 },
		"feature":                func(s *state.ScenariosState) { s.Feature = "" },
		"scenarios":              func(s *state.ScenariosState) { s.Scenarios = nil },
		"scenarios[0].criterion": func(s *state.ScenariosState) { s.Scenarios[0].Criterion = "" },
		"scenarios[0].name":      func(s *state.ScenariosState) { s.Scenarios[0].Name = "" },
		"scenarios[0].when":      func(s *state.ScenariosState) { s.Scenarios[0].When = nil },
		"scenarios[0].then":      func(s *state.ScenariosState) { s.Scenarios[0].Then = nil },
	} {
		t.Run(field, func(t *testing.T) {
			scenarios := validScenarios()
			breakIt(&scenarios)
			err := scenarios.Validate()
			if err == nil || !strings.Contains(err.Error(), `"`+field+`"`) {
				t.Errorf("Validate() = %v, want a failure naming %q", err, field)
			}
		})
	}
}

func TestScenariosRenderAsTracedGherkin(t *testing.T) {
	payload, err := json.Marshal(validScenarios())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	name, body, err := state.RenderView(state.KindScenarios, payload)
	if err != nil {
		t.Fatalf("RenderView: %v", err)
	}
	if name != "acceptance-scenarios.md" {
		t.Errorf("view file = %q, want acceptance-scenarios.md", name)
	}
	for _, want := range []string{
		"Verifies: A user can reset a forgotten password by email",
		"Scenario: reset link arrives for a registered address",
		"  Given a registered user\n  And who has forgotten their password\n",
		"  When they ask for a reset link\n",
		"  Then a single-use link is emailed to them\n  And it expires after one hour\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered view lacks %q:\n%s", want, body)
		}
	}
}

func TestQAReceivesEveryScenarioUnchanged(t *testing.T) {
	payload, err := json.Marshal(validScenarios())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	projected, err := state.ProjectionFor("qa-engineer", state.KindScenarios, payload)
	if err != nil {
		t.Fatalf("ProjectionFor: %v", err)
	}
	var given state.QAScenariosInput
	if err := json.Unmarshal(projected, &given); err != nil {
		t.Fatalf("unmarshal projection: %v", err)
	}
	if len(given.Scenarios) != 1 || given.Scenarios[0].Name != validScenarios().Scenarios[0].Name {
		t.Errorf("qa-engineer was given %+v, want the scenario unchanged", given)
	}
	if _, err := state.ProjectionFor("qa-engineer", state.KindScenarios, []byte("{")); err == nil {
		t.Error("an unreadable scenarios document was projected")
	}
}
