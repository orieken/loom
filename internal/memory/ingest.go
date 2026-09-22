package memory

// Ingesting one run's records into the store.
//
// Idempotent by construction: a run's rows are deleted and rewritten inside
// a transaction, so ingesting twice leaves the same counts. That matters
// more than it sounds — a retrospective that double-counts a review loop's
// rounds is worse than one with no data, because it looks like an answer.

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/orieken/loom/internal/orchestrator"
)

// Run is one ingested run's summary, returned so a caller can report what
// it stored without querying for it.
type Run struct {
	ID       string
	Feature  string
	Stages   int
	Events   int
	Complete bool
}

// Ingest reads a run's state and timeline and writes them to the store,
// replacing anything already recorded for that run.
func (s *Store) Ingest(state *orchestrator.RunState, events []orchestrator.Event) (Run, error) {
	if state == nil {
		return Run{}, fmt.Errorf("no run state to ingest")
	}
	run := Run{
		ID: RunID(state.FeatureName, state.StartedAt), Feature: state.FeatureName,
		Stages: len(state.Stages), Events: len(events), Complete: isComplete(events),
	}
	transaction, err := s.db.Begin()
	if err != nil {
		return Run{}, fmt.Errorf("begin ingest: %w", err)
	}
	if err := writeRun(transaction, run, state, events); err != nil {
		_ = transaction.Rollback()
		return Run{}, err
	}
	if err := transaction.Commit(); err != nil {
		return Run{}, fmt.Errorf("commit ingest: %w", err)
	}
	return run, nil
}

// isComplete reads the timeline rather than the stage statuses: a run that
// halted at a gate has completed stages but is not a completed run, and the
// distinction is the one anyone analysing history cares about.
func isComplete(events []orchestrator.Event) bool {
	for _, event := range events {
		if event.Kind == orchestrator.EventRunCompleted {
			return true
		}
	}
	return false
}

func writeRun(tx *sql.Tx, run Run, state *orchestrator.RunState, events []orchestrator.Event) error {
	if err := clearRun(tx, run.ID); err != nil {
		return err
	}
	if err := insertRun(tx, run, state); err != nil {
		return err
	}
	if err := insertStages(tx, run.ID, state); err != nil {
		return err
	}
	if err := insertEvents(tx, run.ID, events); err != nil {
		return err
	}
	if err := insertCorrections(tx, run.ID, state); err != nil {
		return err
	}
	if err := insertPolicyDecisions(tx, run.ID, state); err != nil {
		return err
	}
	return insertGateApprovals(tx, run.ID, state)
}

// clearRun removes a previous ingest. The child tables cascade, which is
// why foreign keys are enabled on open.
func clearRun(tx *sql.Tx, runID string) error {
	if _, err := tx.Exec(`DELETE FROM runs WHERE run_id = ?`, runID); err != nil {
		return fmt.Errorf("clear previous ingest: %w", err)
	}
	return nil
}

func insertRun(tx *sql.Tx, run Run, state *orchestrator.RunState) error {
	total := state.TotalUsage()
	_, err := tx.Exec(`INSERT INTO runs
		(run_id, feature, plan, created_by, spec_path, started_at, updated_at, completed,
		 waiting_gate, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
		 cost_usd, ingested_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, state.FeatureName, state.PlanName, string(state.CreatedBy), state.SpecPath,
		timestamp(state.StartedAt), timestamp(state.UpdatedAt), boolean(run.Complete),
		state.WaitingGate(), total.InputTokens, total.OutputTokens,
		total.CacheReadTokens, total.CacheCreationTokens, total.CostUSD,
		timestamp(time.Now().UTC()))
	if err != nil {
		return fmt.Errorf("insert run: %w", err)
	}
	return nil
}

func insertStages(tx *sql.Tx, runID string, state *orchestrator.RunState) error {
	for _, stageID := range state.StagesInSequence() {
		if err := insertStage(tx, runID, stageID, state.Stages[stageID]); err != nil {
			return err
		}
	}
	return nil
}

func insertStage(tx *sql.Tx, runID, stageID string, record orchestrator.StageRecord) error {
	usage := record.Usage
	if usage == nil {
		usage = &orchestrator.Usage{}
	}
	_, err := tx.Exec(`INSERT INTO stages
		(run_id, stage_id, agent, status, sequence, iterations, gate, skip_reason,
		 started_at, finished_at, duration_ms, input_tokens, output_tokens,
		 cache_read_tokens, cache_creation_tokens, cost_usd)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID, stageID, record.Agent, string(record.Status), record.Sequence, record.Iteration,
		record.Gate, record.SkipReason, timestamp(record.StartedAt), finishedAt(record),
		durationMillis(record), usage.InputTokens, usage.OutputTokens,
		usage.CacheReadTokens, usage.CacheCreationTokens, usage.CostUSD)
	if err != nil {
		return fmt.Errorf("insert stage %q: %w", stageID, err)
	}
	return nil
}

