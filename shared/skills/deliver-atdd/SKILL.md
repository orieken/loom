---
name: deliver-atdd
description: Runs an ATDD (Acceptance-Test-Driven Development) delivery loop -- qa-engineer writes scenarios, human reviews if configured, qa-engineer writes step definitions, human reviews if configured, developer implements to green autonomously, qa-engineer runs the full suite, human reviews before ship. Which review gates are active is per-project config, so a team can phase them out as trust is earned rather than living with fixed ceremony forever.
triggers:
  keywords: ["deliver-atdd", "atdd delivery", "acceptance test driven", "atdd loop"]
  intentPatterns: ["/deliver-atdd *", "Deliver via ATDD *", "Run ATDD loop on *"]
standalone: true
---

## When To Use
When the user wants a feature delivered via an explicit acceptance-test-first workflow (Gherkin scenarios
approved before step definitions, step definitions approved before implementation, autonomous inner
red-green loop to satisfy them) rather than `deliver-feature`'s 14-agent full-pipeline shape. Accepts a
feature file already produced by `spec-writer` or hand-written to `features/TEMPLATE.md`.

**Why this workflow specifically**: the acceptance tests are written from the acceptance criteria
before any implementation exists. `qa-engineer` writes the scenarios and step definitions; `developer`
implements against them. That independence catches "built the wrong thing", which tests derived from
the finished code cannot (ADR-009). It is not TDD design pressure — two agents sharing a model and an
analysis share most of what that separation withheld; see `docs/patterns/testing-pyramid.md`.

Do NOT use for features that need architectural review, security review, accessibility review, or the
other conditional agents `deliver-feature` gates on — use `deliver-feature` instead; it's a superset in
review coverage, not just a longer version of this. Do NOT use before a feature spec exists — run
`/spec-writer` first, same pre-pipeline gate `deliver-feature` respects.

## Context To Load First
1. The feature file (passed as argument or in `features/`)
2. `.claude/atdd-config.json` — per-project gate configuration; see "Config File" below. Create with
   all-active defaults if it doesn't exist.
3. `ARCHITECTURE_RULES.md`
4. `DOMAIN_DICTIONARY.md`
5. `CLAUDE.md`
6. `shared/rules/approval-gates.md` — gates #2 (git commit) and #6 (files out of boundary) both apply
   to any commit this skill produces and to test-file location decisions

## Config File

`.claude/atdd-config.json` at the project root, checked into git — the trust progression is a repo
property that any human reading the git history should be able to reconstruct, not tribal knowledge.

```json
{
  "gates": {
    "scenario-review": "active",
    "test-code-review": "active"
  },
  "history": [
    {
      "gate": "test-code-review",
      "phasedOutAt": "2026-08-15",
      "reason": "6 consecutive runs with no changes requested at this gate"
    }
  ]
}
```

- Two gate names are recognized: `scenario-review` (after Phase 1, checks the Gherkin matches the
  feature spec's intent) and `test-code-review` (after Phase 2, checks the step definitions correctly
  automate the Gherkin).
- Two states per gate: `active` (this skill pauses for explicit human confirmation) or `phased-out`
  (skipped). Anything else, or a missing entry, is treated as `active` — the safe default.
- `history` is a log of when each gate was phased out and why. Optional but strongly recommended so the
  reasoning stays in the repo.
