package orchestrator

// The budget governor (roadmap L4.3): a ceiling on what one run may spend,
// checked before every stage, so a stuck delivery stops at its limit rather
// than when a human notices.
//
// It reads the same numbers `loom state show` prints — the run's accumulated
// Spend and each stage's recorded times — so it needs no metrics pipeline.
// One property is inherited from the policy evaluator rather than
// re-decided: usage the governor cannot see is unknown, not zero. A stage
// that finished and reported no cost makes a dollar or token ceiling
// unverifiable, and an unverifiable ceiling halts the run — until a human
// accepts that stage's unknown spend (unmeasured.go).

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ErrBudgetExhausted marks a run halted by its budget. It is a terminal
// state, not a failure: the run checkpointed cleanly before the next stage,
// and a resume with a larger budget continues from there.
var ErrBudgetExhausted = errors.New("run budget exhausted")

// budgetWarnFraction is where the governor warns before it halts.
const budgetWarnFraction = 0.8

// The ceilings a budget can set.
const (
	BudgetLimitUSD    = "usd"
	BudgetLimitTokens = "tokens"
	BudgetLimitHours  = "hours"
)

// Budget is the ceiling on one run. A zero field sets no ceiling.
type Budget struct {
	MaxUSD    float64 `json:"maxUsd,omitempty"`
	MaxTokens int64   `json:"maxTokens,omitempty"`
	// MaxHours bounds the time stages spent running, not the run's age: a
	// run waiting three days on a human has not spent three days of work.
	MaxHours float64 `json:"maxHours,omitempty"`
}

// IsZero reports whether the budget sets no ceiling at all.
func (b Budget) IsZero() bool {
	return b.MaxUSD <= 0 && b.MaxTokens <= 0 && b.MaxHours <= 0
}

// BudgetHalt records why and where a budget stopped a run.
type BudgetHalt struct {
	Limit    string  `json:"limit"`
	Allowed  float64 `json:"allowed"`
	Consumed float64 `json:"consumed"`
	// Unmeasured lists the stages whose cost was never reported, when that
	// is the reason the ceiling could not be verified.
	Unmeasured []string `json:"unmeasured,omitempty"`
	// BeforeStage is the stage the halt kept from starting.
	BeforeStage string    `json:"beforeStage"`
	At          time.Time `json:"at"`
}

// BudgetExhaustedError carries the halt and the per-stage spend behind it.
type BudgetExhaustedError struct {
	Halt     BudgetHalt
	PerStage []StageSpend
}

// StageSpend is what one stage consumed, for the report a halt prints.
type StageSpend struct {
	Stage    string
	CostUSD  float64
	Tokens   int64
	Measured bool
}

func (e *BudgetExhaustedError) Error() string {
	halt := e.Halt
	if len(halt.Unmeasured) > 0 {
		return fmt.Sprintf("%v before stage %q: the %s ceiling cannot be verified — %s reported no usage",
			ErrBudgetExhausted, halt.BeforeStage, halt.Limit, strings.Join(halt.Unmeasured, ", "))
	}
	return fmt.Sprintf("%v before stage %q: spent %s of the %s %s ceiling",
		ErrBudgetExhausted, halt.BeforeStage, formatBudgetAmount(halt.Limit, halt.Consumed),
		formatBudgetAmount(halt.Limit, halt.Allowed), halt.Limit)
}

// Unwrap lets callers match the halt with errors.Is(err, ErrBudgetExhausted).
func (e *BudgetExhaustedError) Unwrap() error { return ErrBudgetExhausted }

func formatBudgetAmount(limit string, amount float64) string {
	switch limit {
	case BudgetLimitUSD:
		return fmt.Sprintf("$%.4f", amount)
	case BudgetLimitHours:
		return fmt.Sprintf("%.4fh", amount)
	}
	return fmt.Sprintf("%.0f", amount)
}

// BudgetWarning is the soft limit: a ceiling is close.
type BudgetWarning struct {
	Limit    string
	Allowed  float64
	Consumed float64
}

// WithBudget sets this invocation's budget. On a fresh run it is recorded;
// on a resume it replaces the recorded one, which is how a human raises the
// ceiling on a run the budget halted. Unset, a resume keeps what was
// recorded — a budget belongs to the run, not to one invocation.
func (e *Executor) WithBudget(budget Budget) *Executor {
	e.budget = &budget
	return e
}

// OnBudgetWarning reports a ceiling crossing its warning threshold.
func (e *Executor) OnBudgetWarning(report func(BudgetWarning)) { e.onBudgetWarning = report }

// adoptBudget settles which budget this invocation enforces, and clears a
// previous halt: whether the run is still over budget is re-decided now.
func (e *Executor) adoptBudget(state *RunState) {
	if e.budget != nil {
		if e.budget.IsZero() {
			state.Budget = nil
		} else {
			budget := *e.budget
			state.Budget = &budget
		}
	}
	state.BudgetHalt = nil
}

// checkBudget halts the run before a stage when any ceiling is reached or
// cannot be verified, persisting the halt so state says why the run stopped.
func (e *Executor) checkBudget(plan Plan, stage Stage, state *RunState) error {
	if state.Budget == nil || stage.Internal {
		return nil
	}
	halt, warnings := evaluateBudget(*state.Budget, plan, state)
	e.reportBudgetWarnings(warnings)
	if halt == nil {
		return nil
	}
	halt.BeforeStage, halt.At = stage.ID, time.Now().UTC()
	state.BudgetHalt = halt
	if err := e.store.Save(state); err != nil {
		return fmt.Errorf("persist budget halt: %w", err)
	}
	return &BudgetExhaustedError{Halt: *halt, PerStage: SpendByStage(plan, state)}
}

