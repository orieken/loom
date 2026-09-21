package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// StateSchemaVersion identifies the run-state JSON shape. Bump on any
// incompatible change so a future reader can refuse or migrate old files.
const StateSchemaVersion = 10

// RunStateFileName is the executor-owned state file inside the feature
// workspace. NOTE: this lives beside the markdown pipeline's
// pipeline-state.json; roadmap L2.12 migrates pipeline-state.json semantics
// (checksum-verified resume, tamper refusal) into this file and retires the
// prompt-owned one.
const RunStateFileName = "run-state.json"

// StageStatus is the lifecycle state of one stage in a run.
type StageStatus string

// Stage lifecycle values. A stage is only skipped on resume when COMPLETED;
// RUNNING, INTERRUPTED, and FAILED stages are re-run.
const (
	StageStatusRunning     StageStatus = "RUNNING"
	StageStatusCompleted   StageStatus = "COMPLETED"
	StageStatusFailed      StageStatus = "FAILED"
	StageStatusInterrupted StageStatus = "INTERRUPTED"
	// StageStatusWaitingApproval marks a gated stage the executor refused to
	// start because its gate has no approval yet (roadmap L2.13).
	StageStatusWaitingApproval StageStatus = "WAITING_APPROVAL"
	// StageStatusStale marks a stage that completed once but whose artifact
	// no longer matches its recorded digest, or whose input went stale
	// (roadmap L2.12). A stale stage re-runs.
	StageStatusStale StageStatus = "STALE"
	// StageStatusSkipped marks a stage the router routed around (roadmap
	// L3.0). It is terminal: a skipped stage does not run, and is not
	// re-run on resume.
	StageStatusSkipped StageStatus = "SKIPPED"
)

// StageRecord is the persisted result of one stage invocation.
type StageRecord struct {
	Status         StageStatus `json:"status"`
	StartedAt      time.Time   `json:"startedAt"`
	FinishedAt     *time.Time  `json:"finishedAt,omitempty"`
	ArtifactPath   string      `json:"artifactPath,omitempty"`
	ArtifactSHA256 string      `json:"artifactSha256,omitempty"`
	Gate           string      `json:"gate,omitempty"`
	Sequence       int         `json:"sequence"`
	PreviousStatus StageStatus `json:"previousStatus,omitempty"`
	StaleReason    StaleReason `json:"staleReason,omitempty"`
	FoundSHA256    string      `json:"foundSha256,omitempty"`
	SkipReason     string      `json:"skipReason,omitempty"`
	// PostureViolation is set when a stage changed the working tree without
	// declaring a tool that writes files (roadmap L3.30). Recorded, not
	// fatal: run 4's accessibility-engineer did this and its edits were
	// correct — the defect is that nothing noticed, not that it happened.
	PostureViolation string `json:"postureViolation,omitempty"`
	// EditedAfterReview marks a stage that changed the working tree after
	// the review had approved it (roadmap L3.36). Recorded, not prevented:
	// a post-review stage may be entitled to write, and the defect is that
	// the divergence was invisible rather than that it happened.
	EditedAfterReview bool `json:"editedAfterReview,omitempty"`
	// Iteration counts the rounds a looping stage has run (roadmap L2.17).
	// Zero and one both mean a first pass; Sequence is unaffected, because
	// a re-run is the same step of the run, not a new one.
	Iteration int `json:"iteration,omitempty"`
	// StateKind is the typed document kind this stage produced, empty for an
	// untyped stage. Recorded on the record rather than looked up in the
	// plan because a gate has no plan in hand and still has to find the
	// review verdict or the security findings (roadmap L2.16).
	StateKind string `json:"stateKind,omitempty"`
	// Agent is who produced this stage's artifact. Recorded on the record
	// rather than looked up in the plan so attribution is self-contained —
	// an approval has no plan in hand, and a correction must still name the
	// agent whose output was corrected (roadmap L4.5).
	Agent string `json:"agent,omitempty"`
	// ViewPath is the rendered markdown a typed stage produces alongside its
	// state document (roadmap L2.9). Empty for an untyped stage, whose
	// artifact IS its markdown. It is recorded because it — not the tracked
	// artifact — is the file a human reads and corrects (roadmap L4.5).
	ViewPath string `json:"viewPath,omitempty"`
	// Usage is what the provider reported this stage's model call consumed
	// (roadmap L3.8). Nil when the provider reported nothing — which is not
	// the same as a provider reporting zero.
	Usage *Usage `json:"usage,omitempty"`
	// IterationArtifacts retains what each earlier round produced, each
	// with its own digest.
	IterationArtifacts []IterationArtifact `json:"iterationArtifacts,omitempty"`
	Error              string              `json:"error,omitempty"`
}

