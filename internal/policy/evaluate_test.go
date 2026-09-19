package policy_test

import (
	"testing"

	"github.com/orieken/loom/internal/policy"
)

func fullContext() policy.GateContext {
	verdict := "APPROVED"
	criticals := 0
	pass := true
	return policy.GateContext{
		Gate: policy.GateGitCommit, ReviewVerdict: &verdict, SecurityCriticals: &criticals,
		TestsPass: &pass, PathsKnown: true,
		ChangedPaths: []string{"docs/guide.md", "docs/api/reference.md"},
	}
}

// conditionFrom builds a require-human policy, not an auto-approve one.
// These tests are about how a condition RESOLVES, and auto-approve refuses
// self-reported fields at load (see validateSelfReportedFields) — which
// would make every case using codeReviewer.behaviorChange a parse failure
// rather than the evaluation it is testing.
func conditionFrom(t *testing.T, body string) policy.Condition {
	t.Helper()
	raw := "name: t\nversion: \"1.0\"\nmatcher:\n  gate: git-commit\ncondition:\n" + body +
		"action:\n  type: require-human\n  reason: r\n"
	parsed, err := policy.Parse("t.policy.yaml", []byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return parsed.Condition
}

func TestEvaluateChecks(t *testing.T) {
	cases := []struct {
		name string
		body string
		want policy.Outcome
	}{
		{"verdict matches", "  codeReviewer.verdict:\n    equals: \"APPROVED\"\n", policy.OutcomeTrue},
		{"verdict differs", "  codeReviewer.verdict:\n    equals: \"CHANGES_REQUESTED\"\n", policy.OutcomeFalse},
		{"criticals equal zero", "  securityReviewer.criticals:\n    equals: 0\n", policy.OutcomeTrue},
		{"criticals less than", "  securityReviewer.criticals:\n    lessThan: 1\n", policy.OutcomeTrue},
		{"tests pass", "  testsPass: true\n", policy.OutcomeTrue},
		{"all paths match", "  filePaths:\n    allMatch: \"docs/**\"\n", policy.OutcomeTrue},
		{"none match", "  filePaths:\n    noneMatch: \"**/security/**\"\n", policy.OutcomeTrue},
		{"any match finds none", "  filePaths:\n    anyMatch: \"**/auth/**\"\n", policy.OutcomeFalse},
		{"all match fails on one", "  filePaths:\n    allMatch: \"docs/api/**\"\n", policy.OutcomeFalse},
		// A sourceable field is still UNKNOWN when this run lacks the fact —
		// the producing stage has not run, or its document is missing. That
		// is different from a field nothing can ever answer, which is why
		// diffType, dryRunPass and fitnessFunction.allPass were removed
		// from the vocabulary rather than left resolving here (L2.20).
		{"diff size is unknown when absent", "  diffLines:\n    lessThan: 500\n", policy.OutcomeUnknown},
		{"behaviour change is unknown when absent", "  codeReviewer.behaviorChange: false\n", policy.OutcomeUnknown},
		// Composition. `codeReviewer.behaviorChange` stands in for an absent
		// boolean fact; fullContext() deliberately does not set it.
		{"AND of true and unknown is unknown", "  testsPass: true\n  codeReviewer.behaviorChange: true\n", policy.OutcomeUnknown},
		{"AND short-circuits on false", "  codeReviewer.verdict:\n    equals: \"NOPE\"\n  codeReviewer.behaviorChange: true\n", policy.OutcomeFalse},
		{"OR wins on one true", "  any:\n    - testsPass: true\n    - codeReviewer.behaviorChange: true\n", policy.OutcomeTrue},
		{"OR of false and unknown is unknown", "  any:\n    - testsPass: false\n    - codeReviewer.behaviorChange: true\n", policy.OutcomeUnknown},
		{"NOT inverts", "  not:\n    testsPass: false\n", policy.OutcomeTrue},
		{"NOT leaves unknown alone", "  not:\n    codeReviewer.behaviorChange: true\n", policy.OutcomeUnknown},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := policy.Evaluate(conditionFrom(t, testCase.body), fullContext())
			if got != testCase.want {
				t.Errorf("Evaluate = %q, want %q", got, testCase.want)
			}
		})
	}
}

// A condition that already fails cannot be rescued by information nobody
// has, so FALSE beats UNKNOWN in an AND. The reverse — treating unknown as
// false — would silently fail policies that should have matched.
func TestUnknownNeverSatisfiesAndIsNamed(t *testing.T) {
	condition := conditionFrom(t, "  testsPass: true\n  diffLines:\n    lessThan: 500\n")

	if got := policy.Evaluate(condition, fullContext()); got != policy.OutcomeUnknown {
		t.Errorf("Evaluate = %q, want UNKNOWN when a fact is missing", got)
	}
	missing := policy.MissingFacts(condition, fullContext())
	if len(missing) != 1 || missing[0] != policy.FieldDiffLines {
		t.Errorf("MissingFacts = %v, want exactly diffLines", missing)
	}
}

// "Changed no files" and "cannot see the implementation" are different
// facts: allMatch over an empty set is vacuously true, and returning that
// for a run nobody can see would be a confident wrong answer.
func TestUnknownPathsAreNotAnEmptyFileList(t *testing.T) {
	context := fullContext()
	context.PathsKnown = false
	context.ChangedPaths = nil

	got := policy.Evaluate(conditionFrom(t, "  filePaths:\n    allMatch: \"docs/**\"\n"), context)
	if got != policy.OutcomeUnknown {
		t.Errorf("Evaluate = %q, want UNKNOWN when no implementation state exists", got)
	}
}

func TestDoubleStarGlobs(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"**/security/**", "internal/security/auth.go", true},
		{"**/security/**", "security/auth.go", true},
		{"**/security/**", "internal/state/auth.go", false},
		{"docs/**", "docs/guide.md", true},
		{"docs/**", "src/docs/guide.md", false},
		{"**/*.md", "docs/guide.md", true},
	}
	for _, testCase := range cases {
		t.Run(testCase.pattern+" vs "+testCase.path, func(t *testing.T) {
			context := fullContext()
			context.ChangedPaths = []string{testCase.path}
			got := policy.Evaluate(conditionFrom(t,
				"  filePaths:\n    anyMatch: \""+testCase.pattern+"\"\n"), context)
			if (got == policy.OutcomeTrue) != testCase.want {
				t.Errorf("anyMatch %q against %q = %q, want match=%v",
					testCase.pattern, testCase.path, got, testCase.want)
			}
		})
	}
}
