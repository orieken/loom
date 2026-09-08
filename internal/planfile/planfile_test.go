package planfile_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/planfile"
)

// TestTheBuiltInPlanIsExpressibleInTheFormat is L3.27's done-when: "the
// built-in deliver-feature plan is expressible in the format without
// special-casing".
//
// The YAML is generated from the built-in plan's own stage order rather than
// hand-written, so adding a stage to the built-in plan does not require
// editing this test — and if the format ever cannot carry something the
// built-in plan needs, this fails instead of the gap being discovered by a
// project that wrote a plan.
//
// The name necessarily differs: the format refuses the built-in's name so
// the two cannot be confused. Everything that decides behaviour is compared.
func TestTheBuiltInPlanIsExpressibleInTheFormat(t *testing.T) {
	builtIn := orchestrator.DefaultDeliverFeaturePlan()

	parsed, err := planfile.Parse(yamlFor("round-trip", builtIn), "generated.yaml")
	if err != nil {
		t.Fatalf("the built-in plan does not survive its own format: %v", err)
	}

	if !reflect.DeepEqual(parsed.Stages, builtIn.Stages) {
		t.Errorf("stages differ after a round trip:\n got %+v\nwant %+v", parsed.Stages, builtIn.Stages)
	}
	if !reflect.DeepEqual(parsed.Loops, builtIn.Loops) {
		t.Errorf("loops differ after a round trip:\n got %+v\nwant %+v", parsed.Loops, builtIn.Loops)
	}
}

// A plan selects stages; it cannot redefine them. Everything that makes a
// stage safe is inherited, so a plan cannot drop a gate, un-type a contract,
// or make a routed stage unconditional.
func TestAPlanCannotWeakenAStage(t *testing.T) {
	plan := mustParse(t, `
version: 1
name: gated
stages: [context-engineer, analyst, router, developer, code-reviewer]
`)

	developer := stageNamed(t, plan, "developer")
	if developer.Gate != orchestrator.GateConfirmDesign {
		t.Errorf("developer gate = %q, want the built-in %q — a plan must not be able to drop it",
			developer.Gate, orchestrator.GateConfirmDesign)
	}
	if developer.StateKind == "" {
		t.Error("developer lost its typed contract in the round trip")
	}
}

// The shipped deliver-bugfix plan must parse, or the framework ships a
// broken example.
func TestTheShippedBugfixPlanParses(t *testing.T) {
	path := filepath.Join("..", "..", "shared", "plans", "deliver-bugfix.yaml")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shipped plan: %v", err)
	}

	plan, err := planfile.Parse(source, path)
	if err != nil {
		t.Fatalf("the shipped deliver-bugfix plan does not parse: %v", err)
	}

	if plan.Name != "deliver-bugfix" {
		t.Errorf("name = %q", plan.Name)
	}
	// It must be genuinely shorter than the built-in, or it is not a
	// different pipeline.
	if len(plan.Stages) >= len(orchestrator.DefaultDeliverFeaturePlan().Stages) {
		t.Errorf("deliver-bugfix has %d stages, not fewer than deliver-feature's %d",
			len(plan.Stages), len(orchestrator.DefaultDeliverFeaturePlan().Stages))
	}
	// And it must keep the gates that guard writing code and shipping it.
	if stageNamed(t, plan, "developer").Gate != orchestrator.GateConfirmDesign {
		t.Error("deliver-bugfix writes code without a design gate")
	}
}

func TestInvalidPlansAreRejectedWithTheLineThatIsWrong(t *testing.T) {
	cases := map[string]struct{ source, wants string }{
		"unknown stage": {`
version: 1
name: typo
stages:
  - context-engineer
  - my-custom-linter
`, `unknown stage "my-custom-linter"`},
		"missing upstream": {`
version: 1
name: no-analyst
stages:
  - context-engineer
  - developer
  - code-reviewer
  - qa-engineer
`, `reads state from`},
		// L3.24's saving must not be lost by omission: a routable stage with
		// no router always runs, and nothing would report the route that was
		// never computed.
		"routable stage without the router": {`
version: 1
name: unrouted
stages:
  - context-engineer
  - analyst
  - architect
  - developer
  - code-reviewer
`, `omits "router"`},
		"the built-in name": {`
version: 1
name: deliver-feature
stages: [context-engineer]
`, `built-in plan's name`},
		"a future version": {`
version: 99
name: ahead
stages: [context-engineer]
`, `this build reads 1`},
		"no stages": {`
version: 1
name: empty
stages: []
`, `lists no stages`},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := planfile.Parse([]byte(testCase.source), "plan.yaml")
			if err == nil {
				t.Fatal("accepted an invalid plan")
			}
			if !strings.Contains(err.Error(), testCase.wants) {
				t.Errorf("error %q does not explain the problem (%q)", err, testCase.wants)
			}
			if !strings.Contains(err.Error(), "plan.yaml:") {
				t.Errorf("error %q names no line; the reader has to search a file "+
					"they were just told is broken", err)
			}
		})
	}
}

// A loop may only name one the framework defines, so a project cannot write
// its own bound — L2.17 put a number on the review loop for a reason.
func TestAPlanCannotInventALoop(t *testing.T) {
	_, err := planfile.Parse([]byte(`
version: 1
name: unbounded
stages: [context-engineer, developer, code-reviewer]
loops: [forever]
`), "plan.yaml")

	if err == nil || !strings.Contains(err.Error(), `unknown loop "forever"`) {
		t.Fatalf("error = %v, want a rejection naming the unknown loop", err)
	}
}

// A named loop whose span the plan does not contain is rejected, rather than
// silently never firing.
func TestALoopMustSpanStagesThePlanContains(t *testing.T) {
	_, err := planfile.Parse([]byte(`
version: 1
name: no-reviewer
stages: [context-engineer, analyst, developer]
loops: [review]
`), "plan.yaml")

	if err == nil || !strings.Contains(err.Error(), "does not include") {
		t.Fatalf("error = %v, want a rejection naming the missing endpoint", err)
	}
}

func mustParse(t *testing.T, source string) orchestrator.Plan {
	t.Helper()
	plan, err := planfile.Parse([]byte(source), "plan.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return plan
}

func stageNamed(t *testing.T, plan orchestrator.Plan, id string) orchestrator.Stage {
	t.Helper()
	for _, stage := range plan.Stages {
		if stage.ID == id {
			return stage
		}
	}
	t.Fatalf("plan %q has no stage %q", plan.Name, id)
	return orchestrator.Stage{}
}

// yamlFor renders a plan as a plan file, so the round-trip test describes
// the built-in plan rather than restating it.
func yamlFor(name string, plan orchestrator.Plan) []byte {
	var builder strings.Builder
	fmt.Fprintf(&builder, "version: %d\nname: %s\nstages:\n", planfile.Version, name)
	for _, stage := range plan.Stages {
		fmt.Fprintf(&builder, "  - %s\n", stage.ID)
	}
	if len(plan.Loops) > 0 {
		builder.WriteString("loops:\n")
		for _, loop := range plan.Loops {
			fmt.Fprintf(&builder, "  - %s\n", loop.ID)
		}
	}
	return []byte(builder.String())
}
