---
name: backfill-unit-tests
description: Writes unit tests for existing code without modifying it, then automatically runs code-reviewer against just the new test files -- for raising coverage on working code or building a characterization-test safety net around legacy code before a refactor/migration. Coordinates the unit-tester and code-reviewer agents.
triggers:
  keywords: ["backfill tests", "add unit tests to", "raise coverage", "characterization tests", "wrap tests around legacy", "/backfill-unit-tests"]
  intentPatterns: ["Add unit tests to *", "Backfill coverage for *", "Write characterization tests for *", "/backfill-unit-tests *"]
standalone: true
---

## When To Use
When the user wants tests added to code that already exists and should not change — raising coverage on
trusted code, or building a characterization-test safety net before refactoring or migrating legacy code.
Accepts a file, directory, or module.

Do NOT use when the code doesn't exist yet or is expected to change to satisfy the tests — use
`test-driven-developer` instead (tests-first, code conforms to the tests). Do NOT use inside a
`deliver-feature` run — `qa-engineer` already covers testing for a feature just implemented, including its
own Legacy Code section for touching pre-existing files in that context; this skill is for standalone
coverage/characterization work with no accompanying feature delivery.

## Context To Load First
1. `CLAUDE.md` — project constraints and clean code rules
2. `ARCHITECTURE_RULES.md` — architectural guardrails, particularly the 85% coverage threshold
3. `shared/rules/approval-gates.md` — gate #6 (Writing Files out of Boundary) governs any seam `unit-tester`
   flags as a blocker

## Process
1. **Resolve the scope** — the file, directory, or module the user pointed at.
2. **Check whether `context-engineer` should run first.** It's meant to self-invoke proactively per its own
   trigger criteria (`shared/skills/context-engineer/SKILL.md`: 3+ files, or a codebase area not already
   scoped this session) — don't skip that just because this skill doesn't explicitly orchestrate it the way
   it orchestrates `unit-tester`/`code-reviewer`. In practice: a single already-familiar file being
   backfilled for coverage rarely needs it; a directory or module being characterized for a legacy
   refactor/migration almost always does, and benefits specifically from its bounded-context mapping and
   prior-delivery lookup (`shared/agents/context-engineer.md` steps 3 and 6) — context `unit-tester`'s own
   `search-ki` step doesn't cover.
3. **Capture baseline coverage** — invoke `run-tests` for the target scope before any new tests exist, so
   the final report shows a real before/after delta. Skip only if the target has no existing test
   infrastructure to run at all (typical for genuinely untested legacy code) — note "no baseline" rather
   than fabricating a 0%.
4. **Run `unit-tester`** against the resolved scope. It determines coverage-backfill vs. characterization
   mode itself and produces `.claude/feature-workspace/unit-test-report.md`.
5. **Run `code-reviewer`** against only the new/modified test files from step 4 — never against the
   untouched source. This is the counterbalance `unit-tester` doesn't have on its own: nothing else checks
   whether the tests it wrote are well-structured, correctly scoped, and free of the complexity/SOLID issues
   this framework flags everywhere else.
