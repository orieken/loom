package state

import (
	"fmt"
	"strings"
)

// render renders a documenter result as markdown: the view a human
// reads at the approval gate. It never invents text — every line is a field.
func (d RepoDocState) render() string {
	var out strings.Builder
	fmt.Fprintf(&out, "# %s\n\n", d.Path)
	if d.Purpose == "" {
		out.WriteString("_Purpose unknown — the facts do not establish it._\n\n")
	} else {
		out.WriteString(d.Purpose + "\n\n")
	}
	fmt.Fprintf(&out, "Layer: %s\n\n", d.Layer)
	writeList(&out, "Responsibilities", d.Responsibilities)
	writeList(&out, "Key entrypoints", d.KeyEntrypoints)
	writeList(&out, "Tags", d.Tags)
	writeInvariants(&out, d.Invariants)
	writeList(&out, "Open questions", d.OpenQuestions)
	return out.String()
}

func writeList(out *strings.Builder, heading string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(out, "## %s\n\n", heading)
	for _, item := range items {
		fmt.Fprintf(out, "- %s\n", item)
	}
	out.WriteString("\n")
}

func writeInvariants(out *strings.Builder, invariants []DocInvariant) {
	if len(invariants) == 0 {
		return
	}
	out.WriteString("## Invariants\n\n")
	for _, invariant := range invariants {
		fmt.Fprintf(out, "- %s _(evidence: %s)_\n", invariant.Statement, strings.Join(invariant.Evidence, ", "))
	}
	out.WriteString("\n")
}
