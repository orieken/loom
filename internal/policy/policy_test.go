package policy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/policy"
)

// examplesDir is the shipped example set. Loading it in the test suite is
// the check that the documentation and the parser agree — the previous
// arrangement had neither, and one of those examples did not parse.
const examplesDir = "../../shared/policies/examples"

func TestShippedExamplesAllLoad(t *testing.T) {
	policies, err := policy.Load(examplesDir)
	if err != nil {
		t.Fatalf("shipped examples do not load: %v", err)
	}
	if len(policies) < 4 {
		t.Fatalf("loaded %d policies, expected every example in %s", len(policies), examplesDir)
	}
	for _, loaded := range policies {
		if loaded.Name == "" || loaded.Action.Reason == "" {
			t.Errorf("example %s loaded without a name or reason: %+v", loaded.Source, loaded)
		}
	}
}

// The example approval-gates.md cites as its reference policy for the
// git-commit gate had a duplicate `filePaths` key and had never been
// parsed by anything. Half of what it claimed to check could not have been
// checked.
func TestTheReferenceRefactorPolicyParses(t *testing.T) {
	policies, err := policy.Load(examplesDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	found := false
	for _, loaded := range policies {
		if loaded.Name != "auto-approve-refactor" {
			continue
		}
		found = true
		assertChecksBothPaths(t, loaded)
	}
	if !found {
		t.Fatal("auto-approve-refactor did not load")
	}
}

// Both excluded paths must survive the fix — the point of repairing the
// duplicate key was to make the policy check what it always claimed to.
func assertChecksBothPaths(t *testing.T, loaded policy.Policy) {
	t.Helper()
	globs := collectGlobs(loaded.Condition)
	for _, want := range []string{"**/security/**", "**/auth/**"} {
		if !contains(globs, want) {
			t.Errorf("auto-approve-refactor no longer excludes %q; globs = %v", want, globs)
		}
	}
}

func collectGlobs(condition policy.Condition) []string {
	globs := make([]string, 0, 4)
	for _, check := range condition.Checks {
		if check.Field == policy.FieldFilePaths {
			globs = append(globs, check.Text)
		}
	}
	for _, branch := range condition.Any {
		globs = append(globs, collectGlobs(branch)...)
	}
	if condition.Not != nil {
		globs = append(globs, collectGlobs(*condition.Not)...)
	}
	return globs
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// A policy targeting an always-human gate fails at LOAD, not at
// evaluation. Someone who writes it must find out before a run, not never.
func TestAlwaysHumanGatesAreRejectedAtLoad(t *testing.T) {
	for _, gate := range []string{"deploy", "db-migration", "db-contract-phase", "external-api", "ship-to-friday", "test-removal"} {
		t.Run(gate, func(t *testing.T) {
			_, err := policy.Parse("test.policy.yaml", []byte(policyTargeting(gate)))
			if err == nil {
				t.Fatalf("a policy targeting %q loaded; it must be rejected", gate)
			}
			if !strings.Contains(err.Error(), "always human") {
				t.Errorf("error does not explain the always-human rule: %v", err)
			}
		})
	}
}

// Gate #9 was reachable only as `unknown gate` — the message a typo gets —
// for as long as it had no GateID. That distinction is the whole control:
// "unknown" invites someone reconciling the rule against the code to add
// the gate to eligible(), which is precisely how a test removal would
// become policy-approvable. The rejection must name the rule, and the
// reason, so the reconciliation goes the other way.
func TestRemovingTestCoverageIsRejectedAsAlwaysHumanNotAsUnknown(t *testing.T) {
	_, err := policy.Parse("test.policy.yaml", []byte(policyTargeting("test-removal")))
	if err == nil {
		t.Fatal("a policy targeting test-removal loaded; gate #9 is always human")
	}
	if strings.Contains(err.Error(), "unknown gate") {
		t.Errorf("test-removal rejected as an unknown gate, not as always human: %v", err)
	}
	for _, want := range []string{"always human", "regression signal", "approval-gates.md"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("rejection does not mention %q: %v", want, err)
		}
	}
}

// A self-reported fact may not open a gate. codeReviewer.behaviorChange is
// the reviewer grading the significance of its own work, and an
// auto-approve policy reading it lets a mistaken or compromised reviewer
// approve itself by calling the change trivial.
func TestASelfReportedFactCannotOpenAGate(t *testing.T) {
	_, err := policy.Parse("t.policy.yaml", []byte(policyWithBehaviorChange("auto-approve")))
	if err == nil {
		t.Fatal("an auto-approve policy testing codeReviewer.behaviorChange loaded")
	}
	for _, want := range []string{"self-reported", "behaviorChange", "require-human"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("rejection does not mention %q: %v", want, err)
		}
	}
}

