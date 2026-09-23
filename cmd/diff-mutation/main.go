// Command diff-mutation mutation-tests the Go lines a change adds or modifies
// and reports how many of those mutants the tests kill (roadmap L3.59,
// ADR-008 clause 2).
//
//	go run ./cmd/diff-mutation --base origin/main
//
// It needs gremlins v0.6.0 on PATH (or --gremlins). With --floor 0, the
// default, it only reports: the floor is set later from measurement, the way
// the coverage ratchet's was. Mutants that never compiled are listed and never
// counted as killed; mutants that timed out count against the score.
package main

import (
	"os"

	"github.com/orieken/loom/internal/gitdiff"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, gitdiff.Diff, runGremlins))
}
