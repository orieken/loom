package state

import "fmt"

// render writes the context manifest a human reads. The budget section
// reports measurement, not the agent's arithmetic, and says so — a reader
// who cannot tell which they are looking at has no reason to trust either
// (roadmap L3.25).
func (c ContextState) render() string {
	var out document
	out.title("Context Manifest: " + c.Feature)
	out.section("## Scope", c.headerLines())
	out.section("## Pinned Files", c.pinnedLines())
	out.section("## Token Budget", c.budgetLines())
	out.section("## Knowledge Items", codeBullets(c.KnowledgeItems))
	out.section("## ADRs", codeBullets(c.ADRs))
	out.section("## Pruned", codeBullets(c.Pruned))
	return out.String()
}

func codeBullets(values []string) []string {
	lines := make([]string, 0, len(values))
	for _, value := range values {
		lines = append(lines, "- `"+value+"`")
	}
	return lines
}

func (c ContextState) headerLines() []string {
	lines := []string{fmt.Sprintf("- **Tier**: %s", c.Tier)}
	if c.TargetStage != "" {
		lines = append(lines, fmt.Sprintf("- **Built for**: %s", c.TargetStage))
	}
	return lines
}

func (c ContextState) pinnedLines() []string {
	lines := make([]string, 0, len(c.PinnedFiles))
	for _, pinned := range c.PinnedFiles {
		lines = append(lines, fmt.Sprintf("- `%s` — %s", pinned.Path, pinned.Reason))
	}
	return lines
}

func (c ContextState) budgetLines() []string {
	if c.Budget == nil {
		return []string{"- Not measured."}
	}
	lines := []string{
		fmt.Sprintf("- **Status**: %s", c.Budget.Summary(c.Tier)),
		"- Measured by loom from the files above (bytes ÷ 4), not estimated by an agent.",
		"",
		"| File | Bytes | ~Tokens |",
		"|---|---:|---:|",
	}
	lines = append(lines, measuredRows(c.Budget.Files)...)
	return append(lines, c.Budget.unmeasuredNote()...)
}

func measuredRows(files []MeasuredFile) []string {
	rows := make([]string, 0, len(files))
	for _, file := range files {
		if file.Unmeasured != "" {
			rows = append(rows, fmt.Sprintf("| `%s` | — | not measured: %s |", file.Path, file.Unmeasured))
			continue
		}
		rows = append(rows, fmt.Sprintf("| `%s` | %d | %d |", file.Path, file.Bytes, file.EstimatedTokens))
	}
	return rows
}

// unmeasuredNote makes an incomplete total say so. A budget missing files it
// could not open is a floor, and reporting it as a figure is the same class
// of defect as reporting one 7x under.
func (b *ContextBudget) unmeasuredNote() []string {
	if b.Unmeasured == 0 {
		return nil
	}
	return []string{"", fmt.Sprintf(
		"> %d pinned file(s) could not be measured, so the total above is a floor, not a figure.",
		b.Unmeasured)}
}
