{
  "schemaVersion": 2,
  "path": "internal/ledger",
  "inputHash": "sha256:7f3a9c0d51e2b6a48f1d09c3e7b2a5d61c8f40e93b7a2d5c6e1f0a8b4d3c9e72",
  "purpose": "internal/ledger defines a `Ledger` type with `Record`, `Verify`, `Reconcile` and `Digest` operations.",
  "responsibilities": [
    "It imports `internal/telemetry`.",
    "It is used by `cmd/loom` and `internal/orchestrator`.",
    "It has tests in `ledger_test.go`."
  ],
  "keyEntrypoints": [
    "Ledger",
    "Record",
    "Verify",
    "Reconcile",
    "Digest"
  ],
  "tags": [
    "integrity",
    "state"
  ],
  "layer": "unknown",
  "invariants": [],
  "openQuestions": [
    "The fact sheet does not say what `Reconcile` reconciles or what `Verify` checks against, or whether these operations call one another; the export names alone do not establish that relationship.",
    "`internal/legacy` appears only under approximate_imports; whether `internal/ledger` actually depends on it is unconfirmed by the fact sheet.",
    "No entrypoints are listed, so which export `cmd/loom` and `internal/orchestrator` actually call is unknown.",
    "The fact sheet gives no indication of architectural layer (e.g. no adapter-style imports, no framework imports), so `layer` could not be determined beyond 'unknown'."
  ],
  "evidence": [
    {
      "claim": "internal/ledger defines a `Ledger` type with `Record`, `Verify`, `Reconcile` and `Digest` operations.",
      "fields": [
        "exports:Ledger",
        "exports:Record",
        "exports:Verify",
        "exports:Reconcile",
        "exports:Digest"
      ]
    },
    {
      "claim": "It imports `internal/telemetry`.",
      "fields": [
        "imports:internal/telemetry"
      ]
    },
    {
      "claim": "It is used by `cmd/loom` and `internal/orchestrator`.",
      "fields": [
        "dependents:cmd/loom",
        "dependents:internal/orchestrator"
      ]
    },
    {
      "claim": "It has tests in `ledger_test.go`.",
      "fields": [
        "tests:ledger_test.go"
      ]
    }
  ],
  "filesRead": []
}
