package policy

// Evaluating a policy's condition against the facts of a run.
//
// The important type here is Outcome, and specifically that it has three
// values rather than two. A framework that only knows true and false has to
// decide what a fact it cannot see means, and every choice is wrong: false
// silently fails policies that should have matched, true silently approves
// on evidence nobody has. Unknown propagates, never satisfies a condition,
// and is recorded by name — so a decision explains itself.

import "sort"

// Outcome is the result of a check, a condition, or a whole policy.
type Outcome string

// The three outcomes.
const (
	OutcomeTrue    Outcome = "TRUE"
	OutcomeFalse   Outcome = "FALSE"
	OutcomeUnknown Outcome = "UNKNOWN"
)

// GateContext is what the executor knows at a gate. A nil pointer means
// the fact is unavailable — which is different from a zero value, and the
// distinction is the whole reason these are pointers.
type GateContext struct {
	Gate GateID
	// ReviewVerdict is the code-reviewer's typed verdict (roadmap L2.17).
	ReviewVerdict *string
	// SecurityCriticals counts CRITICAL findings in the security report.
	SecurityCriticals *int
	// TestsPass is true when the QA report records zero failures.
	TestsPass *bool
	// ReviewBehaviorChange is the reviewer's self-report on whether the
	// diff changes behaviour. Usable only by a policy that cannot open a
	// gate — see CheckFieldUsage.
	ReviewBehaviorChange *bool
	// DiffLines is lines added and removed since the run's starting
	// commit, counting files the run created. Nil when there is no work
	// tree, no starting commit, or git would not answer.
	DiffLines *int
	// ChangedPaths is every file the implementation created or modified.
	// Nil means no implementation state; empty means it changed nothing.
	ChangedPaths []string
	// PathsKnown separates "changed nothing" from "cannot see".
	PathsKnown bool
}

// Evaluate resolves a condition against a context.
func Evaluate(condition Condition, context GateContext) Outcome {
	outcomes := make([]Outcome, 0, len(condition.Checks)+len(condition.Any)+1)
	for _, check := range condition.Checks {
		outcomes = append(outcomes, evaluateCheck(check, context))
	}
	if len(condition.Any) > 0 {
		outcomes = append(outcomes, evaluateAny(condition.Any, context))
	}
	if condition.Not != nil {
		outcomes = append(outcomes, negate(Evaluate(*condition.Not, context)))
	}
	return conjunction(outcomes)
}

// conjunction is AND over three values: any FALSE makes the whole thing
// false regardless of what is unknown, because a condition that already
// fails cannot be rescued by information nobody has.
func conjunction(outcomes []Outcome) Outcome {
	result := OutcomeTrue
	for _, outcome := range outcomes {
		if outcome == OutcomeFalse {
			return OutcomeFalse
		}
		if outcome == OutcomeUnknown {
			result = OutcomeUnknown
		}
	}
	return result
}

// evaluateAny is OR over three values: any TRUE wins outright, since one
// satisfied branch is enough however little is known about the others.
func evaluateAny(branches []Condition, context GateContext) Outcome {
	result := OutcomeFalse
	for _, branch := range branches {
		switch Evaluate(branch, context) {
		case OutcomeTrue:
			return OutcomeTrue
		case OutcomeUnknown:
			result = OutcomeUnknown
		case OutcomeFalse:
		}
	}
	return result
}

// negate leaves UNKNOWN alone: not-knowing something is not knowing its
// opposite.
func negate(outcome Outcome) Outcome {
	switch outcome {
	case OutcomeTrue:
		return OutcomeFalse
	case OutcomeFalse:
		return OutcomeTrue
	default:
		return OutcomeUnknown
	}
}

// MissingFacts returns the fields a condition tests that the context
// cannot supply, sorted. A decision that names what it could not see is
// auditable; one that just says "no match" is not.
func MissingFacts(condition Condition, context GateContext) []Field {
	missing := map[Field]bool{}
	collectMissing(condition, context, missing)
	fields := make([]Field, 0, len(missing))
	for field := range missing {
		fields = append(fields, field)
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i] < fields[j] })
	return fields
}

func collectMissing(condition Condition, context GateContext, missing map[Field]bool) {
	for _, check := range condition.Checks {
		if evaluateCheck(check, context) == OutcomeUnknown {
			missing[check.Field] = true
		}
	}
	for _, branch := range condition.Any {
		collectMissing(branch, context, missing)
	}
	if condition.Not != nil {
		collectMissing(*condition.Not, context, missing)
	}
}
