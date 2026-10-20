package cmd

// `loom install --capture`: the opt-in registration of the session capture
// hooks. Capture never turns on by level. Without the flag the installer
// registers nothing and, from level 3, prints a one-line hint.
//
// The stage has two parts. preflight runs before the installer writes
// anything, and every refusal comes from it. apply runs after the platform
// install and writes the salt, the hook entries and the manifest record.

import (
	"errors"
	"fmt"
	"runtime"
	"slices"

	"github.com/orieken/loom/cmd/loom/internal/ignorerules"
	"github.com/orieken/loom/cmd/loom/internal/manifest"
	"github.com/orieken/loom/cmd/loom/internal/settings"
	"github.com/orieken/loom/internal/capture"
	"github.com/orieken/loom/internal/capture/localfs"
)

// captureInstall is the parameter object of the capture stage. It keeps the
// capture inputs out of installRequest.
type captureInstall struct {
	layout    capture.Layout
	platforms []string
	version   string
	level     int
	isEnabled bool
	isDryRun  bool
	goos      string
}

func newCaptureInstall(request installRequest, flags installFlags) captureInstall {
	return captureInstall{
		layout:    capture.NewLayout(request.target),
		platforms: request.platforms,
		version:   request.frameworkVersion,
		level:     request.level,
		isEnabled: flags.withCapture,
		isDryRun:  request.isDryRun,
		goos:      runtime.GOOS,
	}
}

// preflight refuses before anything is written. It changes nothing.
func (c captureInstall) preflight() error {
	if !c.isEnabled {
		return nil
	}
	if c.goos == "windows" {
		return errors.New("loom: --capture is not supported on Windows (the day files rely on POSIX append semantics). Nothing was installed")
	}
	if !slices.Contains(c.platforms, claudePlatform) {
		return errors.New("loom: --capture needs Claude Code, and it is not a detected platform here (no .claude directory or CLAUDE.md; or pass --platform claude-code). Nothing was installed")
	}
	if err := c.requireIgnoredStore(); err != nil {
		return err
	}
	return settings.CheckMergeable(c.layout.SettingsLocalPath(), captureHooks(c.layout.Root()))
}

// requireIgnoredStore stops when git would commit the session data. The
// decision is final: loom refuses and does not append the rule itself.
func (c captureInstall) requireIgnoredStore() error {
	isIgnored, err := ignorerules.IgnoresDirectory(c.layout.Root(), captureIgnoredDirectory)
	if err != nil {
		return fmt.Errorf("loom: read .gitignore: %w", err)
	}
	if isIgnored {
		return nil
	}
	return fmt.Errorf("loom: --capture stops. %s is not ignored by git in %s, so session data could be committed.\n"+
		"Add this line to .gitignore and run the command again:\n\n  %s\n\nNothing was installed", captureIgnoredDirectory, c.layout.Root(), captureIgnoreLine)
}

// apply runs after the platform install.
func (c captureInstall) apply(output installOutput) error {
	if !c.isEnabled {
		c.printHint(output)
		return nil
	}
	if c.isDryRun {
		c.printPlan(output)
		return nil
	}
	return c.register(output)
}

func (c captureInstall) printHint(output installOutput) {
	if c.level >= captureHintLevel {
		output.line("  · capture: not enabled — run `loom install --capture` to record how often you correct the agent (off by default)")
	}
}

func (c captureInstall) printPlan(output installOutput) {
	output.line("  · capture: would create the salt at " + relativeSlash(c.layout.Root(), c.layout.SaltPath()))
	output.line("  · capture: would register the hooks in " + relativeSlash(c.layout.Root(), c.layout.SettingsLocalPath()))
}

func (c captureInstall) register(output installOutput) error {
	outcome, err := localfs.InstallSalt(c.layout.SaltPath(), nil)
	if err != nil {
		return err
	}
	output.line("  ✓ capture: " + saltMessage(outcome))
	hooks := captureHooks(c.layout.Root())
	merged, err := settings.Merge(c.layout.SettingsLocalPath(), hooks)
	if err != nil {
		return err
	}
	output.line(fmt.Sprintf("  ✓ capture: %d hook entries added to %s", merged.Added, relativeSlash(c.layout.Root(), c.layout.SettingsLocalPath())))
	if err := c.recordInManifest(hooks, merged); err != nil {
		return err
	}
	output.line("  · capture: defaults apply; create .claude/capture.yaml to change them")
	return nil
}

func saltMessage(outcome localfs.SaltInstallOutcome) string {
	if outcome == localfs.SaltKept {
		return "kept the existing salt (never rotated)"
	}
	return "salt created (mode 0600)"
}

// recordInManifest adds the capture record to the manifest the platform
// install just wrote. A second install keeps what the first one recorded.
func (c captureInstall) recordInManifest(hooks []settings.Hook, merged settings.MergeResult) error {
	installed, _, err := manifest.ReadIfExists(c.layout.Root())
	if err != nil {
		return err
	}
	origin := settings.ExistedBefore
	commands := hookCommands(hooks)
	if installed.Capture != nil {
		commands = mergeStrings(installed.Capture.HookCommands, commands)
	}
	if merged.Created || (installed.Capture != nil && installed.Capture.SettingsCreated) {
		origin = settings.CreatedByLoom
	}
	installed.Capture = newCaptureRecord(commands, origin)
	if installed.Version == "" {
		installed.Version = c.version
	}
	return manifest.Write(c.layout.Root(), installed)
}
