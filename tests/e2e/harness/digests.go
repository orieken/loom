package harness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/orieken/loom/internal/factory"
	"github.com/orieken/loom/internal/orchestrator"
)

// checkDigests judges the run's integrity record while its worktree still
// exists: every completed stage's recorded digest must match its artifact,
// and the executor's own verification must catch an artifact changed behind
// its back. The second check is the control — a verifier that passes
// everything would pass the first.
func checkDigests(report *Report, order *factory.OrderState) {
	state, err := orchestrator.NewStateStore(factory.RunStatePath(order.Claim.Worktree, order.ID)).Load()
	if err == nil && state == nil {
		err = errors.New("no run state")
	}
	if err != nil {
		report.record("recorded digests match every artifact", err)
		return
	}
	report.record("recorded digests match every artifact", digestsMatch(state))
	report.record("verification catches a tampered artifact", tamperIsCaught(state))
}

func digestsMatch(state *orchestrator.RunState) error {
	checked := 0
	for _, stageID := range state.StagesInSequence() {
		record := state.Stages[stageID]
		if record.Status != orchestrator.StageStatusCompleted || record.ArtifactPath == "" {
			continue
		}
		if err := matches(record.ArtifactPath, record.ArtifactSHA256); err != nil {
			return fmt.Errorf("stage %s: %w", stageID, err)
		}
		checked++
	}
	if checked == 0 {
		return errors.New("no completed stage recorded an artifact")
	}
	return nil
}

func matches(path, recorded string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(content)
	if actual := hex.EncodeToString(sum[:]); actual != recorded {
		return fmt.Errorf("%s hashes to %s, recorded %s", path, actual, recorded)
	}
	return nil
}

// tamperIsCaught copies the run's state and the first artifact elsewhere,
// changes the copy, and asks the executor's verification about the copy —
// so the real run is never touched.
func tamperIsCaught(state *orchestrator.RunState) error {
	copied, stageID, err := stateWithTamperedArtifact(state)
	if err != nil {
		return err
	}
	defer os.RemoveAll(filepath.Dir(copied.Stages[stageID].ArtifactPath))
	for _, stale := range orchestrator.VerifyCompletedStages(copied) {
		if stale.StageID == stageID {
			return nil
		}
	}
	return fmt.Errorf("an edit to %s's artifact went unnoticed", stageID)
}

func stateWithTamperedArtifact(state *orchestrator.RunState) (*orchestrator.RunState, string, error) {
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, "", err
	}
	var copied orchestrator.RunState
	if err := json.Unmarshal(raw, &copied); err != nil {
		return nil, "", err
	}
	for _, stageID := range copied.StagesInSequence() {
		record := copied.Stages[stageID]
		if record.Status == orchestrator.StageStatusCompleted && record.ArtifactPath != "" {
			record.ArtifactPath, err = tamperedCopy(record.ArtifactPath)
			copied.Stages[stageID] = record
			return &copied, stageID, err
		}
	}
	return nil, "", errors.New("no artifact to tamper with")
}

func tamperedCopy(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp("", "loom-e2e-tamper-")
	if err != nil {
		return "", err
	}
	copied := filepath.Join(dir, filepath.Base(path))
	return copied, os.WriteFile(copied, append(content, []byte("\ntampered\n")...), 0o644)
}
