# Documentation Cleanup TODO

Folder-by-folder audit ledger for root documentation and `docs/`. Do not move or delete an artifact
until its consumers have been checked and the proposed action has been approved.

## Repository Root

- [x] Consolidate the root blueprint prompts. Move the live `API_FRAMEWORK_BLUEPRINT_PROMPT.md` into
  `docs/blueprints/`; remove `E2E_FRAMEWORK_BLUEPRINT_PROMPT.md` and
  `BLUEPRINT_GENERATOR_PROMPT.md` after retiring their only consumer. Update the `api-ingest` skill,
  `docs/patterns/sunday-framework-patterns.md`, and the root-Markdown scan commentary in
  `scripts/health-check.sh`.
- [x] Retire the legacy extensionless `install`/`uninstall` path in favor of `install.sh` and
  `uninstall.sh`. Remove its unconsumed `.claude.md` compatibility source and record the convention in
  ADR-005.
- [x] Keep `README.md`, `LICENSE-CONTENT.md`, `AGENTS.md`, `CLAUDE.md`, `.openai.md`, and
  `CODEMAP.md` at the repository root. Their locations are conventional or explicitly consumed by
  platform tooling and validation scripts.
- [x] Keep the root `ARCHITECTURE_RULES.md`, `DOMAIN_DICTIONARY.md`, and `TEAM_TOPOLOGY.md` symlinks.
  They expose canonical files from `shared/` at fixed compatibility paths used by skills and generated
  platform configurations. *Verified 2026-09-22: all six root files present; the three symlinks
  resolve into `shared/`.*

## `docs/` Root

- [x] Delete or consolidate `docs/CLAUDE.md`. It is a divergent copy of root `CLAUDE.md`, has not been
  updated since 2026-03-01, and is only indexed as a reference document. Before deletion, review its
  unique legacy/refactoring guidance and merge anything still authoritative into the canonical source.
- [x] Consolidate `docs/RUNBOOKS.md` into `docs/runbooks/README.md`, or turn it into a short redirect.
  It currently summarizes only `debug-environment` and `debug-tests`, while the runbooks directory has
  become the real operational index. Update references in `docs/README.md`,
  `shared/agents/documentation-manager.md`, and queued epic prompts if the file is removed.
- [x] Fix the five broken links in `docs/roadmap-2026-08-07.md`; each Epic 69-73 table link needs the
  `prompts/` prefix.
- [x] Refresh `docs/ARCHITECTURE.md` against canonical inventory and `shared/platform-registry.json`.
  It claims 25 agents and 56 skills instead of 39 and 69, describes six platform outputs despite the
  current nine-entry registry (covering ten named tools), and contains tier/generation details that should
  be revalidated rather than patched as isolated counts.
- [x] Refresh `docs/ONBOARDING.md`. Its opening claims 24 agents and 53 skills instead of 39 and 69.
  Prefer linking to generated/current inventory rather than embedding counts that drift.
- [x] Expand `docs/README.md` so its directory tree includes the current root documents and collections:
  `THREAT_MODEL.md`, `human-tasks.md`, `roadmap-2026-08-07.md`, this cleanup ledger, `aos/`, `prompts/`,
  and any other directory retained after its own audit.
- [x] Reconcile `docs/human-tasks.md`. Three listed prompt paths now live under `docs/prompts/done/`, the
  Phase 2 handoff is complete, and the roadmap says Phase 4 is complete. Remove resolved decisions from
  the queued table or move them into a clearly historical section; verify the external `saturday-mcp`
  entries separately.
- [x] Update `docs/THREAT_MODEL.md` with an implementation-status annotation. It still presents itself as
  “Op 1 of 2” and labels mitigations as candidates even though multiple Epic 65 Op 2 controls are present
  (`memory-trust-boundary`, spec-ingestion checks, sync provenance, and disabled hook examples).
- [x] Correct or clarify the approval language in `docs/CONTRIBUTING.md` under “Adding a new rule.” It
  currently says every `shared/rules/` change is Gate #7, while Gate #7 specifically governs wiring a new
  fitness function into CI/CD.
- [x] Extend `scripts/check-inventory-drift.sh` to inspect living count claims in at least
  `docs/ARCHITECTURE.md` and `docs/ONBOARDING.md`. The current check reported zero drift while both files
  contained stale counts. Wiring this new fitness-function coverage requires the repository's approval
  gate before implementation.
- [x] Keep `docs/AGENT_REFERENCE.md`: all 39 canonical agent names are represented and its stated count is
  current. *Re-verified 2026-09-22: 40 agents (41 files less `CHANGELOG.md`), every name present;
  `check-inventory-drift.sh` reports 0 drift.*
- [x] Keep `docs/MIGRATION.md`: it documents the supported pre-`shared/` migration and current version
  marker behavior.

### Link-check result

The relative-link scan of files directly under `docs/` found five broken Markdown links, all in
`docs/roadmap-2026-08-07.md` and all covered by the task above. Subdirectories have not yet been scanned.

---

*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context
Engineering Framework by Oscar Rieken — licensed under
[CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy
or adapt this file, please keep this attribution.*
