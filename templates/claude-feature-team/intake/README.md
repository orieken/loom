# intake/ — the factory's work orders

Every piece of work the factory may pick up is one markdown file here:
`intake/<id>.md`, copied from [`TEMPLATE.md`](TEMPLATE.md). The file is both the **work order**
(its frontmatter: priority, budget, finish line) and the **spec** (its body: the same sections as
`features/TEMPLATE.md`), so `loom run --spec intake/<id>.md` runs it with no conversion step.

**Status: contract only.** This folder and its template define what a factory supervisor reads.
No supervisor ships with loom yet, so nothing here is enforced today — a field below is a promise
about what the supervisor will check, not a check that runs. Until it exists, an intake file is a
spec you can hand to `loom run` or `/deliver-feature` yourself.

## Lifecycle

```
draft ──(spec-writer READY)──► ready ──(supervisor claims)──► [ledger: running → waiting-approval → shipped | parked]
  ▲                              │
  └────────── hold ◄─────────────┘   (a human pulls it back; the supervisor stops claiming it)
```

| `status` | Meaning | Who sets it |
|---|---|---|
| `draft` | Being written. Never picked up. | human or import adapter |
| `ready` | Eligible for pickup once every `depends-on` id is shipped. | human only |
| `hold` | Paused. Not picked up; a run already in flight finishes its current stage and stops at the next gate. | human only |

Everything after pickup — running, waiting on a gate, parked after a failure, shipped — is run
state, and lives in the supervisor's ledger, not in this file.

## Who writes what

**The supervisor never writes an intake file.** Two reasons:

1. **The file is the spec.** `loom run` hashes artifacts and invalidates an approval when anything it
   covered changes. A supervisor that rewrote `status:` in the spec mid-run would be editing the input
   of its own run.
2. **One writer per fact.** If both a human and the supervisor wrote `status`, each could overwrite
   the other. Humans own intent (`ready`, `hold`, priority); the ledger owns progress.

An import adapter (GitHub issues, Linear, Jira) may create files here, always with `status: draft`
and `trust: external`. Only a human promotes it to `ready`.

## Rules a supervisor enforces at pickup

Each is checkable from the file alone, before a single model call:

- `id` equals the file name without `.md`, and no other file in `intake/` or `features/` has the same
  stem (both map to `.claude/feature-workspace/<id>/`).
- `intake` is a schema version the supervisor knows; unknown versions are refused, not guessed at.
- `plan` names a plan loom ships (`deliver-feature`, `deliver-bugfix`), and `type: bug` uses
  `deliver-bugfix`.
- `autonomy` can only tighten `.claude/delivery-policy.yaml`. An order cannot raise its own autonomy,
  for the same reason a plan cannot weaken a stage.
- `trust: external` orders are scanned with `loom ki scan` before pickup and stop at the design gate
  whatever `autonomy` says.
- `depends-on` ids must all be shipped in the ledger.
- `budget` limits are recorded with the claim, so a parked order says which limit stopped it.

## Naming

`<id>` is kebab-case and describes the work, not the ticket: `user-auth-lockout`, not `PROJ-1234`.
The ticket key belongs in `source-ref`. A readable id is what you will see in workspace paths, branch
names, and the approval inbox.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
