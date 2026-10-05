package repodoc

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/orieken/loom/internal/factsheet"
	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/repograph"
)

// RunsDir is where run records and candidate documents live, under .loom/.
const RunsDir = "runs"

// Outcome is what happened to one directory.
type Outcome struct {
	Path      string `json:"path"`
	InputHash string `json:"inputHash"`
	// Status is "documented" or "failed".
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	// FilesRead counts the source reads the agent recorded: reading source is
	// the exception, and the count shows how often the exception happened.
	FilesRead int                 `json:"filesRead"`
	Usage     *orchestrator.Usage `json:"usage,omitempty"`
}

// Outcome statuses.
const (
	StatusDocumented = "documented"
	StatusFailed     = "failed"
)

// RunRecord is `.loom/runs/<id>/run.json`: what was sent, what came back and
// what it cost. Results are candidates; nothing here is in the index.
type RunRecord struct {
	RunID       string              `json:"runId"`
	StartedAt   string              `json:"startedAt"`
	Agent       string              `json:"agent"`
	Interrupted bool                `json:"interrupted,omitempty"`
	Outcomes    []Outcome           `json:"outcomes"`
	TotalUsage  *orchestrator.Usage `json:"totalUsage,omitempty"`
}

// Failed counts directories that did not produce a valid document.
func (r RunRecord) Failed() int {
	failed := 0
	for _, outcome := range r.Outcomes {
		if outcome.Status == StatusFailed {
			failed++
		}
	}
	return failed
}

// RunDir is the directory holding one run's artifacts.
func RunDir(root, runID string) string {
	return filepath.Join(root, repograph.OutputDir, RunsDir, runID)
}

// CandidatePath is where a directory's candidate document is stored in a run.
func CandidatePath(runDir, dir string) string {
	if dir == factsheet.RootPath {
		return filepath.Join(runDir, "docs", "_root.json")
	}
	return filepath.Join(runDir, "docs", filepath.FromSlash(dir)+".json")
}

// stageIDEscaper makes the directory-to-stage mapping injective: underscores
// and slashes become two-character codes that nothing else can produce, so
// "a/b" and "a__b" cannot share an id. The root keeps "_root", which the
// escaper never emits.
var stageIDEscaper = strings.NewReplacer("_", "_u", "/", "_s")

// stageID is a filesystem-safe, unique stage name for a directory.
func stageID(dir string) string {
	if dir == factsheet.RootPath {
		return "repo-doc-_root"
	}
	return "repo-doc-" + stageIDEscaper.Replace(dir)
}

func writeRecord(runDir string, record RunRecord) error {
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode run record: %w", err)
	}
	return repograph.WriteFileAtomically(filepath.Join(runDir, "run.json"), append(raw, '\n'))
}

func writeCandidate(runDir, dir string, payload []byte) error {
	var pretty json.RawMessage = payload
	raw, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		return fmt.Errorf("encode candidate %s: %w", dir, err)
	}
	return repograph.WriteFileAtomically(CandidatePath(runDir, dir), append(raw, '\n'))
}
