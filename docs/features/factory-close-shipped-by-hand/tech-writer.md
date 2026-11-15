Now I'll produce the docs-report.md artifact.

```markdown
---
feature: "factory-close-shipped-by-hand"
bounded_context: "Factory supervisor — order lifecycle and ledger (ADR-010)"
domain_terms: ["order", "ledger", "phase", "shipped", "parked", "stopped", "worktree", "supervisor", "approver"]
files_touched: ["templates/claude-feature-team/intake/README.md"]
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Documentation Report: factory-close-shipped-by-hand

## Files Updated

- `templates/claude-feature-team/intake/README.md` — Already carries the new "An order finished by hand" section (added during implementation, since it sits in `scope.paths` as the feature's designated docs file per `architecture-notes.md`'s Component Placement). Reviewed it against this report's checklist and against `shared/rules/writing-style.md`:
  - States exactly when to use `close`: after an order parks or stops and a person finishes the work in their own pull request, once that pull request merges.
  - States what `close` does: records the order shipped the way the supervisor would have, removes it from `loom factory inbox`, and unblocks any order whose `depends-on` names it, with no edit to either order.
  - States the trust boundary explicitly: `close` never calls GitHub — it trusts the person running it that the URL merged. This matches the spec's Trust Boundaries section and `architecture-notes.md`'s decision to give `Close` no GitHub-reachable dependency at all.
  - No change was needed beyond what the developer already wrote; the prose matches the surrounding file's existing tone (technical, dense, operator-facing) and contains no "coming soon" or speculative language.

## Files Unchanged (and why)

- `cmd/loom/cmd/factory.go` — `factoryCloseCmd`'s `Short` and `Long` cobra help text already states what `close` records, when to use it, and that it never calls GitHub or checks that the URL merged (verified by reading the command definition directly). No further code-level doc needed; cobra help is this CLI's inline documentation mechanism and it was written by the developer stage as part of the command, not left for this stage.
- `cmd/loom/README.md` — This is the natural place to add `close` to the top-level command table and the "Running the factory" narrative, alongside `stop`/`resume`/`rerun`. It is **not** in this order's `scope.paths` (`internal/factory/**`, `cmd/loom/cmd/factory*.go`, `templates/claude-feature-team/intake/README.md`, `docs/features/factory-close-shipped-by-hand/**`), so it is left unchanged here. Flagging this as a real gap, not a judgment call: a reader of that file today sees `stop`/`resume`/`rerun` documented but no `close`. Recommend a follow-up order to add one row to the command table (`| `loom factory close <order> --shipped-by <url>` | Record that a parked or stopped order shipped by hand, in its own pull request |`) and a short mention in the "Running the factory" section once scope allows it.
- `docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md` — No new ADR, and no amendment to ADR-010, is warranted: the spec's own Domain Language section states no new terms were introduced, and "ship" and "park" keep the meanings ADR-010 already gives them. `close` is a new way to reach the existing `shipped` phase, not a new phase or policy. Also out of `scope.paths`.
- Root `CHANGELOG.md` — This repository has no root-level changelog (only nested ones under `shared/agents/`, `shared/schemas/`, and a test fixture, none of which this feature touches), so there is nothing to append to and no file to create outside `scope.paths`.
- `docs/features/README.md` — Describes the generic pipeline-artifact convention; this feature introduces no new artifact type, so no change applies.

## Notes for DevOps

- None. No new environment variables, infrastructure, or deployment runbook entries result from this feature — `close` only appends a ledger line, archives run state, and removes a worktree, all using mechanisms (`Repository`, `Layout`, the ledger) that already exist and are already operated today.
```