6. **Prove the net with mutation** — a characterization net that has never been tested against a change is
   an assumption, and green is not evidence. Pick **three** lines of the target that carry behavior (a
   comparison, a returned value, a branch condition — not a log line or an import). Change each one, run
   the new tests, and confirm **each mutation fails at least one test**. Revert every mutation.

   **Run this in a throwaway git worktree, never in the working tree:**

   ```bash
   git worktree add --detach "$(mktemp -d)/mutation-check" HEAD
   # mutate, run the suite in that directory, record which tests failed
   git worktree remove --force <path>
   ```

   This is why the check belongs to the skill and not to `unit-tester`: that agent may never modify
   source, full stop, and a worktree keeps the rule absolute rather than granting it an exception that a
   crashed run would leave behind as mutated source. If the project is not a git repository, copy the
   scope to a temp directory instead — never mutate in place.

   **A mutant must apply AND still build before its result means anything.** Two different
   failures, and only one of them looks like a failure.

   - **It did not apply.** In the worktree, `git diff --quiet` must report a change. An empty diff
     means the edit did not match and the run that follows proves nothing. A mutation whose anchor
     missed produces a passing test that reads exactly like "the net does not cover this line".
   - **It applied and broke the build.** This is the dangerous one, because it fails in the
     *reassuring* direction. A compile error makes the test command exit non-zero, and a non-zero
     exit is what "the mutant was caught" looks like — so the check reports the net as strong when
     nothing exercised it at all. Run the project's build after mutating and **before** running the
     tests (`go build ./...`, `tsc --noEmit`, `cargo check`, `mvn -q compile`). If it fails, the
     mutant is invalid: revert it and pick another line.

     Two ways this happens, both observed rather than imagined:
     - the mutation removes the last use of a variable or import, which some languages reject
       outright — in Go, deleting a call can turn any mutant into a build error
     - the mutation introduces a cycle or type error the compiler refuses, so the compiler is what
       rejected the change and the test was never consulted

   Neither failure announces itself. "The tests failed" only means the net caught something once
   you know the mutant landed and the project still compiled.
   **A mutant that never landed and an uncovered line are indistinguishable from the test output
   alone.** The diff is what separates them.

   **Mutate what an assertion depends on**, not whatever is easiest to edit: a compared value, a
   returned value, a branch condition. Adding a statement, or editing something the function's result
   does not flow through, produces a *live diff with no behavioural change* — which survives every
   test for the same reason a vacuous test passes, and looks identical to a real gap. A deferred
   write to a local in a function with an unnamed return is the example that got past a careful
   reader.

   **A surviving mutation is a blocking finding — once the mutant is shown to be live.** A line you
   could change with every test still green is a line the net does not cover. Before recording that,
   mutate the same line a second way: if the second mutant is also survived, the gap is real. Then
   either add the test that catches it, or record the gap explicitly in the report's NOT COVERED
   list. Do not report the backfill complete with a surviving mutation unrecorded.
7. **Capture final coverage** — invoke `run-tests` again for the same scope, compute the delta against step
   3's baseline.
8. **Produce the combined report** — display to the user.

## Output Format

```markdown
# Test Backfill: [target]

## Mode
Coverage backfill | Characterization (legacy/migration)

## Coverage Delta
- **Before**: N% (or "No baseline — untested")
- **After**: N%

## Tests Written
- `tests/test_x.py` — [what it covers]

## Code Review (tests only)
| Check | Result | Details |
|---|---|---|
| Cyclomatic complexity | PASS / FAIL | [specifics] |
| Test independence / no brittle coupling | PASS / FAIL | [specifics] |
| Naming conventions | PASS / FAIL | [specifics] |

## Behavior Notes (characterization mode only)
- [Bug-like behavior captured as-is, not fixed] / "N/A"

## Mutation Verification
- **Lines mutated**: [file:line x3, and what was changed at each — "returned true unconditionally",
  not "mutated"]
- **Mutant confirmed applied**: [yes, `git diff` non-empty for each] — a mutant that never landed
  reads exactly like an uncovered line
- **Result**: [each mutation failed at least one test] / [N survived, re-mutated a second way, still
  survived — listed under NOT COVERED]

## Blocked by Structure
- [Code that needs a seam to be testable — proposed seam, awaiting explicit "approve file write"] / "None"

## Summary
[2-3 sentences: what's covered now, what's still not, whether anything needs the user's attention]
```

## Guardrails
- Never let `unit-tester` or `code-reviewer` modify the source under test — both operate read-only against
  it. A seam is the one structural exception, and it requires the explicit approval described in
  `shared/rules/approval-gates.md` gate #6 — never performed automatically by this skill.
- Never mark the backfill complete if `code-reviewer` finds a Critical-severity issue in the new tests —
  fix the tests (not the source) before reporting done.
- Never mutate the working tree. Step 6 runs in a throwaway worktree or a temp copy, and every mutation
  is reverted by discarding it — not by editing the line back.
- In characterization mode, never "correct" behavior the tests capture — that's what the next
  refactor/migration is for, not this skill.

## Standalone Mode
Pure local analysis + test execution — no external calls required beyond whatever the project's own test
runner needs.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
