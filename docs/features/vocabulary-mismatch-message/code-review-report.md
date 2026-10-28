---
feature: "vocabulary-mismatch-message"
bounded_context: ""
domain_terms: ["Closed vocabulary"]
files_touched: []
issue_refs: ["I-4", "PR #13", "vocabulary-mismatch-message"]
linked_adrs: ["docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md"]
linked_kis: []
---

# Code Review: vocabulary-mismatch-message

## Overall Status

**APPROVED**

## Design Narrative

The change swaps errorRecorder's boolean failed field for a messages []string field so TestVocabularyCheckCatchesAMismatch can assert on the actual content of the failure message, not just that a failure occurred. A small extracted helper, messageNamesStatusFixtureQueued, names the three substrings AC3 requires, keeping the test body and its complexity low. Behaviour change: no — assertVocabularyMatches's production message format and logic are untouched; only the test double and the test's assertions changed.

## Design Score

- **Clarity** 5/5
- **Cohesion** 5/5
- **Coupling** 5/5
- **Craft** 5/5

## Security Surface

None

## Performance Surface

None

## Test Design Review

- TestVocabularyCheckCatchesAMismatch (modified) — YES: asserts len(recorder.messages) == 1 and that the single message contains "status", "fixture", and "queued" via messageNamesStatusFixtureQueued; independently verified by temporarily weakening assertVocabularyMatches's Errorf format string (dropping the two "only in..." clauses) and confirming the test fails with the expected Fatalf message, then restoring the original string exactly and confirming the full package suite passes again.
- messageNamesStatusFixtureQueued is a plain helper function, not a test — not in scope for this question.

## Verification of Developer Self-Review

- Developer's complexity claim verified independently with gocyclo: highest score in the file is 5 (TestVocabularyCheckCatchesAMismatch, statusTableValues), messageNamesStatusFixtureQueued scores 3 — matches the self-review exactly.
- Developer's negative-control claim (weaken format string, see assertion fail, restore exactly) was independently reproduced rather than taken on trust; restoration confirmed byte-for-byte via git diff after reverting.
- Scope claim (only internal/intake/vocabulary_test.go changed, plus this run's own delivery record) verified via git status — accurate.

## Feedback for the Developer

None
