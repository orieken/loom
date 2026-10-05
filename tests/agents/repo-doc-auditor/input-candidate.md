# Fixture: repo-doc-auditor / one overreaching claim among sound ones

Feed the agent the three blocks below exactly as `loom document` would. The candidate **already passes the Go
verifier** — every name, tag and citation exists in the sheet — so the only thing wrong with it is a matter of
judgement, which is what the auditor is for. The fixture plants one overreach and two sound claims.

## fact-sheet

```json
{
  "path": "internal/ledger",
  "files": ["ledger.go", "ledger_test.go", "verify.go"],
  "exports": ["Digest", "Ledger", "Record", "Verify"],
  "imports": ["internal/telemetry"],
  "approximate_imports": [],
  "dependents": ["cmd/loom"],
  "approximate_dependents": [],
  "entrypoints": [],
  "tests": ["ledger_test.go"],
  "children": [],
  "input_hash": "sha256:4be1d0a9c37f2e85b6a01d7c9e3f48a2d5c60b17e9f3a4d8c2b5e1f07a96d3c4"
}
```

## child-summaries

```json
[]
```

## candidate

```json
{
  "schemaVersion": 2,
  "path": "internal/ledger",
  "inputHash": "sha256:4be1d0a9c37f2e85b6a01d7c9e3f48a2d5c60b17e9f3a4d8c2b5e1f07a96d3c4",
  "purpose": "Keeps a ledger of records and checks them with `Verify`.",
  "responsibilities": [
    "Cryptographically signs every `Record` with a per-tenant key",
    "Is relied on by `cmd/loom`"
  ],
  "keyEntrypoints": ["Ledger", "Verify"],
  "tags": ["integrity"],
  "layer": "application",
  "invariants": [],
  "openQuestions": [],
  "evidence": [
    {"claim": "Keeps a ledger of records and checks them with `Verify`.", "fields": ["exports:Ledger", "exports:Verify", "exports:Record"]},
    {"claim": "Cryptographically signs every `Record` with a per-tenant key", "fields": ["exports:Digest", "exports:Record"]},
    {"claim": "Is relied on by `cmd/loom`", "fields": ["dependents:cmd/loom"]}
  ],
  "filesRead": []
}
```

## Task

Audit the `candidate` against the `fact-sheet`. Return the JSON document described by your output contract.

## What is planted

- **Overreach (blocking).** "Cryptographically signs every `Record` with a per-tenant key" cites `exports:Digest` and
  `exports:Record`. An export named `Digest` supports "computes a digest" at most. Nothing in the sheet mentions
  signing, cryptography, keys or tenants. A correct audit is `CHANGES_REQUESTED` with this claim as a blocking finding.
- **Sound (must not be flagged).** The purpose and "Is relied on by `cmd/loom`" are supported by what they cite. A
  finding against either is a false positive.
