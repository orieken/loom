package cmd

// `loom plan new` — compose a pipeline by picking stages (roadmap L3.37).
//
// The form does not validate. It gathers a name and a stage set, renders the
// same YAML a human would have written, and hands it to planfile.Parse — so
// a plan built here is subject to exactly the rules a hand-written one is,
// and a rule added to the loader applies to this command for free. A form
// with its own idea of what is legal is how the two drift.
//
// Stages are offered in the built-in plan's order and can only be
// deselected. Reordering is a text edit: the built-in order is the only one
// this command could offer without inventing a second source of truth for
// which order is right, and "the same pipeline minus the stages we do not
// staff" is the case that motivated L3.27.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/planfile"
	"github.com/spf13/cobra"
)

type planNewFlags struct {
	name         string
	output       string
	isAccessible bool
}

var planNewArgs planNewFlags

var planNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Compose a new plan by picking stages",
	Args:  cobra.NoArgs,
	RunE:  runPlanNew,
}

func init() {
	planNewCmd.Flags().StringVar(&planNewArgs.name, "name", "", "plan name (skips the name prompt)")
	planNewCmd.Flags().StringVar(&planNewArgs.output, "out", ".claude/plans", "directory to write the plan into")
	planNewCmd.Flags().BoolVar(&planNewArgs.isAccessible, "accessible", false,
		"use plain prompts instead of the full-screen form (screen readers, and terminals that do not answer capability queries)")
}

func runPlanNew(cmd *cobra.Command, _ []string) error {
	if !stdinIsInteractive() {
		return fmt.Errorf("`loom plan new` needs a terminal — write the YAML directly instead, " +
			"see shared/plans/deliver-bugfix.yaml for the shape")
	}
	draft := newDraft(planNewArgs.name)
	if err := draft.ask(planNewArgs.isAccessible); err != nil {
		return err
	}
	return draft.write(cmd, planNewArgs.output)
}

// draft is what the form fills in.
type draft struct {
	name        string
	description string
	stages      []string
	loops       []string
}

func newDraft(name string) *draft {
	return &draft{name: name, stages: planfile.SelectableStages()}
}

// ask runs the form. Accessible mode swaps the full-screen TUI for plain
// sequential prompts: it exists for screen readers, and it is also the only
// mode that works in a terminal which does not answer the capability queries
// (OSC 11, cursor position) the full-screen renderer waits on.
func (d *draft) ask(isAccessible bool) error {
	form := huh.NewForm(
		huh.NewGroup(d.nameField(), d.descriptionField()),
		huh.NewGroup(d.stageField()),
		huh.NewGroup(d.loopField()),
	).WithAccessible(isAccessible)
	return form.Run()
}

func (d *draft) nameField() huh.Field {
	return huh.NewInput().Title("Plan name").
		Description("Used as the filename and as `loom run --plan <name>`.").
		Value(&d.name).Validate(validatePlanName)
}

func (d *draft) descriptionField() huh.Field {
	return huh.NewText().Title("What is this pipeline for?").
		Description("Optional. Shown by `loom plan list`.").
		Value(&d.description)
}

// stageField offers the catalogue in the built-in plan's order, everything
// selected. Composing a pipeline is deselecting the stages a project does
// not want, which is the shape of the request L3.27 came from.
func (d *draft) stageField() huh.Field {
	options := make([]huh.Option[string], 0, len(planfile.SelectableStages()))
	for _, stage := range planfile.SelectableStages() {
		options = append(options, huh.NewOption(stageLabel(stage), stage).Selected(true))
	}
	return huh.NewMultiSelect[string]().Title("Stages").
		Description("Space toggles. Order is the built-in plan's — reorder by editing the file.").
		Options(options...).Value(&d.stages)
}

// stageLabel says what each stage brings with it, because a gate or a
// routing predicate is the reason to keep a stage and is invisible in a bare
// list of names.
func stageLabel(id string) string {
	stage := orchestrator.BuiltInStages()[id]
	var notes []string
	if stage.Gate != "" {
		notes = append(notes, "gate: "+stage.Gate)
	}
	if stage.Skippable {
		notes = append(notes, "routable")
	}
	if len(notes) == 0 {
		return id
	}
	return fmt.Sprintf("%s (%s)", id, strings.Join(notes, ", "))
}

func (d *draft) loopField() huh.Field {
	options := make([]huh.Option[string], 0, len(planfile.SelectableLoops()))
	for _, loop := range planfile.SelectableLoops() {
		options = append(options, huh.NewOption(loop, loop).Selected(true))
	}
	return huh.NewMultiSelect[string]().Title("Loops").
		Description("Bounds are the framework's; a plan names a loop, it does not define one.").
		Options(options...).Value(&d.loops)
}

// validatePlanName rejects what the loader would reject anyway, at the point
// the human can still fix it, plus what a filename cannot carry.
func validatePlanName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("a plan needs a name")
	}
	if trimmed == orchestrator.DefaultDeliverFeaturePlanName {
		return fmt.Errorf("%q is the built-in plan — choose another name", trimmed)
	}
	if strings.ContainsAny(trimmed, `/\ .`) {
		return fmt.Errorf("use a bare name without spaces, dots or slashes — it becomes a filename")
	}
	return nil
}

// write renders the plan, parses it the way `loom run` will, and only then
// puts it on disk. A plan that cannot be run is not written.
func (d *draft) write(cmd *cobra.Command, directory string) error {
	// Normalise before use, not only before validation. validatePlanName
	// trimmed and then the raw value became the filename, so a name typed
	// with a trailing space produced `my-plan .yaml` — found by driving the
	// real form, which is the only place the two paths meet.
	d.name = strings.TrimSpace(d.name)
	d.description = strings.TrimSpace(d.description)
	if err := validatePlanName(d.name); err != nil {
		return err
	}
	source := planfile.Render(d.name, d.description, d.stages, d.loops)
	path := filepath.Join(directory, d.name+".yaml")
	if _, err := planfile.Parse(source, path); err != nil {
		return fmt.Errorf("that combination is not a runnable plan: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists — delete it or choose another name", path)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", directory, err)
	}
	if err := os.WriteFile(path, source, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return reportNewPlan(cmd, d, path)
}

func reportNewPlan(cmd *cobra.Command, d *draft, path string) error {
	cmd.Printf("Wrote %s — %d stages, omitting %s.\n",
		path, len(d.stages), joinOrNone(omittedStages(d.stages)))
	cmd.Printf("Run it with: loom run --spec <spec> --plan %s\n", d.name)
	return nil
}
