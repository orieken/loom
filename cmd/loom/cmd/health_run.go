package cmd

import (
	"errors"
	"io"
	"path/filepath"
	"time"

	frameworkfs "github.com/orieken/loom/cmd/loom/internal/fs"
	"github.com/orieken/loom/cmd/loom/internal/platform"
	"github.com/orieken/loom/internal/capture"
	"github.com/orieken/loom/internal/capture/localfs"
	"github.com/spf13/cobra"
)

var errHealthFailed = errors.New("loom health checks failed")

type healthRequest struct {
	target  string
	display string
	// timeSource gives the current time. A test sets it; nil means the
	// system clock.
	timeSource capture.Clock
}

func (r healthRequest) clock() capture.Clock {
	if r.timeSource == nil {
		return localfs.SystemClock{}
	}
	return r.timeSource
}

func (r healthRequest) now() time.Time { return r.clock().Now() }

func runHealth(command *cobra.Command, _ []string) error {
	request, err := prepareHealth(healthArgs)
	if err != nil {
		return err
	}
	return executeHealth(request, frameworkFS, command.OutOrStdout())
}

func prepareHealth(flags healthFlags) (healthRequest, error) {
	target, err := frameworkfs.ResolveTarget(flags.target)
	if err != nil {
		return healthRequest{}, err
	}
	return healthRequest{target: target, display: filepath.Clean(flags.target)}, nil
}

func executeHealth(request healthRequest, content platform.Content, writer io.Writer) error {
	check := healthCheck{request: request, content: content, output: healthOutput{writer: writer}}
	check.output.heading(request.display)
	if !check.verifyManifest() {
		// The context index and pattern specs are independent of an install:
		// a project may only have run `loom index`, or only hold specs.
		check.verifyContextIndex()
		check.verifyPatterns()
		return check.finish()
	}
	check.verifyVersion()
	check.verifyPaths()
	check.verifySymlinks()
	check.verifyAgentCounts()
	check.verifyMaturity()
	check.verifyContextIndex()
	check.verifyPatterns()
	check.verifyCapture()
	return check.finish()
}
