---
feature: "i-4-vocabulary-test"
bounded_context: "intake"
domain_terms: ["Closed vocabulary"]
files_touched: []
issue_refs: ["I-4"]
linked_adrs: []
linked_kis: []
---

# Documentation Report: Tie the intake template's vocabulary to the Go sets

## Files Updated

None. This change adds test coverage only — `internal/intake/vocabulary_test.go` and the fixture
`internal/intake/testdata/mismatched_vocabulary_template.md` — with no change to any documented
behavior, API, configuration, or setup step. The spec's own acceptance criteria (AC6) and Out of
Scope section restrict this delivery to `internal/intake/**` plus its delivery record, so no
documentation file outside that boundary was touched.

## Files Unchanged (and why)

- `CHANGELOG.md` — No root changelog exists in this repository, and the change has no user-facing
  behavior: the vocabulary values, the template's wording, and the README's wording are all
  unchanged (explicitly Out of Scope). There is nothing for a changelog entry to describe.
- `README.md` — Adds no new capability, command, or flag a user needs to know about. It is a test
  that runs under the existing `go test ./internal/intake/...` CI step.
- `templates/claude-feature-team/intake/TEMPLATE.md` and
  `templates/claude-feature-team/intake/README.md` — These are the documents the new test reads
  and checks for drift against `internal/intake/vocabulary.go`; the spec's Out of Scope section
  forbids changing their wording or values in this delivery. They already passed the new test
  unmodified, so no edit is needed.
- `docs/adrs/` — No new architectural decision was made. This closes a gap (I-4) identified in an
  existing audit by adding a fitness function; it does not introduce a new structural pattern, so
  no ADR is warranted.
- `docs/audits/factory-intake-and-deliver-feature-review-2026-10-23.md` — This is the audit that
  raised I-4. Marking its finding resolved there would be a file outside `internal/intake/`, which
  AC6 forbids for this run; that update is a separate, human-owned edit outside this pipeline.
- Code-level docs (module/function docstrings) — `vocabulary_test.go` is a test file inside an
  already-documented package (`internal/intake`); Go test files in this codebase carry no separate
  package doc comment, matching the convention already established by `parse_test.go`,
  `validate_test.go`, and `check_test.go`.

## Notes for DevOps

None. No new environment variables, infrastructure, or CI wiring is needed: the new tests run
under the existing `go test ./internal/intake/...` step with no network or git dependency (per the
spec's Non-Functional Requirements), and QA confirmed the full package suite passes in ~0.3s.
