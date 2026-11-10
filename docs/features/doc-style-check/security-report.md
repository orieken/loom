---
feature: "doc-style-check"
bounded_context: ""
domain_terms: []
files_touched: []
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Security Review: doc-style-check

## Threat Model Summary

loom docs check is a local, offline CLI linter: it walks caller-supplied file/directory paths plus a fixed-name DOMAIN_DICTIONARY.md in the working directory, reads Markdown with os.ReadFile, and reports findings to stdout. No auth, no network calls, no secrets, no privilege boundary — the invoking user already has filesystem access to every path it reads, so this is not a trust-boundary crossing in the sense STRIDE targets. All pattern matching uses Go's regexp package (RE2), which is immune to catastrophic-backtracking ReDoS by construction, so even adversarial Markdown content cannot cause algorithmic blowup. No new dependencies were introduced (go.mod/go.sum unchanged). Reviewed internal/docstyle/check.go, markdown.go, dictionary.go, and cmd/loom/cmd/docs_check.go directly. No findings; no files modified.

## Dependency Audit

- No new dependencies introduced — go.mod and go.sum are unchanged by this feature; stdlib-only implementation (os, path/filepath, regexp, strings, sort).

## STRIDE Analysis

None

### Spoofing

No identity, session, or auth concept exists in this feature — it is an unauthenticated local CLI command. Not applicable.

### Tampering

Input is Markdown read via os.ReadFile from caller-given paths; output is a formatted text report to stdout. No data is written back to the source files, and the report does not feed any downstream system that trusts it as authoritative (spec itself says 'a report before it is ever a gate').

### Repudiation

No audit trail requirement applies to a local read-only lint command; findings are ephemeral stdout output with no state mutation to repudiate.

### Information Disclosure

Egress: zero network calls anywhere in internal/docstyle or cmd/loom/cmd/docs_check.go (confirmed by reading every file) — nothing for this component to exfiltrate through. Output stays on stdout under the invoking process's own control.

### Denial of Service

Checked every regexp.MustCompile call site (markdown.go, dictionary.go, ignore.go, modals.go, passive.go, sentences.go, vocabulary.go) for backtracking-prone constructs; Go's RE2-based regexp engine guarantees linear-time matching regardless of pattern, so ReDoS is structurally impossible. filepath.WalkDir does not follow symlinked directories, so no symlink-cycle unbounded walk. No size limits on file reads, but this operates on local Markdown docs the user chose to scan, not attacker-supplied payloads over a network boundary — accepted as out of scope per the spec's stated trust boundary.

### Elevation of Privilege

Runs with the invoking user's own filesystem permissions; performs no setuid, no privilege changes, no cross-user data access. Not applicable.

## Findings

None

## Files Modified

None

## Security Checklist

- [x] No hardcoded secrets, API keys, or credentials
- [x] No network calls introduced
- [x] No new external dependencies added
- [x] No command/shell injection surface (no exec.Command, no shell interpolation)
- [x] No SQL or raw query construction
- [x] Regex matching uses RE2 (Go stdlib) — no ReDoS-prone backtracking engine
- [x] Directory walk does not follow symlinks into unbounded traversal
- [x] Output (findings/excerpts) contains only content from files the invoking user already has read access to

## Notes for QA

- No security-driven test requirements beyond what analysis.md/acceptance criteria already specify — this feature has no auth, no network, and no sensitive-data path to add regression coverage for.
- If `loom docs check` is later wired into CI over untrusted contributor-authored Markdown (e.g. a PR's doc changes), revisit the DENIAL_OF_SERVICE line above: today it's scoped to a human deliberately running the linter locally, which is a materially different trust boundary than running it unattended over arbitrary input in CI.

## Notes for Tech Writer

None
