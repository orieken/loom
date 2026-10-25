# Context Manifest: e2e-fixture-gitignore

## Scope

- **Tier**: developer
- **Built for**: developer

## Pinned Files

- `tests/e2e/testdata/repo/.gitignore` — Does not exist yet — this is the file the task creates. Must ignore at least .claude/memory/ and .claude/feature-workspace/ per AC1.
- `.gitignore` — Root .gitignore holds the two entries (.claude/memory/, .claude/feature-workspace/) the fixture's new .gitignore must mirror for the per-machine run state a loom-adopting repo ignores.
- `tests/e2e/harness/deliver.go` — setUpProject() runs copyTree then `git add -A` + `git commit` to make the project's first commit — the exact commit AC2 requires the .gitignore to land in and be read back from.
- `tests/e2e/harness/exec.go` — copyTree() walks the fixture tree via filepath.WalkDir — resolves the spec's Open Question about whether the copy skips dotfiles (it does not).
- `tests/e2e/harness/units_test.go` — Home for small, package-internal assertions against Deliver/setUpProject (see fixtureWithoutRepo, TestDeliverRefusesABrokenFixture) — the natural place for the new first-commit .gitignore assertion required by AC2.
- `tests/e2e/harness/harness_test.go` — Contains TestTheFactoryDeliversTheFixtureOrder and TestTheHarnessFailsARunThatDeliversTheWrongThing, which AC3 and AC4 require to keep passing unchanged.
- `tests/e2e/harness/harness.go` — Defines Config, project()/origin() path helpers the new test needs to locate the project worktree and read its first commit.

## Token Budget

- **Status**: INCOMPLETE — ~6406 tokens, 4.0% of the developer tier budget (160000 tokens)
- Measured by loom from the files above (bytes ÷ 4), not estimated by an agent.

| File | Bytes | ~Tokens |
|---|---:|---:|
| `tests/e2e/testdata/repo/.gitignore` | — | not measured: cannot read tests/e2e/testdata/repo/.gitignore |
| `.gitignore` | 977 | 244 |
| `tests/e2e/harness/deliver.go` | 6219 | 1554 |
| `tests/e2e/harness/exec.go` | 1603 | 400 |
| `tests/e2e/harness/units_test.go` | 8102 | 2025 |
| `tests/e2e/harness/harness_test.go` | 4164 | 1041 |
| `tests/e2e/harness/harness.go` | 4571 | 1142 |

> 1 pinned file(s) could not be measured, so the total above is a floor, not a figure.

## Knowledge Items

None

## ADRs

None

## Pruned

- `internal/worktree/** and internal/factory/** — own the :(exclude) pathspec / commit fix from #12; out of scope per spec, and this task reaches no further than tests/e2e/`
- `cmd/** — explicitly out of scope`
- `tests/e2e/testdata/solution/** and tests/e2e/testdata/hidden/** — the mock developer/QA solution and hidden acceptance tests; unrelated to a fixture .gitignore`
- `tests/e2e/testdata/repo/go.mod, README.md, calc/** — fixture's own source files, untouched by adding a .gitignore`
- `docs/features/e2e-fixture-gitignore/** — this run's own delivery record, the one path the AC explicitly allows outside tests/e2e/ but not something downstream needs to read to implement the change`
