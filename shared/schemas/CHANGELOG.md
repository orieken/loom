# Schema Changelog

Tracks versions of the versioned schemas in `shared/schemas/`. Today that is the pattern spec schema only. The
pipeline, telemetry and repo graph schemas are generated from their Go types and versioned with the code that
writes them.

## Versioning
- A change that rejects a spec that used to be valid (a new required field, a narrower type, a removed value)
  bumps `schemaVersion`. The loader reads exactly one version and names it when it refuses another.
- A change that only accepts more (a new optional field, a new enum value) keeps `schemaVersion`.
- The Go types in `internal/patternspec` are the source. Change them, run `go run ./cmd/gen-schemas`, and commit
  the regenerated schema. `TestCommittedPatternSpecSchemaMatchesTheTypes` fails when you forget.

## How to add an entry
Add a row under a new dated heading in the same commit as the schema change.

## 2026-10-21 — Pattern spec schema, version 1 (pattern mining F2)

| Schema | schemaVersion | Change |
|---|---|---|
| pattern-spec.schema.json | 1 | New. Linear steps, each with a mandatory executor tag (`tool`, `skill`, `human`); a closed set of input types; verify references to skills and MCP tools; provenance with mined evidence. Contract: `shared/contracts/pattern-spec-contract.md` |
