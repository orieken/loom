# Brutal review — the factory intake work and `deliver-feature`

**Date**: 2026-10-23 · **Scope**: this PR's intake contract, validator and installer changes
(roadmap L3.63–L3.64), and `shared/skills/deliver-feature/SKILL.md` as it stands on `main` @ `a3ded2d`
· **Method**: every finding below was checked against the code or the file it cites; nothing is
reported from the prose alone. Line numbers are as of that commit.

---

## Verdict

**Intake**: the validator is solid — 100% statement coverage, every hand mutation killed — but the
contract around it made two claims that were false when written, and an instruction that would have
destroyed the file it describes. Those three are fixed in this PR. What remains is mostly decisions
the supervisor will force anyway (where intake lives in git, what `hold` means), plus one executor
gap that matters more than anything in intake: **the spec is never digested**.

**`deliver-feature`**: it has become the documentation of two pipelines at once. About a quarter
of its 498 lines explain what does *not* apply when `loom run` executes instead, its step numbers
have drifted from its own cross-references, its policy path auto-proceeds without recording anything, and the
`CLAUDE.md` that `loom install` puts into every project describes a pipeline this skill retired. The
skill is not wrong about the pipeline; it is wrong about itself in a dozen small places, and an
agent reading it cannot tell which.

---

## Fixed in this PR

| # | Finding | Fix |
|---|---|---|
| F-1 | `install.sh` is a second install path and copied `features/` but not `intake/`, so projects installed through it never got the template. | One `install_team_template_dir` helper copies both; verified fresh, rerun and `--dry-run`. |
| F-2 | `intake/README.md` justified "the supervisor never writes the file" by saying `loom run` hashes its input. It does not: run state stores `specPath` only (`internal/orchestrator/state.go:107`). | Claim corrected; the gap itself is **I-1** below. |
| F-3 | `intake/TEMPLATE.md` step 3 said to let `/spec-writer` fix the file. spec-writer's fix mode rewrites the file with its own three-key frontmatter (`shared/agents/spec-writer.md:242`, `:297`), which would drop the work order. | Template now says to take the critique and edit by hand. Root fix is **I-2**. |

---

## Action items — to clean up later

Priority: **P1** wrong or unsafe today · **P2** contradiction or drift that will mislead an agent ·
**P3** cleanup. "Where" is the file to change first.

### Intake and the factory floor

- [ ] **I-1 (P1) — The executor never digests the spec.** Approvals bind to artifact digests, but the
  spec they were derived from can change mid-run — acceptance criteria included — and nothing
  notices. This is an executor defect that predates intake; intake just makes it likelier, because
  humans now edit the frontmatter of in-flight specs. *Fix*: record the spec's SHA-256 at run start,
  verify it on resume like any artifact, and treat a change as invalidating every stage.
  *Where*: `internal/orchestrator/executor.go`, `state.go`.
- [ ] **I-2 (P1) — spec-writer clobbers frontmatter it does not know.** F-3 is a warning, not a fix.
  *Fix*: spec-writer preserves unknown frontmatter keys in fix mode; a golden test in
  `tests/agents/spec-writer/` with an intake file proves it. *Where*: `shared/agents/spec-writer.md`.
- [ ] **I-3 (P2) — `trust` is self-declared by the person who may have pasted the untrusted text.**
  The body scan runs only for `trust: external`, so a ticket pasted into a hand-written order is
  never scanned. *Fix*: scan every order; report findings as violations for external orders and as
  warnings for internal ones. *Where*: `internal/intake/check.go`.
- [ ] **I-4 (P2) — The vocabulary exists three times**: Go sets (`internal/intake/vocabulary.go`), the
  template's comments, and the README's rule list. Only the template's *parseability* is tested.
  *Fix*: a test that reads the README's listed values and compares them with the Go sets.
- [ ] **I-5 (P2) — `loom intake check` answers differently by working directory.** `plan` is resolved
  through `planfile.SearchPath()`, which is relative; run from a subdirectory, every project plan
  vanishes. *Fix*: resolve from the repository root. *Where*: `cmd/loom/cmd/intake.go`.
- [ ] **I-6 (P2) — Where intake lives in git is undefined.** Is `status: ready` read from the working
  tree, or only once committed to `base-branch`? A supervisor reading the working tree runs
  uncommitted specs; one reading the branch needs a commit per status flip. Decide before L3.65.
- [ ] **I-7 (P2) — `hold` has no meaning for a run in flight.** The README first promised a held run
  would finish its stage and stop at the next gate; `loom run` has no external stop signal but
  SIGINT. The promise is withdrawn in this PR; the decision is not made. *Fix*: give the executor a
  stop-at-next-gate request, or define `hold` as affecting only orders not yet claimed. *Where*: L3.65.
- [ ] **I-8 (P3) — Priority ties break on `created`, which has day granularity**, so same-day orders
  start in filesystem order. Add `id` as the final tie-break.
- [ ] **I-9 (P3) — Budget defaults are unfounded.** `$10 / 4h` came from nowhere. The first real run
  cost $3.57 for five stages (roadmap header, 2026-09-06); a full fourteen-stage run may exceed
  $10 and park spuriously. *Fix*: derive defaults from `loom memory agents` cost data.
- [ ] **I-10 (P3) — Duplicate stems across `intake/` and `features/` are checkable now**, with
  directory context, but are deferred to the supervisor. Cheap to add to `loom intake check`.
- [x] **I-11 (P3) — Unverified paths.** *Closed 2026-10-23: the command package passes in CI's go-build and test-image jobs (#2, #3), and an intake file runs to completion under the mock (#4).* The command's tests ran in a harness package (sqlite was
  unreachable in the authoring sandbox), and L3.63's Done-when — `loom run --provider mock` on an
  intake file — has not been run. Confirm both in CI before L3.65 builds on them.