// The restriction is on opening a gate, not on the field. Every action that
// cannot open one may read it.
func TestASelfReportedFactMayCloseAGate(t *testing.T) {
	for _, action := range []string{"require-human", "auto-reject", "escalate"} {
		t.Run(action, func(t *testing.T) {
			if _, err := policy.Parse("t.policy.yaml", []byte(policyWithBehaviorChange(action))); err != nil {
				t.Errorf("a %s policy testing codeReviewer.behaviorChange was rejected: %v", action, err)
			}
		})
	}
}

// Nesting must not evade the rule: a restricted field under `not:` or
// `any:` is still being tested.
func TestANestedSelfReportedFactIsStillCaught(t *testing.T) {
	nested := `name: sneaky
version: "1.0"
matcher:
  gate: git-commit
condition:
  not:
    any:
      - codeReviewer.behaviorChange: true
action:
  type: auto-approve
  reason: r
`
	if _, err := policy.Parse("t.policy.yaml", []byte(nested)); err == nil {
		t.Error("a nested codeReviewer.behaviorChange slipped past the auto-approve restriction")
	}
}

func policyWithBehaviorChange(action string) string {
	escalate := ""
	if action == "escalate" {
		escalate = "  escalateTo: someone\n"
	}
	return `name: t
version: "1.0"
matcher:
  gate: git-commit
condition:
  codeReviewer.behaviorChange: false
action:
  type: ` + action + `
  reason: r
` + escalate
}

func policyTargeting(gate string) string {
	return `name: sneaky
version: "1.0"
matcher:
  gate: ` + gate + `
condition:
  testsPass: true
action:
  type: auto-approve
  reason: "tests are green"
`
}

// Duplicate keys are an error, not a last-one-wins overwrite. In an
// authorization document, a silently discarded check is a check that never
// ran.
func TestDuplicateKeysFailToParse(t *testing.T) {
	raw := `name: dupe
version: "1.0"
matcher:
  gate: git-commit
condition:
  filePaths:
    noneMatch: "**/security/**"
  filePaths:
    noneMatch: "**/auth/**"
action:
  type: auto-approve
  reason: "why not"
`
	if _, err := policy.Parse("dupe.policy.yaml", []byte(raw)); err == nil {
		t.Fatal("a policy with a duplicate condition key parsed")
	}
}

func TestRejectsMalformedPolicies(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{"unknown gate", policyTargeting("not-a-gate"), "unknown gate"},
		{"no gate", "name: n\nversion: \"1.0\"\ncondition:\n  testsPass: true\naction:\n  type: auto-approve\n  reason: r\n", "watches no gate"},
		{"no name", "version: \"1.0\"\nmatcher:\n  gate: git-commit\ncondition:\n  testsPass: true\naction:\n  type: auto-approve\n  reason: r\n", "no name"},
		{"empty condition", "name: n\nversion: \"1.0\"\nmatcher:\n  gate: git-commit\ncondition: {}\naction:\n  type: auto-approve\n  reason: r\n", "empty condition"},
		{"unknown action", "name: n\nversion: \"1.0\"\nmatcher:\n  gate: git-commit\ncondition:\n  testsPass: true\naction:\n  type: yolo\n  reason: r\n", "unknown action"},
		{"no reason", "name: n\nversion: \"1.0\"\nmatcher:\n  gate: git-commit\ncondition:\n  testsPass: true\naction:\n  type: auto-approve\n", "no reason"},
		{"escalate without target", "name: n\nversion: \"1.0\"\nmatcher:\n  gate: git-commit\ncondition:\n  testsPass: true\naction:\n  type: escalate\n  reason: r\n", "escalateTo"},
		{"unknown field", "name: n\nversion: \"1.0\"\nmatcher:\n  gate: git-commit\ncondition:\n  notAField: true\naction:\n  type: auto-approve\n  reason: r\n", "unknown condition field"},
		{"wrong value kind", "name: n\nversion: \"1.0\"\nmatcher:\n  gate: git-commit\ncondition:\n  diffLines:\n    lessThan: \"lots\"\naction:\n  type: auto-approve\n  reason: r\n", "expects a number"},
		{"unknown operator", "name: n\nversion: \"1.0\"\nmatcher:\n  gate: git-commit\ncondition:\n  diffLines:\n    roughly: 5\naction:\n  type: auto-approve\n  reason: r\n", "does not accept operator"},
		{"unknown top-level field", "name: n\nversion: \"1.0\"\nmatcher:\n  gate: git-commit\ncondition:\n  testsPass: true\naction:\n  type: auto-approve\n  reason: r\nsurprise: true\n", "field surprise not found"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := policy.Parse("t.policy.yaml", []byte(testCase.yaml))
			if err == nil {
				t.Fatalf("parsed successfully; expected an error mentioning %q", testCase.want)
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Errorf("error %q does not mention %q", err, testCase.want)
			}
		})
	}
}

