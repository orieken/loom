// Package settings merges and unmerges the capture hook entries in
// `.claude/settings.local.json` without losing anything else in the file.
//
// The file belongs to the user. The adapter parses it as a map of raw JSON,
// so every key and every entry it does not own keeps its content. It
// refuses to touch a file it does not understand and never rewrites that
// file.
//
// What is NOT preserved: the layout of the file. The adapter writes the top
// level keys in alphabetical order with a two-space indent, and it drops
// comments (a file with comments is not valid JSON and is refused). Each
// entry keeps its own content.
//
// PRECONDITION: the entry shape below ({"hooks":[{"type":"command",
// "command":...}]}) matches the installed Claude Code. It is UNVERIFIED.
// ENFORCED-BY: judgment-only (manual check against the installed Claude
// Code before release; open question 3 in analysis.md).
package settings

import (
	"fmt"
	"strings"
)

// Hook is one command registered for one hook event.
type Hook struct {
	Event   string
	Command string
}

// Selector says which commands belong to the caller. A command matches when
// it equals one of Exact or starts with Prefix. An empty Prefix matches
// nothing by prefix.
type Selector struct {
	Exact  []string
	Prefix string
}

// Matches reports whether a command belongs to the caller.
func (s Selector) Matches(command string) bool {
	for _, exact := range s.Exact {
		if command == exact {
			return true
		}
	}
	return s.Prefix != "" && strings.HasPrefix(command, s.Prefix)
}

// FileOrigin says who made the settings file. Unmerge deletes a file that
// has no key left only when loom made it.
type FileOrigin int

const (
	// ExistedBefore marks a file the user had before the install.
	ExistedBefore FileOrigin = iota
	// CreatedByLoom marks a file that the install created.
	CreatedByLoom
)

// MergeResult tells what Merge did.
type MergeResult struct {
	// Created is true when the file did not exist before the merge.
	Created bool
	// Added is the number of entries the merge added.
	Added int
}

// UnmergeResult tells what Unmerge did.
type UnmergeResult struct {
	// Removed is the number of commands the unmerge removed.
	Removed int
	// DeletedFile is true when the unmerge deleted a file that held no key.
	DeletedFile bool
}

// NotMergeableError means the file has a shape that the adapter does not
// understand. The adapter wrote nothing. The message tells the user what to
// add by hand.
type NotMergeableError struct {
	Path   string
	Reason string
	Hooks  []Hook
}

func (e *NotMergeableError) Error() string {
	var lines strings.Builder
	fmt.Fprintf(&lines, "cannot edit %s: %s. loom changed nothing.", e.Path, e.Reason)
	if len(e.Hooks) > 0 {
		lines.WriteString("\nAdd these entries by hand, each under \"hooks\" in that file:")
		for _, hook := range e.Hooks {
			fmt.Fprintf(&lines, "\n  %q: [{\"hooks\": [{\"type\": \"command\", \"command\": %q}]}]", hook.Event, hook.Command)
		}
	}
	return lines.String()
}
