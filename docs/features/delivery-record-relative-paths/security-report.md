---
feature: "delivery-record-relative-paths"
bounded_context: ""
domain_terms: ["delivery record", "workspace", "feature archive"]
files_touched: []
issue_refs: ["PR #13"]
linked_adrs: []
linked_kis: []
---

# Security Review: delivery-record-relative-paths

## Threat Model Summary

This change rewrites files loom itself already wrote — no new auth, network, or user-input surface. The sole security property in scope is Information Disclosure: run-state.json, run-events.jsonl, and state/*.json are committed to git under docs/features/<feature>/, so any absolute path they name (which embeds the operator's home-directory username and local checkout layout, per the originating bug — PR #13 leaked /Users/oscarrieken/Projects/... 48 times) is published in the repository's history. relativizeRunPaths/copyRelativized strip root+separator from the three in-scope file types via a plain byte substring replace, verified against the developer's own tests (TestArchivedRecordsNeverContainTheRoot, TestArchiveStripsRootFromEventText, TestArchiveLeavesTheWorkspaceByteIdentical) plus an independent `go build`/`go test ./internal/memory/... ./cmd/loom/...` run, both green. No new dependency, no new network/auth/session code, no privilege boundary crossed. Spoofing, Tampering, Repudiation, Denial of Service and Elevation of Privilege are not meaningfully in play for a same-process, same-trust-boundary file rewrite with no attacker-reachable input — confirmed against the claim's own 'Trust Boundaries: None' statement.

## Dependency Audit

- No new dependency was introduced. workspace.go adds only bytes and extends existing path/filepath, os, strings imports already present in go.mod's stdlib surface. go.mod/go.sum are unchanged (git status confirms only the three Go files named in the claim's scope.paths changed).

## STRIDE Analysis

None

### Spoofing

No identity, session, or credential is involved — the change rewrites local files loom already wrote in the same process. No spoofable actor exists in this data flow.

### Tampering

The workspace copy is read-only to this code path (os.ReadFile only) and TestArchiveLeavesTheWorkspaceByteIdentical asserts it; only the archive copy is written. No external input reaches the transform, so there is no tampering surface beyond what already existed for any file loom writes to its own workspace.

### Repudiation

Not applicable — no user action or audit trail is created or altered by this change; it is a format transform on an existing record.

### Information Disclosure

Primary category. Verified the fix correctly strips root+separator from run-state.json, run-events.jsonl and state/*.json (confirmed by reading workspace.go, the developer's tests, and git-history evidence that the bug is real: docs/features/i-4-vocabulary-test/run-state.json is already committed in commit bd9cf85 with the full local path and username). Found three residual gaps, detailed in findings: (1) a fail-open path when the root can't be resolved, (2) markdown artifacts and already-committed history are explicitly out of the fix's scope and still leak paths, (3) a latent cross-platform gap in the substring match against JSON-escaped backslashes.

### Denial of Service

bytes.ReplaceAll operates on files loom itself bounded in size (run-state.json, run-events.jsonl, typed stage JSON); there is no attacker-controlled or unbounded input reaching this path, and Go's ReplaceAll is not susceptible to catastrophic backtracking (it is not regex-based). No DoS surface.

### Elevation of Privilege

Not applicable — no authorization check exists or is bypassed; the function runs with whatever filesystem privilege the loom process already has over its own workspace and archive directories.

## Findings

### [LOW] — Root-resolution failure fails open into the exact leak this feature exists to prevent
**Location**: `cmd/loom/cmd/run_memory.go:35-39`
**Threat category**: Information Disclosure
**Description**: recordEpisode resolves root via filepath.Abs("."); if that fails, root falls back to "", and ArchiveRecords documents that an empty root is a no-op — meaning the archived run-state.json/run-events.jsonl/state/*.json are committed with absolute, username-bearing paths unchanged, with only a cmd.PrintErrf to stderr as a signal. In an automated pipeline run (which is how this code always executes), that stderr line is easy to miss, so the sanitization control can silently disengage on its one failure path rather than blocking the write. This is a deliberate, spec-mandated tradeoff — the claim's Non-Functional Requirements state archiving 'MUST stay best-effort... never fails the run' — so I did not override it; filepath.Abs(".") also only fails in rare conditions (e.g. a deleted cwd). Flagging so a human can decide whether 'best-effort' should mean 'skip the archive write entirely' rather than 'write it unredacted' when root can't be resolved.
**Fix applied**: Recommendation only
**Verification**: Simulate filepath.Abs failure is impractical to unit test directly (os.Getwd rarely fails), so QA should instead verify the stderr message text is distinctive enough to alert on ('memory: could not resolve repository root') if this pipeline's stderr is ever monitored/aggregated.
### [INFO] — Markdown artifacts and already-committed delivery records still carry the absolute checkout path
**Location**: `internal/memory/workspace.go:93-108 (archiveArtifacts); docs/features/i-4-vocabulary-test/run-state.json (git history)`
**Threat category**: Information Disclosure
**Description**: Two residual leak paths remain by explicit design, not defect: (1) archiveArtifacts copies .md files (analysis.md, code-review-report.md, etc.) byte-for-byte with no relativization — confirmed docs/features/delivery-record-relative-paths/analysis.md already contains the local /Users/oscarrieken/... path, and the claim scopes relativization only to run-state.json/run-events.jsonl/state/*.json, not markdown. (2) docs/features/i-4-vocabulary-test/run-state.json and run-events.jsonl are already committed (commit bd9cf85) with 48+ lines naming the absolute local path and OS username; the claim's Out of Scope section explicitly excludes rewriting already-committed records. Both are correct per spec, but the original complaint ('every run publishes the path of the checkout it ran in') is only partially closed: typed/event records going forward are fixed, prose artifacts and git history are not.
**Fix applied**: Recommendation only
**Verification**: No action needed to pass this feature's acceptance criteria. Recommend a follow-up ticket if the markdown-artifact and git-history exposure matter for this repository's actual visibility (public vs. private).
### [LOW] — Substring strip assumes forward-slash paths; silently no-ops on Windows due to JSON backslash-escaping
**Location**: `internal/memory/workspace.go:210-215`
**Threat category**: Information Disclosure
**Description**: relativizeRunPaths builds its search pattern as root+string(filepath.Separator) — a single backslash on Windows. encoding/json escapes a literal backslash as two backslash characters in the serialized file, so the single-backslash pattern never lines up against the double-backslash text actually present in run-state.json on that platform, and the redaction becomes a silent no-op (not even the documented empty-root no-op — it just never matches) rather than an error. This repo shows no Windows CI (.github/workflows has no Windows job) and no other GOOS-aware path handling outside one install script, so this is a latent gap rather than an active regression today.
**Fix applied**: Recommendation only
**Verification**: Not independently reproducible in this macOS/Linux session. If Windows support is ever added, add a test that marshals a Windows-style root through filepath.ToSlash or normalizes separators before the byte-level strip.

## Files Modified

None

## Security Checklist

- [x] No hardcoded secrets, tokens, or credentials introduced
- [x] No new external dependency added (go.mod/go.sum unchanged)
- [x] Workspace's own copy of run-state.json/run-events.jsonl is never mutated by archiving (byte-identity test present and passing)
- [x] Archived run-state.json/run-events.jsonl/state/*.json never contain the absolute repository root as a substring (test present and passing)
- [x] A path outside root is preserved unchanged, never turned into a ../ path or dropped (test present and passing)
- [x] No OTel/logging code added in this change (guardrail #8 n/a)
- [x] Fail-open behavior on root-resolution failure reviewed and accepted as a deliberate best-effort tradeoff per the claim's NFRs — See LOW finding — flagged for human visibility, not blocked.
- [x] Markdown artifacts and pre-existing committed records independently verified to still contain absolute paths (confirms the fix's scope boundary is as documented, not a hidden gap)

## Notes for QA

- The acceptance criteria's own test list (TestArchiveRelativizesPathsUnderRoot, TestArchivePreservesAPathOutsideRoot, TestArchiveStripsRootFromEventText, TestArchivedRecordsNeverContainTheRoot, TestArchiveLeavesTheWorkspaceByteIdentical) already covers every STRIDE-relevant property I checked; I did not re-run them beyond `go test ./internal/memory/... ./cmd/loom/...`, which is green.
- Before this feature ships, check what the final docs/features/delivery-record-relative-paths/ delivery record looks like once this run completes end-to-end — the claim explicitly carves this directory out as the one exception to 'no file outside scope.paths changes', anticipating that loom's own later archiving pass (using the fixed binary) will relativize it. Confirm that actually happens rather than leaving the mid-run unredacted snapshot I observed (run-state.json currently still names the local /Users/... path because it was archived before this fix was built).

## Notes for Tech Writer

- If documentation ever describes this archiving behavior, note the scope boundary precisely: relativization applies to run-state.json, run-events.jsonl, and state/*.json only — not the .md artifacts (analysis.md, code-review-report.md, etc.) that archiveArtifacts copies unchanged.
