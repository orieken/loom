package cmd

// `loom capture hook <event>` — the command that the tool hooks of Claude
// Code call. The command is hidden: a person never types it. The installer
// writes the hook entries that call it.
//
// The command always exits 0 and prints nothing, on every path. A hook
// that fails or talks must never disturb the session it watches.

import "github.com/spf13/cobra"

const captureRootFlag = "root"

var captureCmd = &cobra.Command{
	Use:    "capture",
	Short:  "Record session events for correction measurement",
	Hidden: true,
	Args:   cobra.ArbitraryArgs,
	// The parent prints nothing either: a hook shell may call it by mistake.
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          func(*cobra.Command, []string) error { return nil },
}

var captureHookCmd = &cobra.Command{
	Use:           "hook <event>",
	Short:         "Record one hook event (called by the tool hooks)",
	Hidden:        true,
	Args:          cobra.ArbitraryArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	// A flag that Claude Code or a later version adds must not make cobra
	// print an error and exit non-zero.
	FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	RunE:               runCaptureHook,
}

// stayQuiet prints nothing, in place of the help and usage of cobra.
func stayQuiet(*cobra.Command, []string) {}

func init() {
	for _, command := range []*cobra.Command{captureCmd, captureHookCmd} {
		command.SetHelpFunc(stayQuiet)
		command.SetUsageFunc(func(*cobra.Command) error { return nil })
	}
	captureHookCmd.SetFlagErrorFunc(swallowFlagError)
	captureHookCmd.Flags().String(captureRootFlag, "", "project root that holds .claude/memory")
	captureCmd.AddCommand(captureHookCmd)
	rootCmd.AddCommand(captureCmd)
}
