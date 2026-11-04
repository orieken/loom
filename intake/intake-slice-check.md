---
intake: 1
id: intake-slice-check
type: feature
plan: deliver-feature
status: draft
priority: 2
created: 2026-11-04
requested-by: oscar
source: manual
trust: internal
base-branch: main
finish: pr
autonomy: inherit
depends-on: []
budget:
  max-attempts: 2
  max-usd: 25
  max-hours: 3
scope:
  paths: [internal/intake/**, cmd/loom/cmd/intake*.go, templates/claude-feature-team/intake/**, docs/features/intake-slice-check/**]
---

# Prove a split feature's slices cover its whole spec

## Request

> We broke one big feature into six smaller ones by hand. Build that into the factory, starting with
> the check.

## Summary

The saturday-monorepo pilot split order `reagent-map-view` into six orders, because one developer
stage could not finish it inside the 30-minute stage limit (decision D-31). Nothing checks such a
split. A criterion can fall between two slices, and every slice still ships green. `loom intake
check` should prove that a set of slices covers its spec. This order is the deterministic check only:
no agent, no skill, and no automatic splitting.

Today `intake.Check` validates one file at a time, so this adds the first check across orders.

## Acceptance Criteria

- [ ] Given an order's frontmatter, then it accepts an optional `slice` block, and an order without
      one is validated exactly as today:
      ```yaml
      slice:
        of: reagent-map-spec          # the spec package this order is a slice of
        spec: docs/specs/reagent-map-spec/acceptance-criteria.json
        digest: sha256:18f9820d...    # SHA-256 of the manifest below
        manifest: docs/specs/reagent-map-spec/package.sha256
        covers: [AC1:ui, AC2, AC14]   # a criterion id, optionally :api or :ui
      ```
- [ ] Given the spec file's `criteria[]`, each with an `id` and `observableVia` (`api`, `ui` or
      `both`), then a criterion has one level to cover, or two when `both`. `AC1:ui` covers that level
      of AC1, and a bare `AC2` covers every level of AC2.
- [ ] Given all the orders in the checked directory with the same `slice.of`, then `loom intake check`
      reports a violation naming the criterion and level for:
      - a level no slice covers;
      - a level two slices both cover, naming both orders;
      - a `covers` entry the spec does not have, or a level that criterion does not have.
- [ ] Given slices of one spec, then it reports a violation when their `slice.digest` values differ,
      when a digest does not match the SHA-256 of its `manifest` file, or when a file the manifest lists
      no longer matches its recorded hash, naming the file.
- [ ] Given slices of one spec, then it reports a violation for a `depends-on` cycle among them, naming
      the orders in the cycle, and for a `depends-on` entry that names no order in the directory.
- [ ] Given a spec or manifest file that cannot be read, then that is a violation on the order naming
      it: an unverified split is never reported valid. Paths are relative to the project root, and a
      path that leaves the project root is a violation.
- [ ] Given a split that passes, then `loom intake check` prints one line per spec with its slice and
      level counts, and the exit code contract is unchanged (0 only when every order is valid).
- [ ] Given `templates/claude-feature-team/intake/TEMPLATE.md` and `README.md`, then they document the
      `slice` block, and the existing template vocabulary test still passes.
- [ ] Given tests, then a fixture modelled on the reagent-map split passes (AC1 and AC3 covered once at
      each level by different slices, AC14 covered bare by one slice), and each violation above has a
      test built by breaking that fixture in one way, which fails when the check is removed.
- [ ] Given the change is complete, then every new or changed function has cyclomatic complexity
      below 7, every changed line is covered by a test that can fail, and no file outside
      `scope.paths` changes, the run's own delivery record under
      `docs/features/intake-slice-check/` excepted.

## Out of Scope

- The `slice-order` skill that drafts a split, the size warning, and `deliver-spec` emitting slices.
- `intake: 2` and `inputs`. The `slice` block stays optional under schema version 1, and `loom intake
  pin` can replace `digest` later.
- Enforcing anything at claim time. This is `loom intake check` only.
- Adding `slice` blocks to saturday-monorepo's existing orders; that is a follow-up in that repository.

## Domain Language

- **Slice**: an order that delivers part of a spec package and ships on its own.
- **Level**: where a criterion is observable, `api` or `ui`, from the spec's `observableVia`.

## Non-Functional Requirements

- A directory with no `slice` blocks reads no extra files.

## Trust Boundaries

- `slice.spec` and `slice.manifest` come from an order's frontmatter and may arrive with an external
  order, so a path outside the project root is refused rather than read.

## Test Approach

Go unit tests in `internal/intake`, and a `cmd/loom/cmd` test that runs the command over the fixture
directory.

## Open Questions

- A spec with no `observableVia` on a criterion: Assumption if unresolved: a violation naming the
  criterion, since its levels cannot be known.

## Infrastructure / Deploy Notes

- None
