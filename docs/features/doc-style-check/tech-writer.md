This confirms the required documentation (AC6) is already in place from the developer stage. My job is to verify completeness and record what's in/out of scope.

```markdown
---
feature: "doc-style-check"
bounded_context: "Documentation Knowledge Base"
domain_terms: ["Rule"]
files_touched: ["shared/rules/writing-style.md"]
issue_refs: ["doc-style-check"]
linked_adrs: ["docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md"]
linked_kis: []
---

# Documentation Report: doc-style-check

## Files Updated

- `shared/rules/writing-style.md` — Already carries the required documentation from the
  implementation stage (AC6): a new "Mechanical check" section names `loom docs check <path>...`,
  lists the rules it checks (sentence length, plain vocabulary, modal conflicts, estimated
  passive-voice share, `DOMAIN_DICTIONARY.md` "Synonyms to AVOID" terms), and states plainly which
  rules stay with a human or agent reviewer (active voice as a whole, noun clusters,
  one-term-one-meaning) and why the noun-cluster rule itself is not checked (no part-of-speech
  tagging available without a new dependency). Verified this section accurately describes the
  shipped command by reading `cmd/loom/cmd/docs_check.go`'s help text and `internal/docstyle/check.go`
  — the two match. No further edit was needed here.
- `cmd/loom/cmd/docs_check.go` — Already documents itself: `docsCheckLongHelp` names every rule the
  command runs and states the noun-cluster omission and its reason, matching
  `shared/rules/writing-style.md`. Verified against `TestDocsCheckHelpDocumentsTheNounClusterOmission`
  (qa-report.md) — no doc change needed beyond what the developer wrote.

## Files Unchanged (and why)

- `README.md` (repo root) — Outside this feature's `scope.paths`; also, the root README documents
  framework installation and maturity levels, not individual `loom` subcommands, so a report-only,
  non-gating CLI check has no section to land in there.
- `cmd/loom/README.md` — This is the file that enumerates `loom docs` subcommands
  (`loom docs plan|approve|apply`) in its command table, and is the natural place to add a
  `loom docs check` row. It is outside `scope.paths` for this feature
  (`internal/docstyle/**`, `cmd/loom/cmd/docs*.go`, `shared/rules/writing-style.md`,
  `docs/features/doc-style-check/**`), and AC8 requires no file outside that scope to change. Adding
  the row is a one-line follow-up for a separate, explicitly scoped change rather than something to
  slip into this one.
- `CHANGELOG.md` — No root `CHANGELOG.md` exists in this repository (only package-local ones under
  `shared/agents/`, `shared/schemas/`, and a test fixture); there is no `[Unreleased]` ledger this
  feature's user-facing addition belongs in. Nothing to add.
- `docs/features/README.md` — Already describes the generic feature-archive convention
  (`docs/features/<feature-name>/`); this delivery follows that convention without needing any
  change to the convention document itself.
- API documentation, configuration docs, migration guides — Not applicable; this feature adds a
  local CLI subcommand with no API surface, no config options, and no database or breaking changes.

## Notes for DevOps

None — no new environment variables, infrastructure, or deployment steps. `loom docs check` is a
local, offline, read-only command with no network dependency (confirmed in
`implementation-notes.md` and the feature's own Non-Functional Requirements).
```