// RunState is the executor-owned durable state for one feature delivery run.
type RunState struct {
	SchemaVersion int     `json:"schemaVersion"`
	PlanName      string  `json:"planName"`
	CreatedBy     Creator `json:"createdBy"`
	FeatureName   string  `json:"featureName,omitempty"`
	SpecPath      string  `json:"specPath,omitempty"`
	// Provider is which stage provider this run was started with. A run is
	// mock or it is not, and half of each is meaningless — so this belongs
	// to the run's identity rather than to one invocation (roadmap L3.17).
	// Recorded at creation and adopted on resume: the printed resume
	// command carried no --provider, so following it silently moved a free
	// mock run onto the real binary mid-flight, and billed for it.
	Provider  string    `json:"provider,omitempty"`
	StartedAt time.Time `json:"startedAt"`
	// StartCommit is the commit the tree was at when this run began. A
	// gate measures the run's diff against it, so the number covers work
	// the run committed mid-flight as well as what is still uncommitted.
	// Empty when there is no work tree or the repository has no commits,
	// and the diff fact is then absent rather than guessed.
	StartCommit string `json:"startCommit,omitempty"`
	// DiffLines is how many lines the run had changed when it reached each
	// gate, keyed by gate name. Recorded rather than recomputed: a number
	// measured later describes whatever the tree holds then, and a dry-run
	// against a finished run would silently disagree with what the live
	// gate actually saw.
	DiffLines map[string]int `json:"diffLines,omitempty"`
	// ReviewedDigest fingerprints the tree as the review left it. Anything
	// changing afterwards is what the approval did not cover. Empty when no
	// reviewing stage has completed, which is why an empty PostReviewEdits
	// list is ambiguous without it.
	ReviewedDigest string                 `json:"reviewedDigest,omitempty"`
	Stages         map[string]StageRecord `json:"stages"`
	Approvals      map[string]Approval    `json:"approvals"`
	// Baselines records what a human was last shown at each gate (roadmap
	// L4.5), keyed by gate name. Retained so a later edit can be described
	// rather than merely detected — a digest says that something changed
	// and never what.
	Baselines map[string]GateBaseline `json:"baselines,omitempty"`
	// PolicyDecisions records what the policies watching each gate decided
	// (roadmap L2.16). Recorded, not acted on: this build still halts for a
	// human at every gate. The record is the audit trail approval-gates.md
	// promised and never had.
	PolicyDecisions []PolicyRecord `json:"policyDecisions,omitempty"`
	// Corrections records every human edit to a stage's output detected at
	// a gate (roadmap L4.5), so the signal survives without reading the
	// timeline. Append-only within a run.
	Corrections []Correction `json:"corrections,omitempty"`
	// Spend is every provider call this run has made, accumulated as it
	// goes and never derived from the stage records.
	//
	// L3.22 made a stage record sum its own attempts, which fixed a retry
	// overwriting a failure. Run 4 found the same under-report through a
	// second door: re-running one stage requires deleting its record by
	// hand — `loom` has no rollback command (run 4 §9.1) — and deleting the
	// record deleted the $1.2389 it had already cost. The executor reported
	// $20.2718 for a run whose spans total $21.5106, short by exactly the
	// discarded attempt.
	//
	// Money spent is a fact about the run, not a property of a record
	// someone may remove, so it is accumulated here where nothing about a
	// stage's later fate can subtract from it.
	Spend     *Usage    `json:"spend,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Creator identifies which pipeline owns a state file. The two pipelines
// route differently — the markdown one skips conditional agents and loops
// developer/code-reviewer, the executor's plan is linear — so sharing a
// file is fine but resuming each other's runs is not.
type Creator string

// The two pipelines that write run state.
const (
	CreatedByExecutor Creator = "executor"
	CreatedByMarkdown Creator = "markdown"
)

// NewRunState returns an empty state for a fresh run of the named plan.
func NewRunState(planName string, createdBy Creator) *RunState {
	return &RunState{
		SchemaVersion: StateSchemaVersion,
		PlanName:      planName,
		CreatedBy:     createdBy,
		StartedAt:     time.Now().UTC(),
		Stages:        map[string]StageRecord{},
		Approvals:     map[string]Approval{},
		Baselines:     map[string]GateBaseline{},
	}
}

// CheckCreatedBy refuses a state file written by the other pipeline.
func (s *RunState) CheckCreatedBy(want Creator) error {
	if s.CreatedBy == want {
		return nil
	}
	return fmt.Errorf("this run state was written by the %s pipeline, not the %s one — the two route differently and cannot resume each other's runs",
		s.CreatedBy, want)
}

// NextSequence returns the sequence number for a stage record being created
// for the first time. Sequences are monotonic in recording order and are
// preserved when a stage is re-recorded (a re-run after a review loop is
// still the same position in the run), so they order stages for a pipeline
// that has no fixed plan to order by.
func (s *RunState) NextSequence() int {
	highest := 0
	for _, record := range s.Stages {
		if record.Sequence > highest {
			highest = record.Sequence
		}
	}
	return highest + 1
}

// StagesInSequence returns the recorded stage IDs in recording order.
func (s *RunState) StagesInSequence() []string {
	ids := make([]string, 0, len(s.Stages))
	for id := range s.Stages {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return s.Stages[ids[i]].Sequence < s.Stages[ids[j]].Sequence })
	return ids
}

// IsStageCompleted reports whether a stage already finished successfully and
// may be skipped on resume.
func (s *RunState) IsStageCompleted(stageID string) bool {
	return s.Stages[stageID].Status == StageStatusCompleted
}

// StateStore persists RunState to a single JSON file with atomic writes.
type StateStore struct {
	path string
}

// NewStateStore returns a store writing to the given file path.
func NewStateStore(path string) *StateStore {
	return &StateStore{path: path}
}

// Path returns the file the store reads and writes.
func (st *StateStore) Path() string { return st.path }

// Load reads persisted state. It returns (nil, nil) when no state file
// exists — a fresh run, not an error.
func (st *StateStore) Load() (*RunState, error) {
	raw, err := os.ReadFile(st.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read run state: %w", err)
	}
	return decodeRunState(raw, st.path)
}

func decodeRunState(raw []byte, path string) (*RunState, error) {
	var state RunState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("parse run state %s: %w", path, err)
	}
	if state.SchemaVersion != StateSchemaVersion {
		return nil, fmt.Errorf("run state %s has schema version %d, this executor supports %d — finish or delete the old run",
			path, state.SchemaVersion, StateSchemaVersion)
	}
	if state.Stages == nil {
		state.Stages = map[string]StageRecord{}
	}
	if state.Approvals == nil {
		state.Approvals = map[string]Approval{}
	}
	if state.Baselines == nil {
		state.Baselines = map[string]GateBaseline{}
	}
	return &state, nil
}

// Save persists state atomically: it writes a temp file in the same
// directory, then os.Rename over the target. A crash between the temp write
// and the rename leaves the previous state file intact — readers never see
// a partial JSON document.
func (st *StateStore) Save(state *RunState) error {
	state.UpdatedAt = time.Now().UTC()
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode run state: %w", err)
	}
	return st.writeAtomic(raw)
}

func (st *StateStore) writeAtomic(raw []byte) error {
	dir := filepath.Dir(st.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, RunStateFileName+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	return st.commitTemp(tmp, raw)
}

func (st *StateStore) commitTemp(tmp *os.File, raw []byte) error {
	tmpName := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temp state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp state file: %w", err)
	}
	if err := os.Rename(tmpName, st.path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename temp state file into place: %w", err)
	}
	return nil
}

// FeatureNameFromSpec derives the feature name from its spec file, matching
// the workspace convention (features/user-auth.md -> user-auth).
func FeatureNameFromSpec(specPath string) string {
	base := filepath.Base(specPath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// ArtifactSHA256 computes the hex SHA-256 of an artifact file, in Go —
// never delegated to a prompt (roadmap L2.12 alignment).
func ArtifactSHA256(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read artifact for checksum: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
