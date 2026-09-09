package cmd

// `loom plan` — see which pipelines this project can run, and compose new
// ones (roadmap L3.37).
//
// L3.27 gave a pipeline a definition; this is the surface that makes the
// definition usable without knowing the YAML by heart. `list` and `show`
// work anywhere, including CI. `new` needs a terminal and says so.

import (
	"fmt"
	"strings"

	"github.com/orieken/loom/internal/planfile"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "List, inspect and compose pipeline plans",
	Args:  cobra.NoArgs,
	RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
}

var planListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the plans this project can run",
	Args:  cobra.NoArgs,
	RunE:  runPlanList,
}

var planShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show what a plan runs, stage by stage",
	Args:  cobra.ExactArgs(1),
	RunE:  runPlanShow,
}

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.AddCommand(planListCmd, planShowCmd, planNewCmd)
}

func runPlanList(cmd *cobra.Command, _ []string) error {
	for _, available := range planfile.Discover(planfile.SearchPath()) {
		cmd.Println(planListLine(available))
	}
	return nil
}

// planListLine keeps a broken plan visible. A plan that vanished from the
// list because it failed to parse is harder to debug than one that says so.
func planListLine(available planfile.Available) string {
	if available.Err != nil {
		return fmt.Sprintf("  %-20s BROKEN — %v", available.Name, available.Err)
	}
	return fmt.Sprintf("  %-20s %2d stages  %s", available.Name, len(available.Stages),
		planOrigin(available))
}

func planOrigin(available planfile.Available) string {
	if available.IsBuiltIn() {
		return "(built in)"
	}
	return available.Path
}

func runPlanShow(cmd *cobra.Command, args []string) error {
	for _, available := range planfile.Discover(planfile.SearchPath()) {
		if available.Name != args[0] {
			continue
		}
		return showPlan(cmd, available)
	}
	return fmt.Errorf("unknown plan %q — `loom plan list` shows what this project has", args[0])
}

func showPlan(cmd *cobra.Command, available planfile.Available) error {
	if available.Err != nil {
		return fmt.Errorf("plan %q does not parse: %w", available.Name, available.Err)
	}
	cmd.Printf("%s  %s\n", available.Name, planOrigin(available))
	if available.Description != "" {
		cmd.Printf("\n%s\n", available.Description)
	}
	cmd.Printf("\nStages (%d):\n", len(available.Stages))
	for index, stage := range available.Stages {
		cmd.Printf("  %2d. %s\n", index+1, stage)
	}
	cmd.Printf("\nRun it with: loom run --spec <spec> --plan %s\n", available.Name)
	return nil
}

// omittedStages names what a plan leaves out, which is the interesting half
// when comparing a custom plan against the built-in one.
func omittedStages(chosen []string) []string {
	selected := make(map[string]bool, len(chosen))
	for _, stage := range chosen {
		selected[stage] = true
	}
	var omitted []string
	for _, stage := range planfile.SelectableStages() {
		if !selected[stage] {
			omitted = append(omitted, stage)
		}
	}
	return omitted
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "nothing"
	}
	return strings.Join(values, ", ")
}
