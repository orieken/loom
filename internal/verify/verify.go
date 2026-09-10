// Package verify reproduces a stage's measurements instead of believing them
// (roadmap L2.24, the measurement half).
//
// Run 7 settled what the path check could not. With its dependencies
// removed, qa-engineer reported 171 passing tests and 86.08% package
// coverage. Neither was obtainable — there was nothing to run the suite
// with — and 86.08 appears nowhere in the repository except the stage's own
// report. Its `knownGaps` was empty and its report stated the figures were
// "both above the 85% threshold". The control, the same stage with real
// dependencies, reported 169 and 86.08%, which `pnpm test` confirms.
//
// **It reported the same coverage figure whether or not it measured
// anything.**
//
// So the executor measures. This is L3.25's argument — arithmetic over known
// quantities is not a model's job — applied to a fact the model was being
// asked to observe rather than compute.
//
// The command comes from PROJECT configuration, never from the state
// document. An executor that ran a string a model chose would be executing
// model output with the executor's privileges, which is a different trust
// boundary from the agent running its own tools, and a worse one. A project
// declares what proves its tests pass; the agent gets no say.
package verify

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	yaml "go.yaml.in/yaml/v4"
)

// ConfigFile is where a project declares how its tests are run.
const ConfigFile = ".claude/delivery-policy.yaml"

// DefaultTimeout bounds a verification command. A suite that cannot finish
// inside it is reported as unverifiable rather than allowed to hang a run.
const DefaultTimeout = 15 * time.Minute

// Config is the subset of delivery-policy.yaml this package reads.
type Config struct {
	// TestCommand proves the suite passes. Empty disables verification, and
	// the result then says so rather than passing silently.
	TestCommand string `yaml:"testCommand"`
}

// Outcome is what reproducing a claim established.
type Outcome struct {
	// Verified is true only when the command ran and succeeded.
	Verified bool
	// Unverifiable is true when no command is configured or it could not be
	// run at all. It is NOT a failure of the claim — it is the absence of
	// evidence, and must never be reported as confirmation.
	Unverifiable bool
	Reason       string
	Command      string
}

// LoadConfig reads the project's verification configuration. A missing file
// is not an error: verification is opt-in, and a project that has not opted
// in gets claims marked unverified rather than a broken run.
func LoadConfig(root string) (Config, error) {
	raw, err := os.ReadFile(root + "/" + ConfigFile)
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", ConfigFile, err)
	}
	var config Config
	if err := yaml.Unmarshal(raw, &config); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", ConfigFile, err)
	}
	return config, nil
}

// Tests runs the project's declared test command and reports what happened.
func Tests(ctx context.Context, root string, config Config) Outcome {
	command := strings.TrimSpace(config.TestCommand)
	if command == "" {
		return Outcome{Unverifiable: true,
			Reason: "no testCommand is configured in " + ConfigFile}
	}
	timed, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()
	// A shell is used deliberately: a test command is a pipeline in most
	// projects, and this string comes from project configuration a human
	// wrote, not from a model.
	execution := exec.CommandContext(timed, "sh", "-c", command)
	execution.Dir = root
	output, err := execution.CombinedOutput()
	if err != nil {
		return Outcome{Command: command,
			Reason: fmt.Sprintf("%v — %s", err, lastLines(string(output), 6))}
	}
	return Outcome{Verified: true, Command: command}
}

// lastLines keeps a failure's tail, which is where a test runner puts the
// reason, without pasting an entire suite's output into run state.
func lastLines(output string, count int) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) > count {
		lines = lines[len(lines)-count:]
	}
	return strings.Join(lines, " | ")
}
