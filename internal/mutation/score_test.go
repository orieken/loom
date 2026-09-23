package mutation_test

import (
	"reflect"
	"testing"

	"github.com/orieken/loom/internal/diffcover"
	"github.com/orieken/loom/internal/mutation"
)

func mutant(file string, line int, status string) mutation.Mutant {
	return mutation.Mutant{File: file, Line: line, Type: "CONDITIONALS_NEGATION", Status: status}
}

func TestScopeKeepsOnlyMutantsOnChangedLines(t *testing.T) {
	mutants := []mutation.Mutant{
		mutant("pkg/a.go", 3, mutation.StatusKilled),
		mutant("pkg/a.go", 9, mutation.StatusLived), // unchanged line
		mutant("pkg/b.go", 3, mutation.StatusLived), // unchanged file
		mutant("pkg/a.go", 4, mutation.StatusLived),
	}
	scoped := mutation.Scope(mutants, diffcover.Changes{"pkg/a.go": {3, 4}})

	want := []mutation.Mutant{mutants[0], mutants[3]}
	if !reflect.DeepEqual(scoped, want) {
		t.Errorf("scoped = %+v, want %+v", scoped, want)
	}
}

func TestScoreCountsTimeoutsAgainstAndLeavesUnrunMutantsOut(t *testing.T) {
	summary := mutation.Summarize([]mutation.Mutant{
		mutant("a.go", 1, mutation.StatusKilled),
		mutant("a.go", 2, mutation.StatusKilled),
		mutant("a.go", 3, mutation.StatusLived),
		mutant("a.go", 4, mutation.StatusTimedOut),
		mutant("a.go", 5, mutation.StatusNotViable),
		mutant("a.go", 6, mutation.StatusNotCovered),
		mutant("a.go", 7, mutation.StatusSkipped),
		mutant("a.go", 8, mutation.StatusRunnable),
	})

	// 2 killed of 4 tested (killed + lived + timed out) — not 2 of 2 as
	// gremlins' own efficacy, which leaves timeouts out, would report.
	score, ok := summary.Score()
	if !ok || score != 50 {
		t.Errorf("score = %v (ok=%v), want 50", score, ok)
	}
	counts := []int{len(summary.Killed), len(summary.Lived), len(summary.TimedOut),
		len(summary.NotViable), len(summary.NotCovered), len(summary.Skipped)}
	if !reflect.DeepEqual(counts, []int{2, 1, 1, 1, 1, 2}) {
		t.Errorf("bucket sizes = %v, want [2 1 1 1 1 2]", counts)
	}
}

func TestNothingTestedHasNoScoreAndMeetsAnyFloor(t *testing.T) {
	summary := mutation.Summarize([]mutation.Mutant{mutant("a.go", 1, mutation.StatusNotViable)})
	if _, ok := summary.Score(); ok {
		t.Error("a change with no tested mutant produced a score")
	}
	if !summary.MeetsFloor(90) {
		t.Error("nothing to score failed the floor")
	}
}

func TestFloorIsInclusive(t *testing.T) {
	summary := mutation.Summarize([]mutation.Mutant{
		mutant("a.go", 1, mutation.StatusKilled),
		mutant("a.go", 2, mutation.StatusKilled),
		mutant("a.go", 3, mutation.StatusKilled),
		mutant("a.go", 4, mutation.StatusLived),
	})
	if !summary.MeetsFloor(75) || summary.MeetsFloor(75.1) {
		t.Error("a 75% score must meet a 75 floor and miss 75.1")
	}
}
