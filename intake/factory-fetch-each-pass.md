---
intake: 1
id: factory-fetch-each-pass
type: feature
plan: deliver-feature
status: ready
priority: 2
created: 2026-11-23
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
  max-hours: 3
scope:
  paths: [internal/factory/**, internal/worktree/**, cmd/loom/cmd/factory*.go, templates/claude-feature-team/intake/README.md, docs/features/factory-fetch-each-pass/**]
---

# Fetch the base branch before each supervisor pass

## Request

> Lets do 1: fetch from GitHub each pass, so I stop having to pull main before every order.

## Summary

`loom factory run` reads `intake/` from the local `--branch` (default `main`) and creates each
worktree from it. It never fetches. A pull request merged on GitHub, such as a promoted order or a
dependency update, is invisible until a person pulls. In the pilot this held up a merged order for a
day, and orders could start from code that was out of date.

At the start of each pass, the supervisor fetches the base branch from its remote. It then reads
`intake/` and creates worktrees from the remote-tracking ref (`origin/main`), not the local branch.
The person's own checkout is never changed: no pull, no checkout, no reset.

## Acceptance Criteria

- [ ] Given a pass, when it starts, then the supervisor runs `git fetch <remote> <branch>` once, and
      reads `intake/` at `<remote>/<branch>`. `--remote` defaults to `origin`.
- [ ] Given an order promoted to `ready` on the remote only, then the next pass claims it with no pull
      by a person, and the claim's `sourceCommit` is the fetched commit.
- [ ] Given a claim, then its worktree's branch starts from `<remote>/<branch>` at that commit.
- [ ] Given the person's checkout, then a pass never changes it: its branch, index and working tree are
      the same before and after, including uncommitted changes and local commits that are not pushed.
- [ ] Given a fetch that fails (no network, no remote, bad credentials), then the pass goes on from the
      last fetched `<remote>/<branch>` and prints one line saying the fetch failed and why. It does not
      park or fail any order.
- [ ] Given a repository with no remote, or `--no-fetch`, then the supervisor reads the local `<branch>`
      exactly as today. The existing tests keep passing with no change to what they assert.
- [ ] Given a running or waiting order, then a fetch that moves `<remote>/<branch>` never changes it.
      Orders keep their claimed `sourceCommit`, and only `loom factory rerun` takes a newer version.
- [ ] Given `templates/claude-feature-team/intake/README.md`, then it says orders are read from the
      remote after a fetch, so a promotion must be pushed, and how `--no-fetch` restores the old
      behaviour.
- [ ] Given the change is complete, then every new or changed function has cyclomatic complexity
      below 7, every changed line is covered by a test that can fail, `go test ./...` and the lint pass,
      and no file outside `scope.paths` changes, the run's own delivery record under
      `docs/features/factory-fetch-each-pass/` excepted.

## Out of Scope

- Pulling, merging or rebasing the person's checkout.
- Fetching more often than once a pass.

## Domain Language

- No new terms introduced.

## Non-Functional Requirements

- A fetch adds at most one network call per pass. With the default 30 s interval, that is two a
  minute.

## Trust Boundaries

- The fetch uses the repository's configured remote and the person's own git credentials. Nothing
  new is sent.

## Test Approach

Unit tests with a local bare repository as the remote: push an order to it only, run one pass, and
check the claim. Also a failing-fetch case, a no-remote case, and a dirty checkout left untouched.

## Open Questions

- Should `loom factory status` and `inbox` also fetch? Assumption if unresolved: no. They read the
  ledger and the remote-tracking ref as the last pass left them.

## Infrastructure / Deploy Notes

- After this ships, restart both supervisors with the new build.
