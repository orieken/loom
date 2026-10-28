---
feature: "vocabulary-mismatch-message"
bounded_context: "intake"
domain_terms: ["Closed vocabulary"]
files_touched: ["internal/intake/vocabulary_test.go"]
issue_refs: ["I-4", "PR #13", "vocabulary-mismatch-message"]
linked_adrs: ["docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md"]
linked_kis: []
---

# Feature Analysis: vocabulary-mismatch-message

## Summary

TestVocabularyCheckCatchesAMismatch proves assertVocabularyMatches can fail, but its errorRecorder test double only records a boolean, so nothing in the suite catches a regression that makes the failure message stop naming the field, document, and missing values — even though that naming was AC3 of the prior i-4-vocabulary-test feature. This chore makes errorRecorder keep each formatted message and has the test assert on that message's content, so AC3's guarantee is checked by a test rather than by reading assertVocabularyMatches's source, closing the ADR-008 gap of a check that executes without depending on what it claims to verify.

### Acceptance Criteria

- errorRecorder MUST keep every formatted message passed to Errorf, not only whether a call happened.
  - Example: Current: type errorRecorder struct { failed bool }, with Errorf setting failed = true and ignoring format/args.
  - Example: Target: type errorRecorder struct { messages []string }, with Errorf appending fmt.Sprintf(format, args...) to messages.
- TestVocabularyCheckCatchesAMismatch MUST assert exactly one mismatch was recorded, naming field status, the document, and value queued.
  - Example: Only the fixture's status comment lists queued in addition to draft | ready | hold; the other five fields (type, source, trust, finish, autonomy) match their Go sets exactly, so len(recorder.messages) MUST equal 1.
  - Example: recorder.messages[0] MUST contain field "status", the document name used in the call ("fixture"), and queued under the "only in" clause assertVocabularyMatches already formats.
- Temporarily weakening assertVocabularyMatches's message MUST make the new content assertion fail; qa-report.md MUST record this and its reversal.
  - Example: Change the Errorf format string from 'field %q: %s and vocabulary.go disagree\n  only in %s: %v\n  only in vocabulary.go: %v' to a bare 'field %q: %s and vocabulary.go disagree', rerun the test, observe the content assertion fail, then restore the original format string exactly.
- The full internal/intake suite MUST pass, every function's cyclomatic complexity MUST stay below 7, and no file outside internal/intake/ MUST change except this run's own delivery record.
  - Example: go test ./internal/intake/... exits 0.
  - Example: git status / git diff --stat shows only internal/intake/vocabulary_test.go and docs/features/vocabulary-mismatch-message/ changed.

### Non-Functional Requirements

None

## Proposed Fitness Functions

- **The negative control's message assertion depends on assertVocabularyMatches's actual message content, not just on whether Errorf was called.**
  - **Verification**: QA temporarily drops the two "only in..." clauses from assertVocabularyMatches's Errorf format string, reruns go test ./internal/intake/ -run TestVocabularyCheckCatchesAMismatch to confirm the new assertion fails, then restores the original string and reruns the full package suite to confirm green. Recorded in qa-report.md.
  - **Owner**: QA

## Out of Scope

- Changing assertVocabularyMatches's message format, logic, or any other production code in vocabulary.go.
- Changing testdata/mismatched_vocabulary_template.md or any other fixture.
- Changing any file outside internal/intake/, other than this run's own delivery record under docs/features/vocabulary-mismatch-message/.

## Technical Breakdown

None

### Bounded Context

- **Owning Context**: intake
- **Context Crossings**: None

### Domain Events (Event Storming Lite)

- **Commands**: None
- **Events Produced**: None
- **Owning Aggregates**: None
- **Read Models / Projections**: None

### Affected Components

- `internal/intake/vocabulary_test.go` — Change errorRecorder to store each formatted Errorf message instead of a bool, and update TestVocabularyCheckCatchesAMismatch to assert exactly one message was recorded naming field status, the document, and value queued.

### Data Model Changes

None

### API Changes

None

### New Dependencies

None

## Task List

None

### Developer Tasks

1. In internal/intake/vocabulary_test.go, replace errorRecorder's failed bool field with a messages []string field.
2. Update errorRecorder.Errorf to append fmt.Sprintf(format, args...) to messages instead of setting a boolean.
3. In TestVocabularyCheckCatchesAMismatch, replace the if !recorder.failed check with an assertion that len(recorder.messages) equals 1, failing with the count and the full slice on mismatch.
4. Add an assertion in TestVocabularyCheckCatchesAMismatch that recorder.messages[0] contains field "status", the document name passed to the call ("fixture"), and queued.
5. Keep every changed function's cyclomatic complexity below 7; extract a small substring-matching helper if the new assertion needs more than one or two checks.

### QA Tasks

1. Run go test ./internal/intake/... and confirm the updated test passes.
2. Temporarily edit assertVocabularyMatches's Errorf format string to drop its "only in..." clauses, rerun TestVocabularyCheckCatchesAMismatch, and record in qa-report.md that the new message-content assertion fails as expected.
3. Revert the temporary edit, rerun the full internal/intake suite, and confirm it passes again.
4. Confirm every function in vocabulary_test.go has cyclomatic complexity below 7.
5. Confirm via git status / git diff --stat that no file outside internal/intake/ changed, other than this run's delivery record under docs/features/vocabulary-mismatch-message/.

### Tech Writer Tasks

None

### DevOps Tasks

None

## Edge Cases and Risks

- Map iteration order for the Go vocabulary sets is random, so sortedValuesNotIn's output order could vary between runs.: assertVocabularyMatches already sorts both the document-only and Go-set-only slices before formatting, so the message text is stable; the new assertion can match on substrings without depending on iteration order.
- A message assertion that checks only the field name would still pass if the document name or missing-value clause were dropped, silently narrowing AC3's coverage.: Assert all three required substrings — field status, the document name, and value queued — so a regression dropping any one of them individually fails the test.
- A future change introduces a second mismatched field in the fixture, which a boolean-only check could not distinguish from one mismatch.: Assert len(recorder.messages) == 1, not just that messages is non-empty, so an unexpected second mismatch is also caught.

## Definition of Done

- errorRecorder keeps every formatted Errorf message, not only a boolean.
- TestVocabularyCheckCatchesAMismatch asserts exactly one recorded message, naming field status, the document, and value queued.
- Temporarily weakening assertVocabularyMatches's message was verified to fail the new assertion, then reverted, recorded in qa-report.md.
- go test ./internal/intake/... passes.
- Every function in the changed file has cyclomatic complexity below 7.
- No file outside internal/intake/ changed, except this run's own delivery record under docs/features/vocabulary-mismatch-message/.
