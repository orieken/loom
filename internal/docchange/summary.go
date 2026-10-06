package docchange

import (
	"fmt"
	"strings"
)

// maxListed caps each list in the summary.
const maxListed = 15

// Counts tallies a changeset's artifacts by action.
func (c Change) Counts() map[string]int {
	counts := map[string]int{}
	for _, artifact := range c.Artifacts {
		counts[artifact.Action]++
	}
	return counts
}

// Summary renders the whole changeset for the approval gate: what would be
// written, what is withheld and why, and what the documents leave open. It is
// the one thing a reviewer reads before approving.
func Summary(c Change) string {
	var out strings.Builder
	counts := c.Counts()
	source := "from run " + c.RunID
	switch {
	case c.RunID == "":
		source = "no documenter run"
	case len(c.Runs) > 1:
		source = fmt.Sprintf("from %d runs, newest %s", len(c.Runs), c.RunID)
	}
	fmt.Fprintf(&out, "changeset %s (%s): %d directories documented\n", c.ID, source, c.Documented)
	fmt.Fprintf(&out, "  %d to create, %d to update, %d unchanged, %d conflicts, %d skipped\n",
		counts[ActionCreate], counts[ActionUpdate], counts[ActionUnchanged], counts[ActionConflict], counts[ActionSkip])
	writeArtifactList(&out, "Would be written", c, func(a Artifact) bool { return a.Staged() })
	writeArtifactList(&out, "CONFLICTS — generated blocks edited by hand, not overwritten", c, func(a Artifact) bool { return a.Action == ActionConflict })
	writeArtifactList(&out, "Skipped", c, func(a Artifact) bool { return a.Action == ActionSkip })
	writeSkippedCandidates(&out, c)
	writeQuestions(&out, c)
	return out.String()
}

func writeArtifactList(out *strings.Builder, heading string, c Change, keep func(Artifact) bool) {
	var lines []string
	for _, artifact := range c.Artifacts {
		if keep(artifact) {
			line := fmt.Sprintf("%s  [%s, %s]", artifact.Target, artifact.Kind, artifact.Action)
			if artifact.Reason != "" {
				line += " — " + artifact.Reason
			}
			lines = append(lines, line)
		}
	}
	writeLines(out, heading, lines)
}

func writeSkippedCandidates(out *strings.Builder, c Change) {
	var lines []string
	for _, skipped := range c.Skipped {
		lines = append(lines, skipped.Path+" — "+skipped.Reason)
	}
	writeLines(out, "Verified results left out of this changeset", lines)
}

func writeQuestions(out *strings.Builder, c Change) {
	var lines []string
	for _, question := range c.OpenQuestions {
		lines = append(lines, question.Path+": "+question.Question)
	}
	writeLines(out, "Open questions the documents leave unanswered", lines)
}

func writeLines(out *strings.Builder, heading string, lines []string) {
	if len(lines) == 0 {
		return
	}
	fmt.Fprintf(out, "\n%s (%d):\n", heading, len(lines))
	for index, line := range lines {
		if index == maxListed {
			fmt.Fprintf(out, "  … and %d more\n", len(lines)-maxListed)
			return
		}
		fmt.Fprintf(out, "  %s\n", line)
	}
}
