// Package harness is the factory's end-to-end harness (roadmap L3.73): it
// hands a fixture repository one work order, lets `loom factory` deliver it
// — approving every gate explicitly, as a human would — and then judges what
// came out against acceptance tests the run never saw.
//
// It runs in two halves so the judge can be kept from the run. Deliver sets
// up the repository and drives the factory; Verify, given the hidden tests
// only then, checks the pushed branch. In a container the hidden tests are
// mounted for Verify alone, so a real agent cannot read its own judge.
package harness

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Defaults for a harness run.
const (
	DefaultMaxPasses   = 12
	DefaultPassTimeout = 30 * time.Minute
	reportFile         = "report.json"
)

// Config is one harness run.
type Config struct {
	// Loom is the loom binary under test.
	Loom string
	// Fixture holds repo/ (the project), order.md (the work order) and,
	// for the mock, solution/ (what its developer and QA stages write).
	Fixture string
	// Hidden holds the acceptance tests, copied into a clone of the pushed
	// branch by Verify and read by nothing else.
	Hidden string
	// Workdir is an empty directory the harness owns.
	Workdir string
	// Provider and RunArgs are passed to the factory for every run.
	Provider string
	RunArgs  []string
	// Acceptance is the command that runs the hidden tests in the clone.
	Acceptance  []string
	MaxPasses   int
	PassTimeout time.Duration
	Output      io.Writer
}

func (c Config) withDefaults() Config {
	if c.MaxPasses <= 0 {
		c.MaxPasses = DefaultMaxPasses
	}
	if c.PassTimeout <= 0 {
		c.PassTimeout = DefaultPassTimeout
	}
	if len(c.Acceptance) == 0 {
		c.Acceptance = []string{"go", "test", "./..."}
	}
	if c.Output == nil {
		c.Output = io.Discard
	}
	return c
}

func (c Config) project() string { return filepath.Join(c.Workdir, "project") }
func (c Config) origin() string  { return filepath.Join(c.Workdir, "origin.git") }
func (c Config) clone() string   { return filepath.Join(c.Workdir, "verify") }

// Check is one judged property of the run.
type Check struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

// Report is what a harness run found.
type Report struct {
	Order    string   `json:"order"`
	Approved []string `json:"approved"`
	Checks   []Check  `json:"checks"`
}

// Passed reports whether every check passed. A report with no checks has
// judged nothing, and does not pass.
func (r Report) Passed() bool {
	for _, check := range r.Checks {
		if !check.Passed {
			return false
		}
	}
	return len(r.Checks) > 0
}

// record adds a check: passed when err is nil, with err as the detail.
func (r *Report) record(name string, err error) {
	check := Check{Name: name, Passed: err == nil}
	if err != nil {
		check.Detail = err.Error()
	}
	r.Checks = append(r.Checks, check)
}

// Failed lists the checks that did not pass.
func (r Report) Failed() []Check {
	var failed []Check
	for _, check := range r.Checks {
		if !check.Passed {
			failed = append(failed, check)
		}
	}
	return failed
}

// Run delivers and verifies in one process — the mock's mode, where nothing
// in the run can read the hidden tests anyway.
func Run(config Config) (Report, error) {
	if _, err := Deliver(config); err != nil {
		return Report{}, err
	}
	return Verify(config)
}

func saveReport(config Config, report Report) error {
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(config.Workdir, reportFile), raw, 0o644)
}

// ErrNoDelivery reports a Verify with no Deliver before it.
var ErrNoDelivery = errors.New("no delivery to verify")

func loadReport(config Config) (Report, error) {
	raw, err := os.ReadFile(filepath.Join(config.Workdir, reportFile))
	if err != nil {
		return Report{}, fmt.Errorf("%w in %s: %v", ErrNoDelivery, config.Workdir, err)
	}
	var report Report
	if err := json.Unmarshal(raw, &report); err != nil {
		return Report{}, fmt.Errorf("read %s: %w", reportFile, err)
	}
	return report, nil
}

// Print writes the report for a person.
func Print(out io.Writer, report Report) {
	fmt.Fprintf(out, "order %s — approved: %v\n", report.Order, report.Approved)
	for _, check := range report.Checks {
		status := "PASS"
		if !check.Passed {
			status = "FAIL"
		}
		fmt.Fprintf(out, "  %s  %s", status, check.Name)
		if check.Detail != "" {
			fmt.Fprintf(out, " — %s", check.Detail)
		}
		fmt.Fprintln(out)
	}
}
