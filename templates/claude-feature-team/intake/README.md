# intake/ — the factory's work orders

Every piece of work the factory may pick up is one markdown file here:
`intake/<id>.md`, copied from [`TEMPLATE.md`](TEMPLATE.md). The file is both the **work order**
(its frontmatter: priority, budget, finish line) and the **spec** (its body: the same sections as
`features/TEMPLATE.md`), so `loom run --spec intake/<id>.md` runs it with no conversion step.

**Status: validated, not yet run.** `loom intake check` enforces every rule below that can be
checked from the file alone, and is safe to run in CI today. The supervisor that claims `ready`
orders and runs them (roadmap L3.65) does not exist yet, so until it does an intake file is a spec
you hand to `loom run --spec intake/<id>.md` yourself.

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

1. **The file is the spec.** A supervisor that rewrote `status:` would be editing the input of a run
   in flight. Today `loom run` records the spec's path but not its digest, so nothing would even
   notice — which is a gap in the executor (see `docs/audits/`), not a licence to write here.
2. **One writer per fact.** If both a human and the supervisor wrote `status`, each could overwrite
   the other. Humans own intent (`ready`, `hold`, priority); the ledger owns progress.

An import adapter (GitHub issues, Linear, Jira) may create files here, always with `status: draft`
and `trust: external`. Only a human promotes it to `ready`.

## Rules

### Enforced now, by `loom intake check`

Run with no arguments to check every order in `intake/` (the template and this README are skipped);
it exits 1 if any order breaks a rule, and names the file and field for each.

- `intake` is a schema version this build reads; any other version is refused, not guessed at.
- Unknown keys are rejected: a misspelt `prority` is a setting its author believes they made.
- `id` is kebab-case and equals the file name without `.md`, because that name becomes the run's
  workspace, `.claude/feature-workspace/<id>/`.
- `type`, `status`, `source`, `trust`, `finish` and `autonomy` hold one of their listed values.
  `autonomy` has no value that loosens `.claude/delivery-policy.yaml`, so an order cannot raise its
  own autonomy, for the same reason a plan cannot weaken a stage.
- `plan` is a plan this project can run — the built-in `deliver-feature`, or a plan file under
  `.claude/plans/`. `deliver-bugfix` skips the design and verification stages, so it is accepted
  for `type: bug` only; a bug may still take the full plan.
- `priority` is 1–4, `created` is a real `YYYY-MM-DD` date, and each `budget` limit is positive.
- An imported order (`source` other than `manual`) has `trust: external`.
- A `ready` order names `requested-by`: nobody is there to ask during an unattended run, so someone
  must answer for the spec.
- `finish: pr` or `branch` names a `base-branch`.
- `depends-on` lists order ids, without itself or duplicates.
- A `trust: external` order's body is scanned with the same rules as `loom ki scan`, and each match
  is reported with its line in the file. A clean scan means the order may be read, not that it is safe.

### Enforced at pickup, by the supervisor (not built yet)

- No other file in `intake/` or `features/` has the same stem.
- Every `depends-on` id is shipped in the ledger.
- `budget` limits are recorded with the claim, so a parked order says which limit stopped it.

## Naming

`<id>` is kebab-case and describes the work, not the ticket: `user-auth-lockout`, not `PROJ-1234`.
The ticket key belongs in `source-ref`. A readable id is what you will see in workspace paths, branch
names, and the approval inbox.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
