# tests/e2e — the factory, end to end

Roadmap L3.73. One work order goes in, a pushed branch comes out, and the branch
is judged by acceptance tests the factory never saw.

## What a run does

1. **Set up.** Copies `testdata/repo/` into a fresh git repository, commits
   `testdata/order.md` as `intake/divide.md` on `main`, and pushes it to a bare
   `origin`.
2. **Deliver.** Runs `loom factory run --once` pass after pass. Whatever the
   inbox asks for, it approves explicitly with `loom factory approve`, the way a
   human would. Nothing auto-approves. Before it approves the commit, while the
   run's worktree still exists, it checks two things:
   - every recorded artifact digest matches the file on disk;
   - the executor's own verification catches an artifact changed behind its back.
     This runs on a copy, so the real run is untouched.
3. **Verify.** Reads the run's archived state and checks:
   - every stage settled;
   - every run gate was approved with `--approve` and bound to digests;
   - spend stayed within the order's budget.

   It then clones the pushed `factory/divide` branch and checks that the diff
   stays inside the order's `scope.paths`. Only now are `testdata/hidden/` copied
   into the clone and run.

A broken gate, broken digest verification, or broken intake parsing fails a
check, or stops the order from shipping at all. A second test runs the factory
without a solution and asserts that the harness fails it.

## The mock's solution

With `--provider mock`, no model writes code, so `loom run --mock-solution
testdata/solution` hands the mock's developer and QA stages the files to write.
Each stage claims exactly what it wrote. The hidden tests then judge code that
really moved from a stage, through the worktree, onto a pushed branch. With a
real provider (L3.74) there is no solution: the agents write the code.

## Running it

```bash
go test ./tests/e2e/...          # in-process, mock provider — runs in CI with every `go test ./...`
make e2e-in-container            # delivery and verification in separate containers
```

In the container, delivery runs with no access to `testdata/hidden/`. Those tests
are mounted read-only into a second container that only verifies, so an agent in
the run cannot read or write its own judge. `docker/e2e.Dockerfile`'s build
context excludes them, so they reach no image layer.
