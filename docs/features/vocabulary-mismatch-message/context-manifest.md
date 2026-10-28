# Context Manifest: vocabulary-mismatch-message

## Scope

- **Tier**: analyst
- **Built for**: analyst

## Pinned Files

- `internal/intake/vocabulary_test.go` — The file the chore changes. Contains errorRecorder (today a bool, must become a kept-messages store), assertVocabularyMatches's test double, vocabularyReporter, and TestVocabularyCheckCatchesAMismatch — the analyst needs the current shape of all four before writing acceptance criteria and a task breakdown.
- `internal/intake/vocabulary.go` — Defines assertVocabularyMatches and the closed-vocabulary Go sets (status, type, source, trust, finish, autonomy) it compares against. The analyst needs this to confirm which field and value the fixture actually mismatches (status/queued) and to describe the message format AC3 depends on, rather than taking the claim's wording on faith.
- `internal/intake/testdata/mismatched_vocabulary_template.md` — The fixture TestVocabularyCheckCatchesAMismatch runs against. The analysis's acceptance criteria must match this fixture's real frontmatter content (which field actually diverges, and what value), not an assumption from the spec text.
- `docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md` — The chore's own rationale — a negative-control test whose double discards the message content is 'tested' only in the sense this ADR warns against. The analyst should ground the acceptance criteria and fitness function in this ADR's definition rather than re-deriving it.
- `docs/features/i-4-vocabulary-test/analysis.md` — Prior delivery in the same bounded context (intake) — this is the feature whose AC3 first required the failure message to name the field, document, and missing values. It is the source of the requirement this chore is closing a gap on, so the analyst should read it rather than take the claim's paraphrase as the only record of what AC3 said.

## Token Budget

- **Status**: OK — ~8851 tokens, 7.4% of the analyst tier budget (120000 tokens)
- Measured by loom from the files above (bytes ÷ 4), not estimated by an agent.

| File | Bytes | ~Tokens |
|---|---:|---:|
| `internal/intake/vocabulary_test.go` | 9178 | 2294 |
| `internal/intake/vocabulary.go` | 1491 | 372 |
| `internal/intake/testdata/mismatched_vocabulary_template.md` | 5578 | 1394 |
| `docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md` | 7678 | 1919 |
| `docs/features/i-4-vocabulary-test/analysis.md` | 11489 | 2872 |

## Knowledge Items

None

## ADRs

- `docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md`

## Pruned

- `internal/intake/parse.go — defines splitFrontmatter, used unchanged by frontmatterOf; not modified by this chore and its contract isn't in question`
- `internal/intake/check.go, internal/intake/check_test.go, internal/intake/validate.go, internal/intake/validate_test.go, internal/intake/order.go — same package but unrelated to the vocabulary comparison path; out of scope per the claim's scope.paths and 'Out of Scope' note`
- `templates/claude-feature-team/intake/TEMPLATE.md, templates/claude-feature-team/intake/README.md — read by TestTemplateVocabularyMatchesGoSets and TestReadmeStatusTableMatchesGoSets, neither of which this chore touches`
- `docs/features/i-4-vocabulary-test/code-review-report.md, qa-report.md, implementation-notes.md, security-report.md, tech-writer.md, performance-engineer.md — supporting pipeline artifacts of the prior delivery; the AC3 rationale the analyst needs is already in that feature's own analysis.md, which is pinned, so reading the rest would duplicate rather than add signal`
- `.claude/feature-workspace/vocabulary-mismatch-message/analysis.md, architecture-notes.md, context-manifest.md, acceptance-scenarios.md — this feature's own artifacts from the pipeline pass before the current SPEC_CHANGED restart (the respec was a budget bump in the claim's frontmatter, max-usd 15 to 20, confirmed via `git show d5221aa` — not a scope change); the analyst should re-derive the analysis from the claim and source rather than anchor on stale prior output, and architecture-notes.md in particular is produced by a stage that runs after the analyst, so pinning it here would be circular`
