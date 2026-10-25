# Context Manifest: i-4-vocabulary-test

## Scope

- **Tier**: developer
- **Built for**: developer

## Pinned Files

- `internal/intake/vocabulary.go` — The six unexported Go sets (knownTypes, knownStatuses, knownSources, knownTrust, knownFinishes, knownAutonomies) the new test must read directly, since it lives in package intake — this is the change's seed file.
- `templates/claude-feature-team/intake/TEMPLATE.md` — The other side of the comparison: the trailing `# a | b | c` comment on each of the six frontmatter fields (lines 9, 11, 15, 17, 19, 20) that the new test must parse and diff against vocabulary.go's sets.
- `templates/claude-feature-team/intake/README.md` — Holds the `status` table (the Lifecycle table, lines ~21-25) whose first column the second AC requires reading and comparing against knownStatuses.
- `internal/intake/order.go` — Defines Order and SchemaVersion and the package doc-comment; the developer needs this to know the existing field names and package conventions before adding a new file to the same package.
- `internal/intake/validate.go` — Distance-1 exact dependent: the only existing consumer of vocabulary.go's known* sets (checkVocabulary, listOf). Shows the established pattern for iterating the six fields that the new test should mirror for consistency.
- `internal/intake/parse_test.go` — TestTheShippedTemplateParses (lines 122-136) is the existing precedent for a test in this package reading the shipped TEMPLATE.md via runtime.Caller-relative pathing — the new test needs the same path-resolution pattern.
- `internal/intake/validate_test.go` — Shows this package's table-driven test conventions and the hasViolation-style helper pattern the new test should follow for reporting per-field mismatches with a readable failure message.
- `internal/intake/check_test.go` — Smallest example of this package's test style (table setup, naming) for a newer, less load-bearing file than validate_test.go — quick second reference point.

## Token Budget

- **Status**: OK — ~8647 tokens, 5.4% of the developer tier budget (160000 tokens)
- Measured by loom from the files above (bytes ÷ 4), not estimated by an agent.

| File | Bytes | ~Tokens |
|---|---:|---:|
| `internal/intake/vocabulary.go` | 1491 | 372 |
| `templates/claude-feature-team/intake/TEMPLATE.md` | 5569 | 1392 |
| `templates/claude-feature-team/intake/README.md` | 5735 | 1433 |
| `internal/intake/order.go` | 2414 | 603 |
| `internal/intake/validate.go` | 7107 | 1776 |
| `internal/intake/parse_test.go` | 4237 | 1059 |
| `internal/intake/validate_test.go` | 6553 | 1638 |
| `internal/intake/check_test.go` | 1497 | 374 |

## Knowledge Items

None

## ADRs

None

## Pruned

- `docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md — covers claim-time pickup and run state, a different concern (factory scheduling) from this task's closed-vocabulary consistency check; excluded as a different bounded context within the same package.`
- `internal/intake/check.go — Check() composes Parse+Validate+scanBody but does not touch vocabulary; not needed to write a vocabulary-comparison test.`
- `internal/intake/parse.go — frontmatter splitting mechanics are orthogonal to comparing a closed vocabulary; the new test only needs Parse's already-demonstrated usage pattern from parse_test.go, not its internals.`
