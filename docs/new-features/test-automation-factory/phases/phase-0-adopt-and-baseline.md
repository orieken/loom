# Phase 0 — Adopt and Baseline

## Goal
Learn what the project is, record where it stands, and write the first decision record. Change no product code.

## Inputs
- The repository root.
- `BLUEPRINT.md` in this pack.

## Steps
1. **Detect the project state.** Decide: new (no test code), or existing (tests exist). Report your evidence.
2. **Detect facts.** Record the language, package manager, test runner, coverage tool, linter, CI system, and the
   command that runs the whole suite. Write unknowns as `unknown`. Do not guess.
3. **Existing project only: baseline.**
   - Run the suite. Record pass, fail, and skipped counts.
   - Record overall coverage and the five least-covered files.
   - Record the ten functions with the highest cyclomatic complexity.
   - Do not fix anything. Write findings down. If you find legacy code that lacks tests, note it for a later order.
4. **New project only:** propose a minimal layout from the `BLUEPRINT.md` directory tree. Ask the human to choose
   the language and test runner, because later phases write scripts in that language. Do not pick one from the tools
   installed on this machine. Wait for both answers before you create folders. Record the choice in `BASELINE.md`.
5. **Write `DOMAIN_DICTIONARY.md`.** One entry per term in the system under test. Add a "Synonyms to avoid" line
   per term. Leave a stub when you know nothing yet.
6. **Write `docs/adr/ADR-000-test-automation-factory.md`** from the seed in `BLUEPRINT.md`. Fill the project name.
7. **Write `docs/test-factory/BASELINE.md`** with the numbers from steps 2 and 3, the date, and the commit hash.

## Files
- `DOMAIN_DICTIONARY.md`
- `docs/adr/ADR-000-test-automation-factory.md`
- `docs/test-factory/BASELINE.md`

## Verification
- `BASELINE.md` lists every fact from step 2, each with a source (a file path or a command output).
- The suite result in `BASELINE.md` matches a fresh run.
- `git diff --stat` shows only the three files above. Commit this pack (`docs/test-factory/`) in its own commit
  before you start, so that the diff is clean.
- A repository with no commits has no hash. Write `none` in `BASELINE.md` in that case.

## Budget
`max-usd: 2`, `max-attempts: 1`, `max-hours: 1`. Park and ask a human if you cannot find the test command.

## Time-box
Stage timeout 30 minutes. Running the full suite counts toward the time-box. If the suite takes longer than 20 minutes,
record that fact, run a sample, and mark the baseline `partial`.
