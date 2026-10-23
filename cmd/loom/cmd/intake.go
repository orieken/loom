package cmd

// `loom intake check` — validate factory work orders before anything claims
// them (roadmap L3.64). The factory supervisor (L3.65) runs the same check at
// pickup; running it in CI catches a bad order when it is written instead.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/orieken/loom/internal/intake"
	"github.com/orieken/loom/internal/planfile"
	"github.com/spf13/cobra"
)

// defaultIntakeDirectory is where `loom install` puts the template and where
// orders live.
const defaultIntakeDirectory = "intake"

// notOrders are the files in intake/ that describe orders rather than being
// one. They are skipped when a directory is expanded, and checked when named.
var notOrders = map[string]bool{"TEMPLATE.md": true, "README.md": true}

// errIntakeInvalid reports that at least one order broke a rule.
var errIntakeInvalid = errors.New("invalid work orders")

var intakeCmd = &cobra.Command{
	Use:   "intake",
	Short: "Work orders for the factory (intake/*.md)",
	Args:  cobra.NoArgs,
}

var intakeCheckCmd = &cobra.Command{
	Use:   "check [file|directory]...",
	Short: "Validate work orders against the intake contract",
	Long: `Validates each work order's frontmatter against every rule checkable from
the file alone: schema version, id matching the file name, known values for
type, status, source, trust, finish and autonomy, a plan this project can run,
priority, dates, budgets, and dependencies. An order with trust: external also
has its spec body scanned for instruction-override text.

With no arguments, checks intake/. A directory stands for the .md files
directly inside it, except TEMPLATE.md and README.md.

Exits 0 when every order is valid and 1 otherwise, including when a file
cannot be read: an order that was not checked is never reported valid.`,
	RunE: runIntakeCheck,
}

func init() {
	rootCmd.AddCommand(intakeCmd)
	intakeCmd.AddCommand(intakeCheckCmd)
}

func runIntakeCheck(cmd *cobra.Command, args []string) error {
	files, err := orderFiles(args)
	if err != nil {
		return err
	}
	plans := runnablePlans()
	invalid := 0
	for _, file := range files {
		violations, err := checkOrderFile(file, plans)
		if err != nil {
			return fmt.Errorf("%w: %s: %v", errIntakeInvalid, file, err)
		}
		printViolations(cmd, file, violations)
		if len(violations) > 0 {
			invalid++
		}
	}
	return summarizeIntakeCheck(cmd, invalid, len(files))
}

func summarizeIntakeCheck(cmd *cobra.Command, invalid, total int) error {
	if invalid > 0 {
		return fmt.Errorf("%w: %d of %d order(s)", errIntakeInvalid, invalid, total)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "valid: %d order(s)\n", total)
	return nil
}

func checkOrderFile(file string, plans []string) ([]intake.Violation, error) {
	source, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	stem := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	return intake.Check(source, intake.Context{FileStem: stem, Plans: plans}), nil
}

func printViolations(cmd *cobra.Command, file string, violations []intake.Violation) {
	for _, violation := range violations {
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", file, violation)
	}
}

// orderFiles expands the arguments, defaulting to intake/. Files named
// explicitly are always checked; the template and README are skipped only
// when they arrive as part of a directory.
func orderFiles(args []string) ([]string, error) {
	if len(args) == 0 {
		args = []string{defaultIntakeDirectory}
	}
	var files []string
	for _, arg := range args {
		expanded, err := expandArgument(arg)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errIntakeInvalid, err)
		}
		files = append(files, withoutNonOrders(arg, expanded)...)
	}
	return files, nil
}

func withoutNonOrders(arg string, expanded []string) []string {
	if len(expanded) == 1 && expanded[0] == arg {
		return expanded
	}
	var orders []string
	for _, file := range expanded {
		if !notOrders[filepath.Base(file)] {
			orders = append(orders, file)
		}
	}
	return orders
}

// runnablePlans are the plans `loom run --plan` could resolve here. A plan
// file that does not parse is listed by Discover with its error; it cannot
// run, so an order naming it is invalid.
func runnablePlans() []string {
	var names []string
	for _, available := range planfile.Discover(planfile.SearchPath()) {
		if available.Err == nil {
			names = append(names, available.Name)
		}
	}
	return names
}
