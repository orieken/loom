package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/orieken/loom/internal/diffcover"
	"github.com/orieken/loom/internal/mutation"
)

const (
	exitPass  = 0
	exitFail  = 1
	exitUsage = 2
)

// diffSource returns `git diff --unified=0` output from base to the working tree.
type diffSource func(base string) (io.Reader, error)

// mutationRunner runs gremlins for one target, writing its JSON report to
// output. A run that finds no mutant writes no report; that is not an error.
type mutationRunner func(binary string, target mutation.Target, output string) error

type options struct {
	base     string
	floor    float64
	gremlins string
}

func run(args []string, out io.Writer, diff diffSource, mutate mutationRunner) int {
	parsed, err := parseOptions(args)
	if err != nil {
		fmt.Fprintln(out, err)
		return exitUsage
	}
	summary, err := mutateChanges(parsed, diff, mutate, out)
	if err != nil {
		fmt.Fprintln(out, "diff-mutation:", err)
		return exitUsage
	}
	printSummary(out, summary, parsed.floor)
	if !summary.MeetsFloor(parsed.floor) {
		return exitFail
	}
	return exitPass
}

func parseOptions(args []string) (options, error) {
	parsed := options{}
	flags := flag.NewFlagSet("diff-mutation", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&parsed.base, "base", "", "commit to diff against (required)")
	flags.Float64Var(&parsed.floor, "floor", 0, "minimum mutation score on changed lines; 0 reports only")
	flags.StringVar(&parsed.gremlins, "gremlins", "gremlins", "path to the gremlins binary")
	if err := flags.Parse(args); err != nil {
		return options{}, fmt.Errorf("diff-mutation: %w", err)
	}
	if parsed.base == "" {
		return options{}, errors.New("diff-mutation: --base is required")
	}
	return parsed, nil
}

func mutateChanges(parsed options, diff diffSource, mutate mutationRunner, out io.Writer) (mutation.Summary, error) {
	changes, err := readChanges(parsed.base, diff)
	if err != nil {
		return mutation.Summary{}, err
	}
	targets, err := mutation.Plan(".", changes, diffcover.RepositoryExclusions)
	if err != nil {
		return mutation.Summary{}, err
	}
	fmt.Fprintf(out, "mutating %d changed package(s)\n", len(targets))
	mutants, err := mutateTargets(parsed.gremlins, targets, mutate)
	if err != nil {
		return mutation.Summary{}, err
	}
	return mutation.Summarize(mutation.Scope(mutants, changes)), nil
}

func readChanges(base string, diff diffSource) (diffcover.Changes, error) {
	diffOutput, err := diff(base)
	if err != nil {
		return nil, err
	}
	return diffcover.ParseDiff(diffOutput)
}

func mutateTargets(binary string, targets []mutation.Target, mutate mutationRunner) ([]mutation.Mutant, error) {
	reports, err := os.MkdirTemp("", "diff-mutation-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(reports)
	var mutants []mutation.Mutant
	for index, target := range targets {
		found, err := mutateTarget(binary, target, filepath.Join(reports, strconv.Itoa(index)+".json"), mutate)
		if err != nil {
			return nil, err
		}
		mutants = append(mutants, found...)
	}
	return mutants, nil
}

func mutateTarget(binary string, target mutation.Target, output string, mutate mutationRunner) ([]mutation.Mutant, error) {
	if err := mutate(binary, target, output); err != nil {
		return nil, fmt.Errorf("gremlins on %s: %w", target.Dir, err)
	}
	report, err := os.Open(output)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil // no mutable operator in the changed files
	}
	if err != nil {
		return nil, err
	}
	defer report.Close()
	return mutation.ParseResults(report, target.Dir)
}

func printSummary(out io.Writer, summary mutation.Summary, floor float64) {
	printHeadline(out, summary, floor)
	printMutants(out, "LIVED      ", summary.Lived)
	printMutants(out, "TIMED OUT  ", summary.TimedOut)
	printMutants(out, "NOT VIABLE ", summary.NotViable)
	printMutants(out, "NOT COVERED", summary.NotCovered)
}

func printHeadline(out io.Writer, summary mutation.Summary, floor float64) {
	tested := len(summary.Killed) + len(summary.Lived) + len(summary.TimedOut)
	score, ok := summary.Score()
	switch {
	case !ok:
		fmt.Fprintln(out, "mutation score on changed lines: nothing to score (no tested mutant on a changed line)")
	case floor == 0:
		fmt.Fprintf(out, "mutation score on changed lines: %.1f%% (%d of %d tested killed; report-only)\n",
			score, len(summary.Killed), tested)
	default:
		fmt.Fprintf(out, "mutation score on changed lines: %.1f%% (%d of %d tested killed; floor %.1f%%)\n",
			score, len(summary.Killed), tested, floor)
	}
}

func printMutants(out io.Writer, label string, mutants []mutation.Mutant) {
	for _, mutant := range mutants {
		fmt.Fprintf(out, "  %s %s:%d:%d %s\n", label, mutant.File, mutant.Line, mutant.Column, mutant.Type)
	}
}
