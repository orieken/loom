---
feature: "vocabulary-mismatch-message"
bounded_context: ""
domain_terms: []
files_touched: []
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Acceptance Scenarios: vocabulary-mismatch-message

## errorRecorder keeps the full text of every formatted error message

Verifies: errorRecorder MUST keep every formatted message passed to Errorf, not only whether a call happened.

```gherkin
Scenario: errorRecorder keeps the full text of every formatted error message
  Given an errorRecorder test double stands in for the real test-reporting interface
  When Errorf is called on the recorder with a format string and arguments
  Then the recorder stores the fully formatted message text
  And the recorder does not merely flag that a call happened, with the message text discarded
```

## Vocabulary check reports exactly one mismatch, naming the field, document, and extra value

Verifies: TestVocabularyCheckCatchesAMismatch MUST assert exactly one mismatch was recorded, naming field status, the document, and value queued.

```gherkin
Scenario: Vocabulary check reports exactly one mismatch, naming the field, document, and extra value
  Given a fixture document whose status vocabulary includes an extra value, queued, not present in vocabulary.go's status set
  And every other vocabulary field in the fixture (type, source, trust, finish, autonomy) matches vocabulary.go exactly
  When the vocabulary check runs the fixture document against vocabulary.go
  Then exactly one mismatch is recorded
  And the recorded message names the field status
  And the recorded message names the fixture document
  And the recorded message names the extra value queued
```

## Weakening the mismatch message breaks the content assertion, and reverting restores a passing suite

Verifies: Temporarily weakening assertVocabularyMatches's message MUST make the new content assertion fail; qa-report.md MUST record this and its reversal.

```gherkin
Scenario: Weakening the mismatch message breaks the content assertion, and reverting restores a passing suite
  Given assertVocabularyMatches's Errorf format string is temporarily changed to drop its "only in" clauses
  When TestVocabularyCheckCatchesAMismatch runs against the weakened format string
  Then the message-content assertion fails
  And after the format string is reverted to its original text, the full internal/intake suite passes again
```

## The internal/intake suite stays green, complexity-compliant, and scoped to its own package

Verifies: The full internal/intake suite MUST pass, every function's cyclomatic complexity MUST stay below 7, and no file outside internal/intake/ MUST change except this run's own delivery record.

```gherkin
Scenario: The internal/intake suite stays green, complexity-compliant, and scoped to its own package
  Given the vocabulary-mismatch-message change is complete
  When go test ./internal/intake/... runs
  Then the suite passes
  And every function in the changed test file has cyclomatic complexity below 7
  And no file outside internal/intake/ has changed, except this run's own delivery record under docs/features/vocabulary-mismatch-message/
```
