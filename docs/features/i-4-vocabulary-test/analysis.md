---
feature: "i-4-vocabulary-test"
bounded_context: "intake"
domain_terms: ["Closed vocabulary"]
files_touched: ["internal/intake/vocabulary_test.go", "internal/intake/testdata/mismatched_vocabulary_template.md", "docs/features/i-4-vocabulary-test/"]
issue_refs: ["I-4"]
linked_adrs: []
linked_kis: []
---

# Feature Analysis: i-4-vocabulary-test

## Summary

The six closed-vocabulary fields on an intake work order (type, status, source, trust, finish, autonomy) are spelled out three times today: the Go sets in internal/intake/vocabulary.go, the trailing comments in templates/claude-feature-team/intake/TEMPLATE.md, and the status table in templates/claude-feature-team/intake/README.md. Only the template's parseability is tested (TestTheShippedTemplateParses in parse_test.go); nothing catches the three copies drifting apart. This change adds a new test, internal/intake/vocabulary_test.go, that parses the template's trailing `# a | b | c` comments and the README's status table, and asserts each against the matching Go set in vocabulary.go, so a person filling in an order is never told a value loom's validator actually rejects.

### Acceptance Criteria

- Given templates/claude-feature-team/intake/TEMPLATE.md, when the new test runs, then for each of type, status, source, trust, finish and autonomy it MUST read the values in that field's trailing `# a | b | c` comment and assert they equal the matching Go set in vocabulary.go, ignoring order.
  - Example: type: feature                  # feature | bug | spike | chore  ->  {feature, bug, spike, chore} == knownTypes
  - Example: autonomy: inherit              # inherit | strict-human — may tighten...  ->  {inherit, strict-human} == knownAutonomies (text after the em dash is not a value)
- Given templates/claude-feature-team/intake/README.md, when the new test runs, then it MUST read the values in the first column of the status table and assert they equal knownStatuses.
  - Example: | `draft` | ... |
| `ready` | ... |
| `hold` | ... |  ->  {draft, ready, hold} == knownStatuses
- Given a value present in a Go set but missing from the template comment, or the reverse, when the test runs, then it MUST fail naming the field, the document, and every value missing from either side.
  - Example: knownTypes gains "epic" with no template edit -> failure names field "type", document "TEMPLATE.md", and missing value "epic"
- Given a template whose comment for one of the six fields cannot be found or parsed, when the test runs, then it MUST fail naming that field, and MUST NOT pass by finding nothing to compare.
- Given the change is complete, a second test MUST feed the comparison a deliberately mismatched template fixture under internal/intake/testdata/ and assert the comparison fails, proving the check can catch a real mismatch.
- Given the change is complete, go test ./internal/intake/... MUST pass, every new function MUST have cyclomatic complexity below 7, and no file outside internal/intake/ MUST change except this run's own delivery record under docs/features/i-4-vocabulary-test/.

### Non-Functional Requirements

- **performance**: The new tests MUST run well within the package's existing fast unit-test budget. (new test functions' combined wall-clock duration 1s)
- **other**: The new tests MUST NOT require network access or invoke git — they read two files already on disk from the repository the test binary is built from.

## Proposed Fitness Functions

- **The template's per-field comment list and the README's status table stay equal to the matching Go closed-vocabulary set**
  - **Verification**: go test ./internal/intake/... (TestTemplateVocabularyMatchesGoSets, TestReadmeStatusTableMatchesGoSets), already run by the project's existing CI test step — no new CI wiring needed
  - **Owner**: Developer
- **The vocabulary-comparison check can actually detect a real mismatch, not just pass vacuously**
  - **Verification**: go test -run TestVocabularyCheckCatchesAMismatch ./internal/intake/... against a deliberately wrong fixture in internal/intake/testdata/
  - **Owner**: Developer

## Out of Scope

- Changing any vocabulary value, the template's wording, or the README's wording.
- Validating `plan` values — they come from plan files discovered at runtime, not from a fixed set.
- Generating the template or README from the Go sets, or any other drift-prevention mechanism beyond a failing test.
- Adding a `closed vocabulary` entry to shared/DOMAIN_DICTIONARY.md — see Open Questions; the spec's own AC6 restricts this change to internal/intake/ plus the delivery record, which conflicts with the analyst's standing default to update the dictionary on a new term.

## Technical Breakdown

None

### Bounded Context

- **Owning Context**: intake
- **Context Crossings**: templates/claude-feature-team/intake/TEMPLATE.md and README.md — documentation owned by the template directory, read (never written) by a test in internal/intake/

### Domain Events (Event Storming Lite)

- **Commands**: None
- **Events Produced**: None
- **Owning Aggregates**: None
- **Read Models / Projections**: None

### Affected Components

- `internal/intake/vocabulary_test.go` — New test file (package intake, so it can read the unexported knownTypes/knownStatuses/knownSources/knownTrust/knownFinishes/knownAutonomies sets) implementing AC1–AC4: parse TEMPLATE.md's six trailing comments and README.md's status table, diff each against its Go set, and fail naming the field and missing values on either side.
- `internal/intake/testdata/mismatched_vocabulary_template.md` — AC5's deliberately-wrong fixture: a copy of TEMPLATE.md with one field's comment edited to disagree with its Go set, read by a test asserting the comparison fails. Proves the check is not a vacuous pass.
- `docs/features/i-4-vocabulary-test/` — This run's own delivery record — the one path outside internal/intake/ that AC6 permits to change.

