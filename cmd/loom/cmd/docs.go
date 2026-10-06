package cmd

// `loom docs` — the gate between verified documenter results and your
// repository (ADR-010, Phase 6). `plan` stages a changeset and writes nothing
// else. `approve` records a human's decision, bound to the digest of every
// staged file. `apply` writes only what was approved, and only if neither the
// staged files nor the targets changed since.

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/orieken/loom/internal/docchange"
	"github.com/orieken/loom/internal/repodoc"
	"github.com/orieken/loom/internal/repograph"
	"github.com/spf13/cobra"
)

type docsFlags struct {
	root          string
	run           string
	inTreeReadmes bool
	docComments   bool
	noCodemap     bool
	codemapOnly   bool
	yes           bool
}

var docsArgs docsFlags

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Review and apply the documenter's verified results",
	Long: `Verified documenter results never reach your repository directly. They go through
a changeset you review and approve:

  loom docs plan      stage a changeset from the latest documenter run
  loom docs approve   record your approval, bound to every staged file
  loom docs apply     write exactly what you approved

plan writes only under .loom/changes/. By default a changeset holds the rendered
documents under .loom/docs/, the updated .loom/index.json and a regenerated
CODEMAP.md. --in-tree-readmes and --doc-comments add real READMEs and native doc
comments to your source tree, always inside loom:generated markers; text you
wrote outside the markers is never touched, and a block you edited by hand is
reported as a conflict instead of being overwritten.

Editing any staged file after approving resets the approval.`,
}

var docsPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Stage a changeset from the latest documenter run (writes nothing to your repo)",
	Args:  cobra.NoArgs,
	RunE:  runDocsPlan,
}

var docsApproveCmd = &cobra.Command{
	Use:   "approve [changeset]",
	Short: "Approve a changeset (defaults to the latest)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDocsApprove,
}

var docsApplyCmd = &cobra.Command{
	Use:   "apply [changeset]",
	Short: "Write an approved changeset into your repository (defaults to the latest)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDocsApply,
}

func init() {
	docsCmd.PersistentFlags().StringVar(&docsArgs.root, "root", ".", "repository root")
	docsPlanCmd.Flags().StringVar(&docsArgs.run, "run", "", "documenter run to plan from (default: the latest)")
	docsPlanCmd.Flags().BoolVar(&docsArgs.inTreeReadmes, "in-tree-readmes", false, "also write README.md files into your source tree, inside generated-block markers")
	docsPlanCmd.Flags().BoolVar(&docsArgs.docComments, "doc-comments", false, "also write native doc comments (doc.go, package-info.java, module docstrings, JSDoc @module), inside generated-block markers")
	docsPlanCmd.Flags().BoolVar(&docsArgs.noCodemap, "no-codemap", false, "do not regenerate CODEMAP.md")
	docsPlanCmd.Flags().BoolVar(&docsArgs.codemapOnly, "codemap-only", false, "plan only CODEMAP.md from the graph and index; needs no documenter run")
	docsApproveCmd.Flags().BoolVar(&docsArgs.yes, "yes", false, "approve without the interactive prompt (required when stdin is not a terminal)")
	docsCmd.AddCommand(docsPlanCmd, docsApproveCmd, docsApplyCmd)
	rootCmd.AddCommand(docsCmd)
}

func runDocsPlan(cmd *cobra.Command, _ []string) error {
	vocabulary, err := repodoc.LoadVocabulary(frameworkFS, docsArgs.root)
	if err != nil {
		return err
	}
	change, err := docchange.Plan(docchange.PlanInput{
		Root: docsArgs.root, RunID: docsArgs.run, Vocabulary: vocabulary, Lister: repograph.GitLister{}, Now: time.Now,
		Options: docchange.Options{InTreeReadmes: docsArgs.inTreeReadmes, DocComments: docsArgs.docComments, NoCodemap: docsArgs.noCodemap, CodemapOnly: docsArgs.codemapOnly},
	})
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprint(out, docchange.Summary(change))
	fmt.Fprintf(out, "\nNothing was written to your repository. Staged files are in %s.\nReview them, then: loom docs approve %s\n",
		docchange.Dir(docsArgs.root, change.ID), change.ID)
	return nil
}

// resolveChange picks the named changeset or the latest.
func resolveChange(args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	latest, err := docchange.LatestChange(docsArgs.root)
	if err != nil {
		return "", err
	}
	if latest == "" {
		return "", fmt.Errorf("%w: run `loom docs plan` first", docchange.ErrNoSuchChange)
	}
	return latest, nil
}

func runDocsApprove(cmd *cobra.Command, args []string) error {
	id, err := resolveChange(args)
	if err != nil {
		return err
	}
	change, err := docchange.Load(docsArgs.root, id)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprint(out, docchange.Summary(change))
	if !docsArgs.yes {
		approved, err := confirmApproval(cmd.InOrStdin(), out, id)
		if err != nil {
			return err
		}
		if !approved {
			fmt.Fprintln(out, "not approved")
			return nil
		}
	}
	if _, err := docchange.Approve(docsArgs.root, id, time.Now()); err != nil {
		return err
	}
	fmt.Fprintf(out, "\napproved %s. Editing any staged file now resets this approval.\nApply it with: loom docs apply %s\n", id, id)
	return nil
}

// errApprovalNeedsTerminal is returned when approval cannot be asked for.
var errApprovalNeedsTerminal = fmt.Errorf("approval needs a person: run this in a terminal, or pass --yes to say you reviewed the changeset")

// approvalIsInteractive says a person can be asked. A variable so tests can
// stand in for a terminal.
var approvalIsInteractive = stdinIsInteractive

// confirmApproval asks on a terminal. Without one it refuses rather than
// assuming consent.
func confirmApproval(in io.Reader, out io.Writer, id string) (bool, error) {
	if !approvalIsInteractive() {
		return false, errApprovalNeedsTerminal
	}
	fmt.Fprintf(out, "\napprove changeset %s? [y/N] ", id)
	answer, _ := bufio.NewReader(in).ReadString('\n')
	return strings.EqualFold(strings.TrimSpace(answer), "y"), nil
}

func runDocsApply(cmd *cobra.Command, args []string) error {
	id, err := resolveChange(args)
	if err != nil {
		return err
	}
	result, err := docchange.Apply(docsArgs.root, id, time.Now())
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "applied %s: %d written, %d already present, %d withheld (conflicts or skips)\n",
		id, len(result.Written), len(result.AlreadyPresent), len(result.Withheld))
	return nil
}
