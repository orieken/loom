# Build a software factory — the blueprint

A reusable prompt. It describes the patterns that made `loom factory` + `deliver-feature` work, and
asks for a new factory (a new **plan** on the same floor, or a floor in a new codebase) built the same
way. The three specialised prompts in this folder (`02`–`04`) each start by pointing here.

**Scope of this prompt: DESIGN first, then a phased build plan.** Produce an ADR and a phased plan
before any code. Each phase ends with an exit criterion that is demonstrated, not asserted.

## Prior context (read first)

The reference implementation lives in `/Users/oscarrieken/Projects/Rieken/ai-assistant-dot-files`:

- `docs/roadmaps/factory-phases.md` — how the floor was built, in dependency order
- `docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md` — where an order's state lives
- `docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md`, `ADR-009-retire-unit-level-tdd-ritual-for-agents.md`
- `docs/audits/factory-intake-and-deliver-feature-review-2026-10-23.md` — what went wrong the first time
- `templates/claude-feature-team/intake/README.md` and `TEMPLATE.md` — the work-order contract
- `internal/orchestrator/plan.go` — a plan as data (stages, gates, typed state, loops)
- `internal/planfile/` — plans discovered from `.claude/plans/` and `shared/plans/`
- `tests/e2e/README.md` — the harness: hidden acceptance tests the factory never sees
- `.claude/factory/ledger.jsonl` — real run history. Read it for failure modes before designing.
- `shared/rules/approval-gates.md` — the always-human gates

## The patterns, and why each exists

Keep every one unless the ADR says why it does not apply. Each line is a lesson the reference
implementation paid for.

### Intake

1. **One file is both the work order and the spec.** Frontmatter is the order (priority, budget,
   finish line, scope). Body is the spec. No conversion step between "ticket" and "input".
2. **Humans own intent; the ledger owns progress.** The supervisor never writes an intake file. One
   writer per fact. `status` has three values a human sets: `draft | ready | hold`.
3. **Orders are read from a commit, not a working tree.** Promoting to `ready` is a reviewed commit.
4. **Snapshot at claim.** The run's spec is a copy taken at the claimed commit. Editing the intake
   file afterwards never changes a running order; the inbox says "changed since claim".
5. **The spec is digested.** A changed spec invalidates every completed stage and every approval
   bound to it.
6. **Strict, versioned schema.** `intake: <n>` version; unknown keys are rejected (a misspelt key is a
   setting its author believes they made). A deterministic `check` command enforces it in CI.
7. **Trust is explicit.** `trust: internal | external`. Imported text is scanned for
   instruction-override language and always stops at the first human gate.
8. **`requested-by` is mandatory for `ready`.** Nobody is there to ask during an unattended run, so
   someone must answer for the spec.
9. **Open Questions carry an assumption.** "Assumption if unresolved: …" — an unattended run will
   guess, so make the guess explicit and reviewable.

### The run

10. **A plan is data, not prose.** Stages have stable IDs (never step numbers), an agent, an optional
    gate, a typed-state kind, the upstream stages they consume, and whether the router may skip them.
11. **Typed state between stages.** Each stage returns a schema-validated JSON document; downstream
    stages get a deterministic projection of only the fields their contract declares. Prose
    handoffs paraphrase, drift and bloat context.
12. **Route once, from the analysis, and record why.** Skippable stages are decided in code, written
    to `route.md` with a reason per stage. Review stages are never skippable.
13. **Bounded loops.** A producer↔reviewer loop is a declared span with a named exit condition and a
    round limit. Exhausting it halts at a human gate; it never loops forever or proceeds silently.
14. **Write the judge before the work.** Acceptance scenarios are written from the criteria alone,
    before the producer exists. The plan validator rejects a plan that orders them after.
15. **Hidden tests decide "done".** The end-to-end harness judges the output with tests the factory
    never saw, mounted into a separate container the run cannot reach.
16. **Run the project's own checks before asking a human to ship.** Pre-ship checks are commands the
    project declares — the same ones CI runs. A human is never asked to approve what CI will reject.

### The floor

17. **One worktree and branch per order.** Parallel orders never share a git index.
18. **Budgets are ceilings, recorded.** `max-usd`, `max-hours`, `max-attempts` per order. Spend that
    cannot be measured parks the run until a human accepts it.
19. **Transient vs terminal failures.** Transient retries with backoff; terminal parks on the first
    attempt with a reason. A parked order is cleared by editing it.
20. **Gates bind to digests.** An approval is for exactly what the human was shown. Changing an
    artifact after approval resets the approval.
21. **Policy auto-approval in one engine, recorded.** No silent auto-proceed path anywhere. An order
    may tighten autonomy, never loosen it.
22. **The finish line stops before the irreversible step.** Push the branch, propose the PR in the
    inbox; opening it is a human action.
23. **Corrections are signal.** What a human edits at a gate is recorded against the producing agent
    (`artifact.corrected`). It is the factory's best quality metric.

## What actually failed in the reference runs

Read the ledger yourself, but as of 2026-10-29: 11 claims, 6 shipped. Every failure was plumbing,
not domain understanding — an agent returning prose instead of a JSON state document, a state
payload with a field the schema did not know (twice), a sandbox permission block, a provider session
limit, and spend the provider did not report. Design the new factory for these first: strict output
contracts the agent is shown verbatim, schema errors fed back as a bounded retry, and a provider
abstraction that distinguishes "limit reached" from "failed".

## Deliverables

1. **ADR** in the target repo: what this factory produces, for whom, what "done" means, which
   patterns above it keeps and which it drops (with why), and what its hidden judge is.
2. **Plan definition**: every stage with ID, agent, gate, typed-state kind, consumes, skippable.
   Draw it as a mermaid diagram like `docs/patterns/deliver-feature-workflow.md`.
3. **Intake body schema** for this plan (see `01-shared-intake.md` — the frontmatter is shared,
   the body is per plan).
4. **Stage contracts**: one JSON schema per typed stage, and the projection each consumer receives.
5. **The judge**: the hidden-test harness for this factory's output, and a negative test proving the
   harness fails a bad output.
6. **Phased build plan** in the style of `docs/roadmaps/factory-phases.md`, with exit criteria.

## Rules for the design

- Reuse the floor. A new factory is a new plan file, new agents and new contracts — not a new
  supervisor, ledger, inbox or intake parser. Justify any exception in the ADR.
- Every gate names the human role that answers it. If the answer is "the author", say so.
- Measure before automating. No policy auto-approval in the first phase.
- Cyclomatic complexity < 7, changed-line coverage ≥ 85%, every test able to fail (ADR-008).
