// Command factory-e2e runs the factory's end-to-end harness (roadmap L3.73)
// outside `go test` — in the e2e container, where delivery and verification
// run in separate containers so the hidden tests exist only for the second.
//
//	factory-e2e deliver --loom loom --fixture DIR --workdir DIR [--provider mock] [--run-arg ARG]...
//	factory-e2e verify  --fixture DIR --hidden DIR --workdir DIR
//	factory-e2e run     (both, in one process)
//
// Exit codes: 0 every check passed, 1 a check failed, 2 the harness itself
// could not run.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/orieken/loom/tests/e2e/harness"
)

const (
	exitPassed = 0
	exitFailed = 1
	exitBroken = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

type repeated []string

func (r *repeated) String() string     { return fmt.Sprint(*r) }
func (r *repeated) Set(v string) error { *r = append(*r, v); return nil }

var phases = map[string]func(harness.Config) (harness.Report, error){
	"deliver": harness.Deliver, "verify": harness.Verify, "run": harness.Run,
}

func run(args []string, out io.Writer) int {
	if len(args) == 0 || phases[args[0]] == nil {
		fmt.Fprintln(out, "usage: factory-e2e deliver|verify|run [flags]")
		return exitBroken
	}
	config, err := parse(args[0], args[1:], out)
	if err != nil {
		fmt.Fprintln(out, err)
		return exitBroken
	}
	report, err := phases[args[0]](config)
	if err != nil {
		fmt.Fprintln(out, "factory-e2e:", err)
		return exitBroken
	}
	harness.Print(out, report)
	if !report.Passed() {
		return exitFailed
	}
	return exitPassed
}

func parse(phase string, args []string, out io.Writer) (harness.Config, error) {
	config := harness.Config{Output: out}
	var runArgs repeated
	flags := flag.NewFlagSet(phase, flag.ContinueOnError)
	flags.SetOutput(out)
	flags.StringVar(&config.Loom, "loom", "loom", "the loom binary under test")
	flags.StringVar(&config.Fixture, "fixture", "", "fixture directory: repo/, order.md, solution/")
	flags.StringVar(&config.Hidden, "hidden", "", "hidden acceptance tests, read only when verifying")
	flags.StringVar(&config.Workdir, "workdir", "", "an empty directory the harness owns")
	flags.StringVar(&config.Provider, "provider", "mock", "stage provider for every run")
	flags.IntVar(&config.MaxPasses, "max-passes", harness.DefaultMaxPasses, "factory passes before giving up")
	flags.Var(&runArgs, "run-arg", "extra argument for every loom run (repeatable)")
	if err := flags.Parse(args); err != nil {
		return config, err
	}
	config.RunArgs = runArgs
	if config.Fixture == "" || config.Workdir == "" {
		return config, errors.New("--fixture and --workdir are required")
	}
	return config, nil
}
