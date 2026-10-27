package orchestrator

import (
	"testing"
	"time"
)

// The third dogfood run: an edited order demoted five stages that ran for
// minutes the day before, and the 1-hour ceiling read 101 hours.
func TestAStaleStageChargesNoHours(t *testing.T) {
	started := time.Now().UTC().Add(-20 * time.Hour)
	demotedAt := time.Now().UTC()
	state := NewRunState("deliver-feature", CreatedByExecutor)
	state.Stages["analyst"] = StageRecord{Status: StageStatusStale, StartedAt: started, FinishedAt: &demotedAt}
	ran := started.Add(30 * time.Minute)
	state.Stages["developer"] = StageRecord{Status: StageStatusFailed, StartedAt: started, FinishedAt: &ran}

	if got := activeHours(state); got != 0.5 {
		t.Errorf("active hours = %v, want only the developer's half hour", got)
	}
}

func TestDemotionKeepsWhenTheStageFinished(t *testing.T) {
	finished := time.Date(2026, 10, 26, 5, 11, 0, 0, time.UTC)
	state := NewRunState("deliver-feature", CreatedByExecutor)
	state.Stages["analyst"] = StageRecord{Status: StageStatusCompleted, FinishedAt: &finished}

	markStale(state, "analyst", StaleReasonSpecChanged)

	record := state.Stages["analyst"]
	if record.Status != StageStatusStale || !record.FinishedAt.Equal(finished) {
		t.Errorf("record = %s finished %v, want stale and still finished at %v", record.Status, record.FinishedAt, finished)
	}
}
