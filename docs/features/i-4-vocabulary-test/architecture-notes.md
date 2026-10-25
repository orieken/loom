---
feature: "i-4-vocabulary-test"
bounded_context: "intake"
domain_terms: ["Closed vocabulary", "work order", "intake order"]
files_touched: ["internal/intake", "internal/intake/testdata"]
issue_refs: ["i-4-vocabulary-test"]
linked_adrs: []
linked_kis: []
---

# Architecture Notes: i-4-vocabulary-test

## Structural Decisions

### Add the new test as internal/intake/vocabulary_test.go in package intake (white-box, same package), not package intake_test.
- **Rationale**: It must read the unexported knownTypes/knownStatuses/knownSources/knownTrust/knownFinishes/knownAutonomies sets in vocabulary.go directly. Exporting them solely so an external test package could see them would widen the package's public API for a reason that has nothing to do with its callers.
- **Trade-offs**:
  - Couples the test tightly to vocabulary.go's internal representation (a map[string]bool) — acceptable here because the test's entire purpose is comparing that internal representation against two documents, not treating it as a black box.
- **Fitness Function**: vocabulary_test.go is declared `package intake` and the package still builds — verified by go build ./internal/intake/... (fails to compile if the file references unexported identifiers from a non-matching package)
### Decompose the comment/table parsing into small single-purpose functions (e.g. extractFieldComment, cutAtEmDash, splitPipeList, parseStatusTableColumn, assertSetsEqual) instead of one large comparison function, mirroring checkVocabulary's existing field-table style in validate.go.
- **Rationale**: CLAUDE.md's cyclomatic-complexity-<7 rule applies to every function, test code included. Six fields times (find line, cut comment, split, trim, diff) in one function would cross that budget quickly.
- **Trade-offs**:
  - More functions to read for the same behavior — mitigated by each one being named for exactly what it does, so no function needs a comment to explain itself.