- Two gates are **not** configurable and always run: the initial pairing with `spec-writer`/`analyst`
  (which happens *before* this skill starts, so it's not in the flow below) and the final
  ship-readiness gate before persistence.

If the config file doesn't exist on first run, create it with both configurable gates set to `active`.
Never silently phase a gate out based on run history alone — surfacing a suggestion is fine (see
"Suggesting Gate Progression" below), but the actual state change is a human decision persisted through
a git commit, same as any other rule change in this repo.

## Process

### Phase 0: Setup
1. **Read the feature file** — confirm it follows `features/TEMPLATE.md` structure. If not, stop and
   ask the user to run `/spec-writer` first.
2. **Derive the feature name** — kebab-case from the feature file name.
3. **Load or create `.claude/atdd-config.json`** per the "Config File" section above.
4. **Check for an existing `.claude/feature-workspace/<feature-name>/pipeline-state.json`.** If one exists for this
   feature and its `pipeline` field is `deliver-atdd`, invoke `resume-pipeline` instead of continuing
   here. If it exists for a different pipeline (e.g. `deliver-feature`), stop and surface the conflict
   — don't overwrite another pipeline's state without explicit confirmation.
5. **Create the feature workspace and archive** — `.claude/feature-workspace/<feature-name>/` and
   `docs/features/<feature-name>/`, same convention as `deliver-feature`. Reuse the same
   `pipeline-state.json` and `pipeline-trace.json` shapes documented in
   `shared/skills/deliver-feature/SKILL.md`, with `"pipeline": "deliver-atdd"` as an added top-level
   field so a resumer can tell the two apart.
6. **Invoke context-engineer** → produces `context-manifest.md` in
   `.claude/feature-workspace/<feature-name>/`. Scopes the bounded context, pins specific files,
   lists relevant KIs/ADRs, and estimates the token budget. If it flags a budget WARNING, tell the
   user which files it recommends cutting before continuing. **Checkpoint**: record in
   `pipeline-state.json`. qa-engineer reads `context-manifest.md` before writing scenarios.

### Phase 1: Scenario writing
7. **Invoke qa-engineer** with narrow scope: write the acceptance scenarios (Gherkin `Given/When/Then`)
   for this feature only. No step definitions yet, no run. Output: `.claude/feature-workspace/<feature-name>/scenarios.feature`.
8. **Gate: scenario-review.** If `gates.scenario-review == "active"`: PAUSE. Show the user the scenario
   file and ask "do these scenarios match the intent? (yes to proceed / edit to revise)." If edits are
   requested, back up the current scenarios to `.claude/feature-workspace/<feature-name>/.history/scenarios.feature.<timestamp>`,
   send back to qa-engineer with the specific corrections, re-present. Repeat until approved. If
   `gates.scenario-review == "phased-out"`: continue directly to Phase 2, but note the skip in
   `pipeline-trace.json`.

### Phase 2: Step definition writing
9. **Invoke qa-engineer** with narrow scope: write the step definitions that automate `scenarios.feature`
   using the project's existing test framework and conventions (per `CLAUDE.md` and any framework rules
   in `shared/rules/testing-conventions.md`). The step definitions are **expected to fail at this
   phase** — implementation doesn't exist yet. That's the point of ATDD, not a bug. Output: step
   definitions written to the project's normal test location + summary in
   `.claude/feature-workspace/<feature-name>/test-code-report.md`.
10. **Gate: test-code-review.** If `gates.test-code-review == "active"`: PAUSE. Show the user the
    `test-code-report.md` and the diff of new test files. Ask "does the step definition code correctly
    automate the scenarios?" (Not "does it pass" — it won't yet.) Same edit/re-present loop as Phase 1.
    If `phased-out`: continue directly to Phase 3.

### Phase 3: Implementation loop (always autonomous)
11. **Invoke developer** with the feature spec + `scenarios.feature` as its acceptance criteria. It
    implements until the scenarios pass, writing the unit tests for the code it changes (ADR-008's
    definition of done). Output: `.claude/feature-workspace/<feature-name>/implementation-notes.md`.
12. **No gate here.** The implementation loop is autonomous by design — reintroducing a gate here would defeat the whole reason to
    use this workflow over `deliver-feature`, which already has an in-loop `code-reviewer`.

### Phase 4: Acceptance run
13. **Invoke qa-engineer** to run the full scenario suite against the finished implementation. Output:
    `.claude/feature-workspace/<feature-name>/acceptance-report.md` — which scenarios passed, which failed, coverage
    if available. Uses the `run-tests` skill (`shared/skills/run-tests/SKILL.md`) under the hood, same
    as qa-engineer already does inside `deliver-feature`.
14. **If any scenario fails**: this is a real problem, not a gate. Either the implementation is
    incomplete (send back to Phase 3 with the specific failing scenarios), the scenarios themselves
    were wrong (send back to Phase 1, requiring a human review even if `scenario-review` is
    phased-out — a phased-out gate doesn't mean "never look at scenarios again," it means "don't
    require a human between writing them and moving on"), or a step definition was wrong (send back to
    Phase 2). Which one is the analyst-level judgment call the model should make and explain, not just
    guess.

### Phase 5: Ship (always gated)
15. **Write delivery summary** to `.claude/feature-workspace/<feature-name>/delivery-summary.md` — see "Delivery
    Summary Format" below.
16. **Ship-readiness gate.** PAUSE. Show the summary + acceptance report + all scenarios green.
    Ask "ready to ship?" This gate is NOT in the config — it always runs, matching
    `approval-gates.md`'s general principle that irreversible/external-facing actions never delegate
    the final "yes."
17. **On confirmation**: persist all artifacts from `.claude/feature-workspace/<feature-name>/` to
    `docs/features/<feature-name>/`, write the feature archive README, add an entry to
    `docs/features/README.md` — same persistence steps as `deliver-feature`, Phase 4.

## Output Format

### `scenarios.feature`
Standard Gherkin, no framework-specific dressing. Whatever runner the project already uses picks it up
via the project's own convention (`cucumber.js` config, `pytest-bdd` collection root, etc.).

