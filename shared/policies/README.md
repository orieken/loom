# Policy Layer

The policy layer enables **graduated automation** for the `FeatureDeliveryWorkflow` and other AOS
pipelines. Policies are **strictly opt-in per-project** — the absence of any policy file guarantees
identical behavior to v3.2.

---

## What policies do

Each policy declares a *matcher* (which pipeline gate it watches), a *condition* (what must be true to
trigger), and an *action* (`auto-approve`, `auto-reject`, `require-human`, `escalate`). The
`FeatureDeliveryWorkflow` calls the policy evaluator at every stage boundary; if no matching policy
exists, the stage boundary defaults to `require-human` — the same behavior as all prior versions.

A team that upgrades to v3.3 but places no `.claude/policies/` files in their project sees **zero
behavior change**.

---

## Where to put project policies

Policies live in the *project* (not in this framework repo) at:

```
.claude/policies/<your-policy-name>.policy.yaml
```

The evaluator loads every `.policy.yaml` file found in that directory at pipeline startup. File order
is undefined — write policies that do not depend on evaluation order.

---

## Audit trail (non-negotiable)

Every policy decision — whether it auto-approves, rejects, escalates, or falls through to a human —
emits a `policy.evaluated` event onto the run event timeline, recording every policy's outcome and the facts it could not answer. The rule that there are no silent auto-approvals
in this framework. Teams that disable telemetry lose their audit trail and should not enable
auto-approve policies.

The event's shape is generated from the event vocabulary — see `shared/schemas/telemetry/run-event-types.md`.

---

## Schema

The declarative policy format is documented in `shared/policies/policy-schema.md`.

## Examples

`shared/policies/examples/` contains six reference policies demonstrating common patterns:
- `auto-approve-refactor.policy.yaml` — auto-proceed on a small, reviewed, contained commit
- `auto-approve-doc-changes.policy.yaml` — auto-proceed when every changed file is documentation
- `auto-approve-test-additions.policy.yaml` — auto-proceed when every changed path is a test
- `require-human-on-critical-findings.policy.yaml` — force human on a critical security finding
- `require-human-review-security.policy.yaml` — inversion: force human regardless of other policies
- `require-human-on-behaviour-change.policy.yaml` — force human when the review reports behaviour changed

The count and this list are checked by nothing; `TestShippedExamplesAllLoad` asserts the files
parse, not that they are described here. It said "three" while listing four and shipping six.

---

## Which examples evaluate today

All of them, as of roadmap **L2.20**. `internal/policy` answers a condition from the run's own
state, and every field the vocabulary still declares has a source there: `codeReviewer.verdict`,
`securityReviewer.criticals`, `testsPass`, `filePaths`, `diffLines`, and
`codeReviewer.behaviorChange`.

The three fields that never had one — `diffType`, `dryRunPass`, `fitnessFunction.allPass` — were
removed rather than left resolving to unknown on every run. A policy naming one now **fails to
load**. See `policy-schema.md` for why each went and what to use instead.

A field can still answer **unknown** for a particular run, which is different: the fact is
sourceable in general but absent here, because the stage that produces it has not run yet. That is
the honest answer and never becomes a guess — see the next section, which is mostly about this.

---

## Which gate to watch, and what it can see

**This decides whether your policy ever runs at all.** Two separate traps, and the examples
directory contains victims of the first.

### `loom run` halts at four gates, and only those

| Gate | Guards | Evaluated by `loom run`? |
|---|---|---|
| `confirm-design` | `developer` | **Yes** |
| `confirm-security` | `qa-engineer` | **Yes** |
| `confirm-ship` | `devops-engineer` | **Yes** |
| `confirm-unresolved-review` | `code-reviewer` loop bound | **Yes** |
| `git-commit`, `out-of-boundary-write`, `fitness-function-wiring` | — | **No** — prose gates in `approval-gates.md` that the executor does not run |

A policy watching one of the last three is valid, loads fine, and is **never evaluated under
`loom run`** — the executor never reaches that barrier, so no decision is recorded and nothing
accumulates. Four of the six shipped examples are in this position. They are not wrong; they
target the markdown pipeline's gates, which the executor does not yet run (`approval-gates.md`,
"Honest scope").

### Facts arrive progressively, so an early gate sees less

A gate that precedes a stage cannot see that stage's output. Measured on a real run:

| At | Available | Not yet |
|---|---|---|
| `confirm-design` | nothing — it precedes `developer` | verdict, criticals, paths, diffLines, testsPass |
| `confirm-security` | verdict, criticals, changed paths, diffLines | `testsPass` — it guards `qa-engineer` |
| `confirm-ship` | everything above, plus `testsPass` | — |

So a policy testing `codeReviewer.verdict` at `confirm-design` resolves to unknown on **every**
run, forever. That is correct behaviour and a useless policy. `require-human-on-critical-findings`
watches both `confirm-design` and `confirm-security` deliberately: unknown at the first, decided at
the second.

**If you are writing a policy to accumulate the evidence roadmap L2.19 needs, watch
`confirm-security` or `confirm-ship`.** Those are the gates where a decision means something.

Run `loom run --spec <file> --dry-run-policies` against a finished run to see this for yourself.

---

## Emergency override

To disable all policies for a project without deleting them:

```yaml
# .claude/delivery-policy.yaml
policiesEnabled: false
```

This is a global kill-switch, and as of roadmap L2.16 it is read by `loom run` rather than only
documented — until that release it appeared in three files and nothing acted on it. Only an
explicit `false` disables: a typo cannot silently switch evaluation off, because a control that
turns itself off by accident is worse than one nobody set. The policy files are preserved —
re-enable by removing or flipping the flag.

---

## Gate classification

Not every approval gate is policy-eligible. Gates 1, 3, 4, 5, and 8 from
`shared/rules/approval-gates.md` are permanently human-only regardless of any policy you write —
the evaluator ignores policies targeting those gates and always returns `require-human`. See
`docs/aos/policy-authoring-guide.md` for the full classification and rationale.

---

*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) AOS Phase 4
Policy Layer. Licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md).*
