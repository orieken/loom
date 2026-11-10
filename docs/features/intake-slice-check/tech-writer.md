Code-level doc comments are already present and complete throughout the salvaged implementation — no gaps to fill there. Documentation is already current: `templates/claude-feature-team/intake/TEMPLATE.md` and `README.md` were updated by the salvaged commit, QA confirmed they match `order.go`'s `Slice` fields exactly. No root `CHANGELOG.md` exists in this repo, and `cmd/loom/README.md`'s CLI reference row falls outside this order's `scope.paths`, so updating it would breach AC10's file-scope lock — I'm leaving it alone and noting it as a follow-up rather than editing out of scope. No new ADR is warranted: this is an additive, backward-compatible schema extension already covered by the existing ADR-008/ADR-010 references, not a new architectural decision.

Here is the docs report.

```markdown
---
feature: "intake-slice-check"
bounded_context: "Agent Orchestration"
domain_terms: ["Slice", "Level"]
files_touched: []
issue_refs: ["intake-slice-check", "decision D-31"]
linked_adrs: ["docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md", "docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Documentation Report: intake-slice-check

## Files Updated
- None. Every documentation change this feature needs was already made in the salvaged implementation (commit `432f912`), before this stage ran:
  - `templates/claude-feature-team/intake/TEMPLATE.md` — carries the commented `slice:` block (`of`, `spec`, `digest`, `manifest`, `covers`) and a "SLICE" explanation section describing what gets checked and when the block may be deleted.
  - `templates/claude-feature-team/intake/README.md` — documents the cross-order check under "Enforced now, by `loom intake check`": coverage per criterion level, naming both orders on a double-cover, digest agreement against the manifest's own file hashes, and the `depends-on` cycle/dangling checks among slices of one spec.
  - Verified against `internal/intake/order.go`'s `Slice` struct field-by-field (`of`, `spec`, `digest`, `manifest`, `covers`): no drift between docs and code, matching QA's independent confirmation in `qa-report.md`.

## Files Unchanged (and why)
- `cmd/loom/README.md` — its CLI reference table documents `loom intake check [file|dir]...` at the single-order level today. Adding the cross-order slice behavior there would be the natural "new capability" update, but this file is outside this order's `scope.paths` (`internal/intake/**`, `cmd/loom/cmd/intake*.go`, `templates/claude-feature-team/intake/**`, `docs/features/intake-slice-check/**`), and AC10 requires no file outside that set to change. Flagging as a follow-up rather than editing it.
- `CHANGELOG.md` — no root changelog exists in this repository (only unrelated nested ones under `shared/agents/`, `shared/schemas/`, and a test fixture). Nothing to add an entry to.
- `docs/ARCHITECTURE.md`, `docs/AGENT_REFERENCE.md` — neither mentions `intake` or `loom intake check` today; this feature extends an existing CLI validation path rather than the agent roster or system architecture, so neither needs a new section.
- `docs/adrs/` — no new ADR. The `slice` block is an additive, backward-compatible frontmatter field under intake schema version 1 (an order with no `slice` block validates exactly as before); the decision to express it this way was already captured in `analysis.md`'s fitness functions and rests on the existing ADR-008 (test-can-fail discipline) and ADR-010 (order state) patterns rather than introducing a new one.
- `internal/intake/slice.go`, `internal/intake/slice_check.go`, `internal/intake/order.go`, `cmd/loom/cmd/intake.go` — all public types and functions already carry intention-revealing doc comments from the salvaged implementation (`FileReader`, `SpecPackage`, `Criterion`, `Slice`, `CheckSlices`, `SliceSummary`, `checkIntakeSlices`, `slicedSpecs`, etc.). Read-through against the Code-Level Docs checklist found nothing missing.
- `docs/features/README.md` — its artifact-type table is generic across all features and already covers `docs-report.md`; no feature-specific edit applies.

## Notes for DevOps
- None required by this delivery — no new env vars, infra, or deployment runbook entries. The feature is a pure addition to the existing `loom intake check` Go command, invoked the same way it already is in CI (`.github/workflows/framework-ci.yml`).
```