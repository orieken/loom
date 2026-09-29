package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/orieken/loom/cmd/loom/internal/platform"
	"github.com/orieken/loom/internal/orchestrator"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "loom",
	Short:         "AI assistant framework installer",
	Long:          "Loom installs agents, skills, and rules for Claude Code, Cursor, Windsurf, and other AI platforms.",
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.SetVersionTemplate("{{.Version}}\n")
}

// Execute runs the loom command-line interface.
func Execute(frameworkFS, mcpFS platform.Content) {
	configureInstall(frameworkFS, mcpFS)
	err := rootCmd.Execute()
	if errors.Is(err, errHealthFailed) || errors.Is(err, errStateVerifyFailed) {
		os.Exit(1)
	}
	if errors.Is(err, orchestrator.ErrWaitingApproval) {
		os.Exit(ExitCodeWaitingApproval)
	}
	exitOnKIScan(err)
	cobra.CheckErr(err)
}

func runStub(_ *cobra.Command, _ []string) error {
	fmt.Println("not yet implemented")
	return nil
}
