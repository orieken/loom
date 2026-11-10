package cmd

import (
	"errors"
	"fmt"

	"github.com/orieken/loom/internal/docstyle"
	"github.com/spf13/cobra"
)

// errDocStyleFindingsFound signals a --strict failure. root.go's existing
// cobra.CheckErr(err) fallthrough already turns any other non-nil error
// into exit 1, so nothing there needs to change for this command.
var errDocStyleFindingsFound = errors.New("docs check: style findings found")

// docsCheckLongHelp names every rule loom docs check runs, and states
// plainly that the noun-cluster rule is not one of them — matching the note
// shared/rules/writing-style.md carries for the same reason.
const docsCheckLongHelp = `check reports where a Markdown file breaks the deterministic parts of
shared/rules/writing-style.md (Simplified Technical English): an over-length
sentence or list step, a word the rule replaces (utilize, prior to, with
regard to), a weak modal (can, might, ought to) next to an RFC 2119 keyword
(MUST, SHOULD, MAY), an estimated passive-voice share over 10%, and a
DOMAIN_DICTIONARY.md "Synonyms to AVOID" term.

It does not check a run of more than three consecutive nouns: reliable
detection needs part-of-speech tagging, not available in the Go standard
library or this project's go.mod without a new dependency, and a crude
heuristic would not deterministically check the grammatical property the
rule names. Active voice as a whole, noun clusters, and one-term-one-meaning
stay with a human or agent reviewer.

Exits 0 by default regardless of findings. --strict exits 1 when there is a
finding.`

var docsCheckCmd = &cobra.Command{
	Use:   "check <path>...",
	Short: "Report where a Markdown file breaks shared/rules/writing-style.md's mechanical rules",
	Long:  docsCheckLongHelp,
	Args:  cobra.MinimumNArgs(1),
	RunE:  runDocsCheck,
}

func runDocsCheck(cmd *cobra.Command, args []string) error {
	report, err := docstyle.Check(args)
	if err != nil {
		return err
	}
	fmt.Fprint(cmd.OutOrStdout(), docstyle.Render(report))
	if docsArgs.strict && len(report.Findings) > 0 {
		return errDocStyleFindingsFound
	}
	return nil
}
