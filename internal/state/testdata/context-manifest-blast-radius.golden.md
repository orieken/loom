# Context Manifest: rename-ledger-digest

## Scope

- **Tier**: developer
- **Built for**: developer

## Pinned Files

- `internal/ledger/digest.go` — the change itself
- `internal/orchestrator/integrity.go` — distance-1 exact dependent of internal/ledger (impact)
- `cmd/loom/cmd/verify.go` — distance-2 dependent; the signature it calls changes

## Blast Radius

- **Change touches**: `internal/ledger`
- Computed by loom from the repo graph at `abcdef01` (fresh); 3 directories depend on it.
  - `internal/orchestrator` — 1 import away, exact
  - `internal/policy` — 1 import away, approximate
  - `cmd/loom/cmd` — 2 imports away, exact

## Token Budget

- Not measured.

## Knowledge Items

None

## ADRs

None

## Pruned

None
