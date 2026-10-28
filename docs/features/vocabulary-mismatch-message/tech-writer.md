# Documentation Report: vocabulary-mismatch-message

## Files Updated

None.

## Files Unchanged (and why)

- `CHANGELOG.md` — No root changelog exists in this repository (confirmed via repo-wide search; only subdirectory changelogs exist under `internal/references/testdata/fixture/`, `shared/agents/`, and `shared/schemas/`, none of which cover `internal/intake/`). This chore also changes no user-facing behavior: `assertVocabularyMatches`'s message format, the fixture, and all production code are explicitly out of scope and were left untouched (confirmed by diff: only `internal/intake/vocabulary_test.go` changed).
- `README.md` (repo root and `templates/claude-feature-team/intake/README.md`) — The vocabulary check itself, and the Lifecycle status table it validates, are unchanged. This chore only makes an existing test (`TestVocabularyCheckCatchesAMismatch`) assert on message content it previously ignored; it adds no new capability for a reader of either README to learn about.
- `docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md` — This chore is an application of ADR-008's existing principle (a test must depend on what it claims to verify), not a new decision. The ADR already covers this case; no amendment is needed.
- `docs/features/README.md` — Documents the pipeline-artifact convention itself, which this delivery follows rather than changes.
- Inline code docs in `internal/intake/vocabulary_test.go` — `errorRecorder`, `Errorf`, and the new `messageNamesStatusFixtureQueued` helper already carry doc comments from implementation (per `implementation-notes.md`'s Named Refactoring Log), and the test functions' existing comments (`TestVocabularyCheckCatchesAMismatch` proves AC5...) still accurately describe the test's purpose after the change — no comment was made stale by this diff.

## Notes for DevOps

None. This change touches only a test file in `internal/intake/`; it adds no new environment variables, infrastructure, or CI wiring beyond the suite DevOps already runs.