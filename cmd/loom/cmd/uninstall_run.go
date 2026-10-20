package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	frameworkfs "github.com/orieken/loom/cmd/loom/internal/fs"
	"github.com/orieken/loom/cmd/loom/internal/manifest"
	"github.com/spf13/cobra"
)

type uninstallRequest struct {
	target   string
	display  string
	platform string
	isDryRun bool
}

func runUninstall(command *cobra.Command, _ []string) error {
	request, err := prepareUninstall(uninstallArgs)
	if err != nil {
		return err
	}
	return executeUninstall(request, command.OutOrStdout())
}

func prepareUninstall(flags uninstallFlags) (uninstallRequest, error) {
	target, err := frameworkfs.ResolveTarget(flags.target)
	if err != nil {
		return uninstallRequest{}, err
	}
	return uninstallRequest{target, filepath.Clean(flags.target), flags.platform, flags.isDryRun}, nil
}

func executeUninstall(request uninstallRequest, writer io.Writer) error {
	installed, err := readUninstallManifest(request)
	if err != nil {
		return err
	}
	selected, remaining, err := selectOwnedRecords(installed.Platforms, request.platform)
	if err != nil {
		return err
	}
	output := uninstallOutput{writer: writer, isDryRun: request.isDryRun}
	output.heading(request.display)
	removed, err := removeOwnedAndCaptureFiles(request, selected, &installed, output)
	if err != nil {
		return err
	}
	return finishUninstall(request, installed, remaining, removed, output)
}

// removeOwnedAndCaptureFiles removes the files of the selected platforms and
// then the capture artifacts. It returns the number of paths removed.
func removeOwnedAndCaptureFiles(request uninstallRequest, selected []manifest.PlatformRecord, installed *manifest.Manifest, output uninstallOutput) (int, error) {
	removed, err := removeOwnedRecords(request, selected, output)
	if err != nil {
		return 0, err
	}
	capturePaths, err := finishCaptureRemoval(request, installed, output)
	return removed + capturePaths, err
}

// finishCaptureRemoval removes the capture artifacts. When the capture stage
// ran, it also clears the capture record of the manifest.
func finishCaptureRemoval(request uninstallRequest, installed *manifest.Manifest, output uninstallOutput) (int, error) {
	capturePaths, didRunCapture, err := uninstallCapture(request, *installed, output)
	if err != nil {
		return 0, err
	}
	if didRunCapture {
		installed.Capture = nil
	}
	return capturePaths, nil
}

func readUninstallManifest(request uninstallRequest) (manifest.Manifest, error) {
	installed, err := manifest.Read(request.target)
	if errors.Is(err, os.ErrNotExist) {
		path := filepath.Join(request.display, manifest.Filename)
		return manifest.Manifest{}, fmt.Errorf("loom: no manifest found at %s — was loom install run here?", path)
	}
	return installed, err
}
