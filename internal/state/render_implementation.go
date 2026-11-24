package state

import "fmt"

// renderImplementation writes implementation-notes.md with the exact
// headings implementation-contract.md requires. The checklists render as
// `[x]`/`[ ]` because that is what the contract's own validation rule looks
// for in the markdown pipeline.
func renderImplementation(implementation ImplementationState) string {
	doc := &document{}
	doc.frontmatter(implementation.frontmatter())
	doc.title("Implementation Notes: " + implementation.Feature)
	doc.section("## Files Created", bullets(implementation.FilesCreated))
	doc.section("## Files Modified", bullets(implementation.FilesModified))
	doc.section("## Files Deleted", bullets(implementation.FilesDeleted))
	doc.section("## Interface Design", bullets(implementation.InterfaceDesign))
	doc.section("## Named Refactoring Log", refactoringEntryLines(implementation.RefactoringLog))
	doc.section("## Self-Review Checklist", checklistLines(implementation.SelfReview))
	doc.section("## Simple Design Verification", checklistLines(implementation.SimpleDesign))
	doc.section("## Key Decisions", decisionEntryLines(implementation.KeyDecisions))
	doc.section("## Deviations from Analysis", deviationLines(implementation.Deviations))
	doc.section("## Dependencies Added", dependencyLines(implementation.DependenciesAdded))
	doc.section("## Notes for QA", bullets(implementation.NotesForQA))
	doc.section("## Notes for DevOps", bullets(implementation.NotesForDevOps))
	return doc.String()
}

func checklistLines(items []ChecklistItem) []string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- [%s] %s%s", checkMark(item.Checked), item.Item, noteSuffix(item.Note)))
	}
	return lines
}

func checkMark(checked bool) string {
	if checked {
		return "x"
	}
	return " "
}

func noteSuffix(note string) string {
	if note == "" {
		return ""
	}
	return " — " + note
}

func refactoringEntryLines(entries []RefactoringEntry) []string {
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, fmt.Sprintf("- **%s** on `%s`%s", entry.Operation, entry.Target, noteSuffix(entry.Reason)))
	}
	return lines
}

func decisionEntryLines(decisions []Decision) []string {
	lines := make([]string, 0, len(decisions))
	for _, decision := range decisions {
		lines = append(lines, fmt.Sprintf("- %s: %s", decision.Decision, decision.Reasoning))
	}
	return lines
}

func deviationLines(deviations []Deviation) []string {
	lines := make([]string, 0, len(deviations))
	for _, deviation := range deviations {
		lines = append(lines, fmt.Sprintf("- %s: %s", deviation.Task, deviation.Reason))
	}
	return lines
}

func dependencyLines(dependencies []DependencyAdded) []string {
	lines := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		lines = append(lines, fmt.Sprintf("- `%s`%s — %s", dependency.Package, versionSuffix(dependency.Version), dependency.Reason))
	}
	return lines
}

func versionSuffix(version string) string {
	if version == "" {
		return ""
	}
	return " " + version
}

func (i ImplementationState) frontmatter() frontmatter {
	touched := append(append([]string{}, i.FilesCreated...), i.FilesModified...)
	touched = append(touched, i.FilesDeleted...)
	return newFrontmatter(i.Feature, "", i.Retrieval, touched)
}

// render satisfies the renderable interface.
func (i ImplementationState) render() string { return renderImplementation(i) }
