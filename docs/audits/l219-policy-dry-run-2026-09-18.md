# L2.19 evidence experiment — policy dry-run against a recorded run

**Date**: 2026-09-18 · **Tree**: `112a0e0` · **Outcome**: the experiment **cannot be run against
recorded history**, and that is the finding.

## Why this was attempted

L2.19 (*Honour a policy decision at a gate*) carries its own stop condition:

> **Do not build this until real runs show the evaluator deciding what a human would.** That is the
> entire reason L2.16 stopped short, and the records it writes are how the question gets answered.

L3.47 cleared a *different* precondition — gate #9's always-human classification, which until
2026-09-16 held only by accident. That made L2.19 *reachable*, not *ready*, and the roadmap note
added on 2026-09-18 overstated it. Corrected in the same commit as this report.

The cheap way to get the evidence L2.19 demands is `loom run --dry-run-policies`, which evaluates
policies against a finished run's recorded state and modifies nothing. Nine e2e runs happened
between 2026-09-06 and 2026-09-09. The question: would a policy have decided what the human decided?

## Setup

A candidate policy using **only** the four facts the executor can source
(`internal/orchestrator/policy_gate.go`, `gateContext`):

```yaml
name: auto-approve-clean-ship
matcher:
  gate: confirm-ship
condition:
  codeReviewer.verdict: { equals: "APPROVED" }
  securityReviewer.criticals: { equals: 0 }
  testsPass: true
  not:
    any:
      - filePaths: { anyMatch: "**/security/**" }
      - filePaths: { anyMatch: "**/auth/**" }
action: { type: auto-approve }
```

Deliberately avoids `diffLines`, `diffType`, `dryRunPass`, `fitnessFunction.allPass` and
`codeReviewer.behaviorChange` — the five fields with no source (L2.20), which is why three of the
five *shipped example* policies can never evaluate.

Run in a scratch sandbox against `docs/audits/loom-e2e-run-2026-09-07/run-state.json`
(`schemaVersion: 10`, matching current, so it loads).

## Result

```
Dry-run against console-log-filtering (1 loaded, no state is modified)
Policy at gate "confirm-ship": 1 policy evaluated, none matched
  auto-approve-clean-ship  UNKNOWN  auto-approve
    (unknown: codeReviewer.verdict, filePaths, securityReviewer.criticals, testsPass)
```

**All four supposedly-sourced facts resolved UNKNOWN.** Not the five known-unsourced ones — the four
that are supposed to work.

## Why, and why it is not a bug

It is not the evaluator failing. The facts are read from the **typed stage documents**, not from
`run-state.json`:

```go
raw, err := os.ReadFile(typedStatePath(e.workspaceDir(), stageID))   // policy_gate.go:120
// typedStatePath = <workspace>/state/<stageID>.json                 // typed.go:21
```

The run had every fact. All four producing stages completed and recorded their kind:

| Stage | stateKind | Status |
|---|---|---|
| `code-reviewer` | `review` | COMPLETED |
| `security-reviewer` | `security` | COMPLETED |
| `qa-engineer` | `qa` | COMPLETED |
| `developer` | `implementation` | COMPLETED |

But the archive contains only `route.md`, `run-events.jsonl`, `run-state.json`, `traces.jsonl`.
**The `state/` directory was not retained.** `completedStageOfKind` succeeds — the kind is in
run-state — and then `os.ReadFile` fails, the fact is absent, and the condition correctly resolves
to UNKNOWN rather than guessing. The evaluator behaved exactly as designed; the inputs are gone.

## What this means for L2.19

**Do not build it.** Two independent reasons, and the second is new:

1. **No policy has ever been written here.** No `.claude/policies/`, no
   `.claude/delivery-policy.yaml`. The only surviving run-state has `policyDecisions: ABSENT`. L2.16's
   audit trail — the mechanism L2.19 names as how the question gets answered — has recorded nothing,
   because there was nothing to evaluate.
2. **The evidence cannot be reconstructed from history.** Even writing policies now cannot recover
   the past runs' decisions, because the facts those decisions would read were discarded at archive
   time. The nine runs are not a corpus for this question.

So the evidence has to be gathered **forward**, from runs that have not happened yet.

## Prerequisites, in order

1. **Retain typed stage state in a run archive** — `<workspace>/state/*.json` alongside
   `run-state.json`. Without it no future run is analysable either, and this experiment repeats its
   own negative result. Cheapest of the three and the one that unblocks the rest.
2. **Write real policies** and run with `policiesEnabled`, letting L2.16 record decisions with
   `honoured: false` across several real runs.
3. **L2.20** — source the five missing facts, or accept that policies are limited to the four and
   say so in the schema. Three of five shipped examples currently cannot evaluate, which makes the
   example set misleading about what a policy can express.

Only then does L2.19's own question — *does the evaluator decide what a human would?* — have data.

## Honest limits of this experiment

- **One run**, not nine: only `loom-e2e-run-2026-09-07` retains a `run-state.json`. The others are
  audit prose and briefs.
- It tests **one candidate policy** at **one gate** (`confirm-ship`). A different policy might have
  matched on facts that survived — but none did survive, so the outcome would not differ.
- It shows nothing about whether the evaluator is *correct*. It shows the question cannot currently
  be asked.

## Artifacts

The candidate policy is reproduced above rather than committed. It is a probe, not a shipped
example. Worth considering separately: the `shared/policies/examples/` set would be more honest if
at least one example used only sourced facts, since three of five cannot evaluate today.