- **Fitness Function**: Cyclomatic complexity < 7 for every function in vocabulary_test.go — verified by golangci-lint / gocyclo (the project's existing lint config) or the analyze-complexity skill, run against internal/intake/vocabulary_test.go
### Resolve TEMPLATE.md and README.md's paths with runtime.Caller(0) + filepath.Join(...), the same pattern TestTheShippedTemplateParses already uses in parse_test.go — no hardcoded absolute paths, no go:embed, no network or git calls.
- **Rationale**: This is the package's existing, working idiom for reaching the template from a test binary regardless of the caller's working directory. Introducing a second way to resolve the same two files would be an unforced inconsistency.
- **Trade-offs**:
  - Still brittle to moving templates/claude-feature-team/intake/ itself, same as the existing test — an accepted, pre-existing trade-off, not a new one this change introduces.
- **Fitness Function**: judgment-only — This is the package's existing, working idiom for reaching the template from a test binary regardless of the caller's working directory. Introducing a second way to resolve the same two files would be an unforced inconsistency.
### Store the deliberately-mismatched fixture as a static file, internal/intake/testdata/mismatched_vocabulary_template.md, rather than generating it at test time with strings.Replace against the real TEMPLATE.md.
- **Rationale**: A statically committed fixture is reviewable on its own — a reader can see exactly which value was changed to disagree with vocabulary.go. A string-replace-generated fixture risks silently matching nothing (and the test passing vacuously) the moment TEMPLATE.md's wording around that field changes.
- **Trade-offs**:
  - The fixture can drift from TEMPLATE.md's surrounding prose over time; acceptable because the fixture's only job is to carry one field's phantom value, not to stay a faithful copy of the whole template.
- **Fitness Function**: The vocabulary-comparison check actually fails against a known-bad fixture, proving it is not a vacuous pass — verified by go test -run TestVocabularyCheckCatchesAMismatch ./internal/intake/... against internal/intake/testdata/mismatched_vocabulary_template.md
### Treat the test's reads of templates/claude-feature-team/intake/TEMPLATE.md and README.md as local file reads of two files checked into this repository, not as a dependency on an external system requiring an interface (architecture-guardrails.md's DB/HTTP/filesystem-as-infrastructure rule does not apply here).
- **Rationale**: The files are documentation shipped in the same repo and read once per test run with no retry, timeout, or failure-mode concern — the same category as parse_test.go's existing TestTheShippedTemplateParses, which was never asked to hide its os.ReadFile behind an interface.
- **Fitness Function**: judgment-only — The files are documentation shipped in the same repo and read once per test run with no retry, timeout, or failure-mode concern — the same category as parse_test.go's existing TestTheShippedTemplateParses, which was never asked to hide its os.ReadFile behind an interface.

## Component Placement

- `vocabulary_test.go` → Test (same-package white-box test, not a Clean Architecture layer) layer, `internal/intake` — Needs direct access to vocabulary.go's unexported closed-vocabulary sets; sits beside check_test.go, parse_test.go, validate_test.go as another test file in the same package.
- `mismatched_vocabulary_template.md` → Test fixture (testdata) layer, `internal/intake/testdata` — Go convention: fixtures the test binary reads but that are not Go source live in a testdata/ subdirectory, which `go build` and `go vet` both ignore by default.

## Bounded Context

- **Owning Context**: intake
- **Context Crossings**: templates/claude-feature-team/intake/TEMPLATE.md and README.md — documentation owned by the template directory, read (never written) by a new test in internal/intake/. No import edge is created; it is a test-time file read of plain text, identical in kind to the existing TestTheShippedTemplateParses.

## Stability Design

None

## Observability Design

None

## Layer Boundary Checks

- [PASS] internal/intake must not import outer-layer/framework packages (Clean Architecture dependency direction, architecture-guardrails.md #1) — The new test file's only imports are stdlib (os, path/filepath, runtime, strings, sort, testing) plus the intake package itself. It reads two markdown files as plain bytes via os.ReadFile — the same mechanism parse_test.go's TestTheShippedTemplateParses already uses — so no new import edge is created between internal/intake and templates/claude-feature-team/intake/. Checked by reading vocabulary.go, validate.go and parse_test.go directly; the mcp__loom__* graph tools were not invoked because this change is confined to one package and one new test file, well below the threshold where the dependency graph adds signal over a direct read. Stated here per the architect's standing instruction to say when the fallback path was used.

## Anti-Pattern Check

- Shotgun surgery: not found — The three existing copies of the vocabulary (Go sets, TEMPLATE.md comments, README table) already require a three-way edit when a value changes; this feature does not fix that, it only adds a test that catches drift between them. No new shotgun-surgery edge is introduced by this change itself.
- God object / anemic domain model: not found — vocabulary.go stays a small set of named constants and maps; the new test does not add behavior to it, only reads it.

## Fitness Functions

- **The template's per-field comment list and the README's status table stay equal to the matching Go closed-vocabulary set**
  - **Verification**: go test ./internal/intake/... (TestTemplateVocabularyMatchesGoSets, TestReadmeStatusTableMatchesGoSets) — already run by the project's existing CI test step, no new CI wiring needed
  - **Owner**: Developer
- **The vocabulary-comparison check can actually detect a real mismatch, not just pass vacuously**
  - **Verification**: go test -run TestVocabularyCheckCatchesAMismatch ./internal/intake/... against the deliberately-wrong fixture in internal/intake/testdata/mismatched_vocabulary_template.md
  - **Owner**: Developer
- **Cyclomatic complexity < 7 for every new function introduced in vocabulary_test.go**
  - **Verification**: golangci-lint / gocyclo (project's existing lint config) or the analyze-complexity skill, run against internal/intake/vocabulary_test.go
  - **Owner**: Developer

## Refactoring Opportunities

None

## Developer Handoff Notes

- Declare the new file as `package intake` (same package as vocabulary.go), not `intake_test` — it needs direct access to knownTypes/knownStatuses/knownSources/knownTrust/knownFinishes/knownAutonomies.
- Reuse validate.go's existing field-table idiom: a slice of {name, extracted-values, allowed-set} structs iterated once, mirroring checkVocabulary — this keeps both TestTemplateVocabularyMatchesGoSets and TestReadmeStatusTableMatchesGoSets table-driven and short.
- Split parsing into named helpers (extractFieldComment, cutAtEmDash, splitPipeList, parseStatusTableColumn, assertSetsEqual) so no single function approaches the complexity-7 ceiling; assertSetsEqual should be the one place that produces the 'field X, document Y, missing on side Z' failure message, shared by both the TEMPLATE.md and README.md checks.
- Resolve both file paths the same way TestTheShippedTemplateParses does in parse_test.go: runtime.Caller(0) + filepath.Join(filepath.Dir(here), "..", "..", "templates", "claude-feature-team", "intake", "TEMPLATE.md"/"README.md").
- AC4 requires failing loudly, never silently passing, when a field's comment can't be found or parsed — use t.Fatalf naming the field, not a skipped subtest or a swallowed error.
- internal/intake/testdata/mismatched_vocabulary_template.md should be a static copy of TEMPLATE.md with exactly one field's trailing comment edited to disagree with its Go set (e.g. add a phantom status `queued`); do not generate it at test time.
- No file outside internal/intake/ should change except this run's own delivery record under docs/features/i-4-vocabulary-test/ — confirm with git status before finishing (AC6).

## Open Architectural Questions

- Should 'Closed vocabulary' (defined in this feature's own spec, Domain Language section) be promoted to shared/DOMAIN_DICTIONARY.md? The analyst flagged this and deferred it: the spec's AC6 restricts file changes to internal/intake/ plus the delivery record, so this architecture leaves DOMAIN_DICTIONARY.md untouched for this feature. A human can promote the term in a separate, narrowly-scoped chore.
