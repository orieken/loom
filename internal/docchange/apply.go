package docchange

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/orieken/loom/internal/repograph"
)

// Result is what Apply did.
type Result struct {
	// Written are the targets written now.
	Written []string `json:"written"`
	// AlreadyPresent are staged targets already holding the staged content —
	// an earlier, interrupted apply wrote them.
	AlreadyPresent []string `json:"alreadyPresent"`
	// Withheld are conflicts and skips, which are never written.
	Withheld  []string `json:"withheld"`
	AppliedAt string   `json:"appliedAt"`
}

// Apply writes an approved changeset into the repository.
//
// It checks, before writing anything: that an approval exists and is valid;
// that every staged file and change.json still match what was approved; and
// that every target is still what it was when planned. Only then does it
// write. A conflict — a generated block edited by hand — is never written; it
// was surfaced at the gate for a human to resolve.
func Apply(root, id string, now time.Time) (Result, error) {
	change, err := Load(root, id)
	if err != nil {
		return Result{}, err
	}
	if _, err := os.Stat(filepath.Join(Dir(root, id), appliedFile)); err == nil {
		return Result{}, fmt.Errorf("%w: %s", ErrAlreadyApplied, id)
	}
	if err := requireValidApproval(root, change, now); err != nil {
		return Result{}, err
	}
	pending, result, err := preflight(root, change)
	if err != nil {
		return Result{}, err
	}
	for _, artifact := range pending {
		if err := writeTarget(root, change.ID, artifact); err != nil {
			return Result{}, err
		}
		result.Written = append(result.Written, artifact.Target)
	}
	result.AppliedAt = now.UTC().Format(time.RFC3339)
	return result, recordApplied(root, id, result)
}

func requireValidApproval(root string, change Change, now time.Time) error {
	approval, found, err := LoadApproval(root, change.ID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%w: run `loom docs approve %s` after reviewing it", ErrNotApproved, change.ID)
	}
	return checkApproval(root, change, approval, now)
}

// preflight sorts the artifacts into what to write and what to leave alone,
// and refuses if a target changed under the plan.
func preflight(root string, change Change) ([]Artifact, Result, error) {
	var pending []Artifact
	result := Result{Written: []string{}, AlreadyPresent: []string{}, Withheld: []string{}}
	for _, artifact := range change.Artifacts {
		if !artifact.Staged() {
			if artifact.Action == ActionConflict || artifact.Action == ActionSkip {
				result.Withheld = append(result.Withheld, artifact.Target)
			}
			continue
		}
		state, err := targetState(root, change.ID, artifact)
		if err != nil {
			return nil, Result{}, err
		}
		switch state {
		case targetAlreadyWritten:
			result.AlreadyPresent = append(result.AlreadyPresent, artifact.Target)
		default:
			pending = append(pending, artifact)
		}
	}
	return pending, result, nil
}

type targetStatus int

const (
	targetNeedsWrite targetStatus = iota
	targetAlreadyWritten
)

// targetState compares a target with what was planned and what is staged.
func targetState(root, id string, artifact Artifact) (targetStatus, error) {
	current, err := digestOfTarget(root, artifact.Target)
	if err != nil {
		return targetNeedsWrite, err
	}
	staged, err := os.ReadFile(StagedPath(root, id, artifact.Target))
	if err != nil {
		return targetNeedsWrite, err
	}
	if current != "" && current == Digest(staged) {
		return targetAlreadyWritten, nil
	}
	if current != artifact.BaseDigest {
		return targetNeedsWrite, &StaleTargetError{Target: artifact.Target}
	}
	return targetNeedsWrite, nil
}

func writeTarget(root, id string, artifact Artifact) error {
	content, err := os.ReadFile(StagedPath(root, id, artifact.Target))
	if err != nil {
		return err
	}
	return repograph.WriteFileAtomically(filepath.Join(root, filepath.FromSlash(artifact.Target)), content)
}

func recordApplied(root, id string, result Result) error {
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("encode applied record: %w", err)
	}
	return repograph.WriteFileAtomically(filepath.Join(Dir(root, id), appliedFile), append(raw, '\n'))
}

// IsStale reports whether err is a StaleTargetError.
func IsStale(err error) bool {
	var stale *StaleTargetError
	return errors.As(err, &stale)
}
