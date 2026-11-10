---
feature: "intake-slice-check"
bounded_context: ""
domain_terms: ["Slice", "Level"]
files_touched: []
issue_refs: ["intake-slice-check", "decision D-31"]
linked_adrs: ["docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md", "docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Security Review: intake-slice-check

## Threat Model Summary

intake-slice-check adds a deterministic, local CLI check (`loom intake check`) that cross-validates a group of work-order files sharing one `slice.of`: coverage of a spec's criterion levels, manifest/digest integrity, and depends-on graph correctness. There is no network exposure, no authentication, no session state, and no end-user-facing surface — the only trust boundary named in the spec is that `slice.spec` and `slice.manifest` paths (and the file paths a manifest lists) originate in an order's frontmatter, which may arrive from an external source (trust: external), so a malicious or careless order could try to point those paths outside the project root. I read slice.go, slice_check.go, order.go, vocabulary.go, and cmd/loom/cmd/intake.go directly (not just implementation-notes.md) and traced every path that reaches FileReader. All three fields are confined by `isWithinProjectRoot`, a lexical, pre-read check in readRooted that rejects an absolute path, an empty path, "..", and anything whose filepath.Clean form starts with "../" — verified by direct testing (TestIsWithinProjectRoot, TestReadRootedRefusesAnEscapingPathWithoutCallingRead, TestReadSpecPackageRefusesAnEscapingPath) that confirm the reader is never invoked for an escaping path, not merely that an error is returned afterward. internal/intake performs no filesystem I/O itself (no os/io/fs imports); the one real implementation of FileReader is cmd/loom/cmd/intake.go's `os.ReadFile(filepath.Join(root, path))`, closed over a project root resolved once via worktree.RepositoryRoot. No new dependencies were introduced (go.mod/go.sum unchanged), so no dependency CVE exposure. Content integrity (tampering) is the feature's actual purpose and is handled correctly: slice.digest is checked against a live SHA-256 of the manifest bytes, and every manifest-listed file is independently re-hashed against its recorded value. No PII, credentials, tokens, or OTel instrumentation are touched by this change. I made no code changes — this is a pass-through review with no findings requiring a fix.

## Dependency Audit

- No new dependencies introduced by this change (go.mod and go.sum are unmodified by this diff) — dependency CVE scan not applicable.

## STRIDE Analysis

None

### Spoofing

No identity, session, or auth concept exists in this feature — it is a local CLI file-content check. Not applicable.

### Tampering

slice.digest is verified against a live SHA-256 of the manifest's own bytes (checkManifestDigests), and every file a manifest lists is re-hashed and compared to its recorded entry (checkManifestFiles/checkListedFile) — exactly the integrity guarantee the feature exists to provide. No case found where a mismatch is silently accepted.

### Repudiation

Not applicable — a deterministic, stateless CLI check with no audit-log requirement in the spec; CheckSlices reports every violation across every group (not just the first), so the printed output is a complete record of what the run found.

### Information Disclosure

Checked whether an externally-sourced order's slice.spec/slice.manifest/manifest-listed paths could be used to read files outside the project root or exfiltrate filesystem error detail to an untrusted party. isWithinProjectRoot performs a lexical, pre-read rejection of absolute paths and any '../'-escaping relative path before FileReader is ever called, independently verified by call-count assertions. Output (including read-error text) goes only to the operator's own terminal/CI log, not to the order's author or any remote party, so this is not a cross-trust-boundary leak. No egress / outbound network calls exist anywhere in this feature.

### Denial of Service

Considered whether a crafted order/manifest could force unbounded work: findCycle is a DFS bounded by the number of orders sharing one slice.of (a small, locally-authored set, not attacker-expandable without separate filesystem write access); loadGroupManifests/loadGroupSpecPackage cache per distinct path within a group so repeated slices sharing one manifest/spec trigger one read each, not N (independently confirmed by TestCheckSlicesReadsNothingWhenNoOrderSetsSlice and the NFR requiring zero extra reads when no slice block exists). No externally reachable network endpoint exists to flood. Residual, non-blocking observation: readManifest/readSpecPackage impose no byte-size cap on the files they read, so an oversized manifest or spec committed into the repository could cause a large single allocation — low severity, since isWithinProjectRoot already confines reads to files that are part of the project tree a human/CI operator controls, and the spec's NFR did not call out a size limit.

### Elevation of Privilege

Not applicable — the process runs with whatever filesystem permissions the invoking user/CI job already has; the feature adds a read-only check, no new write paths, no privilege boundary crossing, and confirmed no new file-write code was added anywhere in this diff (git status shows only internal/intake/**, cmd/loom/cmd/intake*.go, templates/claude-feature-team/intake/**, and the delivery record under docs/features/intake-slice-check/** changed).

## Findings

None

## Files Modified

None

## Security Checklist

- [x] Every externally-sourced path (slice.spec, slice.manifest, manifest-listed files) is confined to the project root before any read is attempted — isWithinProjectRoot + readRooted, verified by call-count tests that the reader is never invoked for an escaping path
- [x] No new dependency introduced requiring a CVE check — go.mod/go.sum unchanged by this diff
- [x] No secrets, credentials, or tokens handled or logged
- [x] No PII or sensitive data in any output or trace — This feature emits no OTel spans at all; output is file paths, order ids, and criterion ids only
- [x] Content-integrity (tampering) checks verify against the actual bytes read, not a cached or assumed value — sha256Digest(m.raw) and sha256Hex(content) are computed from the bytes just read, every time
- [x] No format-string or injection risk in violation messages — violation(field, format, args...) always uses a fixed, code-authored format string; user-controlled data only ever appears as a %s/%q argument

## Notes for QA

- No security-driven test additions were needed — the path-confinement and digest-verification behavior this review focused on is already covered by TestIsWithinProjectRoot, TestReadRootedRefusesAnEscapingPathWithoutCallingRead, TestReadSpecPackageRefusesAnEscapingPath, and the 'leaves the project root' / digest-mismatch / manifest-file-tampered subtests in TestCheckSlicesReportsEachViolationKind.
- Low-severity, non-blocking observation carried forward rather than fixed: readManifest and readSpecPackage have no byte-size cap on the files they read. If this becomes relevant (e.g. the check is ever pointed at untrusted, non-reviewed manifests), consider a size limit before reading.

## Notes for Tech Writer

- The trust-boundary language already in the feature spec ('slice.spec and slice.manifest come from an order's frontmatter and may arrive with an external order, so a path outside the project root is refused rather than read') accurately describes the shipped behavior — no correction needed if this is echoed into user-facing docs for the slice block.
