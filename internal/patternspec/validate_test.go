package patternspec

import (
	"path/filepath"
	"strings"
	"testing"
)

// Every invalid fixture produces the one issue its header declares.
// @issue F2 @ac every validation rule fails with the field path
func TestEachInvalidFixtureReportsTheIssueItDeclares(t *testing.T) {
	for _, path := range fixturePaths(t, "invalid") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			expectation := expectationOf(t, path)

			issues := parseAndValidate(readFixture(t, path))

			if !hasIssue(issues, expectation) {
				t.Errorf("want %s %s: ...%s...\ngot %v", expectation.severity, expectation.field, expectation.message, issues)
			}
		})
	}
}

// A fixture that only warns still has no failure, so health passes it.
func TestAWarningFixtureHasNoFailure(t *testing.T) {
	issues := parseAndValidate(readFixture(t, filepath.Join("testdata", "invalid", "unused-input"+FileSuffix)))

	if failures := failureCount(issues); failures != 0 {
		t.Errorf("an unused input gave %d failure(s): %v", failures, issues)
	}
}

// @issue F2 @ac both examples validate
func TestEachValidFixtureHasNoIssue(t *testing.T) {
	for _, path := range fixturePaths(t, "valid") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			issues := parseAndValidate(readFixture(t, path))

			if len(issues) != 0 {
				t.Errorf("a valid fixture gave issues: %v", issues)
			}
		})
	}
}

func parsedFullSpec(t *testing.T) Spec {
	t.Helper()
	spec, issues := Parse(readFixture(t, filepath.Join("testdata", "valid", "publish-release-notes"+FileSuffix)))
	if len(issues) != 0 {
		t.Fatalf("issues: %v", issues)
	}
	return spec
}

func TestParseReadsTheListsOfAFullSpec(t *testing.T) {
	spec := parsedFullSpec(t)

	if spec.Name != "publish-release-notes" || len(spec.Inputs) != 6 || len(spec.Steps) != 3 || len(spec.Verify) != 2 {
		t.Errorf("spec fields not read: %+v", spec)
	}
}

func TestParseReadsAHumanStepAndItsReason(t *testing.T) {
	step := parsedFullSpec(t).Steps[2]

	if !step.IsHuman() || step.Reason != ReasonApproval {
		t.Errorf("human step = %+v", step)
	}
}

func TestParseReadsEvidenceDatesAsPlainDates(t *testing.T) {
	evidence := parsedFullSpec(t).Provenance.Evidence

	if evidence == nil || evidence.FirstSeen != "2026-08-01" || evidence.LastSeen != "2026-10-01" {
		t.Errorf("evidence dates not read as YYYY-MM-DD: %+v", evidence)
	}
}

// Validation runs the same way every time: same issues, same order.
func TestValidationIsDeterministic(t *testing.T) {
	source := []byte(strings.Join([]string{
		"schemaVersion: 1", "name: Bad_Name", "version: 1", "intent: x", "owner: o", "colour: red", "shade: dark",
		"triggers: {keywords: [], intentPatterns: []}", "steps: [{id: a, description: d}]",
		"provenance: {source: hand-written, status: candidate}",
	}, "\n"))
	first := parseAndValidate(source)

	for range 20 {
		if again := parseAndValidate(source); !equalIssues(first, again) {
			t.Fatalf("issue order changed:\n%v\n%v", first, again)
		}
	}
	if len(first) < 4 {
		t.Errorf("want every leaf reported, got %v", first)
	}
}

func equalIssues(left, right []Issue) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// No message repeats free text from a spec: a canary in every free-text
// field, and in the values that fail a pattern, never reaches an issue.
// @issue F2 @ac messages name fields, never values
func TestNoMessageRepeatsFreeTextFromTheSpec(t *testing.T) {
	const canary = "CANARY-7f3a"
	source := []byte(strings.Join([]string{
		"schemaVersion: 1", "name: " + canary, "version: " + canary, "intent: " + canary, "owner: " + canary,
		"triggers: {keywords: [" + canary + "], intentPatterns: []}",
		"inputs: [{name: " + canary + ", type: " + canary + ", description: " + canary + "}]",
		"steps: [{id: " + canary + ", description: " + canary + ", executor: " + canary + ", consumes: [" + canary + "]}]",
		"provenance: {source: " + canary + ", status: " + canary + "}",
	}, "\n"))

	issues := parseAndValidate(source)

	if len(issues) == 0 {
		t.Fatal("the canary spec should be invalid")
	}
	for _, issue := range issues {
		if strings.Contains(issue.String(), canary) {
			t.Errorf("issue repeats spec text: %s", issue)
		}
	}
}

func TestAMissingResolverResolvesNothing(t *testing.T) {
	spec := Spec{Verify: []VerifyReference{{Kind: VerifySkill, Name: "review-pr"}}}

	issues := verifyReferences(spec, nil)

	if len(issues) != 1 {
		t.Errorf("want the reference reported without a resolver, got %v", issues)
	}
}

func TestATimestampWithATimeOfDayKeepsIt(t *testing.T) {
	source := readFixture(t, filepath.Join("testdata", "valid", "publish-release-notes"+FileSuffix))
	source = []byte(strings.Replace(string(source), "lastSeen: 2026-10-01", "lastSeen: 2026-10-01T10:00:00Z", 1))

	issues := parseAndValidate(source)

	if !hasIssue(issues, fixtureExpectation{severity: SeverityFailure, field: "provenance.evidence.lastSeen", message: "must be a date"}) {
		t.Errorf("a date-time is not a date: %v", issues)
	}
}

func TestAValueWithNoJSONFormFails(t *testing.T) {
	source := []byte(specText("do-thing", StatusCandidate) + "\ninputs: [{name: limit, type: number, description: d, default: .inf}]")

	issues := parseAndValidate(source)

	if !hasIssue(issues, fixtureExpectation{severity: SeverityFailure, field: rootField, message: "has no JSON form"}) {
		t.Errorf("want a JSON-form failure, got %v", issues)
	}
}

func TestANonStringKeyInsideAListFails(t *testing.T) {
	source := []byte(strings.Replace(specText("do-thing", StatusCandidate),
		"steps: [{id: run, description: Run it., executor: tool}]", "steps: [{id: run, description: d, executor: tool, 3: x}]", 1))

	issues := parseAndValidate(source)

	if !hasIssue(issues, fixtureExpectation{severity: SeverityFailure, field: rootField, message: "MUST be a string"}) {
		t.Errorf("want a key failure, got %v", issues)
	}
}
