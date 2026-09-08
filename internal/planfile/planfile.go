// Package planfile loads a pipeline definition from YAML (roadmap L3.27).
//
// Until this existed there was exactly one pipeline and it was a Go
// function: `loom run --plan x` answered "only deliver-feature exists today".
// A team wanting a shorter pipeline for a bugfix, or the same one minus a
// stage they do not staff, had to edit Go and rebuild.
//
// A plan file selects and orders stages from the catalogue the framework
// ships. It does NOT define them. Everything that makes a stage safe — its
// approval gate, its typed contract, what it reads, whether the router may
// skip it, its timeout — comes from orchestrator.BuiltInStages() and cannot
// be restated here.
//
// That restriction is the design, not a limitation to lift later:
//
//   - L3.24 measured a stage marked always-runs on a feature it could not
//     serve at $0.55-0.88 a time. A format letting each project declare its
//     own skippability would hand that bill to every project that wrote one.
//   - L2.17 put a bound on the review loop because prose said "repeat until
//     APPROVED". A plan names a built-in loop; it does not invent a bound.
//   - Gates are inherited. A plan drops a gate only by dropping the stage it
//     guards, which is a visible act, not a field someone can set to "".
package planfile

import (
	"fmt"
	"strings"

	"github.com/orieken/loom/internal/orchestrator"
	yaml "go.yaml.in/yaml/v4"
)

// Version is the plan-file schema version this build reads.
const Version = 1

// Definition is the YAML shape. Deliberately small: a name, an ordered list
// of stage IDs, and the names of the built-in loops that apply.
type Definition struct {
	Version     int      `yaml:"version"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Stages      []string `yaml:"stages"`
	Loops       []string `yaml:"loops"`
}

// Parse reads a plan definition and resolves it against the built-in
// catalogue, returning a plan the executor can run.
func Parse(source []byte, displayPath string) (orchestrator.Plan, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(source, &document); err != nil {
		return orchestrator.Plan{}, fmt.Errorf("%s: %w", displayPath, err)
	}
	var definition Definition
	if err := document.Decode(&definition); err != nil {
		return orchestrator.Plan{}, fmt.Errorf("%s: %w", displayPath, err)
	}
	lines := newLineIndex(&document)
	if err := definition.validate(lines, displayPath); err != nil {
		return orchestrator.Plan{}, err
	}
	return definition.resolve(lines, displayPath)
}

func (d Definition) validate(lines lineIndex, path string) error {
	if d.Version != Version {
		return lines.errorAt("version", path,
			fmt.Sprintf("plan file version is %d, this build reads %d", d.Version, Version))
	}
	if strings.TrimSpace(d.Name) == "" {
		return lines.errorAt("name", path, "plan has no name")
	}
	if d.Name == orchestrator.DefaultDeliverFeaturePlanName {
		return lines.errorAt("name", path,
			fmt.Sprintf("%q is the built-in plan's name; choose another so the two cannot be confused", d.Name))
	}
	if len(d.Stages) == 0 {
		return lines.errorAt("stages", path, "plan lists no stages")
	}
	return nil
}

// resolve turns stage IDs into configured stages, then applies the checks
// that need the resolved plan in hand.
func (d Definition) resolve(lines lineIndex, path string) (orchestrator.Plan, error) {
	catalogue := orchestrator.BuiltInStages()
	stages := make([]orchestrator.Stage, 0, len(d.Stages))
	for index, id := range d.Stages {
		stage, known := catalogue[id]
		if !known {
			return orchestrator.Plan{}, lines.errorAtItem("stages", index, path, unknownStageMessage(id, catalogue))
		}
		stages = append(stages, stage)
	}
	loops, err := d.resolveLoops(lines, path)
	if err != nil {
		return orchestrator.Plan{}, err
	}
	plan := orchestrator.Plan{Name: d.Name, Stages: stages, Loops: loops}
	return plan, validateResolved(plan, lines, path)
}

func (d Definition) resolveLoops(lines lineIndex, path string) ([]orchestrator.Loop, error) {
	available := orchestrator.BuiltInLoops()
	loops := make([]orchestrator.Loop, 0, len(d.Loops))
	for index, id := range d.Loops {
		loop, known := available[id]
		if !known {
			return nil, lines.errorAtItem("loops", index, path,
				fmt.Sprintf("unknown loop %q; the framework defines %s", id, quotedKeys(loopIDs(available))))
		}
		loops = append(loops, loop)
	}
	return loops, nil
}

func validateResolved(plan orchestrator.Plan, lines lineIndex, path string) error {
	if err := plan.Validate(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	present := stageIDSet(plan)
	if err := validateUpstreams(plan, present, lines, path); err != nil {
		return err
	}
	if err := validateLoopSpans(plan, present, lines, path); err != nil {
		return err
	}
	return validateRouterPresence(plan, present, lines, path)
}

// validateUpstreams checks every stage can read what its contract says it
// reads. Presence only, never order: `developer` consumes `code-reviewer`,
// which runs after it, because on a second loop round the developer reads
// the findings that sent it back. Requiring upstreams to appear earlier
// would reject the built-in plan.
func validateUpstreams(plan orchestrator.Plan, present map[string]bool, lines lineIndex, path string) error {
	for index, stage := range plan.Stages {
		for _, upstream := range stage.Consumes {
			if !present[upstream] {
				return lines.errorAtItem("stages", index, path, fmt.Sprintf(
					"stage %q reads state from %q, which this plan does not include", stage.ID, upstream))
			}
		}
	}
	return nil
}

func validateLoopSpans(plan orchestrator.Plan, present map[string]bool, lines lineIndex, path string) error {
	for index, loop := range plan.Loops {
		for _, endpoint := range []string{loop.From, loop.To} {
			if !present[endpoint] {
				return lines.errorAtItem("loops", index, path, fmt.Sprintf(
					"loop %q spans %q, which this plan does not include", loop.ID, endpoint))
			}
		}
		if positionOf(plan, loop.From) > positionOf(plan, loop.To) {
			return lines.errorAtItem("loops", index, path, fmt.Sprintf(
				"loop %q runs from %q to %q, but this plan orders them the other way round",
				loop.ID, loop.From, loop.To))
		}
	}
	return nil
}

// validateRouterPresence keeps L3.24's saving from being lost by omission.
// A skippable stage with no router is a stage that always runs, which is
// exactly the condition L3.24 was filed against — and it would happen
// silently, since nothing else reports a route that was never computed.
func validateRouterPresence(plan orchestrator.Plan, present map[string]bool, lines lineIndex, path string) error {
	if present[orchestrator.RouterStageID] {
		return nil
	}
	for index, stage := range plan.Stages {
		if stage.Skippable {
			return lines.errorAtItem("stages", index, path, fmt.Sprintf(
				"stage %q is routable but this plan omits %q, so it would always run; "+
					"add the router or drop the stage",
				stage.ID, orchestrator.RouterStageID))
		}
	}
	return nil
}

func unknownStageMessage(id string, catalogue map[string]orchestrator.Stage) string {
	return fmt.Sprintf("unknown stage %q; a plan may only select from the stages the framework "+
		"ships: %s", id, quotedKeys(stageIDs(catalogue)))
}

func stageIDSet(plan orchestrator.Plan) map[string]bool {
	present := make(map[string]bool, len(plan.Stages))
	for _, stage := range plan.Stages {
		present[stage.ID] = true
	}
	return present
}

func positionOf(plan orchestrator.Plan, id string) int {
	for index, stage := range plan.Stages {
		if stage.ID == id {
			return index
		}
	}
	return -1
}
