package cmd

import (
	"github.com/orieken/loom/cmd/loom/internal/platform"
	"github.com/spf13/cobra"
)

type installFlags struct {
	platform   string
	stack      string
	target     string
	level      int
	isCopy     bool
	isForced   bool
	withConfig bool
	withMCP    bool
	isDryRun   bool
}

var (
	frameworkFS platform.Content
	mcpFS       platform.Content
	installArgs installFlags
)

var installCmd = &cobra.Command{
	Use:     "install",
	Aliases: []string{"init"},
	Short:   "Install the framework to detected AI platforms",
	Args:    cobra.NoArgs,
	RunE:    runInstall,
}

func init() {
	rootCmd.AddCommand(installCmd)
	registerInstallFlags()
}

func configureInstall(frameworkContent, mcpContent platform.Content) {
	frameworkFS = frameworkContent
	mcpFS = mcpContent
}

func registerInstallFlags() {
	installCmd.Flags().StringVar(&installArgs.platform, "platform", "", "install for one AI platform")
	installCmd.Flags().StringVar(&installArgs.stack, "stack", "", "comma-separated language stacks")
	installCmd.Flags().IntVar(&installArgs.level, "level", 0, "agentic maturity level profile 1-4 (see shared/levels.yaml); omit for the full install")
	installCmd.Flags().BoolVar(&installArgs.isCopy, "copy", false, "copy files instead of creating symlinks")
	installCmd.Flags().BoolVar(&installArgs.isForced, "force", false, "overwrite loom-installed files the project has edited (never touches files loom did not install)")
	installCmd.Flags().BoolVar(&installArgs.withConfig, "with-configs", false, "write linter configs to the target")
	installCmd.Flags().BoolVar(&installArgs.withMCP, "with-mcp", false, "copy the MCP reference source into the target (deprecated — use 'loom mcp serve')")
	installCmd.Flags().BoolVar(&installArgs.isDryRun, "dry-run", false, "print actions without writing files")
	installCmd.Flags().StringVar(&installArgs.target, "target", ".", "project root to install into")
}