- [ ] **I-12 (P3) — `scope.paths` globs and `source-ref` are not validated.** A malformed glob or a
  Jira key under `source: github-issue` passes.

### `deliver-feature` and its surroundings

- [ ] **D-1 (P1) — The installed `CLAUDE.md` teaches a retired pipeline.** `loom install` copies
  `templates/claude-feature-team/CLAUDE.md` into every project as its `CLAUDE.md`, loaded every
  session. It describes the flat pre-Epic-63 workspace (`.claude/feature-workspace/analysis.md`,
  `:61`) that `deliver-feature` ships migration code to repair, omits context-engineer entirely, and
  lists different checkpoints. *Fix*: the template points at `/deliver-feature` and `loom run`
  instead of restating the pipeline.
- [ ] **D-2 (P1) — Policy-driven auto-proceed is silent.** The markdown pipeline auto-proceeds past
  checkpoints in `policy-driven` mode and, by its own admission (`SKILL.md:137`), records nothing.
  `approval-gates.md` promises no silent auto-approvals. *Fix*: when `loom state` is available,
  record each policy decision through it; when it is not, `policy-driven` falls back to
  `strict-human`.
- [ ] **D-3 (P2) — The policy schema disagrees with its reader.** `delivery-policy.template.yaml:5`
  offers `mode: "unattended"`, defined nowhere else; `SKILL.md:131` reads `autoProceedPersistence`,
  which the template does not contain; no Go code reads `mode` at all. *Fix*: one schema, validated.
- [ ] **D-4 (P2) — Step numbers have drifted.** `:144` says Friday is step 40 (it is 42); the state
  and trace examples cite steps 6, 8 and 19 for agents now at 7, 9 and 20 (`:268`, `:276`, `:320`,
  `:332`). Cross-referencing by number inside a 43-step prose list will keep drifting. *Fix*: refer
  to stages by name.
- [ ] **D-5 (P2) — Declining Friday has no defined outcome.** Phase is set to `complete` only on "ship"
  (`:132`), so a declined delivery is never complete and `resume-pipeline` would resume a finished
  run. The gate is also unconditional, while Standalone Mode (`:495`) calls Friday non-blocking.
  *Fix*: Friday is opt-in by configuration; declining completes the run with `Friday: Skipped`.
- [ ] **D-6 (P2) — The guardrails contradict each other**: "context-engineer always runs before
  analyst" (`:479`) and "the analyst is always first" (`:480`).
- [ ] **D-7 (P2) — Four ways to run one pipeline**: `/deliver-feature`, `/orchestrate`,
  `/orchestrate --legacy` (`:37`), and `loom run`. `orchestrate/SKILL.md:87` still logs a
  `workflow.completed` event whose layer L3.9 retired, and its `fork` strategy is the simulated
  concurrency L3.3 says to delete. *Fix*: `loom run` where the binary exists, this skill as the
  fallback, and `/orchestrate` retired or reduced to an alias.
- [ ] **D-8 (P2) — Routing lives in two places with no test between them.** Steps 12, 14, 16, 22 and
  34 restate the `internal/state` predicates in prose. *Fix*: a test that every predicate is named
  in the skill, or generate the conditions from Go.
- [ ] **D-9 (P3) — The prompt pays for the executor's documentation.** About a quarter of its 498 lines
  (§Relationship, §Gate Decisions, §Review Loop, §Which Stages Run, the `loom state` half of
  §Checkpointing — ~125 lines) explain what does not apply under `loom run`. Measured prompt tax already exists
  (`loom-prompt-tax-attribution-2026-09-10.md`). *Fix*: move executor notes to `cmd/loom/README.md`.
- [ ] **D-10 (P3) — A side effect hides in step 40.** Every fifth delivery auto-runs a retrospective,
  counting bugfixes too since they share the namespace (ADR-004). *Fix*: make it a scheduled job.
- [ ] **D-11 (P3) — Gate #9 is missing** from the skill's Always-Human list (`:138`); test-coverage
  removal is Always Human in `approval-gates.md`.
- [ ] **D-12 (P3) — The git-index guardrail (`:492`) is prose a model must remember.** L3.66's
  per-run worktrees make it structural; fold it into that item.
- [ ] **D-13 (P3) — Dead skills ship in the binary.** `templates/claude-feature-team/.claude/skills/`
  — including a 155-line fork of `deliver-feature` — is embedded by `embed.go` but installed by
  neither installer. Claude Code still discovers them as live skills when working in this repo.
  *Fix*: delete them, or install them; not both silently.
- [ ] **T-1 (P2) — A flaky test in `go-build`.** #4's first `go-build` attempt failed inside `go test`
  and passed on a rerun of the same commit; the `test-image` job, running the same tests, passed first
  time. The failing test's name is in that attempt's log (run 40042528576, attempt 1), which the
  authoring sandbox could not read. Candidates are the timing-based interrupt/resume tests in
  `cmd/loom/cmd/run_test.go`. *Fix*: name it from the log, then make it deterministic.
- [ ] **D-14 (P3) — The spec-ingestion check is a keyword list in prose** (`:72`) that duplicates
  `internal/injectionscan`. *Fix*: run `loom ki scan` on the spec when the binary is present.

---

## Suggested order

**I-1** and **D-1** first: one is the executor trusting an input it never checks, the other puts a
contradicting pipeline into every project's always-loaded context. Then **D-2/D-3** together (one
schema, recorded decisions), then **I-6/I-7**, because L3.65 cannot start without those answers.
The P3s can ride along with whichever item touches the same file.
