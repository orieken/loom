package state

// Keeping what a stage was rejected for (roadmap L2.26).
//
// A typed stage that returns an unusable payload fails loudly and is not
// retried — a silent repair would hide the modelling failures typed stages
// exist to surface. But until this existed, failing loudly also meant
// failing un-diagnosably: the payload was decoded, rejected, and dropped.
// The second real end-to-end run lost four stage attempts that way at
// $0.64-0.74 each, and diagnosing one of them needed the prompt rebuilt by
// hand and the CLI re-invoked outside the executor, because there was no
// other way to see what the agent had said.
//
// The evidence is destroyed at the moment of detection, which is the one
// moment it is worth the most. These two functions keep it instead.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Rejected artefacts sit beside the stage document they failed to become,
// under suffixes no reader of stage documents matches on.
const (
	rejectedStateSuffix    = ".rejected.json"
	rejectedResponseSuffix = ".rejected.txt"
)

// IsRejectedArtifact reports whether a filename in the state directory is
// kept evidence rather than a stage document. Callers that iterate the
// directory expecting decodable documents use it to skip these: a rejected
// payload is by definition one that did not decode.
func IsRejectedArtifact(name string) bool {
	return strings.HasSuffix(name, rejectedStateSuffix) || strings.HasSuffix(name, rejectedResponseSuffix)
}

// KeepRejectedState writes the payload a typed stage was rejected for and
// returns where it landed, so the error can name it.
func KeepRejectedState(workspaceDir, stageID string, payload []byte) (string, error) {
	return keepRejected(workspaceDir, stageID+rejectedStateSuffix, payload)
}

// KeepRejectedResponse writes an agent response that was not a state
// document at all — prose, a refusal, a truncated answer — and returns
// where it landed. Kept whole: the useful part of a response that failed to
// parse is as often at the end as at the start.
func KeepRejectedResponse(workspaceDir, stageID string, response []byte) (string, error) {
	return keepRejected(workspaceDir, stageID+rejectedResponseSuffix, response)
}

func keepRejected(workspaceDir, name string, data []byte) (string, error) {
	path := filepath.Join(workspaceDir, TypedStateDir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create state directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

// ClearRejected removes any evidence kept for a stage that has since
// succeeded. Without it a stage that failed, was fixed and re-ran would
// leave a rejected payload on disk describing a failure that no longer
// happened — evidence of the wrong run, which is worse than none.
func ClearRejected(workspaceDir, stageID string) {
	for _, suffix := range []string{rejectedStateSuffix, rejectedResponseSuffix} {
		_ = os.Remove(filepath.Join(workspaceDir, TypedStateDir, stageID+suffix))
	}
}
