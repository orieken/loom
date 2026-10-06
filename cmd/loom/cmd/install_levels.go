package cmd

import (
	"fmt"

	frameworkfs "github.com/orieken/loom/cmd/loom/internal/fs"
	"github.com/orieken/loom/cmd/loom/internal/levels"
	"github.com/orieken/loom/cmd/loom/internal/platform"
)

// mcpConfigContent is the .mcp.json written by the level 2 mcp-registration
// bundle. Written only when absent — an existing file is the user's.
const mcpConfigContent = `{
  "mcpServers": {
    "loom": {
      "command": "loom",
      "args": ["mcp", "serve"]
    }
  }
}
`

// levelRuleSet builds the explicit rule selection for a --level install:
// the level 1 core bundle plus any --stack opt-in modules.
func levelRuleSet(profile levels.Profile, stackFlag string) (platform.RuleSet, error) {
	// Reuse ParseRuleSet purely to validate the stack names.
	stackSet, err := platform.ParseRuleSet(stackFlag)
	if err != nil {
		return platform.RuleSet{}, err
	}
	names, err := profile.CoreRuleNames()
	if err != nil {
		return platform.RuleSet{}, err
	}
	for _, stack := range stackSet.Stacks() {
		names = append(names, profile.OnDemandRuleName(stack))
	}
	return platform.ExplicitRuleSet(names), nil
}

// installLevelBundles installs every level 2..N bundle whose requirements
// have landed; gated bundles are skipped with a warning. Level 1's bundles
// (rules, agents, skills, project base) are the platform installers' job.
func installLevelBundles(request installRequest, files *frameworkfs.Writer, output installOutput) ([]string, error) {
	selected, err := request.profile.Select(request.level)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, level := range selected[1:] {
		levelPaths, installErr := installLevel(request, level, files, output)
		if installErr != nil {
			return nil, installErr
		}
		paths = append(paths, levelPaths...)
	}
	return paths, nil
}

func installLevel(request installRequest, level levels.Level, files *frameworkfs.Writer, output installOutput) ([]string, error) {
	var paths []string
	for _, bundle := range level.Bundles {
		if bundle.Requires != "" && !request.profile.IsLanded(bundle.Requires) {
			output.line(fmt.Sprintf("  ! level %d bundle %q skipped — requires roadmap item %s, which has not landed", level.Level, bundle.ID, bundle.Requires))
			continue
		}
		if bundle.OptIn {
			output.line(fmt.Sprintf("  · opt-in bundle %q is not installed by default — see `loom %s`", bundle.ID, optInCommand(bundle.ID)))
			continue
		}
		bundlePaths, err := installBundle(bundle, files, output)
		if err != nil {
			return nil, fmt.Errorf("install level %d bundle %s: %w", level.Level, bundle.ID, err)
		}
		paths = append(paths, bundlePaths...)
	}
	return paths, nil
}

func installBundle(bundle levels.Bundle, files *frameworkfs.Writer, output installOutput) ([]string, error) {
	if bundle.Action != "" {
		return installBundleAction(bundle, files, output)
	}
	var paths []string
	for _, source := range bundle.Paths {
		destination := levels.InstallDestination(source)
		if _, err := files.Install(source, destination); err != nil {
			return nil, err
		}
		paths = append(paths, destination)
	}
	return paths, nil
}

// optInCommand names the command that uses an opt-in bundle.
func optInCommand(bundleID string) string {
	if bundleID == "repo-docs" {
		return "document --help"
	}
	return "help"
}

func installBundleAction(bundle levels.Bundle, files *frameworkfs.Writer, output installOutput) ([]string, error) {
	if bundle.Action == "repo-index" {
		// Nothing to install: the binary is the install. Say what to do next.
		output.line("  · repo-index: run `loom index` in the project to build the repo graph the MCP repo tools read")
		return nil, nil
	}
	if bundle.Action != "mcp-config" {
		output.line(fmt.Sprintf("  ! bundle %q action %q is not implemented yet — skipped", bundle.ID, bundle.Action))
		return nil, nil
	}
	written, err := files.WriteIfMissing(".mcp.json", []byte(mcpConfigContent))
	if err != nil {
		return nil, err
	}
	if !written {
		output.line("  ! .mcp.json already exists — add the loom server entry manually (command: loom, args: [mcp, serve])")
		return nil, nil
	}
	return []string{".mcp.json"}, nil
}
