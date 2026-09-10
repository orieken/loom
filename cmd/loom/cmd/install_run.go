package cmd

import (
	"fmt"

	frameworkfs "github.com/orieken/loom/cmd/loom/internal/fs"
	"github.com/orieken/loom/cmd/loom/internal/manifest"
	"github.com/orieken/loom/cmd/loom/internal/platform"
	"github.com/spf13/cobra"
)

func runInstall(command *cobra.Command, _ []string) error {
	if frameworkFS == nil {
		return fmt.Errorf("embedded framework content is not configured")
	}
	embeddedVersion, err := readFrameworkVersion(frameworkFS)
	if err != nil {
		return err
	}
	request, err := prepareInstall(installArgs, embeddedVersion, frameworkFS)
	if err != nil {
		return err
	}
	output := installOutput{command.OutOrStdout(), request.isDryRun}
	files, err := ownedWriter(request, output)
	if err != nil {
		return err
	}
	return executeInstall(request, frameworkFS, mcpFS, files, output)
}

// ownedWriter builds the writer already loaded with what a previous install
// recorded as loom's own, so this install writes only where it may
// (roadmap L3.26).
func ownedWriter(request installRequest, output installOutput) (*frameworkfs.Writer, error) {
	previous, _, err := manifest.ReadIfExists(request.target)
	if err != nil {
		return nil, err
	}
	files := frameworkfs.NewWriter(frameworkFS, request.target, request.cache,
		request.isCopy, request.isDryRun, output.action)
	return files.WithOwnership(previous.Ledger(), request.isForced), nil
}

func executeInstall(request installRequest, content, mcpContent platform.Content, files *frameworkfs.Writer, output installOutput) error {
	output.line(fmt.Sprintf("loom: installing framework %s", request.frameworkVersion))
	results, err := installPlatforms(request, content, files, output)
	if err != nil {
		return err
	}
	extras, err := installExtras(request, content, mcpContent, files)
	if err != nil {
		return err
	}
	if request.level > 0 {
		levelPaths, levelErr := installLevelBundles(request, files, output)
		if levelErr != nil {
			return levelErr
		}
		extras = append(extras, levelPaths...)
	}
	if err := writeManifest(request, results, extras, files.Written()); err != nil {
		return err
	}
	return reportInstall(request, results, content, output)
}

func installPlatforms(request installRequest, content platform.Content, files *frameworkfs.Writer, output installOutput) ([]platform.Result, error) {
	environment := platform.Environment{Content: content, Files: files, Rules: request.rules}
	results := make([]platform.Result, 0, len(request.platforms))
	for _, name := range request.platforms {
		result, err := platform.Install(name, environment)
		if err != nil {
			return nil, fmt.Errorf("install %s: %w", name, err)
		}
		results = append(results, result)
		output.platform(result)
	}
	return results, nil
}

func installExtras(request installRequest, content, mcpContent platform.Content, files *frameworkfs.Writer) ([]string, error) {
	var paths []string
	if request.withConfig {
		configs, err := installConfigs(content, files)
		if err != nil {
			return nil, err
		}
		paths = append(paths, configs...)
	}
	if request.withMCP {
		mcpPaths, err := installMCP(mcpContent, request.target, request.cache, request.isDryRun, files.Reporter())
		if err != nil {
			return nil, err
		}
		paths = append(paths, mcpPaths...)
	}
	return paths, nil
}
