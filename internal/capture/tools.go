package capture

// Tool names the detector must know.
//
// PRECONDITION: the tool names of the host tool are exactly "Edit", "Write",
// "MultiEdit" and "Bash". They are UNVERIFIED in this repository.
// ENFORCED-BY: judgment-only (manual verification against installed Claude
// Code before release; open question 1 in analysis.md).
//
// The names live here, in one table, so a verified contract changes one spot.
const (
	ToolEdit      = "Edit"
	ToolWrite     = "Write"
	ToolMultiEdit = "MultiEdit"
	ToolBash      = "Bash"
)

// IsFileWriter reports whether the tool writes a file in a way the agent
// owns: its result starts a hunk and replaces older hunks on the path.
func IsFileWriter(tool string) bool {
	switch tool {
	case ToolEdit, ToolWrite, ToolMultiEdit:
		return true
	}
	return false
}

// IsShell reports whether the tool can change files without a named path.
func IsShell(tool string) bool {
	return tool == ToolBash
}
