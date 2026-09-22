package memory

// Per-agent metrics measured from execution (roadmap L3.13).
//
// `agent-scorecard` scored four metrics, all of them computed by a model
// reading persisted markdown — a fraction of security findings with a
// "Fix applied" line, a fraction of contract sections that are not template
// placeholders. Meanwhile the store held attempt counts, durations, statuses
// and human corrections, measured, and nothing queried them per agent.
//
// Everything here is counted from records. Nothing is inferred, and a
// quantity that was not measured is reported as absent rather than as zero —
// the distinction epics 84-87 kept meeting, and the one a scorecard is most
// likely to destroy by averaging.

import (
	"database/sql"
	"fmt"
	"sort"
)

// AgentMetrics is one agent's measured record across every ingested run.
type AgentMetrics struct {
	Agent  string `json:"agent"`
	Runs   int    `json:"runs"`
	Stages int    `json:"stages"`
	// Retried counts stage executions that took more than one attempt. For
	// a stage inside the review loop this is the reviewer sending it back.
	Retried int `json:"retried"`
	Failed  int `json:"failed"`
	// Corrections counts human edits to this agent's output. A correction
	// was RECORDED, not adopted (roadmap L4.5): it is evidence the output
	// needed fixing, not that anything shipped fixed.
	Corrections int `json:"corrections"`
	// DurationP50Ms and DurationP95Ms are nil when no stage of this agent
	// finished. A stage that never finished has no duration, and writing 0
	// would put it top of any "fastest agent" ordering.
	DurationP50Ms *int64  `json:"durationP50Ms,omitempty"`
	DurationP95Ms *int64  `json:"durationP95Ms,omitempty"`
	Tokens        int64   `json:"tokens"`
	CostUSD       float64 `json:"costUsd"`
	// CostReported distinguishes "cost nothing" from "reported nothing".
	// The mock provider reports nothing, so a corpus of mock runs shows 0
	// everywhere and means nothing by it.
	CostReported bool `json:"costReported"`
}

// RetryRate is the fraction of this agent's stages that took more than one
// attempt, or nil when it has run no stages.
func (a AgentMetrics) RetryRate() *float64 { return rate(a.Retried, a.Stages) }

// CorrectionRate is human corrections per stage execution, or nil when the
// agent has run no stages. It can exceed 1: one stage may be corrected more
// than once.
func (a AgentMetrics) CorrectionRate() *float64 { return rate(a.Corrections, a.Stages) }

// FailureRate is the fraction of this agent's stages that ended FAILED.
func (a AgentMetrics) FailureRate() *float64 { return rate(a.Failed, a.Stages) }

func rate(part, whole int) *float64 {
	if whole == 0 {
		return nil
	}
	value := float64(part) / float64(whole)
	return &value
}

// Agents returns one row per agent, busiest first.
func (s *Store) Agents() ([]AgentMetrics, error) {
	rows, err := s.db.Query(`
		SELECT COALESCE(s.agent, '(unattributed)'),
		       COUNT(DISTINCT s.run_id),
		       COUNT(*),
		       SUM(CASE WHEN s.iterations > 1 THEN 1 ELSE 0 END),
		       SUM(CASE WHEN s.status = 'FAILED' THEN 1 ELSE 0 END),
		       SUM(s.input_tokens + s.output_tokens + s.cache_read_tokens + s.cache_creation_tokens),
		       SUM(s.cost_usd),
		       (SELECT COUNT(*) FROM corrections c WHERE c.agent = s.agent)
		FROM stages s
		GROUP BY s.agent ORDER BY COUNT(*) DESC, s.agent ASC`)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	metrics, err := scanAgents(rows)
	if err != nil {
		return nil, err
	}
	return s.attachDurations(metrics)
}

func scanAgents(rows *sql.Rows) ([]AgentMetrics, error) {
	defer func() { _ = rows.Close() }()
	metrics := make([]AgentMetrics, 0)
	for rows.Next() {
		var row AgentMetrics
		if err := rows.Scan(&row.Agent, &row.Runs, &row.Stages, &row.Retried, &row.Failed,
			&row.Tokens, &row.CostUSD, &row.Corrections); err != nil {
			return nil, fmt.Errorf("scan agent metrics: %w", err)
		}
		row.CostReported = row.Tokens > 0 || row.CostUSD > 0
		metrics = append(metrics, row)
	}
	return metrics, rows.Err()
}

func (s *Store) attachDurations(metrics []AgentMetrics) ([]AgentMetrics, error) {
	for index := range metrics {
		durations, err := s.durationsFor(metrics[index].Agent)
		if err != nil {
			return nil, err
		}
		metrics[index].DurationP50Ms = percentile(durations, 50)
		metrics[index].DurationP95Ms = percentile(durations, 95)
	}
	return metrics, nil
}

// durationsFor reads the measured durations only. A NULL duration is a
// stage that never finished, and it is excluded rather than read as
// instantaneous.
func (s *Store) durationsFor(agent string) ([]int64, error) {
	rows, err := s.db.Query(`
		SELECT duration_ms FROM stages
		WHERE COALESCE(agent, '(unattributed)') = ? AND duration_ms IS NOT NULL
		ORDER BY duration_ms ASC`, agent)
	if err != nil {
		return nil, fmt.Errorf("query durations for %q: %w", agent, err)
	}
	defer func() { _ = rows.Close() }()
	durations := make([]int64, 0)
	for rows.Next() {
		var duration int64
		if err := rows.Scan(&duration); err != nil {
			return nil, fmt.Errorf("scan duration: %w", err)
		}
		durations = append(durations, duration)
	}
	return durations, rows.Err()
}

// percentile returns the nearest-rank percentile of an ascending slice, or
// nil when there is nothing to take one of.
//
// Nearest-rank rather than interpolated: with the handful of samples a real
// corpus holds today, an interpolated p95 invents a duration no stage ever
// took, and this number exists to be compared against stages that really
// ran.
func percentile(ascending []int64, nth int) *int64 {
	if len(ascending) == 0 {
		return nil
	}
	rank := (nth*len(ascending) + 99) / 100
	if rank < 1 {
		rank = 1
	}
	value := ascending[rank-1]
	return &value
}

// SortAgentsByName orders metrics alphabetically, for output that must be
// stable rather than ranked.
func SortAgentsByName(metrics []AgentMetrics) {
	sort.Slice(metrics, func(i, j int) bool { return metrics[i].Agent < metrics[j].Agent })
}
