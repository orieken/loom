---
feature: "i-4-vocabulary-test"
bounded_context: ""
domain_terms: ["closed vocabulary"]
files_touched: []
issue_refs: ["I-4"]
linked_adrs: []
linked_kis: []
---

# Acceptance Scenarios: i-4-vocabulary-test

## Each closed-vocabulary field's template comment matches its Go set, regardless of order

Verifies: Given templates/claude-feature-team/intake/TEMPLATE.md, when the new test runs, then for each of type, status, source, trust, finish and autonomy it MUST read the values in that field's trailing `# a | b | c` comment and assert they equal the matching Go set in vocabulary.go, ignoring order.

```gherkin
Scenario: Each closed-vocabulary field's template comment matches its Go set, regardless of order
  Given the intake template at templates/claude-feature-team/intake/TEMPLATE.md declares type, status, source, trust, finish and autonomy with a trailing `# a | b | c` comment on each field
  And internal/intake/vocabulary.go declares the matching known-value set for each of those six fields
  When the vocabulary comparison test runs for the template
  Then for every one of the six fields, the set of values listed in the template comment equals the set of values in the matching Go set
  And a difference in listed order between the comment and the Go set does not cause a failure
```

## The README's status table lists exactly the known status values

Verifies: Given templates/claude-feature-team/intake/README.md, when the new test runs, then it MUST read the values in the first column of the status table and assert they equal knownStatuses.

```gherkin
Scenario: The README's status table lists exactly the known status values
  Given templates/claude-feature-team/intake/README.md contains a status table whose first column lists one backtick-quoted status value per row
  And internal/intake/vocabulary.go declares the known status set
  When the vocabulary comparison test runs for the README
  Then the set of status values in the table's first column equals the known status set in Go, with backticks and table-pipe delimiters stripped before comparison
```

## A one-sided vocabulary value fails the test with the field, the document, and the missing values named

Verifies: Given a value present in a Go set but missing from the template comment, or the reverse, when the test runs, then it MUST fail naming the field, the document, and every value missing from either side.

```gherkin
Scenario: A one-sided vocabulary value fails the test with the field, the document, and the missing values named
  Given a closed-vocabulary field's Go set and its template (or README) listing disagree on membership — a value exists on one side only
  When the vocabulary comparison test runs
  Then the test fails
  And the failure message names the field that disagreed
  And the failure message names the document the field was read from
  And the failure message lists every value missing from either side, not just that a mismatch exists
```

## An unparseable field comment fails loudly instead of being silently skipped

Verifies: Given a template whose comment for one of the six fields cannot be found or parsed, when the test runs, then it MUST fail naming that field, and MUST NOT pass by finding nothing to compare.

```gherkin
Scenario: An unparseable field comment fails loudly instead of being silently skipped
  Given the template's comment for one of the six closed-vocabulary fields is missing, or does not start at the field's YAML key on its own frontmatter line
  When the vocabulary comparison test runs
  Then the test fails
  And the failure message names the field whose comment could not be found or parsed
  And the test does not pass by treating the absence of a comparable value as an empty, trivially-equal set
```

## A deliberately mismatched template fixture proves the comparison can fail

Verifies: Given the change is complete, a second test MUST feed the comparison a deliberately mismatched template fixture under internal/intake/testdata/ and assert the comparison fails, proving the check can catch a real mismatch.

```gherkin
Scenario: A deliberately mismatched template fixture proves the comparison can fail
  Given a fixture template file exists under internal/intake/testdata/ whose vocabulary comment for at least one field deliberately disagrees with the Go set
  When the vocabulary comparison runs against that fixture instead of the real template
  Then the comparison reports a failure for the mismatched field
  And this demonstrates the check is not a test that can never fail
```

## The full intake suite passes and the change stays within its scope

Verifies: Given the change is complete, go test ./internal/intake/... MUST pass, every new function MUST have cyclomatic complexity below 7, and no file outside internal/intake/ MUST change except this run's own delivery record under docs/features/i-4-vocabulary-test/.

```gherkin
Scenario: The full intake suite passes and the change stays within its scope
  Given the vocabulary comparison tests and fixture have been added to internal/intake/
  When go test ./internal/intake/... runs
  Then every test in the package passes, including the pre-existing parse, validate and check tests
  And every newly added function has cyclomatic complexity below 7
  And no file outside internal/intake/ changed, other than this run's delivery record under docs/features/i-4-vocabulary-test/
```
