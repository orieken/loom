---
feature: "pre-ship-checks"
bounded_context: "Agent Orchestration"
domain_terms: ["Pre-ship check"]
files_touched: ["docs/MIGRATION.md", "docs/ARCHITECTURE.md"]
issue_refs: ["PR #18"]
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md", "docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md"]
linked_kis: []
---

# Documentation Report: pre-ship-checks

## Files Updated

- `docs/MIGRATION.md` — Added a new, additive "`preShipChecks` in `delivery-policy.yaml`" entry near
  the existing ADR-010 "Removed: auto-proceed keys" section. States the entry's shape (`name`,
  `command`, optional `timeout`, defaulting to 10 minutes), where the new `pre-ship-checks` stage
  runs (between `tech-writer` and `devops-engineer`, before `confirm-ship`), what happens on a
  failing or timed-out check (terminal stage failure, no supervisor retry, later checks never run),
  what happens with no checks configured (`SKIPPED`, not `COMPLETED`), and that the key is orthogonal
  to the `policiesEnabled` kill switch. Added one further paragraph, per the security reviewer's
  note for tech-writer, stating explicitly that a check's output may contain secrets from the
  project's environment, that this output is retained only in the run's workspace artifact
  (`pre-ship-checks.md`), and that it never reaches telemetry — spans and events carry only the
  check's name, exit status, and duration.
- `docs/ARCHITECTURE.md` — Extended the one-paragraph `internal/orchestrator/` description in the
  Directory Reference to name the second executor-internal stage, `pre-ship-checks`, and what it
  guards: running the project's declared `delivery-policy.yaml` commands before `confirm-ship` so a
  run never reaches the ship gate with a check CI would reject still failing.

## Files Unchanged (and why)

- `CHANGELOG.md` — This repository has no root `CHANGELOG.md` (searched; only unrelated nested
  fixture/package changelogs exist under `internal/references/testdata/`, `shared/agents/`, and
  `shared/schemas/`). Nothing to append to.
- `README.md` — Documents `loom run`'s existing gates (`confirm-design`, `confirm-security`,
  `confirm-ship`) but has no section enumerating `delivery-policy.yaml` keys; that enumeration lives
  in `docs/MIGRATION.md` and `shared/templates/delivery-policy.template.yaml`. No end-user-facing
  concept in README.md needs a `preShipChecks` mention.
- `shared/templates/delivery-policy.template.yaml` and `shared/policies/README.md` — Both outside
  `scope.paths` for this feature; the spec's own Out of Scope section (and `analysis.md`) name
  `docs/MIGRATION.md` as the one place to document the new key instead. Not touched.
- `shared/DOMAIN_DICTIONARY.md` — Also outside `scope.paths`; the spec defers promoting "Pre-ship
  check" to the dictionary to a human or a later feature. Recorded here in this report's
  `domain_terms` frontmatter instead.
- No module-level docstrings, function docstrings, or JSDoc were added — this iteration (the one
  handed to tech-writer) was a pure Extract Function refactor (`persistCompletion` →
  `persistCompletedStage`, `LoadConfig` → `decodeAndValidate`) fixing two code-review complexity
  findings, with no new public API surface and no behavior change per implementation-notes.md and
  code-review-report.md (status APPROVED). The feature's own doc-facing surface — the
  `preShipChecks` schema, the stage's position, and `scripts/pre-ship-check.sh` — was already
  documented in `docs/MIGRATION.md` and `docs/ARCHITECTURE.md` in an earlier pass; this pass only
  added the secrets/output-retention detail the security reviewer flagged as missing.

## Notes for DevOps

- No new environment variables for a human to set. `LOOM_BASE_COMMIT` is set by the executor itself
  (to the run's `StartCommit`) for every pre-ship check's child process — it is not something a
  deployment runbook needs to configure.
- After this merges, a human must add loom's own `preShipChecks` entry to `.claude/delivery-policy.yaml`
  (agents cannot write under `.claude/`): `preShipChecks: [{name: changed-line coverage, command:
  scripts/pre-ship-check.sh}]`. This is the Infrastructure/Deploy Note the spec itself calls out —
  until that line is added, this repository's own runs get no pre-ship check, same as before this
  feature shipped.
- `scripts/pre-ship-check.sh` must be executable in CI's and any local checkout's environment and
  needs no new runbook entry — it shells out to `go test` and `go run ./cmd/diff-coverage`, both
  already present in this repository's toolchain.
- A pre-ship check's command can print secrets from the project's environment to stdout/stderr. That
  output lands only in the run's workspace artifact (`pre-ship-checks.md`), never in telemetry —
  DevOps should treat the workspace directory with the same handling as any other artifact that may
  carry sensitive output, no new retention or redaction tooling is introduced by this feature.