### `test-code-report.md`
```markdown
# Test Code Report

## Scenarios Automated
- [Scenario 1 name] — step defs at [path]
- [Scenario 2 name] — step defs at [path]

## Files Created
- [path/to/steps/x_steps.ts]

## Files Modified
- [path/to/existing/hooks.ts] — [what was added]

## Expected State
All scenarios currently FAIL (implementation doesn't exist yet). This is the ATDD invariant, not a
regression.
```

### `acceptance-report.md`
```markdown
# Acceptance Run Report

## Scenario Results
| Scenario | Status |
|---|---|
| [name] | PASS / FAIL |

## Coverage (if available)
- Line coverage: N%
- Branch coverage: N%

## Failures (if any)
- [Scenario]: [what failed, which step, which line of implementation]
```

### `delivery-summary.md`
```markdown
# ATDD Delivery Summary: [Feature Name]

## Pipeline
| Phase | Agent | Status | Gate |
|---|---|---|---|
| 0. Context Engineering | context-engineer | PASS | n/a (mandatory, non-skippable) |
| 1. Scenario writing | qa-engineer | PASS | scenario-review: [active/phased-out] |
| 2. Step definitions | qa-engineer | PASS | test-code-review: [active/phased-out] |
| 3. Implementation | developer | PASS | n/a (autonomous by design) |
| 4. Acceptance run | qa-engineer | PASS | n/a |
| 5. Ship | — | PENDING | ship-review: always active |

## Scenarios
- Total: N
- Passing: N
- Failing: 0 (all resolved)

## Config State at Delivery Time
- `scenario-review`: [active / phased-out (since YYYY-MM-DD)]
- `test-code-review`: [active / phased-out (since YYYY-MM-DD)]

## Artifacts
- docs/features/<feature-name>/scenarios.feature
- docs/features/<feature-name>/test-code-report.md
- docs/features/<feature-name>/implementation-notes.md
- docs/features/<feature-name>/acceptance-report.md
- docs/features/<feature-name>/delivery-summary.md
```

## Suggesting Gate Progression

At the end of a successful run, if a gate is `active` and its last N invocations (see
`pipeline-trace.json` across `docs/features/*/`) had zero human-requested edits, surface a suggestion:

> The `scenario-review` gate has been active for 6 consecutive runs with no edits requested. Consider
> phasing it out by setting `.claude/atdd-config.json`'s `gates.scenario-review` to `"phased-out"` and
> adding a `history` entry with today's date and a short reason.

Do NOT change the config yourself. This is exactly the same "surface, don't act" pattern
`memory-engineer` uses when recommending KI expiration — the human owns the decision, the framework
just makes it easy to make. Threshold defaults to 5 consecutive no-edit runs, matching
`deliver-feature`'s "every 5th delivery" retrospective cadence.

## Guardrails
- Never skip context-engineer (Phase 0, step 6). qa-engineer reads `context-manifest.md` before
  writing scenarios — without it, scenario writing may miss bounded-context constraints surfaced by
  prior deliveries or ADRs. If context-engineer fails, halt and surface the error; do not proceed
  with scenario writing against an unscoped codebase.
- Never skip the ship-readiness gate (Phase 5, step 16). It's not in the config for a reason —
  irreversible/external-facing actions always require the explicit "yes," matching
  `approval-gates.md`'s general principle.
- Never phase a gate out automatically based on run history. Surfacing a suggestion is fine and
  encouraged (see "Suggesting Gate Progression"), but the actual state change is a human decision
  persisted through a git commit.
- Never proceed past a Phase 4 acceptance-run failure without explaining which of the three possible
  root causes (bad implementation / bad scenarios / bad step defs) the failing behavior points at, and
  which phase you're sending it back to. "The test failed, retrying" is not enough.
- Never modify the feature file (`features/<name>.md`) from inside this skill — the feature spec is a
  spec-writer artifact; if it's wrong, that's a Phase 4 findings-back-to-Phase-1 signal, not something
  this skill quietly rewrites.
- Never commit anything from this skill without explicit "commit" / "approve commit" from the user,
  per `approval-gates.md` gate #2 — this skill runs entirely locally through Phase 4 and only touches
  the feature archive at Phase 5, but neither phase counts as commit approval on its own.

## Standalone Mode
All agents run locally. Test execution uses whatever the project already has installed (`npm test`,
`pytest`, `go test`, etc., resolved via the `run-tests` skill). No external calls, no Friday POST at
the end of this pipeline (that's a `deliver-feature` step; ATDD delivery is trust-progression-focused,
not reporting-focused, and doesn't currently emit Cucumber-JSON in the shape Friday expects — that's a
worthwhile future extension if teams want it, not a v1 requirement).

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
