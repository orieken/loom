# Policy Schema

Version: 1.0.0

Policies are YAML files with a `.policy.yaml` extension. They live in `.claude/policies/` inside the
project that opts in to automation. The evaluator at `shared/orchestration/policy-evaluator.md`
reads every `.policy.yaml` file in that directory before each stage boundary.

---

## Top-level fields

```yaml
name: string                # unique kebab-case identifier; used in telemetry events
version: "1.0"              # policy schema version; always "1.0" for Phase 4
description: string         # one-line human-readable summary
enabled: true | false       # default true; false = policy loaded but never fires
matcher:                    # which gate(s) this policy watches — see Matcher below
condition:                  # what must be true to trigger the action — see Condition below
action:                     # what the evaluator does when condition is met — see Action below
```

---

## Matcher

Specifies which pipeline gate(s) this policy watches.

```yaml
matcher:
  gate: <gate-id>           # single gate (see gate IDs below)
  # OR
  gates: [<gate-id>, ...]   # multiple gates — policy fires on any matching gate
```

### Valid gate IDs

| Gate ID | Approval Gates reference | Policy-eligible? |
|---|---|---|
| `git-commit` | Gate 2 — creating a git commit | **Yes** |
| `out-of-boundary-write` | Gate 6 — writing files outside workspace | **Yes** |
| `fitness-function-wiring` | Gate 7 — wiring a new fitness function | **Yes** |
| `ship-to-friday` | Gate 1 — POST to Friday dashboard | No — always human |
| `db-migration` | Gate 3 — SQL migration against remote DB | No — always human |
| `db-contract-phase` | Gate 4 — DROP/RENAME in contracting phase | No — always human |
| `external-api` | Gate 5 — third-party API mutation | No — always human |
| `deploy` | Gate 8 — triggering a deployment | No — always human |

Policies targeting non-eligible gates **fail to load**, naming the gate and the reason it is
always human. This used to say they were "silently ignored" — which meant someone who wrote a
policy to auto-approve a deployment saw no error and could reasonably conclude it worked. Silence
is the wrong answer to a request that will never be honoured.

The always-human list is a **compiled constant** in `internal/policy`, not a field in any YAML. A
kill-switch that can be edited by whoever is trying to bypass it is not a control.

---

## Condition

A condition is a YAML map of key-value checks. All checks must pass for the condition to be true
(implicit AND). Use a list under `any:` for OR logic.

### Scalar checks

This is a **catalogue of available checks, not a single condition** — a real condition uses a
subset, and each field may appear at most once in a mapping. (This block previously listed
`diffType` and `filePaths` twice each, which is invalid YAML; no parser had ever read this file.)

**Nothing loads this document.** An earlier version of this paragraph claimed it "is loaded as a
fixture by `internal/policy`'s tests" — it is not, and was not. The examples under
`shared/policies/examples/` *are* load-tested (`TestShippedExamplesAllLoad`), so those are the ones
to trust when the two disagree. Keeping this file correct is manual.

| Field | Operators | Value | Meaning |
|---|---|---|---|
| `diffLines` | `lessThan`, `equals` | number | lines changed since the run started |
| `testsPass` | bare boolean, `equals` | bool | all configured tests pass |
| `filePaths` | `allMatch`, `noneMatch`, `anyMatch` | glob | every / no / any changed file matches |
| `codeReviewer.verdict` | `equals` | string | the review verdict, e.g. `APPROVED` |
| `codeReviewer.behaviorChange` | bare boolean, `equals` | bool | the review reported a behaviour change — **`require-human` only** |
| `securityReviewer.criticals` | `equals`, `lessThan` | number | count of critical findings |

Every field here resolves from run state. A field may still answer **unknown** for a given run,
when the stage that produces it has not run or its document is missing — that is the honest answer
and never becomes a guess.

### Three fields were removed (L2.20)

A vocabulary should not describe questions nothing can answer. These resolved to unknown for every
run, which is correct and useless, and three of the five shipped examples could never evaluate
because of them.

| Removed | Why | What to use instead |
|---|---|---|
| `diffType` | Declared an **open** set (`docs-only`, `test-additions`, …). With no defined list of values it could not be implemented or tested | `filePaths` with `allMatch` — more precise, and it works today |
| `dryRunPass` | Needs a CI dry-run result. The executor runs stages, not CI | Nothing yet. Reinstating it means building CI reporting into run state first |
| `fitnessFunction.allPass` | Same — fitness functions run in CI, and nothing reports back | Nothing yet, same prerequisite |

A policy naming a removed field **fails to load**, rather than being ignored: someone who writes
one should find out before a run, not never.

```yaml
condition:
  diffLines:
    lessThan: 200
  testsPass: true
  codeReviewer.verdict:
    equals: "APPROVED"
  filePaths:
    noneMatch: "**/security/**"
```

To express "none of these paths", either use `noneMatch` once with a single glob, or combine
alternatives under `not: { any: [...] }` — a field may not repeat within one mapping.

**This list is closed.** There is no expression language: the evaluator is typed Go
(`internal/policy`), so adding a check is a code change with a test, not a config change. That is
deliberate for a mechanism whose purpose is skipping human review (roadmap L2.16).

### OR logic

```yaml
condition:
  any:
    - codeReviewer.verdict:
        equals: "APPROVED"
    - diffLines:
        lessThan: 50
```

### NOT / inversion

```yaml
condition:
  not:
    filePaths:
      anyMatch: "**/auth/**"
```

---

## Action

```yaml
action:
  type: auto-approve        # one of: auto-approve | auto-reject | require-human | escalate
  reason: "string"          # human-readable reason written to the telemetry event
  escalateTo: "string"      # optional; used with type: escalate — names who or what to notify
```

| Action | Meaning |
|---|---|
| `auto-approve` | Evaluator approves the gate; pipeline proceeds without human prompt |
| `auto-reject` | Evaluator rejects; pipeline halts and reports reason to operator |
| `require-human` | Evaluator explicitly requires human confirmation (overrides auto-approve policies) |
| `escalate` | Evaluator pauses pipeline and surfaces findings to `escalateTo` target |

**Conflict resolution**: if multiple policies match the same gate and their actions conflict
(`auto-approve` vs `require-human`), `require-human` always wins. The evaluator logs a
`policy.conflict` telemetry event listing the conflicting policy names.

---

## Full example

```yaml
name: auto-approve-doc-only-commits
version: "1.0"
description: "Auto-approve git commits when the diff is documentation-only and tests pass"
enabled: true

matcher:
  gate: git-commit

condition:
  filePaths:
    allMatch: "{docs/**,**/*.md,**/*.mdx}"
  testsPass: true
  diffLines:
    lessThan: 500

action:
  type: auto-approve
  reason: "Docs-only diff, tests green — no review required"
```

---

## Telemetry events emitted

Every evaluation (match or no-match) emits a `policy.evaluated` event:

```json
{
  "event": "policy.evaluated",
  "policyName": "auto-approve-doc-only-commits",
  "gate": "git-commit",
  "conditionMet": true,
  "action": "auto-approve",
  "reason": "Docs-only diff, tests green — no review required",
  "timestamp": "2026-08-01T12:00:00Z",
  "pipelineRunId": "<run-id>"
}
```

Non-matching policies emit `conditionMet: false` and `action: "no-op"`. Skipped non-eligible gate
policies emit event type `policy.skipped`.

---

*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) AOS Phase 4
Policy Layer. Licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md).*
