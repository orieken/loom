package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/orieken/loom/internal/mutation"
)

// The spike's settings. With gremlins' defaults, 39 of 41 mutants in one
// package timed out in under four seconds and it reported 100% efficacy; one
// worker and a twentyfold timeout gave 34 killed, 4 lived, 2 timed out.
const (
	gremlinsWorkers            = 1
	gremlinsTimeoutCoefficient = 20
	// gremlins sizes each mutant's timeout from the package's own test time,
	// but integration mode runs the whole suite (about 25s here). At 20x, all
	// 16 mutants of cmd/diff-mutation timed out; at 150x all 17 were killed
	// in 108s, matching `go test` run by hand on each.
	integrationTimeoutCoefficient = 150
)

// packageTimeout bounds one package's run. A mutant that loops forever waits
// out gremlins' own per-mutant timeout; this is the backstop for gremlins
// itself hanging.
const packageTimeout = 30 * time.Minute

func runGremlins(binary string, target mutation.Target, output string) error {
	ctx, cancel := context.WithTimeout(context.Background(), packageTimeout)
	defer cancel()
	var combined bytes.Buffer
	command := exec.CommandContext(ctx, binary, gremlinsArguments(target, output)...)
	command.Stdout, command.Stderr = &combined, &combined
	if err := command.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, bytes.TrimSpace(combined.Bytes()))
	}
	return nil
}

func timeoutCoefficient(target mutation.Target) int {
	if target.Integration {
		return integrationTimeoutCoefficient
	}
	return gremlinsTimeoutCoefficient
}

// gremlinsArguments never passes --diff: in v0.6.0 it does not scope.
func gremlinsArguments(target mutation.Target, output string) []string {
	args := []string{"unleash",
		"--workers", strconv.Itoa(gremlinsWorkers),
		"--timeout-coefficient", strconv.Itoa(timeoutCoefficient(target)),
		"--output", output}
	for _, pattern := range target.Exclude {
		args = append(args, "--exclude-files", pattern)
	}
	if target.Integration {
		args = append(args, "--integration") // see mutation.Target: v0.6.0 tests the wrong package otherwise
	}
	return append(args, "./"+target.Dir)
}
