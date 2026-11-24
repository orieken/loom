---
feature: "state-files-deleted"
bounded_context: ""
domain_terms: ["PathClaim", "WorkTree", "ClaimError", "posture check"]
files_touched: []
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Security Review: state-files-deleted

## Threat Model Summary

This feature adds filesDeleted/testFilesDeleted claim fields to loom's own pipeline-state schemas, a second claim direction (MustNotExist) to PathClaim, and a requireNoRenameArrows validation rule — all inside loom's own orchestrator/state packages that validate an agent's self-reported work against the filesystem and git-observed changes. There is no user-facing input, no auth, no network call, no secret handling, and no new dependency (go.mod/go.sum unchanged, confirmed via diff). The spec's own Trust Boundaries section states 'None — paths are checked inside the run's own worktree only,' and the implementation does not cross that: resolveWithinRoot (pre-existing, unmodified by this change) still clamps every claimed path to the project root before any stat, so the new deletion-claim path (existingPaths/missingPaths) inherits the same traversal guard as the existing existence-claim path. The only functional diff in this iteration was a doc-comment fix on posture.go with no logic change, confirmed by git diff and by the code-reviewer's own verification. Net effect on the threat model is positive, not neutral: the feature closes a gap where a stage could misreport a rename/deletion and have the claim-checker silently accept it as 'work not done' or silently miss a false deletion claim — it adds integrity checking, it does not remove any.

## Dependency Audit

- No new dependencies introduced — git diff of go.mod/go.sum against main is empty.
- go mod verify: all modules verified.
- go vet ./...: clean.

## STRIDE Analysis

None

### Spoofing

No identity or authentication surface exists in this code path — claims are self-reported by pipeline stages within the same trust level as before, and the orchestrator does not authenticate callers. Not applicable.

### Tampering

This is the category the feature strengthens. verifyDeletionClaim (internal/orchestrator/claims.go) checks a claimed deletion two ways: a filesystem stat (existingPaths, routed through the pre-existing resolveWithinRoot guard which rejects any path resolving outside the project root) and, when a WorkTree is configured, cross-checks against the git-observed changed set (unseenDeletions) so a path that was never actually touched cannot be claimed as deleted. requireNoRenameArrows closes the specific 'old -> new' single-field encoding that previously let a correct rename be misread. No new tampering surface was introduced; an existing gap was closed.

### Repudiation

ClaimError names the stage, field, and paths, so a false or suspicious claim is attributable to the stage that made it, consistent with the existing MustExist claim errors. No audit trail was removed or weakened.

### Information Disclosure

(egress) No outbound network calls, API calls, or egress of any kind are introduced by this change — it is pure in-process filesystem stat and in-memory map lookup.

### Denial of Service

The new code paths (missingPaths/existingPaths/unseenDeletions) are the same O(n) loop-over-claimed-paths shape as the pre-existing MustExist checks, bounded by the same agent-report size as before — no new unbounded iteration or amplification was introduced.

### Elevation of Privilege

No roles, permissions, or privilege levels are modeled or checked in this subsystem — not applicable.

## Findings

None

## Files Modified

None

## Security Checklist

- [x] No hardcoded secrets, API keys, or credentials introduced
- [x] No new external dependency added (go.mod/go.sum unchanged)
- [x] Path handling for the new deletion-claim checks reuses the existing resolveWithinRoot project-root containment guard — no path traversal introduced
- [x] No new network, auth, session, or token handling introduced
- [x] Rendered output (markdown) uses the same bullet/frontmatter pattern as existing fields — no new injection sink
- [x] go vet and go mod verify clean; internal/state and internal/orchestrator test suites pass

## Notes for QA

- No security-driven test additions were needed — this feature has no auth, network, or external-input trust boundary. The existing claims_test.go / implementation_chain_test.go / qa_state_test.go coverage (verified green in this review) already exercises the true-deletion, false-deletion, never-existed, and rename-arrow-rejection cases that map onto the TAMPERING assessment above.

## Notes for Tech Writer

None
