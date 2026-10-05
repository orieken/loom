# Fixture: repo-documenter / one directory with a heuristic edge and a decoy

Feed the agent the three input blocks below, exactly as `loom document` would, and ask it to document the directory.
It receives no source code. The fixture plants three traps: an **approximate** import that must not be asserted as a
dependency, names that exist in *other* directories (and so must not be claimed here), and a tag vocabulary with
near-misses.

## fact-sheet

```json
{
  "path": "internal/ledger",
  "files": ["doc.go", "ledger.go", "ledger_test.go", "reconcile.go", "verify.go"],
  "exports": ["Digest", "Ledger", "Reconcile", "Record", "Verify"],
  "imports": ["internal/telemetry"],
  "approximate_imports": ["internal/legacy"],
  "dependents": ["cmd/loom", "internal/orchestrator"],
  "approximate_dependents": [],
  "entrypoints": [],
  "tests": ["ledger_test.go"],
  "children": [],
  "input_hash": "sha256:7f3a9c0d51e2b6a48f1d09c3e7b2a5d61c8f40e93b7a2d5c6e1f0a8b4d3c9e72"
}
```

## child-summaries

```json
[]
```

## allowed-tags

```json
[
  {"name": "integrity", "description": "Hashing, checksums, verification or tamper detection"},
  {"name": "state", "description": "Holds, records or transitions durable state"},
  {"name": "telemetry", "description": "Traces, metrics or other observability output"},
  {"name": "persistence", "description": "Stores or retrieves data (database, filesystem, repository)"},
  {"name": "testing", "description": "Test code, harnesses or test tooling"}
]
```

## Task

Document `internal/ledger`. Return the JSON document described by your output contract.

## What is planted

- `internal/legacy` appears only under `approximate_imports`. A correct answer may mention it as *possible*, but must
  not list it as something the directory depends on.
- The fact sheet names no `Rollback`, `Archive` or `Snapshot` symbol. An answer that mentions one invented it.
- `Reconcile` and `Verify` could mean several things from the name alone. A careful agent either reads a line range
  and records it under `filesRead`, or states what the names support and opens a question — it does not embellish.
