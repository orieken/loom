# ADR-008: Define "Tested" by What the Tests Can Catch, Measured on the Change

## Status

Accepted — amended 2026-09-22 (clause 3 narrowed; see **Amendment** at the end)

## Date

2026-09-22

## Deciders

Oscar Rieken

## Context

The framework's test requirement is a single number: **unit test coverage ≥ 85%**, "non-negotiable",
in `CLAUDE.md` and `shared/rules/testing-conventions.md`. Two facts make that number weaker than it
reads.

**It measures execution, not detection.** Coverage says a line ran while a test was running. It says
nothing about whether any assertion depended on it. For a human author that gap is mostly theoretical;
for an agent told to reach 85%, it is the cheapest path. A test that calls the function and asserts no
error was returned covers every line it passes through and catches almost nothing. Coverage is a
target an agent will meet, which is exactly why it stops measuring what it was chosen to measure.

**It is not what this repository enforces on itself.** The CI coverage ratchet for the `loom` module
(`framework-ci.yml`, roadmap M0.2) is set at **66.3%**, and its own comment calls 85% "aspirational". A
whole-codebase percentage cannot be enforced on a codebase that is already below it without either
blocking all work or lowering the number, so the rule is honoured as a ratchet and described as a
floor.

The repository has already met the failure this ADR is about, three times in one week, all in its own
checks: the C1 fitness test that saw `any` but not `interface{}`, the A7 check, and the 2026-09-22
drift script that printed `DRIFT` and exited 0. Each passed while testing less than it claimed. Each
was caught only by deliberately breaking the thing it guarded. The practices that caught them already
exist — `backfill-unit-tests` step 6 proves a characterization net by mutation, and `code-reviewer`
asks of every new test "would it fail if the behaviour were broken?" — but one is a manual step in one
skill and the other is judgment.

A test's value is what it can catch. That is measurable, and it should be the requirement.

## Decision

A change is **tested** when all three hold for the code it changes:

1. **Diff coverage ≥ 85%.** At least 85% of the executable statements added or modified by the change
   are executed by the test suite. The whole-codebase ratchet stays as a separate, one-way floor; it
   stops being the thing described as 85%.
2. **Mutation score on the diff meets the project's floor.** Small faults are injected into the changed
   statements; the surviving mutants are reported. Mutants that fail to apply or fail to compile are
   **reported separately and never counted as killed** — a mutant that did not land proves nothing,
   the lesson `backfill-unit-tests` step 6 already records. There is no universal number: the floor
   starts report-only, is set from measurement, and ratchets upward the way the coverage floor does.
3. **No test that cannot fail.** A test with no assertion, or whose only assertion is that no error
   occurred or a value is non-nil, is rejected mechanically, not left to review.
   *(Narrowed by the amendment below: only the first half is mechanical.)*

`code-reviewer`'s "would it fail?" judgement and `shared/rules/test-repair-contract.md` stay as they
are — the human-readable layer over the mechanical one.

This applies in two places: to this repository's own Go code as CI fitness functions, and to consumer
projects as the rule `testing-conventions.md` ships, with per-language tooling named in each
`*-conventions.md` file.

## Consequences

- **Easier**: 85% becomes a number that can actually gate here — it applies to new work, which can
  meet it, rather than to a legacy total, which cannot. Padding stops paying: a test that exercises
  code without depending on it raises coverage and leaves mutants alive, and the second number shows
  it. The question reviewers have been asking by hand gets a measured answer.
- **Harder**: Mutation testing is slow and per-language; scoping it to the diff is what keeps it
  affordable, and it still adds CI minutes. Choosing and maintaining a mutation tool per supported
  language is real work, and some ecosystems have thin tooling. Diff measurement needs a merge base,
  so it behaves differently on direct pushes to `main` than on pull requests.
- **Changed**: The rule text changes from "coverage ≥ 85%" to the three conditions above.
  `testing-conventions.md`, `CLAUDE.md`'s non-negotiables, the `run-tests` skill, and each language's
  conventions file are updated to match. Mutation proof stops being a manual step in one skill and
  becomes a check the framework runs.

## Alternatives Considered

| Option | Why rejected |
|---|---|
| Keep whole-codebase coverage ≥ 85% | Not enforceable here (66.3%), and it is the measure an agent pads most cheaply. |
| Raise the coverage number (90%, 95%) | Raises the padding, not the detection. The defect is the kind of measure, not its level. |
| Rely on `code-reviewer`'s "would it fail?" judgement alone | Already in place, and the three vacuous checks above got past review. Judgement scales badly and cannot be trended. |
| Mutation-test the whole codebase | Correct and unaffordable on every change; the diff is where the new risk is. A periodic full run can be added later as a separate item. |
| Adopt a fixed mutation score (e.g. 80%) now | No measurement exists to choose it from. The coverage ratchet's history shows what happens to a number set by aspiration. |

## Fitness Function

Three CI checks on the `loom` module, each proved red before it gates (roadmap items L3.58–L3.60):

- **Diff coverage** — fails the build when changed statements are below 85%.
- **Mutation on the diff** — report-only until a baseline exists, then fails below the recorded floor;
  un-applied mutants listed separately.
- **Assertion-free test lint** — an AST check in the style of `internal/state/untyped_test.go` that
  fails on a test function with no assertion.

For consumer projects this is the shipped rule plus the tooling named per language; loom cannot run
their CI, so for them it is enforced to the extent the project wires it (documented, not assumed).

## Amendment — 2026-09-22

Clause 3 claimed more than a mechanical check can deliver. Building it (roadmap L3.60) showed:

- **Decidable, and shipped:** a test with **no path to a failure** — no `t.Error`/`t.Fatal`/`t.Fail`
  reachable from its body, its subtests, or the helpers it hands its testing value to. Enforced by
  `internal/testlint` over the whole module.
- **Not decidable in plain Go, and dropped from the mechanical rule:** a test "whose only assertion is
  that no error occurred". In Go's standard style the error check is frequently the assertion itself
  — a failing `os.Stat(archived)` *is* "the file was not archived", a failing `json.Unmarshal` *is*
  "the output does not parse". A survey flagged 22 tests in this module on that rule and every one
  inspected was a real assertion. The decidable form — testify's `NoError`/`NotNil` as a test's only
  assertions — stays valid guidance for projects that use testify; this repository does not.

Where a test asserts something weak rather than nothing, clause 2 (mutation on the diff) is the check
that catches it. Clause 3 now reads, for enforcement purposes: **no test without a path to a failure.**

This is an amendment rather than a superseding ADR because the decision is unchanged — "tested" is
defined by what tests can catch — and only the reach of one mechanical check is corrected, on the day
the ADR was accepted and before anything depended on the dropped half.
