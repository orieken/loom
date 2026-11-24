---
feature: "factory-fetch-each-pass"
bounded_context: "Factory supervisor — intake fetched from the remote before each pass (ADR-010)"
domain_terms: ["intake", "ready", "claim", "worktree", "base-branch", "sourceCommit", "remote-tracking ref", "fetch-before-read"]
files_touched: ["templates/claude-feature-team/intake/README.md"]
issue_refs: ["roadmap L3.66", "roadmap L3.68", "roadmap L3.70"]
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md", "docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md"]
linked_kis: []
---

# Documentation Report: factory-fetch-each-pass

## Files Updated

- `templates/claude-feature-team/intake/README.md` — Already carries the fetch-before-each-pass
  behavior, in its "Enforced at pickup" section (added during implementation, since it is this
  feature's designated docs file per acceptance criterion 8 and `scope.paths`). Checked it line by
  line against AC8 and `shared/rules/writing-style.md`:
  - States the supervisor fetches `--remote` (default `origin`) once per pass, then reads orders
    committed there, never the working tree.
  - States a promotion to `ready` must be pushed, not just committed locally, for the next pass to
    see it — the way code reaches `main` reviewed.
  - States `--no-fetch` restores reading the local branch only, with no fetch, exactly as before
    this behavior existed.
  No change was needed beyond what the developer already wrote; the prose matches the surrounding
  file's existing tone (dense, operator-facing) and contains no "coming soon" or speculative
  language.
- `cmd/loom/cmd/factory.go` — `factoryRunCmd`'s `Long` cobra help text (already written by the
  developer, inside `scope.paths`) accurately documents: the fetch runs once per pass against
  `--remote`/`--branch`; a failed fetch is logged on one line and the pass goes on from the last
  successfully fetched ref, never parking or failing an order over it; the person's own checkout —
  branch, index, and working tree — is never touched; and `--no-fetch`, or no remote by that name,
  falls back to the local branch unchanged. Verified this text against every acceptance criterion;
  no gap found, no edit needed.

## Files Unchanged (and why)

- `cmd/loom/README.md` — This is the top-level, most-visible doc for `loom factory run`. Its
  "Running the factory (experimental)" section still opens with "`loom factory run` is the
  supervisor (ADR-010). It reads `intake/*.md` **as committed** on `--branch`..." — no mention of
  `--remote`, `--no-fetch`, or the fetch-before-read sequence this feature adds, so a reader there
  would not learn that a promotion must be pushed to be seen. It is **not** in this order's
  `scope.paths` (`internal/factory/**`, `internal/worktree/**`, `cmd/loom/cmd/factory*.go`,
  `templates/claude-feature-team/intake/README.md`, `docs/features/factory-fetch-each-pass/**`),
  and acceptance criterion 9 requires no file outside `scope.paths` to change. Flagging this as a
  real gap, not a judgment call: recommend a follow-up order to add one or two sentences to
  "Running the factory" — fetch is the first thing each pass does, `--remote` defaults to `origin`,
  a failed fetch never parks or fails an order, and `--no-fetch` restores the old local-branch
  read — matching the language already shipped in the intake README and the CLI's own `--help`.
- `docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md` — No amendment needed.
  Decision 2 already decided the supervisor reads intake "committed on `base-branch`"; this feature
  changes only *how* that commit is reached — fetching `<remote>/<branch>` first, rather than
  trusting whatever the local branch already points to. The provenance guarantee Decision 2 relies
  on (a reviewed commit, recorded exactly as the claim's `sourceCommit`) is unchanged; it is now
  exact against the pushed/remote state instead of a possibly-stale local checkout. Also out of
  `scope.paths`.
- Root `CHANGELOG.md` — This repository has no root-level changelog (only nested ones under
  `shared/agents/`, `shared/schemas/`, and a test fixture, none touched by this feature), so there
  is nothing to append to and no file to create outside `scope.paths`.
- `docs/features/README.md` — Describes the generic pipeline-artifact convention; this feature
  introduces no new artifact type, so no change applies.
- `README.md` (repository root) — Does not document individual `loom factory` subcommands at this
  level of detail; that is `cmd/loom/README.md`'s job, covered above. No change applies here.

## Notes for DevOps

- None beyond what the spec already states: after this ships, restart both supervisors with the
  new build (spec's "Infrastructure / Deploy Notes"; matches `implementation-notes.md`'s "Notes for
  DevOps"). No new environment variable, configuration key, or infrastructure change results from
  this feature — the fetch uses the repository's already-configured remote and the person's own
  git credentials, nothing new is sent or stored.