func (e *Executor) reportBudgetWarnings(warnings []BudgetWarning) {
	if e.onBudgetWarning == nil {
		return
	}
	for _, warning := range warnings {
		e.onBudgetWarning(warning)
	}
}

type budgetMeasure struct {
	limit    string
	allowed  float64
	consumed float64
	// unmeasured is set when the consumed figure is known to be incomplete.
	unmeasured []string
}

// evaluateBudget checks every ceiling the budget sets, returning the first
// one reached and a warning for each one near its limit.
func evaluateBudget(budget Budget, plan Plan, state *RunState) (*BudgetHalt, []BudgetWarning) {
	var warnings []BudgetWarning
	for _, measure := range budgetMeasures(budget, plan, state) {
		if halt := measure.halt(); halt != nil {
			return halt, warnings
		}
		if measure.consumed >= measure.allowed*budgetWarnFraction {
			warnings = append(warnings, BudgetWarning{Limit: measure.limit, Allowed: measure.allowed, Consumed: measure.consumed})
		}
	}
	return nil, warnings
}

func (m budgetMeasure) halt() *BudgetHalt {
	if len(m.unmeasured) == 0 && m.consumed < m.allowed {
		return nil
	}
	return &BudgetHalt{Limit: m.limit, Allowed: m.allowed, Consumed: m.consumed, Unmeasured: m.unmeasured}
}

func budgetMeasures(budget Budget, plan Plan, state *RunState) []budgetMeasure {
	var measures []budgetMeasure
	total := state.TotalUsage()
	unmeasured := unacceptedUnmeasured(plan, state)
	if budget.MaxUSD > 0 {
		measures = append(measures, budgetMeasure{BudgetLimitUSD, budget.MaxUSD, total.CostUSD, unmeasured})
	}
	if budget.MaxTokens > 0 {
		measures = append(measures, budgetMeasure{BudgetLimitTokens, float64(budget.MaxTokens),
			float64(usageTokens(total)), unmeasured})
	}
	if budget.MaxHours > 0 {
		measures = append(measures, budgetMeasure{BudgetLimitHours, budget.MaxHours, activeHours(state), nil})
	}
	return measures
}

func usageTokens(usage Usage) int64 {
	return usage.InputTokens + usage.OutputTokens + usage.CacheReadTokens + usage.CacheCreationTokens
}

// UnmeasuredStages lists provider stages that finished — completed or
// failed — without reporting usage. An interrupted stage is not listed: it
// was cancelled before the provider could report, and counting it would
// halt every budgeted run that a human paused.
func UnmeasuredStages(plan Plan, state *RunState) []string {
	var stages []string
	for _, stage := range plan.Stages {
		if record, ok := state.Stages[stage.ID]; ok && isUnmeasured(stage, record) {
			stages = append(stages, stage.ID)
		}
	}
	sort.Strings(stages)
	return stages
}

func isUnmeasured(stage Stage, record StageRecord) bool {
	finished := record.Status == StageStatusCompleted || record.Status == StageStatusFailed
	return finished && !stage.Internal && record.Usage == nil
}

// activeHours sums the time stages spent running. A stage still recorded as
// running counts from its start: that is time already spent.
//
// Only a record whose current status is work it did counts (ranStatuses).
// A stale stage counts nothing. Its work was discarded — the spec or its
// artifact changed — and its record cannot say how long that work took:
// demotion used to overwrite FinishedAt with the demotion time, so a stage
// that ran for two minutes yesterday counted twenty hours today. The third
// factory dogfood run's order hit a 1-hour ceiling at 101 hours that way,
// before its first stage. When the stage re-runs, the new attempt counts.
// A skipped stage counts nothing either: a stage the router skips after it
// once ran keeps that run's StartedAt and gets the skip time as FinishedAt,
// and the same order then read 21 hours against a 2-hour ceiling.
func activeHours(state *RunState) float64 {
	var active time.Duration
	for _, record := range state.Stages {
		active += stageDuration(record)
	}
	return active.Hours()
}

// ranStatuses are the statuses whose record times are a stage's own work.
var ranStatuses = map[StageStatus]bool{
	StageStatusCompleted: true, StageStatusFailed: true, StageStatusInterrupted: true, StageStatusRunning: true,
}

func stageDuration(record StageRecord) time.Duration {
	if record.StartedAt.IsZero() || !ranStatuses[record.Status] {
		return 0
	}
	if record.FinishedAt != nil {
		return record.FinishedAt.Sub(record.StartedAt)
	}
	if record.Status == StageStatusRunning {
		return time.Since(record.StartedAt)
	}
	return 0
}

// SpendByStage reports what each provider stage that ran consumed, in plan
// order, marking the ones that reported nothing.
func SpendByStage(plan Plan, state *RunState) []StageSpend {
	var spend []StageSpend
	for _, stage := range plan.Stages {
		record, ok := state.Stages[stage.ID]
		if !ok || stage.Internal || record.StartedAt.IsZero() {
			continue
		}
		entry := StageSpend{Stage: stage.ID, Measured: record.Usage != nil}
		if record.Usage != nil {
			entry.CostUSD, entry.Tokens = record.Usage.CostUSD, usageTokens(*record.Usage)
		}
		spend = append(spend, entry)
	}
	return spend
}
