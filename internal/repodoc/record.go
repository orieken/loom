package repodoc

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/orieken/loom/internal/factsheet"
	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/repograph"
	"github.com/orieken/loom/internal/state"
)

// RunsDir is where run records and candidate documents live, under .loom/.
const RunsDir = "runs"

// RoundRecord is one pass of the loop over a directory: what the verifier and
// the auditor said about the documenter's attempt. Keeping every round is what
// makes "why did this take three rounds?" answerable.
type RoundRecord struct {
	Round      int         `json:"round"`
	Violations []Violation `json:"violations"`
	// Audited says the semantic auditor ran. It runs only when the
	// deterministic checks found nothing.
	Audited bool `json:"audited"`
	// Verdict is the auditor's, empty when it did not run.
	Verdict string `json:"verdict,omitempty"`
}

// Outcome is what happened to one directory.
type Outcome struct {
	Path      string `json:"path"`
	InputHash string `json:"inputHash"`
	// Status is "verified", "unverified" (the round limit was reached with
	// problems outstanding — a human has to look) or "failed" (no valid
	// document could be obtained at all).
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	// Rounds is the loop's history, oldest first.
	Rounds []RoundRecord `json:"rounds,omitempty"`
	// FilesRead counts the source reads the agent recorded: reading source is
	// the exception, and the count shows how often the exception happened.
	FilesRead int                 `json:"filesRead"`
	Usage     *orchestrator.Usage `json:"usage,omitempty"`
}

// Outcome statuses.
const (
	StatusVerified   = "verified"
	StatusUnverified = "unverified"
	StatusFailed     = "failed"
)

// RunRecord is `.loom/runs/<id>/run.json`: what was sent, what came back and
// what it cost. Only verified results are candidates for the index; nothing
// here is in it yet.
type RunRecord struct {
	RunID       string              `json:"runId"`
	StartedAt   string              `json:"startedAt"`
	Agent       string              `json:"agent"`
	Interrupted bool                `json:"interrupted,omitempty"`
	Outcomes    []Outcome           `json:"outcomes"`
	TotalUsage  *orchestrator.Usage `json:"totalUsage,omitempty"`
}

// Count returns how many directories ended in a status.
func (r RunRecord) Count(status string) int {
	count := 0
	for _, outcome := range r.Outcomes {
		if outcome.Status == status {
			count++
		}
	}
	return count
}

// Failed counts directories that never produced a valid document.
func (r RunRecord) Failed() int { return r.Count(StatusFailed) }

// Unverified counts directories whose document still had problems when the
// round limit was reached.
func (r RunRecord) Unverified() int { return r.Count(StatusUnverified) }

// RunDir is the directory holding one run's artifacts.
func RunDir(root, runID string) string {
	return filepath.Join(root, repograph.OutputDir, RunsDir, runID)
}

// CandidatePath is where a directory's candidate document is stored in a run.
func CandidatePath(runDir, dir string) string {
	return filepath.Join(runDir, "docs", mirrored(dir))
}

// stageIDEscaper makes the directory-to-stage mapping injective: underscores
// and slashes become two-character codes that nothing else can produce, so
// "a/b" and "a__b" cannot share an id. The root keeps "_root", which the
// escaper never emits.
var stageIDEscaper = strings.NewReplacer("_", "_u", "/", "_s")

// dirKey is a filesystem-safe, unique key for a directory.
func dirKey(dir string) string {
	if dir == factsheet.RootPath {
		return "_root"
	}
	return stageIDEscaper.Replace(dir)
}

// stageID names the documenter's stage for a directory.
func stageID(dir string) string { return "repo-doc-" + dirKey(dir) }

func writeRecord(runDir string, record RunRecord) error {
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode run record: %w", err)
	}
	return repograph.WriteFileAtomically(filepath.Join(runDir, "run.json"), append(raw, '\n'))
}

// UnverifiedPath is where the last attempt for a directory is kept when the
// loop ran out of rounds. It is never a candidate for the index.
func UnverifiedPath(runDir, dir string) string {
	return filepath.Join(runDir, "unverified", mirrored(dir))
}

// RoundPath is where one round's documenter payload is kept.
func RoundPath(runDir, dir string, round int) string {
	return filepath.Join(runDir, "rounds", mirrored(dir), fmt.Sprintf("round-%d.json", round))
}

// mirrored maps a directory to a path fragment: its own path plus .json, with
// "_root" for the repo root.
func mirrored(dir string) string {
	if dir == factsheet.RootPath {
		return "_root.json"
	}
	return filepath.FromSlash(dir) + ".json"
}

func writePayload(target string, payload []byte) error {
	var pretty json.RawMessage = payload
	raw, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", target, err)
	}
	return repograph.WriteFileAtomically(target, append(raw, '\n'))
}

func writeDocument(target string, doc state.RepoDocState) error {
	raw, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("encode document: %w", err)
	}
	return writePayload(target, raw)
}
