---
feature: "factory-close-shipped-by-hand"
bounded_context: ""
domain_terms: ["order", "ledger", "phase", "shipped", "parked", "stopped", "worktree", "supervisor", "approver"]
files_touched: []
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Security Review: factory-close-shipped-by-hand

## Threat Model Summary

loom factory close is a local, operator-run CLI control that appends one `shipped` ledger entry for a hand-finished order and reuses the supervisor's own archive-then-remove sequence. It has no auth surface, makes no network calls, introduces no new dependencies, and crosses no trust boundary beyond the existing local-user/ledger model that approve, stop, resume and rerun already use. The URL supplied via --shipped-by is stored and displayed only; isHTTPURL (net/url.Parse) makes that structural rather than conventional, since the function imports nothing capable of making a network call. The approver identity reuses approverName() (OS user), the same source `approve` already uses, so no second approval channel exists. Reviewed by reading control.go, finish.go, replay.go, cmd/loom/cmd/factory.go, and their tests; no code execution or dependency audit was needed since go.mod/go.sum are unchanged.

## Dependency Audit

- No new dependencies were added by this change (go.mod and go.sum are unmodified) — dependency vulnerability scan is not applicable.

## STRIDE Analysis

None

### Spoofing

approvedBy comes from approverName() (OS user), identical to loom factory approve's existing source. close introduces no new identity or credential path, so it carries no new spoofing surface beyond what approve/stop/resume already accept.

### Tampering

closeable validates the order exists, the URL is syntactically http(s), and the order's phase is Parked or Stopped before Close appends anything; the ledger write itself goes through the existing Ledger.Append single-write-plus-fsync path, unchanged by this feature. No user-supplied value is interpolated into a shell command, file path outside the worktree, or SQL — the URL is stored as a JSON field and printed as text.

### Repudiation

Close appends an auditable ledger entry recording Order, URL and ApprovedBy, giving a human-finished order the same audit record a supervisor-shipped one gets. This closes rather than opens a repudiation gap: before this feature, a hand-finished order had no shipped record at all.

### Information Disclosure

Checked egress specifically: isHTTPURL is pure syntax (net/url.Parse scheme/host check) and imports nothing network-capable; Close's only calls are ledger.Append, archiveRun (local filesystem copy) and repo.RemoveWorktree (local git worktree removal). There is no code path in this feature that sends the URL, or anything else, anywhere — satisfying the acceptance criterion and NFR that close must never fetch or open the URL.

### Denial of Service

archiveRun walks only the one order's run-state directory (bounded by that run's own output, not user input) and the command is invoked once per human action, not from any automated or remote trigger — no unbounded loop over attacker-controlled input and no new network call needing a timeout or circuit breaker.

### Elevation of Privilege

closeable's phase allow-list (Parked or Stopped only, refusing PhaseShipped and every active/waiting phase with the order's phase named) prevents close from being used to force a transition the supervisor wouldn't also reach, and prevents re-shipping or double-processing an order. There is no role or permission check to bypass because this is a local single-operator tool with no multi-tenant or privilege-tiered model — consistent with every other factory control (approve, stop, resume, rerun).

## Findings

None

## Files Modified

None

## Security Checklist

- [x] No hardcoded secrets, API keys, or credentials introduced
- [x] --shipped-by URL is never fetched, opened, or used to make any network call (isHTTPURL is syntax-only, no network-capable import)
- [x] approvedBy sourced from the same place loom factory approve already uses (OS user) — no second approval channel
- [x] Every external dependency (Repository, Layout) is behind an interface already defined in the consumer package (internal/factory)
- [x] No new third-party dependency added (go.mod/go.sum unchanged) — dependency CVE scan not applicable
- [x] User-supplied input (order id, URL) never interpolated into a shell command or raw file path outside the worktree
- [x] Ledger write path (Ledger.Append) unchanged by this feature — still one atomic append-and-fsync per entry
- [x] No PII, tokens, or secrets recorded in the ledger entry or printed output beyond the operator-supplied PR URL and OS username

## Notes for QA

- No security-driven code changes were required, so nothing new needs re-testing beyond what qa-engineer already planned from analysis.md's QA Tasks.
- Worth confirming at the CLI level: a --shipped-by value containing terminal control/escape sequences (e.g. ANSI codes) still passes isHTTPURL's scheme/host check and gets printed verbatim by `status`'s detail column and close's own success line. This is not a trust-boundary crossing (the operator supplies their own argument) and is INFO-only, but if it's cheap to add a case confirming the printed value doesn't corrupt tabwriter output, it costs nothing to check.

## Notes for Tech Writer

None
