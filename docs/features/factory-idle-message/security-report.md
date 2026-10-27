---
feature: "factory-idle-message"
bounded_context: ""
domain_terms: ["idle", "waiting on a human", "tick", "ledger replay"]
files_touched: []
issue_refs: []
linked_adrs: []
linked_kis: []
---

# Security Review: factory-idle-message

## Threat Model Summary

This feature adds a console status line (idle/waiting) printed to the supervisor's own io.Writer, derived purely from in-memory ledger replay (OrderState map) and the configured intake branch name. There is no auth, no network call, no user-controlled input reaching this code path, no secrets, and no PII in the two message formats (`idle — no ready orders; commit one to intake/ on <branch>` and `waiting on a human — <n> order(s) in the inbox: loom factory inbox`). The spec itself declares Trust Boundaries: None, and that holds on inspection — IntakeBranch and order counts are operator-configured/internal state, not attacker-reachable. No dependencies were added (idle.go uses only the existing factory package types). No findings; no code changes were necessary.

## Dependency Audit

- No new dependencies introduced by this feature (idle.go/idle_test.go use only existing internal/factory and stdlib/test-only imports) — dependency scan not applicable.

## STRIDE Analysis

None

### Spoofing

No identity, session, or credential handling in this change — the message path has no actor to spoof. Not applicable.

### Tampering

Inputs to classify()/announce() are the in-process OrderState map from ledger replay and Config.IntakeBranch, both already trusted internal state produced earlier in the same process; nothing here accepts externally tamperable input.

### Repudiation

This is a console convenience message, not an audit record — the spec explicitly keeps idleness out of the ledger (Out of Scope: 'Changing any ledger event, or recording idleness in the ledger'). No repudiation concern since no action or decision is being attested to.

### Information Disclosure

Checked both printed strings for leakage: the idle line reveals only the configured intake branch name (operator-known, not secret); the waiting line reveals only a count of orders, no order IDs, titles, paths, or ledger content. No PII, tokens, or internal paths are emitted, consistent with architecture-guardrails.md #9.

### Denial of Service

classify()/waitingCount()/hasUnshippedOrder() are single O(n) passes over the already-loaded orders map, called once per tick as required by the spec's non-functional requirement (no extra git/filesystem pass per tick) — no unbounded loop, no new network or disk I/O introduced.

### Elevation of Privilege

No authorization checks exist or are needed here — the supervisor process already has whatever privilege it was launched with, and this change adds no new privileged operation, file write, or command execution.

## Findings

None

## Files Modified

None

## Security Checklist

- [x] No hardcoded secrets, API keys, or tokens introduced
- [x] No new external dependency requiring vulnerability scan
- [x] No PII or sensitive data in printed/logged output — Only branch name and order counts are printed; no order IDs or ledger details.
- [x] No new network call lacking a timeout or circuit breaker
- [x] No user-controlled input reaches this code path
- [x] No auth/session logic touched

## Notes for QA

- No security-driven test additions required — existing acceptance-criteria-driven tests (idle_test.go) already exercise the only user-visible behavior of this change (the two message strings and their once-per-state-entry rule).

## Notes for Tech Writer

None