func insertEvents(tx *sql.Tx, runID string, events []orchestrator.Event) error {
	for index, event := range events {
		_, err := tx.Exec(`INSERT INTO events (run_id, seq, at, kind, stage_id, gate, agent, detail)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			runID, index, timestamp(event.At), string(event.Kind), event.Stage,
			event.Gate, event.Agent, eventDetail(event))
		if err != nil {
			return fmt.Errorf("insert event %d: %w", index, err)
		}
	}
	return nil
}

func insertCorrections(tx *sql.Tx, runID string, state *orchestrator.RunState) error {
	for index, correction := range state.Corrections {
		_, err := tx.Exec(`INSERT INTO corrections
			(run_id, seq, stage_id, agent, gate, added, removed, diff_path, at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			runID, index, correction.Stage, correction.Agent, correction.Gate,
			correction.Stat.Added, correction.Stat.Removed, correction.DiffPath,
			timestamp(correction.At))
		if err != nil {
			return fmt.Errorf("insert correction %d: %w", index, err)
		}
	}
	return nil
}

func insertPolicyDecisions(tx *sql.Tx, runID string, state *orchestrator.RunState) error {
	for index, decision := range state.PolicyDecisions {
		_, err := tx.Exec(`INSERT INTO policy_decisions (run_id, seq, gate, effect, honoured, conflict, at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			runID, index, decision.Gate, decision.Effect, boolean(decision.Honoured),
			joined(decision.Conflict), timestamp(decision.At))
		if err != nil {
			return fmt.Errorf("insert policy decision %d: %w", index, err)
		}
		if err := insertPolicyOutcomes(tx, runID, index, decision); err != nil {
			return err
		}
	}
	return nil
}

// insertPolicyOutcomes keeps each policy's own result, including the fields
// it could not see. The decision's effect alone cannot distinguish a policy
// that looked and did not match from one that was blind (roadmap L2.19).
func insertPolicyOutcomes(tx *sql.Tx, runID string, seq int, decision orchestrator.PolicyRecord) error {
	for index, outcome := range decision.Policies {
		_, err := tx.Exec(`INSERT INTO policy_outcomes (run_id, seq, idx, name, action, outcome, missing, source)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			runID, seq, index, outcome.Name, outcome.Action, outcome.Outcome,
			joined(outcome.Missing), outcome.Source)
		if err != nil {
			return fmt.Errorf("insert policy outcome %d of decision %d: %w", index, seq, err)
		}
	}
	return nil
}

// insertGateApprovals records what the human did at each gate — the half of
// the comparison L2.19 is waiting on that the store did not hold.
func insertGateApprovals(tx *sql.Tx, runID string, state *orchestrator.RunState) error {
	for gate, approval := range state.Approvals {
		_, err := tx.Exec(`INSERT INTO gate_approvals (run_id, gate, method, approver, approved_at, invalidated)
			VALUES (?, ?, ?, ?, ?, ?)`,
			runID, gate, string(approval.Method), approval.Approver,
			timestamp(approval.ApprovedAt), boolean(!approval.IsValid()))
		if err != nil {
			return fmt.Errorf("insert approval for gate %q: %w", gate, err)
		}
	}
	return nil
}
