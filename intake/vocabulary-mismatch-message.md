---
intake: 1
id: vocabulary-mismatch-message
type: chore
plan: deliver-feature
status: ready
priority: 3
created: 2026-10-25
requested-by: oscar
source: manual
trust: internal
base-branch: main
finish: pr
autonomy: inherit
depends-on: [i-4-vocabulary-test]
budget:
  max-attempts: 2
  max-usd: 15
  max-hours: 1
scope:
  paths: [internal/intake/**]
---

# Assert what the vocabulary check's failure message says

## Request

> Found reviewing I-4 (PR #13). Its AC3 requires the vocabulary check's failure to name the field,
> the document, and each value missing from either side. `TestVocabularyCheckCatchesAMismatch` proves
> the check fails on the mismatched fixture, but its `errorRecorder` discards the message. Nothing
> fails if the message stops naming those things.

## Summary

In `internal/intake/vocabulary_test.go`, `errorRecorder` records only that `Errorf` was called. Make
it keep each formatted message, and assert on the message in the negative control, so AC3 is checked
by a test rather than by reading the code.

## Acceptance Criteria

- [ ] Given `errorRecorder`, when `Errorf` is called, then it keeps the formatted message, not only
      the fact that a call happened.
- [ ] Given `TestVocabularyCheckCatchesAMismatch` runs against
      `testdata/mismatched_vocabulary_template.md`, then it asserts that exactly one mismatch was
      reported, and that the message names the field `status`, the document, and the value
      `queued` as present only in the document.
- [ ] Given `assertVocabularyMatches` is changed so its message no longer names the missing values,
      when the control runs, then it fails. Show this in the QA report by making that change
      temporarily and restoring it.
- [ ] Given the change is complete, then `go test ./internal/intake/...` passes, every function has
      cyclomatic complexity below 7, and no file outside `internal/intake/` changes. The run's own
      delivery record under `docs/features/vocabulary-mismatch-message/` is the one exception.

## Out of Scope

- Changing `assertVocabularyMatches`'s message format, the fixture, or any production code.

## Domain Language

- **Closed vocabulary**: the fixed set of values one work-order field accepts.

## Non-Functional Requirements

None.

## Trust Boundaries

None.

## Test Approach

The existing Go unit tests in `internal/intake`.

## Open Questions

None.

## Infrastructure / Deploy Notes

None.
