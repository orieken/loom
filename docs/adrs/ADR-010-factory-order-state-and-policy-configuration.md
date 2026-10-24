# ADR-010: Where a Factory Order's State Lives, What `hold` Means, and Where Automation Is Configured

## Status

**Proposed** — awaiting three choices, marked ☐ below. Each has a recommendation; nothing is built
until it is accepted.

## Date

2026-10-23

## Deciders

Oscar Rieken

## Context

Step 2 of the factory plan (`docs/roadmaps/factory-phases.md`) is decisions the supervisor (L3.65)
cannot be designed around. They come from the 2026-10-23 review
(`docs/audits/factory-intake-and-deliver-feature-review-2026-10-23.md`, items I-6, I-7, D-2, D-3).

Three facts about the code as it stands shape them:

1. **The spec is now immutable to a run (I-1).** `loom run` records the spec's digest; any change to
   the file makes every completed stage stale and resets every approval bound to them.
2. **An intake file holds human-owned `status` inside the spec's own frontmatter.** So with fact 1,
   flipping `status: hold` on an order in flight changes its spec, and resuming would redo all its
   work. The two cannot both stand as built.
3. **Two automation paths exist, and only one is honest.** The Go evaluator (`internal/policy`) reads
   `.claude/policies/*.policy.yaml`, records every decision in run state, and never auto-approves.
   The markdown pipeline reads `mode` and `autoProceed*` from `.claude/delivery-policy.yaml` in prose,
   auto-proceeds, and records nothing (D-2). That file's template offers `mode: unattended`, which
   nothing defines, and the skill reads `autoProceedPersistence`, which the template lacks (D-3). Go
   reads exactly one key from it: `policiesEnabled`.

And one fact makes a cheap option available: `loom run` already turns its first SIGINT into a clean
`INTERRUPTED` checkpoint that `--resume` continues from.

---

## Decision 1 — An order is snapshotted when it is claimed

**Recommended: 1C.**

| | Option | Gain | Cost |
|---|---|---|---|
| ☐ | **1A** Digest only the spec's body; frontmatter edits never invalidate a run | `status` stays editable in place | A special case in I-1 for intake files. Changing `plan`, `scope` or `budget` mid-run would also go unnoticed |
| ☐ | **1B** Move `status` out of the file, into the supervisor's ledger via `loom factory ready/hold <id>` | The spec file never changes after it is ready | Status is invisible in the repository and local to one machine |
| ☐ | **1C** **Snapshot at claim**: the supervisor copies the order into `.claude/factory/claims/<id>.md`, and the run's spec is that copy | I-1 protects the copy; the intake file stays freely editable, `status` included; the claim records exactly what ran | Two copies. An edit after claim needs an explicit choice, so the inbox shows "order changed since claim" with re-run and keep options |

1C is the only option where every guarantee built so far holds unchanged. The executor needs nothing
new: the snapshot is just the spec path it is given.

## Decision 2 — Where intake lives, and which version the factory reads

**Recommended: 2B.**

| | Option | Gain | Cost |
|---|---|---|---|
| ☐ | **2A** **Working tree**: the supervisor reads `intake/` from its checkout as it is | Instant; nothing to commit | Uncommitted, unreviewed text becomes running work, and nothing records which version ran |
| ☐ | **2B** **Committed on `base-branch`**: the supervisor reads `intake/` at the branch head (`git show <sha>:intake/<id>.md`) | An order reaches the factory the way code reaches `main`: reviewed. With 1C, the claim records a commit SHA, so provenance is exact | Every status flip is a commit. Too slow for "stop spending now", which Decision 3 handles separately |
| ☐ | **2C** Outside the repository (`~/.loom/factory/`) | Nothing in git at all | Orders are invisible to the team and to review, and the opposite of how specs live today |

2B matches the trust model already built: an import adapter writes `trust: external` and only a human
promotes it to `ready`. Under 2B, that promotion is a reviewed commit.

## Decision 3 — `hold` and `stop` are different verbs

**Recommended: 3A.**

| | Option | Behavior | Needs |
|---|---|---|---|
| ☐ | **3A** **Two verbs.** `hold` (in the file): unclaimed orders are not picked up, and a running order finishes its current leg to its next natural halt (a gate, completion, or failure) and is not resumed. `loom factory stop <id>` (immediate): the supervisor sends SIGINT, and the executor checkpoints `INTERRUPTED` | `hold` loses no work; `stop` cuts spend at once and re-runs only the interrupted stage | Nothing new in the executor. A run already halts at every gate and the supervisor decides whether to resume; `stop` uses the existing checkpoint |
| ☐ | **3B** `hold` affects unclaimed orders only | Simplest | No way to pause work in flight short of killing it |
| ☐ | **3C** `hold` asks the executor to stop at its next gate | One verb | A new stop request in the executor, for what 3A gets from the supervisor not resuming |

`stop` has to be immediate and local because under 2B a `hold` is a commit, and money is spent while
it travels.

## Decision 4 — Automation is configured in one place, and recorded

**Recommended: 4B.**

| | Option | Gain | Cost |
|---|---|---|---|
| ☐ | **4A** Keep both paths: validate `delivery-policy.yaml` in Go, and make the markdown pipeline record each auto-proceed through a new `loom state decide` | Markdown-only hosts keep auto-proceed | Two automation engines to keep in agreement; recording is prompt discipline wherever the binary is absent |
| ☐ | **4B** **One engine.** Automation lives only in `.claude/policies/*.policy.yaml`, evaluated in Go by `loom run`, which records every decision. `delivery-policy.yaml` keeps `policiesEnabled` and numeric limits, and loses `mode` and every `autoProceed*` key. The markdown pipeline always stops for a human. Go validates the file strictly, so unknown keys fail | The silent path is gone, not patched. One schema, one reader. Consistent with ADR-006: loom executes pipelines | Hosts without the binary lose auto-proceed. They keep the full pipeline, gated by a human at each checkpoint, which is the safe default the framework claims anyway |
| ☐ | **4C** Status quo, documented | No work | Leaves D-2 open: auto-approvals that `approval-gates.md` promises cannot happen silently, happening silently |

The factory is unattended by definition and runs only under `loom run`, so 4B costs it nothing. An
order's `autonomy: strict-human` maps directly: skip policy evaluation for that run.

---

## Consequences, if all four recommendations are accepted

- **L3.65 (the supervisor)** reads committed orders at a recorded SHA, snapshots each claim, never
  writes an intake file, resumes only orders not on hold, and exposes `stop`.
- **I-6, I-7, D-2 and D-3** close. I-1's guarantee holds unchanged for factory runs.
- **New work:** `loom factory stop`; the snapshot and "changed since claim" inbox entry, inside L3.65;
  strict validation of `delivery-policy.yaml`; removing `mode`/`autoProceed*` from the template, the
  skill and `approval-gates.md`'s prose; a migration note for projects that set them.
- **Lost:** auto-proceed on hosts without the `loom` binary.

## Fitness Function

- A test that edits an intake file after claim and asserts the running claim's digest is unchanged
  (1C).
- A test that a supervisor tick reads an order from the base-branch commit, not the working tree (2B).
- A test that `loom factory stop` leaves the run `INTERRUPTED` and resumable (3A).
- `delivery-policy.yaml` with `mode:` or an unknown key fails validation (4B).

---

## How to decide

Tick one box per decision, or reply with the numbers — for example "1C 2B 3A 4B". If a decision
should change, say which and why, and this ADR is rewritten before it is accepted.
