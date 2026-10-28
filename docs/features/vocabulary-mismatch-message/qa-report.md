---
feature: "vocabulary-mismatch-message"
bounded_context: ""
domain_terms: ["status", "vocabulary"]
files_touched: []
issue_refs: []
linked_adrs: []
linked_kis: []
---

# QA Report: vocabulary-mismatch-message

## Test Files Created

None

## Test Files Modified

None

## Coverage Summary

- Acceptance criteria covered: 4/4
- Statement coverage, lowest unit: **100.0%** (`internal/intake`)
  - `internal/intake`: 100.0%

## Test Results

- Passed: 22
- Failed: 0
- Skipped: 0

## Accessibility Check

None

## Bugs Found

None

## Known Gaps

None

## Notes for Tech Writer

- Negative control re-verified independently of the developer's own run: temporarily changed assertVocabularyMatches's Errorf format string from 'field %q: %s and vocabulary.go disagree\n  only in %s: %v\n  only in vocabulary.go: %v' to a bare 'field %q: %s and vocabulary.go disagree', reran go test ./internal/intake/... -run TestVocabularyCheckCatchesAMismatch, and the message-content assertion failed with 'mismatch message does not name field "status", document "fixture", and value "queued": field "status": fixture and vocabulary.go disagree' — confirming the new assertion is not vacuous.
- Reverted the format string to its original text; git diff on vocabulary_test.go afterward matches exactly the developer's committed change (the +21/-6 diff shows only the errorRecorder.messages refactor and the new assertion, no residue from the temporary edit).
- go test ./internal/intake/... -count=1 passes in full (22 top-level tests, 0 failures), including TestTemplateVocabularyMatchesGoSets, TestReadmeStatusTableMatchesGoSets, and TestFieldCommentValuesFailsClosedWhenUnparseable.
- gocyclo internal/intake/vocabulary_test.go: highest complexity is 5 (TestVocabularyCheckCatchesAMismatch and statusTableValues), well under the 7 threshold.
- go test ./internal/intake/... -coverprofile reports 100.0% statement coverage for the internal/intake package.
- git status --porcelain shows only internal/intake/vocabulary_test.go modified, plus the untracked docs/features/vocabulary-mismatch-message/ delivery-record directory — no file outside internal/intake/ changed.
