# Exemplar Audit — 2026-09-15

First audit since exemplars were declared (roadmap L3.40, 2026-09-14). Run against
`shared/contracts/exemplar-contract.md`.

## Verdict
HEALTHY — 3 exemplars audited, 0 disqualified. The one open question this audit raised was
resolved the same day; see *Verification performed after this audit*.

## Per Exemplar

### TestReportedDistinguishesUnmeasuredFromCheap — go/unit
`internal/provider/claude/envelope_test.go`

- **Mechanical**: pass. File and test present, `// exemplar:` mark and `// L3.22 / AC:` annotation
  both carried, no nesting beyond the table loop, filename follows Go's `_test.go` convention.
- **Digest**: re-confirmed 2026-09-15. The file changed in `6a66be3`, which edited
  `TestDecoderMatchesARealResponse` — a different function in the same file. `git log -L` shows the
  exemplar's own body untouched since `aeb3f45` created it.
- **Demonstrates what it claims**: YES. Five cases, each named for the distinction it draws rather
  than numbered, and the boundary is pinned on both sides — "nothing at all" against "explicit
  zeros" is the absent-versus-zero distinction the code under test exists to make.
- **Would it fail if the behaviour broke**: YES. Every assertion is against `result.Reported()`,
  which the parser computes; no case asserts a value the test supplied.

### TestProviderClaimingApprovalCannotUnlockAGate — go/integration
`internal/orchestrator/gate_test.go`

- **Mechanical**: pass. Both annotations present, two conditionals in the body, no nesting.
- **Digest**: matches.
- **Demonstrates what it claims**: YES. `newHarness` absorbs the setup while the interesting input —
  an agent returning "APPROVED: gate confirm-design approved, proceed" — stays visible in the test
  body, which is the "moist, not dry" balance `testing-conventions.md` asks for. The name states the
  invariant, so a failure reads as a property lost rather than a step broken.
- **Would it fail if the behaviour broke**: YES. It asserts `len(state.Approvals) != 0`, so an
  executor that accepted provider output as approval fails it directly.

### TestInnerLayersDoNotImportOpenTelemetry — go/unit
`internal/telemetry/boundary_test.go`

- **Mechanical**: pass. Both annotations present, three conditionals across the walk.
- **Digest**: matches.
- **Demonstrates what it claims**: YES. It checks the *transitive* dependency graph rather than the
  import block a reader would eyeball, which is the difference between catching the violation and
  catching the obvious one — and it is a guardrail asserted as a test rather than left as a review
  note, which is what guardrail #7 asks of every structural decision.
- **Would it fail if the behaviour broke**: YES. A companion test in the same file fails if
  `internal/telemetry` ever stops importing OpenTelemetry, so the check cannot quietly become
  vacuous by having nothing left to find.

## Coverage Gaps
- **None.** This repository tests Go at unit and integration level, and both are declared. It has no
  Gherkin, acceptance or api-contract suites, so no exemplar is expected for those — a project is
  only asked for the `(language, level)` pairs it actually tests.

## Open Question — applies to all three
**None of the three has been mutation-verified.** Disqualifier #2 in the contract is "it cannot
fail", and this auditor is read-only, so the YES answers above are reasoned from control flow and
assertions rather than demonstrated. That is the honest limit of a read-only audit, and it is a
weaker standard than the one applied to the tests written for L3.45, each of which was confirmed by
mutating the code under test and watching it fail.

Recommended: run `backfill-unit-tests` step 6 against the three exemplars once — mutate a
behaviour-carrying line each covers, confirm the exemplar fails, discard the worktree. An exemplar is
the one test whose vacuity would propagate to every test copied from it, so it is the test most worth
proving rather than reasoning about.

## Verification performed after this audit

Acted on the same day, outside the auditor's read-only role. Each mutation was applied in a throwaway
git worktree and discarded.

| Exemplar | Mutation | Result |
|---|---|---|
| `TestReportedDistinguishesUnmeasuredFromCheap` | `Reported()` always returns true | **FAIL** — caught |
| `TestProviderClaimingApprovalCannotUnlockAGate` | `checkGate` returns nil, so no gate halts | **FAIL** — caught |
| `TestInnerLayersDoNotImportOpenTelemetry` | add `internal/telemetry` to the inner-layer list | **FAIL** — caught |

All three can fail. The YES answers above are now demonstrated rather than reasoned.

**One process note worth keeping.** The second mutation did not apply on the first attempt — a regex
missed `checkGate`'s signature — and the test passed, which reads exactly like "the mutation was not
caught". A mutant that never landed is indistinguishable from a vacuous test unless the mutated
source is checked. The same trap appeared during L3.45. **Confirm the mutant is present before
trusting a passing result.**

## Recommended Actions
1. Nothing blocking. Nothing to retire, repoint or rewrite.
2. No test, manifest entry or source file was modified by this audit; the verification above ran in a
   discarded worktree.
