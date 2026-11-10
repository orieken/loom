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
	"github.com/orieken/loom/internal/worktree"
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

Orders that share one slice.of are also checked together: that their covers
lists cover every criterion level their spec declares exactly once, that
they agree on a manifest digest which matches the manifest's own files, and
that their depends-on edges have no cycle and no dangling reference.

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
	root := projectRoot()
	context := intake.Context{Plans: runnablePlans(root), OtherSpecs: featureSpecs(root)}
	invalid := 0
	documents := make([]intake.Document, 0, len(files))
	for _, file := range files {
		violations, document, err := checkOrderFile(file, context)
		if err != nil {
			return fmt.Errorf("%w: %s: %v", errIntakeInvalid, file, err)
		}
		printViolations(cmd, file, violations)
		if len(violations) > 0 {
			invalid++
		}
		documents = append(documents, document)
	}
	invalid += checkIntakeSlices(cmd, documents, root)
	return summarizeIntakeCheck(cmd, invalid, len(files))
}

// checkIntakeSlices runs the cross-order slice check over every order this
// run parsed, reading through a FileReader rooted at root — the only place
// this feature resolves a real project-root path. It prints every violation
// and, for each spec whose slices covered it cleanly, one summary line.
// It returns the number of specs with at least one slice violation, folded
// into the same invalid counter runIntakeCheck already uses so the exit
// code contract (0 only when every order is valid) stays unchanged.
func checkIntakeSlices(cmd *cobra.Command, documents []intake.Document, root string) int {
	read := func(path string) ([]byte, error) {
		return os.ReadFile(filepath.Join(root, path))
	}
	violations, summaries := intake.CheckSlices(documents, read)
	for _, violation := range violations {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n", violation)
	}
	for _, summary := range summaries {
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %d slice(s), %d level(s) covered\n", summary.Of, summary.Slices, summary.Levels)
	}
	return len(slicedSpecs(documents)) - len(summaries)
}

// slicedSpecs is the distinct slice.of values documents declares.
func slicedSpecs(documents []intake.Document) map[string]bool {
	specs := map[string]bool{}
	for _, document := range documents {
		if document.Order.Slice != nil {
			specs[document.Order.Slice.Of] = true
		}
	}
	return specs
}

func summarizeIntakeCheck(cmd *cobra.Command, invalid, total int) error {
	if invalid > 0 {
		return fmt.Errorf("%w: %d of %d order(s)", errIntakeInvalid, invalid, total)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "valid: %d order(s)\n", total)
	return nil
}

// checkOrderFile validates one order file and returns its parsed Document
// so the caller can group it into the cross-order slice check without
// re-reading or re-parsing the file. A parse failure is already reported by
// Check as a "frontmatter" violation; the zero Document returned alongside
// it has a nil Slice, so it takes no part in that check.
func checkOrderFile(file string, context intake.Context) ([]intake.Violation, intake.Document, error) {
	source, err := os.ReadFile(file)
	if err != nil {
		return nil, intake.Document{}, err
	}
	context.FileStem = strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	violations := intake.Check(source, context)
	document, _ := intake.Parse(source)
	return violations, document, nil
}

// projectRoot is the repository the command runs inside, so plans and specs
// are found the same way from any subdirectory (review item I-5). Outside a
// repository it is the current directory.
func projectRoot() string {
	root, err := worktree.RepositoryRoot(".")
	if err != nil {
		return "."
	}
	return root
}

// featureSpecs maps the stem of every features/*.md spec to its path. An
// order sharing one would share its workspace (review item I-10).
func featureSpecs(root string) map[string]string {
	matches, _ := filepath.Glob(filepath.Join(root, "features", "*.md"))
	specs := map[string]string{}
	for _, match := range matches {
		if notOrders[filepath.Base(match)] {
			continue
		}
		stem := strings.TrimSuffix(filepath.Base(match), ".md")
		specs[stem] = filepath.ToSlash(filepath.Join("features", filepath.Base(match)))
	}
	return specs
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
func runnablePlans(root string) []string {
	var names []string
	for _, available := range planfile.Discover(planfile.SearchPathFrom(root)) {
		if available.Err == nil {
			names = append(names, available.Name)
		}
	}
	return names
}
