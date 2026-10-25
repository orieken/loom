---
feature: "i-4-vocabulary-test"
bounded_context: ""
domain_terms: ["Closed vocabulary"]
files_touched: []
issue_refs: ["I-4", "docs/audits/factory-intake-and-deliver-feature-review-2026-10-23.md#I-4"]
linked_adrs: []
linked_kis: ["shared/rules/memory-trust-boundary.md"]
---

# Security Review: i-4-vocabulary-test

## Threat Model Summary

This change adds three Go unit tests (internal/intake/vocabulary_test.go) and one fixture (internal/intake/testdata/mismatched_vocabulary_template.md) that compare closed-vocabulary values in two checked-in repo documents (templates/claude-feature-team/intake/TEMPLATE.md and README.md) against Go constant sets in internal/intake/vocabulary.go. Code runs only under `go test`, in the developer's or CI's local environment, against repo-local files it reads with os.ReadFile. There is no runtime code path, no network call, no auth, no session or token handling, no user-supplied input at execution time, and no secrets. The analyst's own Trust Boundaries section states 'None. The test reads two files from this repository,' and the code-reviewer's Security Surface section independently recorded 'None.' I verified both by reading vocabulary_test.go, vocabulary.go, parse.go, and the fixture directly. No findings; no fixes were needed.

## Dependency Audit

- No new dependencies introduced — vocabulary_test.go imports only os, path/filepath, runtime, sort, strings, testing from the standard library.

## STRIDE Analysis

None

### Spoofing

No identity, session, or credential handling exists anywhere in this change; test code has no caller to spoof.

### Tampering

Inputs are two files checked into the repo and read via os.ReadFile at test time, not user-controlled runtime input. A malicious edit to TEMPLATE.md or README.md would at worst make the test fail loudly (that is its purpose), never corrupt state silently.

### Repudiation

Not applicable — no action is taken on behalf of a user, so there is nothing to repudiate and no audit trail requirement.

### Information Disclosure

The code reads only two non-secret, already-public repo documents (TEMPLATE.md, README.md) and emits only field names and vocabulary values (e.g. 'draft', 'ready') in t.Errorf messages — no PII, tokens, or internal paths beyond the already-public repo-relative filenames.

### Denial of Service

Fixed-size reads of two small local files with no loops over unbounded or attacker-controlled input; measured runtime is 0.18-0.26s per the code-review-report, consistent with the spec's <1s budget. No exhaustible resource is touched.

### Elevation of Privilege

No authorization model exists in this code; it runs with whatever privileges `go test` already has to read files in the working tree, which is the same privilege the developer or CI runner already holds.

## Findings

None

## Files Modified

None

## Security Checklist

- [x] No hardcoded secrets, tokens, or credentials introduced
- [x] No new external dependency requiring vulnerability scan
- [x] No user-supplied input reaches this code at runtime
- [x] No network calls, so no timeout/CircuitBreaker requirement applies
- [x] No OTel/logging instrumentation added, so no PII-in-telemetry risk applies
- [x] Error messages (t.Errorf) expose only non-sensitive field names and vocabulary values, not stack traces or internal paths

## Notes for QA

- No security-driven test requirements — this is a pure documentation/code consistency check with no trust boundary. Proceed directly to QA's normal coverage and correctness review.

## Notes for Tech Writer

None
