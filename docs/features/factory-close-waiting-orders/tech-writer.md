Now producing the artifact.

```markdown
---
feature: "factory-close-waiting-orders"
bounded_context: "Factory supervisor — order lifecycle and ledger (ADR-010)"
domain_terms: ["waiting", "completed", "proposed", "parked", "stopped", "shipped", "running", "committed", "pushed", "retrying", "closeable", "waitingOnHuman"]
files_touched: ["cmd/loom/cmd/factory.go", "templates/claude-feature-team/intake/README.md"]
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Documentation Report: factory-close-waiting-orders

## Files Updated

None by this stage. Both of the two user-facing doc surfaces this feature requires were already brought
up to date by the developer, as part of implementing the behavior, and verified correct by QA:

- `cmd/loom/cmd/factory.go` — `factoryCloseCmd`'s `Short` and `Long` text now name all five phases
  `close` accepts: parked, stopped, waiting at a run gate, completed and awaiting its commit approval,
  and proposed and awaiting its pull-request approval. It also still states, unchanged, that `close`
  never calls GitHub and never checks that `--shipped-by` merged.
- `templates/claude-feature-team/intake/README.md` — the "An order finished by hand" section names the
  same five accepted phases and still lists the refused phases (claimed, running, retrying, committed,
  pushed) that require stopping the order first.

I read both against the implemented behavior in `internal/factory/control.go`'s `closeable` and
`waitingOnHuman` and against QA's verification note, and confirm the wording matches: `waitingOnHuman`
names exactly `PhaseParked`, `PhaseStopped`, `PhaseWaiting`, `PhaseCompleted`, `PhaseProposed`, and every
other phase (including `PhaseRetrying`, left unresolved as "no" in the spec's Open Questions) is still
refused. No edit was needed in either file, and this feature's `scope.paths` does not include any other
documentation surface, so none was made.

## Files Unchanged (and why)

- `CHANGELOG.md` — no root changelog exists in this repository to append to; the three `CHANGELOG.md`
  files present (`internal/references/testdata/fixture/`, `shared/agents/`, `shared/schemas/`) belong to
  unrelated fixtures and sub-packages, not this feature.
- `docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md` — documents the phase model and
  policy configuration decisions; it does not enumerate `close`'s own allow-list today, so widening that
  allow-list is a behavior change within the existing model, not a decision ADR-010 needs amending to
  record. It is also outside this feature's `scope.paths`.
- `docs/ARCHITECTURE.md`, `docs/AGENT_REFERENCE.md`, `cmd/loom/README.md` — none describe `close`'s
  per-phase allow-list at a level of detail this change affects, and all are outside `scope.paths`.
- No API documentation, configuration docs, migration guide, or new ADR applies — this feature changes
  one in-memory phase check (`closeable`/`waitingOnHuman` in `internal/factory/control.go`), adds no
  endpoint, config option, schema, or breaking change (confirmed against `implementation-notes.md`'s Key
  Decisions and `qa-report.md`'s Bugs Found: none).
- No module-level or function-level doc comments were added beyond what the developer already wrote on
  `waitingOnHuman` (`internal/factory/control.go`); that comment names the five phases and the rationale,
  and I verified it reads consistently with the help text and README wording above.

## Notes for DevOps

None. No new environment variables, infrastructure, or deployment steps — this change is a pure
in-memory phase-allow-list widening in the existing `close` control path.
```