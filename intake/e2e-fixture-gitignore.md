---
intake: 1
id: e2e-fixture-gitignore
type: chore
plan: deliver-feature
status: ready
priority: 3
created: 2026-10-24
requested-by: oscar
source: manual
trust: internal
base-branch: main
finish: pr
autonomy: inherit
depends-on: []
budget:
  max-attempts: 2
  max-usd: 20
  max-hours: 2
scope:
  paths: [tests/e2e/**]
---

# Give the e2e fixture repository a .gitignore like a real one

## Request

> The first real factory commit failed with `git add: exit status 1` (fixed in #12). git refuses an
> `:(exclude)` pathspec that names an ignored path, and loom's own `.gitignore` ignores
> `.claude/memory/`. The e2e fixture repository ignores nothing, so the harness never hit this.

## Summary

The factory's end-to-end harness (`tests/e2e/`) delivers an order into a fixture repository copied
from `tests/e2e/testdata/repo/`. That fixture has no `.gitignore`. Real repositories that adopt loom
ignore its per-machine state, and the factory's commit has to work against them. Add a `.gitignore` to
the fixture that ignores what a loom-adopting repository ignores, so the harness exercises the same
git behaviour a real repository does.

## Acceptance Criteria

- [ ] Given the fixture at `tests/e2e/testdata/repo/`, then it contains a `.gitignore` that ignores at
      least `.claude/memory/` and `.claude/feature-workspace/`, the two entries loom's own root
      `.gitignore` holds for per-machine run state.
- [ ] Given the harness copies the fixture into its project, when it does so, then the `.gitignore`
      is copied too and is committed in the project's first commit. Assert this in a harness test:
      reading it back from the project's first commit returns the fixture's contents.
- [ ] Given the change is complete, when `go test ./tests/e2e/...` runs with the mock provider, then
      `TestTheFactoryDeliversTheFixtureOrder` passes and every one of its checks reports PASS.
- [ ] Given the change is complete, then `TestTheHarnessFailsARunThatDeliversTheWrongThing` still
      passes. The negative control still fails the delivery it is meant to fail.
- [ ] Given the change is complete, then every new function has cyclomatic complexity below 7, and no
      file outside `tests/e2e/` changes. The run's own delivery record under
      `docs/features/e2e-fixture-gitignore/` is the one exception.

## Out of Scope

- Any change to `internal/` or `cmd/`. The commit fix itself shipped in #12.
- Adding the fixture's `.gitignore` entries to the harness's own scope or diff checks beyond what is
  listed above.

## Domain Language

- **Fixture repository**: the small Go module under `tests/e2e/testdata/repo/` that the harness hands
  an order to deliver.

## Non-Functional Requirements

- The harness MUST still need no network beyond what it uses today, and the mock-provider run MUST
  take no longer than it does today, within normal variance.

## Trust Boundaries

None.

## Test Approach

The existing harness tests in `tests/e2e/harness`, run with the mock provider, plus one assertion
that the fixture's `.gitignore` reaches the project's first commit.

## Open Questions

- Whether the harness's copy skips dotfiles. Assumption if unresolved: it does not — `copyTree` in
  `tests/e2e/harness/exec.go` walks every file — and the new assertion proves it either way.

## Infrastructure / Deploy Notes

None.