### Data Model Changes

None

### API Changes

None

### New Dependencies

None

## Task List

None

### Developer Tasks

1. Create internal/intake/vocabulary_test.go in package intake.
2. Write a helper that locates each of the six fields' lines in TEMPLATE.md by its YAML key (`type:`, `status:`, `source:`, `trust:`, `finish:`, `autonomy:`) at the start of a line, extracts the trailing `# ...` comment, cuts it at the first em dash (—) if present, splits the remainder on `|`, and trims each piece — mirroring the existing field-table pattern in checkVocabulary (internal/intake/validate.go).
3. Write TestTemplateVocabularyMatchesGoSets: table-driven over the six fields {name, extracted template values, Go set (knownTypes/knownStatuses/knownSources/knownTrust/knownFinishes/knownAutonomies)}; assert set equality ignoring order, and on mismatch report the field name, 'TEMPLATE.md', and the values present on only one side.
4. Write a small markdown-table-row parser for README.md's Lifecycle table (the `| `status`` | Meaning | Who sets it |` block) that extracts the first column's backtick-quoted values.
5. Write TestReadmeStatusTableMatchesGoSets: parse README.md's status table, assert its values equal knownStatuses with the same missing-values-named failure message style.
6. Resolve paths the same way parse_test.go's TestTheShippedTemplateParses does: runtime.Caller(0) + filepath.Join(..., "..", "..", "templates", "claude-feature-team", "intake", "TEMPLATE.md"/"README.md") — no hardcoded absolute paths, no network, no git.
7. If a field's comment cannot be found or its pipe-list cannot be parsed, fail that subtest naming the field — never silently skip it (AC4): e.g. t.Fatalf("could not find a parseable %q comment in TEMPLATE.md", field).
8. Create internal/intake/testdata/mismatched_vocabulary_template.md: a copy of TEMPLATE.md with exactly one field's trailing comment edited to disagree with its Go set (e.g. add a phantom status `queued` not in knownStatuses).
9. Write TestVocabularyCheckCatchesAMismatch (AC5): run the same comparison helper against the testdata fixture and assert it reports a failure, proving the check is not vacuous.
10. Keep every new function's cyclomatic complexity below 7 — extract the comment-cutting, pipe-splitting, and set-diffing logic into separate small functions rather than one long one.
11. Run go test ./internal/intake/... and confirm no file outside internal/intake/ changed (git status / git diff --stat).

### QA Tasks

1. Run go test ./internal/intake/... and confirm both new tests and the full existing suite (parse_test.go, validate_test.go, check_test.go) stay green.
2. Verify TestVocabularyCheckCatchesAMismatch genuinely fails if its assertion is inverted (temporarily flip the assert to confirm the test can fail, per the test-repair-contract's 'prove a new test can fail' requirement), then confirm it is restored correctly.
3. Confirm the new test functions each report well under 1 second in `go test -v` output and require no network or git (NFR).

### Tech Writer Tasks

None

### DevOps Tasks

None

## Edge Cases and Risks

- A field's template comment uses an em dash to add explanatory prose after the pipe list (e.g. autonomy, trust, finish, status).: Cut the comment text at the first em dash (—) before splitting on `|`, so explanatory prose is never mistaken for a seventh vocabulary value.
- The Go set and template list have the same members in a different order.: Compare as sets (map/set membership), not as ordered slices — AC1 explicitly says 'order ignored'.
- A value exists in the Go set but was never added to the template comment (or the reverse).: Report the field, the document, and every value present on only one side — not just a boolean mismatch (AC3).
- A field's comment line does not start the line with the exact YAML key (e.g. extra whitespace, or the key appears elsewhere in a different context such as inside the <!-- HOW TO USE --> comment block).: Anchor the search to the frontmatter block only, matching on `^fieldname:` at line start, so prose mentioning the field name elsewhere in the file is never misread as its comment.
- README.md's status table row values are backtick-quoted (`` `draft` ``) inside a markdown table cell.: Strip backticks and markdown table pipe delimiters before comparing, distinguishing a table-cell `|` from a vocabulary separator `|`.
- A future contributor adds a seventh closed-vocabulary field to vocabulary.go but forgets to add test coverage for it.: Out of scope per the spec (Open Questions) — the README check covers status alone and the template covers all six existing fields; a new field needs a follow-up change to this test, not caught automatically by this change.

## Definition of Done

- internal/intake/vocabulary_test.go exists with TestTemplateVocabularyMatchesGoSets and TestReadmeStatusTableMatchesGoSets, both passing against the real template and README.
- internal/intake/testdata/mismatched_vocabulary_template.md and TestVocabularyCheckCatchesAMismatch exist and the test fails when run against the mismatched fixture, proving the check can fail.
- go test ./internal/intake/... passes in full, including all pre-existing tests.
- Every new function has cyclomatic complexity below 7 (CLAUDE.md's non-negotiable rule).
- No file outside internal/intake/ changed, except docs/features/i-4-vocabulary-test/ (this run's delivery record).
- A field whose comment is missing or unparseable fails the test naming that field, rather than passing vacuously.
