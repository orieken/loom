// Command diff-coverage fails when the Go lines a change adds or modifies are
// less than --threshold percent covered (roadmap L3.58, ADR-008).
//
//	go test ./... -coverprofile=coverage.out
//	go run ./cmd/diff-coverage --base origin/main --profile coverage.out
//
// The base is compared against the working tree, so uncommitted changes are
// measured locally; in CI the working tree is the commit under test.
package main

import (
	"os"

	"github.com/orieken/loom/internal/gitdiff"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, gitdiff.Diff))
}
