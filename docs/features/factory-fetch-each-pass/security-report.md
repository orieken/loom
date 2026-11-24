---
feature: "factory-fetch-each-pass"
bounded_context: ""
domain_terms: ["Intake", "Candidate", "Claim", "Repository", "Order", "base-branch", "remote-tracking ref"]
files_touched: []
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Security Review: factory-fetch-each-pass

## Threat Model Summary

This feature adds one new network call (`git fetch <remote> <branch>`) to the factory supervisor's per-pass loop, used to read intake/ from a remote-tracking ref instead of the local branch. No HTTP endpoints, no user-input parsing, no new auth/session/token handling, and no new dependency are introduced. The attack surface is a local CLI tool run by its own operator against a repository remote that operator already controls — the spec's own Trust Boundaries section accepts this explicitly ('uses the repository's configured remote and the person's own git credentials. Nothing new is sent'). Remote and branch names come from CLI flags set by the operator, not from untrusted input, and are passed to exec.CommandContext as separate argv elements (no shell), so there is no command-injection path. The fetch is timeout-bound (5 min, matching the existing Push pattern) and sets GIT_TERMINAL_PROMPT=0 so an unattended supervisor can never hang on a credential prompt. The person's own checkout is never written to (no pull/checkout/reset) — verified directly in the diff and in TestAPassNeverDisturbsThePersonsOwnCheckout. One pre-existing, non-blocking pattern is worth naming for defense-in-depth: Fetch's error message includes git's raw CombinedOutput (same as the existing Push function), which would surface a credential embedded in a remote URL if one were ever configured that way; this is an existing pattern extended to a second call site, not a new design, and standard git credential helpers/SSH keys don't put credentials in the URL, so it's tracked as Low rather than blocking.

## Dependency Audit

- No go.mod/go.sum changes in this diff — no new or updated dependencies introduced, so no audit was required.

## STRIDE Analysis

None

### Spoofing

Remote and branch are local CLI flags set by the operator, not attacker-controlled input; fetch target is the repository's own pre-configured remote. No new identity or credential-presentation path is introduced.

### Tampering

Fetched ref is only used after a successful ResolveCommit; a failed resolve or failed fetch falls back to the last-known ref/local branch rather than proceeding on an unverified value. Claimed orders pin SourceCommit at claim time, so a later fetch moving the remote ref cannot retroactively change a running or waiting order's source (verified by TestALaterFetchDoesNotMoveARunningOrdersClaimedCommit).

### Repudiation

A failed fetch is printed as one line to the supervisor's own output stream (matching the Push function's existing behavior) and is not written to the persisted order ledger — acceptable since the acceptance criteria only requires it not silently swallow the failure, not provide a durable audit trail.

### Information Disclosure

Fetch's error wrapping includes git's raw stderr/stdout via CombinedOutput, same pattern as the pre-existing Push function. This could echo a credential embedded directly in a remote URL on failure; standard credential-helper/SSH setups do not embed credentials in the URL, so this is Low, not Critical/High, and is an extension of an already-accepted pattern rather than new exposure.

### Denial of Service

Fetch is bounded by a 5-minute context timeout (fetchTimeout), consistent with architecture-guardrails.md #5 and the existing pushTimeout convention. At most one fetch per pass (verified by TestAPassFetchesTheRemoteOnceAndClaimsOrdersPromotedThere), so no unbounded retry loop or hot-loop risk.

### Elevation of Privilege

No change to who can push to the configured remote or what a claimed order is permitted to do once running; orders were already read from a branch the operator trusts (previously local, now the remote-tracking ref of the same remote). No new privilege boundary is crossed.

## Findings

None

## Files Modified

None

## Security Checklist

- [x] No hardcoded secrets introduced
- [x] Every network call has an explicit timeout — fetchTimeout = 5 * time.Minute via context.WithTimeout, mirrors pushTimeout
- [x] No shell interpolation / command injection surface — exec.CommandContext with remote/branch as separate argv elements; inputs are operator-set CLI flags, not untrusted data
- [x] Operator's own checkout left untouched (no pull/checkout/reset) — Verified in diff and by TestAPassNeverDisturbsThePersonsOwnCheckout
- [x] No new dependency introduced
- [x] Credential-prompt hang prevented on unattended runs — GIT_TERMINAL_PROMPT=0 set, matching existing Push
- [ ] Fetch error output does not leak credentials beyond the existing Push pattern's own exposure — Low severity, pre-existing pattern extended to a second call site — tracked as tech debt, not blocking

## Notes for QA

- No security-relevant behavior for QA to add beyond what's already covered: the existing fetch_test.go and manage_test.go suites already assert the no-credential-prompt-hang timeout path indirectly via the real git binary, and the untouched-checkout guarantee via TestAPassNeverDisturbsThePersonsOwnCheckout.
- No Critical/High findings — QA is not blocked by this review.

## Notes for Tech Writer

None
