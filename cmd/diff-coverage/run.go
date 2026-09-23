package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/orieken/loom/internal/diffcover"
)

const (
	exitPass  = 0
	exitFail  = 1
	exitUsage = 2
)

// defaultThreshold is ADR-008's bar for changed code.
const defaultThreshold = 85.0

// diffSource returns `git diff --unified=0` output from base to the working tree.
type diffSource func(base string) (io.Reader, error)

type options struct {
	base      string
	profile   string
	threshold float64
}

func run(args []string, out io.Writer, diff diffSource) int {
	parsed, err := parseOptions(args)
	if err != nil {
		fmt.Fprintln(out, err)
		return exitUsage
	}
	report, err := measure(parsed, diff)
	if err != nil {
		fmt.Fprintln(out, "diff-coverage:", err)
		return exitUsage
	}
	printReport(out, report, parsed.threshold)
	if !report.Passes(parsed.threshold) {
		return exitFail
	}
	return exitPass
}

func parseOptions(args []string) (options, error) {
	parsed := options{}
	flags := flag.NewFlagSet("diff-coverage", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&parsed.base, "base", "", "commit to diff against (required)")
	flags.StringVar(&parsed.profile, "profile", "coverage.out", "Go coverage profile")
	flags.Float64Var(&parsed.threshold, "threshold", defaultThreshold, "minimum percent of changed lines covered")
	if err := flags.Parse(args); err != nil {
		return options{}, fmt.Errorf("diff-coverage: %w", err)
	}
	if parsed.base == "" {
		return options{}, errors.New("diff-coverage: --base is required")
	}
	return parsed, nil
}

func measure(parsed options, diff diffSource) (diffcover.Report, error) {
	profile, err := readProfile(parsed.profile)
	if err != nil {
		return diffcover.Report{}, err
	}
	diffOutput, err := diff(parsed.base)
	if err != nil {
		return diffcover.Report{}, err
	}
	changes, err := diffcover.ParseDiff(diffOutput)
	if err != nil {
		return diffcover.Report{}, err
	}
	modulePath, err := readModulePath("go.mod")
	if err != nil {
		return diffcover.Report{}, err
	}
	return diffcover.Measure(diffcover.Measurement{
		Profile: profile, Changes: changes, ModulePath: modulePath, Excluded: diffcover.RepositoryExclusions,
	}), nil
}

func readProfile(path string) (diffcover.Profile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return diffcover.ParseProfile(file)
}

func readModulePath(goModPath string) (string, error) {
	file, err := os.Open(goModPath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if path, found := strings.CutPrefix(scanner.Text(), "module "); found {
			return strings.TrimSpace(path), nil
		}
	}
	return "", fmt.Errorf("%s declares no module", goModPath)
}

func printReport(out io.Writer, report diffcover.Report, threshold float64) {
	fmt.Fprintf(out, "changed-line coverage: %.1f%% (%d of %d executable changed lines; threshold %.0f%%)\n",
		report.Percent(), report.Covered, report.Executable, threshold)
	for _, line := range report.Uncovered {
		fmt.Fprintf(out, "  UNCOVERED   %s:%d\n", line.File, line.Line)
	}
	for _, file := range report.Unmeasured {
		fmt.Fprintf(out, "  UNMEASURED  %s — changed, but absent from the coverage profile\n", file)
	}
	for _, file := range report.Excluded {
		fmt.Fprintf(out, "  EXCLUDED    %s\n", file)
	}
	if report.Passes(threshold) {
		fmt.Fprintln(out, "PASS")
		return
	}
	fmt.Fprintln(out, "FAIL: add tests that execute the UNCOVERED lines; an UNMEASURED file fails whatever the percentage")
}