// A missing policy directory is the normal case: most projects have none,
// and that must not be an error.
func TestMissingDirectoryLoadsNothing(t *testing.T) {
	policies, err := policy.Load(filepath.Join(t.TempDir(), "absent"))
	if err != nil || len(policies) != 0 {
		t.Errorf("Load of a missing directory = (%v, %v), want (nothing, no error)", policies, err)
	}
}

// Every failure is reported, not just the first: a policy set is fixed in
// one pass.
func TestEveryBrokenPolicyIsReported(t *testing.T) {
	dir := t.TempDir()
	writePolicy(t, dir, "a.policy.yaml", policyTargeting("deploy"))
	writePolicy(t, dir, "b.policy.yaml", policyTargeting("not-a-gate"))

	_, err := policy.Load(dir)
	if err == nil {
		t.Fatal("Load succeeded with two broken policies")
	}
	for _, want := range []string{"always human", "unknown gate"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("combined error does not mention %q: %v", want, err)
		}
	}
}

// Two policies with one name make an audit record ambiguous about which
// fired.
func TestDuplicateNamesAreRejected(t *testing.T) {
	dir := t.TempDir()
	same := policyTargeting("git-commit")
	writePolicy(t, dir, "a.policy.yaml", same)
	writePolicy(t, dir, "b.policy.yaml", same)

	if _, err := policy.Load(dir); err == nil || !strings.Contains(err.Error(), "must be unique") {
		t.Errorf("Load error = %v, want a duplicate-name rejection", err)
	}
}

func writePolicy(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// A disabled policy loads but is not returned for a gate.
func TestDisabledPoliciesAreLoadedButNotMatched(t *testing.T) {
	dir := t.TempDir()
	writePolicy(t, dir, "off.policy.yaml", strings.Replace(policyTargeting("git-commit"),
		"version: \"1.0\"", "version: \"1.0\"\nenabled: false", 1))

	policies, err := policy.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(policies) != 1 {
		t.Fatalf("loaded %d policies, want the disabled one still loaded", len(policies))
	}
	if matched := policy.For(policies, policy.GateGitCommit); len(matched) != 0 {
		t.Errorf("a disabled policy matched: %+v", matched)
	}
}

// The kill-switch was documented in three places for two releases and read
// by nothing. Only an explicit false disables: a typo must not silently
// switch policy evaluation off, because a control that turns itself off by
// accident is worse than one nobody set.
func TestKillSwitch(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{"explicitly disabled", "policiesEnabled: false\n", false},
		{"explicitly enabled", "policiesEnabled: true\n", true},
		{"absent key", "maxDiffLines: 200\n", true},
		{"empty file", "", true},
		{"malformed yaml", "policiesEnabled: [unclosed\n", true},
		{"unrelated keys are ignored", "mode: strict-human\nmaxContractRetries: 3\n", true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "delivery-policy.yaml")
			if err := os.WriteFile(path, []byte(testCase.content), 0o644); err != nil {
				t.Fatalf("write config: %v", err)
			}
			if got := policy.Enabled(path); got != testCase.want {
				t.Errorf("Enabled = %v, want %v", got, testCase.want)
			}
		})
	}
}

// A missing config file is the normal case and must leave evaluation on —
// the default has to be the behaviour the framework had before the switch
// existed.
func TestKillSwitchDefaultsToEnabledWithNoConfig(t *testing.T) {
	if !policy.Enabled(filepath.Join(t.TempDir(), "absent.yaml")) {
		t.Error("a missing delivery-policy.yaml disabled policy evaluation")
	}
}
