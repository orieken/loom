package mutation

import "github.com/orieken/loom/internal/diffcover"

// Scope keeps the mutants that sit on a changed line. gremlins mutates whole
// files; a change is judged only on what it touched.
func Scope(mutants []Mutant, changes diffcover.Changes) []Mutant {
	changed := map[string]map[int]bool{}
	for file, lines := range changes {
		changed[file] = map[int]bool{}
		for _, line := range lines {
			changed[file][line] = true
		}
	}
	var scoped []Mutant
	for _, mutant := range mutants {
		if changed[mutant.File][mutant.Line] {
			scoped = append(scoped, mutant)
		}
	}
	return scoped
}

// Summary groups scoped mutants by what happened to them.
type Summary struct {
	Killed     []Mutant
	Lived      []Mutant
	TimedOut   []Mutant
	NotViable  []Mutant // never compiled: reported, never scored
	NotCovered []Mutant // no test runs the line: diff-coverage's finding, not scored here
	Skipped    []Mutant
}

// Summarize sorts mutants into a Summary.
func Summarize(mutants []Mutant) Summary {
	summary := Summary{}
	buckets := map[string]*[]Mutant{
		StatusKilled: &summary.Killed, StatusLived: &summary.Lived, StatusTimedOut: &summary.TimedOut,
		StatusNotViable: &summary.NotViable, StatusNotCovered: &summary.NotCovered,
		StatusSkipped: &summary.Skipped, StatusRunnable: &summary.Skipped,
	}
	for _, mutant := range mutants {
		bucket := buckets[mutant.Status]
		*bucket = append(*bucket, mutant)
	}
	return summary
}

// Score is the percentage of tested mutants the suite killed. A timed-out
// mutant counts against the score: the spike showed timeouts hiding
// survivors — defaults turned four LIVED mutants into TIMED OUT and reported
// 100%. NOT VIABLE and NOT COVERED mutants are outside the denominator; the
// first never ran and the second is a coverage finding. ok is false when no
// mutant was tested, so there is nothing to score.
func (summary Summary) Score() (score float64, ok bool) {
	tested := len(summary.Killed) + len(summary.Lived) + len(summary.TimedOut)
	if tested == 0 {
		return 0, false
	}
	return 100 * float64(len(summary.Killed)) / float64(tested), true
}

// MeetsFloor reports whether the score reaches floor. A floor of zero is
// report-only; nothing scorable always meets it.
func (summary Summary) MeetsFloor(floor float64) bool {
	score, ok := summary.Score()
	return !ok || score >= floor
}
