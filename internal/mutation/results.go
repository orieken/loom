package mutation

import (
	"encoding/json"
	"fmt"
	"io"
)

// Mutant statuses as gremlins writes them.
const (
	StatusKilled     = "KILLED"
	StatusLived      = "LIVED"
	StatusTimedOut   = "TIMED OUT"
	StatusNotViable  = "NOT VIABLE"
	StatusNotCovered = "NOT COVERED"
	StatusSkipped    = "SKIPPED"
	StatusRunnable   = "RUNNABLE" // dry-run only
)

var knownStatuses = map[string]bool{
	StatusKilled: true, StatusLived: true, StatusTimedOut: true, StatusNotViable: true,
	StatusNotCovered: true, StatusSkipped: true, StatusRunnable: true,
}

// Mutant is one mutation gremlins applied (or tried to), located by
// repository-relative path.
type Mutant struct {
	File   string
	Line   int
	Column int
	Type   string
	Status string
}

type gremlinsReport struct {
	Files []struct {
		FileName  string `json:"file_name"`
		Mutations []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
			Line   int    `json:"line"`
			Column int    `json:"column"`
		} `json:"mutations"`
	} `json:"files"`
}

// ParseResults reads a gremlins JSON report produced for the target
// directory dir. An unknown status is an error rather than a guess: a status
// misfiled as killed is exactly the silent pass this package exists to stop.
func ParseResults(reader io.Reader, dir string) ([]Mutant, error) {
	var report gremlinsReport
	if err := json.NewDecoder(reader).Decode(&report); err != nil {
		return nil, fmt.Errorf("gremlins report for %s: %w", dir, err)
	}
	var mutants []Mutant
	for _, file := range report.Files {
		for _, mutation := range file.Mutations {
			if !knownStatuses[mutation.Status] {
				return nil, fmt.Errorf("gremlins report for %s: unknown status %q at %s:%d",
					dir, mutation.Status, file.FileName, mutation.Line)
			}
			mutants = append(mutants, Mutant{
				File: repositoryPath(dir, file.FileName), Line: mutation.Line, Column: mutation.Column,
				Type: mutation.Type, Status: mutation.Status,
			})
		}
	}
	return mutants, nil
}
