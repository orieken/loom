package orchestrator

import "time"

// A stage that finished without reporting usage makes a dollar or token
// ceiling unverifiable, and the governor halts on it (budget.go). That is
// right, and it was also a trap: the check runs before every stage, the
// unmeasured stage stays unmeasured, so a resume halted again on the same
// fact forever. The second factory dogfood run parked an order this way when
// a session limit cut its developer off mid-stage — the spend was genuinely
// unknown, and nothing short of discarding the approved design let it go on.
//
// A human may accept the unknown. The acceptance is recorded with who gave
// it and when, the stage stays unmeasured in every report, and the ceiling
// is then checked against what was measured. Nothing reads the unknown as
// zero.

// UnmeasuredAcceptance records a human accepting one stage's unknown spend.
type UnmeasuredAcceptance struct {
	// FinishedAt binds the acceptance to the attempt that went unmeasured.
	// A later attempt of the same stage that also reports nothing is a new
	// unknown, and this acceptance does not cover it.
	FinishedAt time.Time `json:"finishedAt"`
	AcceptedBy string    `json:"acceptedBy"`
	AcceptedAt time.Time `json:"acceptedAt"`
}

// WithUnmeasuredAcceptedBy has a resume accept, on the named person's
// behalf, the spend of every stage that is unmeasured when it starts.
func (e *Executor) WithUnmeasuredAcceptedBy(name string) *Executor {
	e.acceptUnmeasuredBy = name
	return e
}

func (e *Executor) acceptUnmeasured(plan Plan, state *RunState) {
	if e.acceptUnmeasuredBy == "" {
		return
	}
	for _, stageID := range unacceptedUnmeasured(plan, state) {
		if state.UnmeasuredAccepted == nil {
			state.UnmeasuredAccepted = map[string]UnmeasuredAcceptance{}
		}
		state.UnmeasuredAccepted[stageID] = UnmeasuredAcceptance{
			FinishedAt: finishedAt(state.Stages[stageID]), AcceptedBy: e.acceptUnmeasuredBy, AcceptedAt: time.Now().UTC(),
		}
	}
}

// unacceptedUnmeasured is what still makes a ceiling unverifiable.
func unacceptedUnmeasured(plan Plan, state *RunState) []string {
	var open []string
	for _, stageID := range UnmeasuredStages(plan, state) {
		if !isUnmeasuredAccepted(state, stageID) {
			open = append(open, stageID)
		}
	}
	return open
}

func isUnmeasuredAccepted(state *RunState, stageID string) bool {
	acceptance, ok := state.UnmeasuredAccepted[stageID]
	return ok && acceptance.FinishedAt.Equal(finishedAt(state.Stages[stageID]))
}

func finishedAt(record StageRecord) time.Time {
	if record.FinishedAt == nil {
		return time.Time{}
	}
	return *record.FinishedAt
}
