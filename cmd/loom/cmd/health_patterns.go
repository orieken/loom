package cmd

// The pattern spec line of `loom health` (pattern mining F2): every
// *.pattern.yaml in shared/patterns/ and .claude/patterns/ is validated
// against the schema and the contract, and each issue names its file and
// field.

import (
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/orieken/loom/internal/patternspec"
	"github.com/orieken/loom/shared/mcp/register"
	"github.com/orieken/loom/tools"
)

// skillFile is where a skill lives, under a skills directory.
const skillFile = "SKILL.md"

func (check *healthCheck) verifyPatterns() {
	report := patternspec.Scan(os.DirFS(check.request.target), check.patternReferences())
	for _, issue := range report.Issues {
		check.reportPatternIssue(issue)
	}
	if report.Failures() == 0 {
		check.output.success(patternSummary(len(report.Documents)))
	}
}

func (check *healthCheck) reportPatternIssue(issue patternspec.Issue) {
	message := "pattern spec " + issue.String()
	if issue.IsFailure() {
		check.fail(message)
		return
	}
	check.warn(message)
}

func patternSummary(count int) string {
	if count == 0 {
		return fmt.Sprintf("pattern specs: none (%s, %s)", patternspec.SharedDir, patternspec.LocalDir)
	}
	return fmt.Sprintf("pattern specs valid (%d)", count)
}

func (check *healthCheck) patternReferences() patternReferences {
	return patternReferences{
		project:   os.DirFS(check.request.target),
		framework: check.content,
		tools:     register.Frameworks(io.Discard),
	}
}

// patternReferences resolves verify references. A skill exists when the
// project installed it or the framework ships it; a tool exists when loom's
// MCP server registers it. Names reaching here already match the schema's
// name pattern, so none can climb out of a skills directory.
type patternReferences struct {
	project   fs.FS
	framework fs.FS
	tools     *tools.Registry
}

func (references patternReferences) HasSkill(name string) bool {
	return hasFile(references.project, ".claude/skills/"+name+"/"+skillFile) ||
		hasFile(references.framework, "shared/skills/"+name+"/"+skillFile)
}

func (references patternReferences) HasTool(name string) bool {
	_, found := references.tools.Get(name)
	return found
}

func hasFile(fsys fs.FS, name string) bool {
	if fsys == nil {
		return false
	}
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}
