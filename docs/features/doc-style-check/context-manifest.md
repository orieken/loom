# Context Manifest: doc-style-check

## Scope

- **Tier**: analyst
- **Built for**: analyst

## Pinned Files

- `cmd/loom/cmd/docs.go` — The existing `loom docs` command family (plan/approve/apply) that `docs check` must extend; shows the cobra.Command + init() registration pattern this feature's new subcommand follows.
- `cmd/loom/cmd/docs_test.go` — Existing test conventions for `loom docs` subcommands (captured IO, repo fixtures) that the new `docs check` command's tests should match.
- `internal/intake/parse.go` — Reusable pattern for splitting a leading YAML frontmatter fence from a markdown body (splitFrontmatter) — directly applicable to AC2's requirement to ignore frontmatter.
- `internal/patternspec/issue.go` — Existing Issue type (File/Field/Message/Severity, String() renders "file: field: message") — the closest in-repo precedent for the `file:line: rule: excerpt` finding format AC1 requires.
- `shared/DOMAIN_DICTIONARY.md` — The actual data file the check must read; shows the real markdown table shape (bold Term, Description, "Synonyms to AVOID" column with comma-separated entries and parenthetical notes) that AC5's parser must handle.
- `shared/rules/writing-style.md` — The normative rule this feature mechanizes — the analyst must map each bullet (sentence length, active voice, noun clusters, modals, plain vocabulary, one-term-one-meaning) to a specific, deterministic check or an explicit 'left for a reviewer' note.
- `docs/features/delivery-record-relative-paths/analysis.md` — Distance-independent prior delivery in the same Owning Context (Documentation Knowledge Base) — no retrospective.md exists for it, so there are no What-Went-Poorly/What-To-Improve lessons to carry forward, but it shows how a prior doc-tooling feature in this context scoped its Technical Breakdown.

## Token Budget

- **Status**: OK — ~16431 tokens, 13.7% of the analyst tier budget (120000 tokens)
- Measured by loom from the files above (bytes ÷ 4), not estimated by an agent.

| File | Bytes | ~Tokens |
|---|---:|---:|
| `cmd/loom/cmd/docs.go` | 8174 | 2043 |
| `cmd/loom/cmd/docs_test.go` | 8161 | 2040 |
| `internal/intake/parse.go` | 2101 | 525 |
| `internal/patternspec/issue.go` | 1578 | 394 |
| `shared/DOMAIN_DICTIONARY.md` | 29692 | 7423 |
| `shared/rules/writing-style.md` | 1863 | 465 |
| `docs/features/delivery-record-relative-paths/analysis.md` | 14167 | 3541 |

## Knowledge Items

None

## ADRs

None

## Pruned

- `internal/genblock/genblock.go — handles loom:generated hash-marker blocks, not prose fenced-code-span detection; different concern despite surface similarity to 'ignore fenced code'.`
- `internal/testlint/module.go — Go-AST 'test can fail' linter; belongs to Craftsmanship Governance/test tooling, a different bounded context than this doc-prose checker.`
- `internal/repodoc/vocabulary.go — loads a YAML tag vocabulary with project-level override; DOMAIN_DICTIONARY.md is a fixed-path markdown table, not YAML, so the loading shape doesn't transfer.`
- `shared/contracts/analysis-contract.md and internal/state/analysis_state.go — the analyst's own output contract/schema; under `loom run` this is appended automatically as the invocation's OUTPUT CONTRACT, so pinning it here would be redundant.`
- `docs/adrs/ADR-001-adopt-rag-friendly-docs-structure.md — about docs directory structure, not prose writing-style mechanics; no KI or ADR specifically about Simplified Technical English enforcement was found in shared/knowledge or docs/adrs.`
