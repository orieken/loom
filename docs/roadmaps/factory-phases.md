# Software factory — phased plan

**Drawn from**: the *KERNEL — Factory Floor* workstream in [`BUILD-ROADMAP.md`](BUILD-ROADMAP.md)
(L3.63–L3.74), and the action items in
[`factory-intake-and-deliver-feature-review-2026-10-23.md`](../audits/factory-intake-and-deliver-feature-review-2026-10-23.md)
(I-1…I-12, D-1…D-14). Every item keeps its original ID; this file only orders them.

**Effort**: S = under a day · M = a few days · L = a week or two.

---

## Phase 1 — build every missing part

**Goal**: hand an intake file to the factory, have it build something unattended in an isolated
worktree, and prove the result with tests the factory never saw.

**The rule for what is in Phase 1**: an item is here if the factory cannot run without it, if an
unattended run would be unsafe without it, or if it corrupts what the factory reads. Everything else
is Phase 2, however worthwhile.

### Step 1 — Prove what already exists

Nothing in Steps 2–4 should build on code that has never run in full.

- [x] **L3.72** (S) — pinned build-and-test image; full suite, lint, mutation and `loom install` in one
  container, the same image in CI.
- [x] **I-11** (S) — `cmd/loom/cmd` passes in that image, and `loom run --provider mock` accepts an
  intake file (L3.63's unverified Done-when).
- [x] **I-1** (M, P1) — the executor digests the spec and invalidates the run when it changes. Every
  unattended run trusts its spec; today nothing checks it.
- [x] **D-1** (S, P1) — the `CLAUDE.md` installed into every project stops teaching the retired
  pipeline. The factory runs agents inside those projects.
- [x] **I-2** (S, P1) — spec-writer preserves frontmatter it does not know, so authoring an order
  cannot destroy it.

### Step 2 — Decide before building the supervisor

These are decisions, not code, and L3.65 cannot be designed around an open question. All four are
laid out, with a recommendation each, in [ADR-010](../adrs/ADR-010-factory-order-state-and-policy-configuration.md).

- [x] **I-6** — where intake lives in git. *Decided (ADR-010, 2B): the factory reads orders committed on `base-branch`.*
- [x] **I-7** — what `hold` means for a run already in flight. *Decided (ADR-010, 1C and 3A): orders are snapshotted at claim; `hold` stops resumption at the next halt, and `loom factory stop` interrupts at once.*
- [x] **D-2 + D-3** (M, P1) — one validated policy schema, and every policy decision recorded. An
  unattended factory is the policy path; it cannot be the silent one. *Built (ADR-010, 4B): strict
  `delivery-policy.yaml` validation in `loom run`; the markdown pipeline never auto-proceeds.*

### Step 3 — Build the floor

In dependency order. Small rules that belong to the supervisor ride with it.

- [x] **L3.66** (M) — one git worktree per run. Also retires **D-12**, the prose-only git-index rule.
- [x] **L3.65** (L) — `loom factory run`: poll, validate, claim, ledger, launch. Includes **I-5**
  (resolve plans from the repo root), **I-10** (duplicate stems across `intake/` and `features/`), and
  **I-8** (`id` as the final priority tie-break).
- [x] **L3.67** (M) — the approval inbox, delegating to the existing approval path.
- [x] **L3.68** (M) — run-level retry with backoff, and parking with a reason.
- [x] **L4.3** (M) — the budget governor. Pulled into Phase 1: an unattended factory with no spend
  ceiling is not safe to leave running.
- [x] **L3.70** (S) — order budgets seed that governor.
- [x] **L3.69** (S) — the finish line: push `factory/<id>`, put a PR proposal in the inbox. Opening
  the PR stays gate #5, Always Human.

### Step 4 — Prove it builds something

- [x] **L3.73** (M) — the end-to-end harness: a fixture repo, hidden acceptance tests, explicit gate
  approvals, mock provider on every PR.
- [ ] **L3.74** (S) — the same harness nightly against the real provider, under a hard budget.

### Phase 1 exit criteria

Phase 1 is done when all four hold, demonstrated rather than asserted:

1. An order dropped into the harness's `intake/` is claimed, run in its own worktree, and stops at
   each gate until the harness approves it.
2. The run ends with a pushed `factory/<id>` branch and a PR proposal in the inbox — and no PR opened.
3. The hidden acceptance tests pass against that branch, every digest verifies, and the diff stays
   inside the order's `scope.paths`.
4. A nightly real-provider run finishes inside its budget, or parks and says why.

**Size**: 17 items — 7 S, 7 M, 1 L, and 2 decisions. Roughly six to eight working weeks for one
person, most of it in L3.65 and the four M items around it.

---

## Phase 2 — fine-tune and clean up

Starts once Phase 1's exit criteria hold. Ordered by what Phase 1's real runs will tell you, then by
how often an agent reads the thing being fixed.

### Tune from real-run data

- [ ] **T-1** (S) — find and fix the flaky test in `go-build` (found while closing I-11).

- [ ] **I-9** (S) — budget defaults derived from recorded run costs, not guessed.
- [ ] **I-3** (S) — scan every order's body; warnings for internal, violations for external.
- [ ] **I-12** (S) — validate `scope.paths` globs and `source-ref` per source.
- [ ] **L4.2** (M) — the shared error taxonomy; L3.68's transient/terminal split becomes its first user.

### Extend

- [ ] **L3.71** (M) — tracker import adapters (GitHub issues first). Not needed to prove the factory;
  needed to feed it without hand-writing every order.
- [ ] **L3.3** (L) — real in-run parallelism. The supervisor already runs orders in parallel as
  separate processes; this is about stages within one run.

- [ ] **L3.75** (S) — the inbox shows a reviewer what is theirs. Each order names a reviewer per gate
  (intake v2, `01-shared-intake.md`); `loom factory inbox --reviewer <handle>` filters to it. Which
  roles a team has is configuration, not a fixed set of three: a team declares its roles and who
  holds each in `.claude/factory/team.yaml`, one person can hold several, and a gate whose role is not
  held falls back to a named default reviewer. With no `team.yaml`, everyone sees everything, as now.
  *(raised 2026-10-29, reagent-map pilot)*
- [ ] **L3.76** (M) — tests in a different repo from the app. The floor clones the order's
  `target.repo` into the run's worktree and opens the PR there. Not needed for the pilot, which keeps
  tests with the app; needed for teams whose testers own a separate repo. *(raised 2026-10-29)*

- [ ] **L3.77** (M, P1) — a run sees only what its branch will have. Worktrees live inside the
  checkout (`.loom/worktrees/<id>`), so an agent can read the main checkout's untracked and ignored
  files through a relative path. Found in saturday-monorepo's first order: `context-engineer` "read
  in full" five files under a gitignored `containers/` that did not exist in the worktree, and built
  the analysis on them. *Fix*: create worktrees outside the checkout (`~/.loom/worktrees/<repo>/<id>`),
  and have `context-engineer`'s measurer flag any pinned file missing from the worktree as a defect,
  not "cannot read". *(raised 2026-10-29)*

- [ ] **L3.78** (S) — stage timeouts a plan or order can raise. `defaultStageTimeout` is 30 minutes
  for every stage, hard-coded in `internal/orchestrator/plan.go`; plan files cannot change it. A
  developer stage that builds a container image under amd64 emulation hit it in saturday-monorepo's
  first order, was killed, reported no usage, and parked. *Fix*: a per-stage `timeout` in plan files,
  an order-level override within the budget's `max-hours`, and a killed stage's partial spend
  recorded rather than "unmeasured". *(raised 2026-10-30)*

### Clean up `deliver-feature` and its surroundings

- [ ] **D-7** (M) — one way to run the pipeline: `loom run` where the binary exists, the skill as
  fallback, `/orchestrate` retired or aliased. Do this first; it makes several below smaller.
- [ ] **D-9** (S) — move the executor's documentation out of the skill's prompt.
- [ ] **D-8** (S) — test that the skill's routing prose names every Go predicate.
- [ ] **D-4** (S) — refer to stages by name, not step number.
- [ ] **D-5** (S) — Friday becomes opt-in; declining completes the run.
- [ ] **D-6** (S) — the contradictory "always first" guardrails.
- [ ] **D-10** (S) — the every-fifth-delivery retrospective becomes a scheduled job.
- [ ] **D-11** (S) — add gate #9 to the skill's Always-Human list.
- [ ] **D-13** (S) — delete or install the dead skill copies under the team template.
- [ ] **D-14** (S) — the spec-ingestion check calls `loom ki scan`.
- [ ] **I-4** (S) — a test tying the intake README's vocabulary to the Go sets.

**Size**: 18 items, almost all S. Most can ride along with whatever touches the same file.

---

## Not in either phase

Nothing from the review is dropped. **D-12** closes inside L3.66; **I-5**, **I-8** and **I-10** close
inside L3.65. If a Phase 1 item turns out bigger than its estimate, split it here rather than moving
work silently between phases.
