package cmd

// The one place that knows how a capture hook command looks and how to tell
// one from a user's hook. Install writes it, uninstall and health find it.
//
// PRECONDITION: Claude Code runs the command through a POSIX shell, from a
// directory of its choice, so the command carries an absolute --root. This is
// UNVERIFIED (architecture-notes.md decision 6).
// ENFORCED-BY: judgment-only (manual check against the installed Claude Code
// before release).

import (
	"path/filepath"
	"strings"

	"github.com/orieken/loom/cmd/loom/internal/manifest"
	"github.com/orieken/loom/cmd/loom/internal/settings"
	"github.com/orieken/loom/internal/capture/claudehook"
)

const (
	// captureCommandPrefix starts every hook command that capture writes.
	// Uninstall uses it to find capture entries when the manifest has no
	// record.
	captureCommandPrefix = "loom capture hook "
	// claudePlatform is the platform name of Claude Code.
	claudePlatform = "claude-code"
	// captureIgnoreLine is the .gitignore line that keeps session data out
	// of a commit.
	captureIgnoreLine = ".claude/memory/"
	// captureIgnoredDirectory is the directory that line stands for.
	captureIgnoredDirectory = ".claude/memory"
	// captureHintLevel is the first level whose install prints the capture
	// hint.
	captureHintLevel = 3
)

// captureHookCommand builds the command for one event. The root is quoted
// for a POSIX shell, so a space or a quote in the path stays one word.
func captureHookCommand(event, root string) string {
	return captureCommandPrefix + event + " --root " + shellQuote(root)
}

// shellQuote wraps a word in single quotes. A single quote inside the word
// ends the quoted part, is escaped, and starts a new quoted part.
func shellQuote(word string) string {
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}

// captureHooks lists the hooks to register for a project root.
func captureHooks(root string) []settings.Hook {
	events := claudehook.RegisteredEvents()
	hooks := make([]settings.Hook, len(events))
	for index, event := range events {
		hooks[index] = settings.Hook{Event: event, Command: captureHookCommand(event, root)}
	}
	return hooks
}

func hookCommands(hooks []settings.Hook) []string {
	commands := make([]string, len(hooks))
	for index, hook := range hooks {
		commands[index] = hook.Command
	}
	return commands
}

// captureSelector finds capture entries: the commands the manifest recorded
// and, as a fallback, anything that starts with the capture prefix.
func captureSelector(record *manifest.CaptureRecord) settings.Selector {
	selector := settings.Selector{Prefix: captureCommandPrefix}
	if record != nil {
		selector.Exact = record.HookCommands
	}
	return selector
}

// relativeSlash renders a path under the project root, with slashes.
func relativeSlash(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

// newCaptureRecord builds the manifest record for a project.
func newCaptureRecord(commands []string, origin settings.FileOrigin) *manifest.CaptureRecord {
	return &manifest.CaptureRecord{
		HookCommands:    commands,
		SettingsCreated: origin == settings.CreatedByLoom,
	}
}
