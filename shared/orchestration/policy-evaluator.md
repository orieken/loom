# Policy Evaluator

Part of the `shared/orchestration/` runtime. The evaluator is the bridge between the declarative
policy files in `.claude/policies/` and the `FeatureDeliveryWorkflow`'s stage boundaries.

---

## Role

At every checkpoint stage in `FeatureDeliveryWorkflow`, the runtime calls the evaluator before
proceeding past the gate. The evaluator:

1. Loads all `.policy.yaml` files from `.claude/policies/` (project-local). The directory is fixed:
   `.claude/delivery-policy.yaml` has no `policyDir` key, and `loom run` refuses a file that sets one
   (ADR-010).
2. Filters to policies whose `matcher.gate` matches the current gate ID.
3. Evaluates each matching policy's `condition` against the current pipeline context.
4. Determines the final action (see Conflict Resolution below).
5. Emits a `policy.evaluated` event for every gate it evaluates, recording each policy's outcome
   and the facts it could not answer — see "Telemetry events" below.
6. Returns one of three signals to the workflow runtime:
   - `proceed` — the pipeline advances without prompting the human operator
   - `halt` — the pipeline stops and reports the reason; human must intervene
   - `require-human` — the pipeline pauses and asks the human operator for confirmation

---

## Inputs

```yaml
evaluatorInputs:
  gate: string                       # gate ID (e.g. "git-commit", "fitness-function-wiring")
  pipelineContext:                   # structured view of current pipeline state
    diffLines: integer               # total lines changed in current diff
    diffType: string                 # "docs-only" | "test-additions" | "source" | "mixed"
    filePaths: [string]              # list of changed file paths
    testsPass: boolean               # did all configured tests pass?
    fitnessFunction.allPass: boolean # did all fitness functions pass?
    codeReviewer.verdict: string     # latest code-reviewer verdict if present
    codeReviewer.behaviorChange: boolean
    codeReviewer.criticals: integer
    securityReviewer.criticals: integer
    dryRunPass: boolean              # for fitness-function-wiring gate
```

---

## Evaluation algorithm

```
1. If policiesEnabled == false in .claude/delivery-policy.yaml → return require-human (kill-switch)
2. Load policies from .claude/policies/ (a file that fails schema parse stops the run)
3. Filter: keep policies where matcher.gate == current gate AND enabled == true
4. Further filter: discard policies targeting non-eligible gates (always-human list):
   ship-to-friday, db-migration, db-contract-phase, external-api, deploy
5. For each candidate policy:
   a. Evaluate condition against pipelineContext
   b. Record the outcome; the gate's `policy.evaluated` event carries every policy's result.
6. Collect actions from policies where conditionMet == true
7. Apply conflict resolution (see below)
8. If no policies matched → return require-human (default)
9. Return resolved signal
```

---

## Conflict resolution

When multiple policies match the same gate with conflicting actions:

| Conflicting actions | Resolution |
|---|---|
| `auto-approve` + `require-human` | `require-human` wins unconditionally |
| `auto-approve` + `auto-reject` | `auto-reject` wins; log `policy.conflict` event |
| `auto-approve` + `escalate` | `escalate` wins |
| Multiple `auto-approve` | `auto-approve` (unanimous) |
| Multiple `require-human` | `require-human` |

A `policy.conflict` telemetry event is emitted whenever two policies disagree on a gate. The
conflict event lists both policy names and their actions so operators can diagnose and deduplicate.

---

## Telemetry events

The evaluator emits one `policy.evaluated` event per gate onto the run event timeline
(`run-events.jsonl`), carrying every policy's name, outcome, and the condition fields it could not
answer. Its shape is generated from the event vocabulary — see
`shared/schemas/telemetry/run-event-types.md`; there is no hand-maintained payload definition here,
because a hand-maintained one is what drifted.

`policy.conflict` and `policy.skipped` no longer exist as separate events. A conflict is recorded
inside the gate's decision, naming the policies that disagreed, and a policy targeting an ineligible
gate never reaches evaluation because it fails to load. Both were specified, neither was ever
emitted, and neither needed to be a distinct type once the decision itself was recorded.

**The guarantee, stated precisely.** Every decision is now recorded and reviewable, which is what
"no silent auto-approvals" was reaching for. But note what the evaluator does *not* do yet: it does
not approve anything. A matching `auto-approve` policy is recorded as what would have happened, and
the executor halts for a human regardless. Honouring a decision is roadmap **L2.19**.

## Dry-run mode

```
/orchestrate --workflow feature-delivery --spec <file> --dry-run-policies
```

`loom run --spec <file> --dry-run-policies` evaluates every policy against a finished run's
recorded state and prints what each gate would have decided. It reads the same facts a live gate
reads, so a dry-run and a real evaluation cannot disagree about what was visible, and it modifies
nothing.

It replaces the specified-but-never-built `--dry-run-policies` on `/orchestrate`, which was to
replay `.claude/telemetry/events.jsonl` — a file that never had a writer and was retired in
roadmap L3.9.

---

*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) AOS Phase 4
Policy Layer. Licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md).*
