package orchestrator

// Usage is what one model invocation consumed (roadmap L3.8). Every number
// here is REPORTED by the provider, never computed here — the cost field
// especially. A pricing table in this repository would be wrong within a
// quarter and would produce a confident figure nobody had been charged,
// which is the same class of defect as a duration recalled by a model.
//
// A nil *Usage means the provider reported nothing, which is different from
// a provider reporting zeros. The mock provider reports nothing.
type Usage struct {
	// Model is what the provider says actually served the request, which is
	// not necessarily what was asked for.
	Model string `json:"model,omitempty"`
	// InputTokens and OutputTokens are the billable token counts.
	InputTokens  int64 `json:"inputTokens,omitempty"`
	OutputTokens int64 `json:"outputTokens,omitempty"`
	// CacheReadTokens and CacheCreationTokens are prompt-cache traffic,
	// kept separate because they price differently from fresh input.
	CacheReadTokens     int64 `json:"cacheReadTokens,omitempty"`
	CacheCreationTokens int64 `json:"cacheCreationTokens,omitempty"`
	// CostUSD is what the provider says the invocation cost.
	CostUSD float64 `json:"costUsd,omitempty"`
	// FinishReason is why generation stopped, as the model reported it
	// ("end_turn", "max_tokens", …). It is the difference between a
	// completion that finished its thought and one cut off at a ceiling,
	// which are indistinguishable from the text alone.
	FinishReason string `json:"finishReason,omitempty"`
	// TerminalReason is how the provider's own process ended, which is a
	// different question: a truncated completion still terminates cleanly.
	TerminalReason string `json:"terminalReason,omitempty"`
}

// Add accumulates another invocation's usage. A nil addend changes nothing,
// so a run mixing reporting and non-reporting providers still totals what
// was actually reported rather than refusing to total at all.
func (u *Usage) Add(other *Usage) {
	if other == nil {
		return
	}
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.CacheReadTokens += other.CacheReadTokens
	u.CacheCreationTokens += other.CacheCreationTokens
	u.CostUSD += other.CostUSD
}

// TotalUsage reports what the run cost.
//
// It prefers the run-level accumulator, which no longer depends on the
// stage records surviving (roadmap L3.22, second occurrence). A state
// written before that field existed has no accumulator, so the derived sum
// stays as the fallback — it is right whenever no record has been removed,
// which is every state this build did not write.
func (s *RunState) TotalUsage() Usage {
	if s.Spend != nil {
		return *s.Spend
	}
	return s.derivedUsage()
}

// derivedUsage sums every stage's reported usage. Stages that reported
// nothing contribute nothing; the total is of what was measured, and the
// absence of a number is not treated as a zero.
func (s *RunState) derivedUsage() Usage {
	var total Usage
	for _, record := range s.Stages {
		total.Add(record.Usage)
	}
	return total
}

// recordSpend adds one provider call to the run's running total. Called
// wherever a stage record's usage is updated, and deliberately additive:
// a stage that fails, is retried, or is later deleted still cost what it
// cost.
func (s *RunState) recordSpend(attempt *Usage) {
	if attempt == nil {
		return
	}
	if s.Spend == nil {
		s.Spend = &Usage{Model: attempt.Model}
	}
	s.Spend.Add(attempt)
	s.Spend.Model = attempt.Model
}

// accumulateUsage folds one attempt's usage into whatever a stage record
// already carries (roadmap L3.22).
//
// It replaced a plain assignment, which silently discarded every attempt but
// the last. In the third real end-to-end run qa-engineer failed to parse,
// billing $0.69, and the successful retry then overwrote that record with
// its own $0.74 — so the run reported $8.7978 against a true $9.4923, a
// difference of exactly the discarded attempt. The error was always in the
// same direction and grew with how badly a run went, which is when the
// number is most likely to be read.
//
// A fresh value is returned rather than the existing one mutated: records
// are copied out of the state map by value while the Usage pointer is
// shared, so mutating in place would edit state nobody asked to edit.
func accumulateUsage(existing, attempt *Usage) *Usage {
	if attempt == nil {
		return existing
	}
	total := Usage{Model: attempt.Model}
	total.Add(existing)
	total.Add(attempt)
	return &total
}
