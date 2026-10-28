---
feature: "vocabulary-mismatch-message"
bounded_context: ""
domain_terms: ["closed vocabulary"]
files_touched: []
issue_refs: ["i-4-vocabulary-test", "PR #13"]
linked_adrs: []
linked_kis: []
---

# Security Review: vocabulary-mismatch-message

## Threat Model Summary

Scope is a single Go test file, internal/intake/vocabulary_test.go, changed to keep each formatted Errorf message in errorRecorder.messages instead of just a boolean, and to assert the mismatch message names field "status", document "fixture", and value "queued". No production code, API surface, auth/session logic, external I/O, or user-facing input path is touched. errorRecorder and assertVocabularyMatches operate entirely in-process on an already-trusted, repo-committed fixture (testdata/mismatched_vocabulary_template.md) during `go test`, never on externally supplied or untrusted data, so none of the six STRIDE categories apply with any meaningful severity. go test ./internal/intake/... and go vet both pass clean, and no new dependency was added (only stdlib fmt/strings), so no dependency audit was warranted.

## Dependency Audit

None

## STRIDE Analysis

None

### Spoofing

No identity, session, or credential handling exists anywhere in this change; errorRecorder is a test double with no authentication concept. Not applicable.

### Tampering

The data compared (fixture file, vocabulary.go sets) is repo-controlled test fixture content read via os.ReadFile at test time, not runtime user input. No transit or at-rest mutation path is introduced.

### Repudiation

Test-only code with no audit trail requirement; go test output itself is the record. Not applicable to a unit test helper.

### Information Disclosure

Messages captured in errorRecorder.messages contain only field names, document labels, and vocabulary values already present in the repo's own TEMPLATE.md/README.md/fixture — no secrets, PII, or internal paths are newly surfaced beyond what the test fixture already contains.

### Denial of Service

Loop bounds are the fixed, small vocabularyFields slice (6 entries); no unbounded iteration over external input, no new network or file-size-unbounded operation introduced.

### Elevation of Privilege

No authorization or privilege boundary exists in this code path; it is a test assertion helper with no access-control implications.

## Findings

None

## Files Modified

None

## Security Checklist

- [x] No hardcoded secrets/credentials introduced
- [x] No new external dependency added (checked go.mod/go.sum untouched)
- [x] No user-supplied input reaches a shell command, file path, or SQL query
- [x] No trust boundary (auth, API, file upload, external call) touched by this diff
- [x] No OTel/logging change that could leak PII or secrets

## Notes for QA

- Scope is test-only; confirm `go test ./internal/intake/...` is green and that TestVocabularyCheckCatchesAMismatch fails when assertVocabularyMatches's message is temporarily changed to drop the missing-value list (AC3's negative control), then confirm it's restored before merge — this is functional/AC verification, not a security concern.

## Notes for Tech Writer

None
