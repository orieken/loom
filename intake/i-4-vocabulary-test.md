---
intake: 1
id: i-4-vocabulary-test
type: chore
plan: deliver-feature
status: ready
priority: 2
created: 2026-10-24
requested-by: oscar
source: manual
source-ref: "docs/audits/factory-intake-and-deliver-feature-review-2026-10-23.md#I-4"
trust: internal
base-branch: main
finish: pr
autonomy: inherit
depends-on: []
budget:
  max-attempts: 2
  max-usd: 25
  max-hours: 2
scope:
  paths: [internal/intake/**]
---

# Tie the intake template's vocabulary to the Go sets

## Request

> **I-4 (P2) — The vocabulary exists three times**: Go sets (`internal/intake/vocabulary.go`), the
> template's comments, and the README's rule list. Only the template's *parseability* is tested.
> *Fix*: a test that reads the README's listed values and compares them with the Go sets.

## Summary

The values a work order may use for `type`, `status`, `source`, `trust`, `finish` and `autonomy` are
written down in three places. A test makes the documentation and the code fail together when they
drift, so a person filling in an order is never told a value the factory rejects.

## Acceptance Criteria

- [ ] Given `templates/claude-feature-team/intake/TEMPLATE.md`, when the new test runs, then for each
      of the six fields `type`, `status`, `source`, `trust`, `finish` and `autonomy` it reads the
      values listed in that field's trailing comment (`# a | b | c`) and asserts they equal the
      matching Go set in `internal/intake/vocabulary.go` — same members, order ignored.
- [ ] Given `templates/claude-feature-team/intake/README.md`, when the new test runs, then it reads
      the values in the first column of the `status` table and asserts they equal the Go status set.
- [ ] Given a value present in a Go set but missing from the template comment, or the reverse, when
      the test runs, then it fails with a message naming the field, the document, and each value
      missing from either side.
- [ ] Given a template whose comment for one of the six fields cannot be found or parsed, when the
      test runs, then it fails naming that field. It never passes by finding nothing to compare.
- [ ] Given the change is complete, then a second test feeds the comparison a deliberately
      mismatched template held under `internal/intake/testdata/` and asserts it fails. This proves
      the check can fail.
- [ ] Given the change is complete, then `go test ./internal/intake/...` passes, every new function
      has cyclomatic complexity below 7, and no file outside `internal/intake/` changes. The run's
      own delivery record under `docs/features/i-4-vocabulary-test/` is the one exception.

## Out of Scope

- Changing any vocabulary value, the template's wording, or the README's wording.
- Validating `plan` values. They come from plan files, not from a fixed set.
- Generating the template or README from the Go sets.

## Domain Language

- **Closed vocabulary**: the fixed set of values one work-order field accepts; anything else is a
  validation violation rather than a default.

## Non-Functional Requirements

- The new tests run in under 1 second and need no network and no git.

## Trust Boundaries

None. The test reads two files from this repository.

## Test Approach

Go unit tests with the standard `testing` package, table-driven over the six fields, in package
`intake` so they can read the unexported sets. Paths are relative to the package directory
(`../../templates/claude-feature-team/intake/`).

## Open Questions

- The README lists only the `status` values in a table; the other five fields appear there by name
  only — Assumption if unresolved: the README check covers `status` alone, and the template covers
  all six.

## Infrastructure / Deploy Notes

None.
