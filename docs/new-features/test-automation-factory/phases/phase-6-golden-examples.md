# Phase 6 — Golden Examples (Exemplars)

## Goal
Pick real tests as models, mark them, list them in a manifest, and check the two never drift apart. Skills cite exemplars by the manifest. They never copy a test inline.

## Inputs
- `templates/EXEMPLAR-ENTRY.md`.
- The project's existing tests (existing project) or the seed examples in `examples/` (new project).
- The skills from phase 5.

## Steps
1. **Choose.** For each level (`ui-e2e`, `api`, `unit`), pick one test that already runs and is the best model.
   - Existing project: pick from the suite. Prefer a test that is small, readable, and follows the pattern.
   - New project: copy the seed from `examples/` into `tests/`, adapt it to a real endpoint or page, and make it run.
   - No target application yet for `ui-e2e`: run the example against a small local fixture server (a few lines of HTTP), in a real browser. Say in the manifest that it is a fixture. Do not leave the level without an exemplar, and do not mark a test that only compiles.
   - Keep browser tests in `*.ui.spec.ts` files with their own runner config and script (`test:ui`), so the default test run needs no browser.
2. **Qualify.** Each choice MUST pass all of these. Record the evidence.
   - It runs and is not skipped.
   - It can fail: change the behavior it checks, see it fail, restore.
   - It carries an issue reference and an acceptance-criterion reference.
   - No function in it has cyclomatic complexity of 7 or more.
   - Its file name follows `name.type.extension`.
3. **Mark twice.** Add the language-native annotation to the test. Add an entry to `.claude/exemplars.yaml`
   with `test`, `file`, `language`, `level`, `blueprint`, `demonstrates`, and `can-fail` (one line: what you broke, that the test failed, and that you restored it). Add the mark on a comment or tag line directly above the test, so a scanner can tell a mark from text that merely mentions one.
4. **Write `scripts/check-exemplars.<ext>`.** It MUST:
   - fail if an annotated test has no manifest entry, and fail if an entry has no annotated test;
   - fail if the file in an entry does not exist;
   - fail if a manifest entry lacks any required field, including `can-fail`;
   - fail if the marked test lacks its issue annotation, or its criterion annotation (a Gherkin scenario needs only the issue tag, because the scenario name is the criterion);
   - count a mark only on a comment or tag line, never inside a string, and build test fixtures from joined lines so the script does not scan its own fixtures as marks;
   - fail if an entry's `blueprint` is missing from the registry;
   - print a sha256 digest of each exemplar file;
   - compare each digest with the one in the manifest, when present, and report a change as a warning, not a failure.
5. **Complete the `exemplar-audit` skill.** It reads the manifest and checks each entry against the disqualifier list in `templates/EXEMPLAR-ENTRY.md`. It reports findings. It never edits.
6. **Update the blueprints.** Each blueprint's `Exemplars` section lists the exemplar names for that type.
7. **Document the limit.** Add a note to `.claude/exemplars.yaml`: exemplars are not edit-protected. A model that drifts misleads agents, so run `exemplar-audit` after a burst of test changes.

## Files
`.claude/exemplars.yaml`, `scripts/check-exemplars.<ext>`, tests for the script, the three annotated tests, updated blueprints and the `exemplar-audit` skill.

## Verification
- `check-exemplars` passes. Remove one annotation: it fails. Restore it.
- Run the `test-audit` skill: the "no exemplars yet" finding is gone.
- Run `exemplar-audit`: no entry is disqualified.
- Each exemplar fails when you break the behavior it checks, and passes again when you restore it. Record that in its `can-fail` field.
- Changed-line coverage of the script is at least 85%, and each test was shown to fail once.

## Budget
`max-usd: 5`, `max-attempts: 2`, `max-hours: 2`.

## Time-box
Stage timeout 30 minutes. Qualification (step 2) involves running tests; if a suite run alone takes more than 10 minutes, qualify with a targeted run and record that.
