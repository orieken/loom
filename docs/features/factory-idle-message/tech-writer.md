---
feature: "factory-idle-message"
bounded_context: "Agent Orchestration"
domain_terms: ["Idle", "Waiting on a human"]
files_touched: ["cmd/loom/README.md"]
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Documentation Report: factory-idle-message

## Files Updated
- `cmd/loom/README.md` — Added a paragraph to the "Running the factory (experimental)" section (after
  the existing paragraph on transient/terminal failure and finish-line approval) documenting the two
  new console lines the supervisor prints when a tick ends idle or waiting on a human, the once-per-state-entry
  rule, and that `--once` follows the same rule. This is the only place in the repo that documents
  `loom factory run`'s observable behavior, and feature commits to this package update it directly
  (see #8, #10) rather than through a separate CHANGELOG.

## Files Unchanged (and why)
- `CHANGELOG.md` — This repository has no top-level `CHANGELOG.md`; `cmd/loom/README.md` is the
  convention this codebase already uses for documenting `loom factory`'s user-visible behavior, and is
  updated instead.
- `docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md` — architecture-notes.md's own
  review found no RFC trigger for this change (no layer boundary touched, no new abstraction, single
  package affected, reversible by a normal `git revert`) and recorded no new ADR. The two console lines
  are additive output on an existing `Tick()` call path, not a structural decision ADR-010 would need to
  capture.
- `internal/factory/idle.go`, `internal/factory/supervisor.go` — Already carry package- and
  function-level doc comments for every new type and function (`tickKind`, `tickState`, `classify`,
  `waitingCount`, `hasUnshippedOrder`, `announce`, `reportTickState`), written by the developer in the
  implementation itself. No additional code-level documentation is needed.
- `docs/roadmaps/factory-phases.md` — This feature was not driven by a numbered roadmap item; it
  originated from operator feedback during the second dogfood run (see the feature spec's "Request"
  section), so there is no roadmap checklist entry to mark complete.
- Setup, configuration, and API docs — No new CLI flag, config field, environment variable, or ledger
  event was introduced. The feature is two new console lines only, computed from state the supervisor
  already holds.

## Notes for DevOps
None.
