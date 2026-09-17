# TODO: alignment-audit findings, lined up against the next-items handoff

**Compiled**: 2026-09-16 against `main` @ `555bb93` — `health-check` 321 passed / 0 failed / 8 warned.
**Current**: `A1`–`A4` shipped (`2859f59`, `f7aaf29`); `health-check` now **330 passed / 0 failed**,
same 8 warnings. Framework v3.3.14 · `go test ./...` ok · `golangci-lint` 0 issues.

Two inputs, merged into one sequence:

1. **The alignment audit** (Phase 1, run 2026-09-15) — Loom measured against the Zero to Agent SDET
   curriculum, principle by principle, verifying enforcement rather than documentation.
2. **`loom-next-items-2026-09-15.md`** — the handoff's eleven credible roadmap items and five loose
   threads.

Every audit finding below was **verified in source**, not read off a SHIPPED marker. Roadmap IDs
`L3.47`+ are *proposed* — nothing is appended to `BUILD-ROADMAP.md` until an item is actually taken.

---

## The thing to read first: the audit and the handoff intersect in two places

These are not two independent backlogs. Two connections change what order things should be done in.

### 1. `A3` was a prerequisite for `L2.19`, and nobody knew — **now fixed** (`2859f59`)

`L2.19` ("Honour a policy decision at a gate") states its safety property as: *"never for an
always-human gate, **which cannot be targeted at all**."*

That property was true **by accident**. Gate #9 (removing test coverage) was added to
`shared/rules/approval-gates.md` and marked *Always Human*, but nothing in Go followed:

- `internal/policy/gate.go:26` said the constants "mirror the **eight** gates" — there are nine.
- `alwaysHuman()` held **five** entries against the rule's **six**, and the rule's own prose said
  *"those five gates"*, contradicting its own table.
- There was no `GateTestRemoval` constant at all, so a policy naming it was rejected as
  **`unknown gate`** — the message a typo gets — rather than as *always human*.

Nothing was broken at rest. The hazard was the repair: `unknown gate` invites whoever reconciles the
two lists to add gate #9 to `eligible()`, and `L2.19` is the item that would then consume it. A test
removal auto-approved by policy is the exact one-way door gate #9 was created to hold.

> **`L2.19` is now unblocked.** The classification it depends on exists, is held by two tests, and
> `A4` reports any future disagreement. Read `A4`'s note before adding a tenth gate.

### 2. `A6` names the mechanism behind one row of `L3.36`'s table

`L3.36` records that `accessibility-engineer` modified production source (+12) and asks how a stage
that "declares write tools? **no**" did that. Verified answer: its tools are
`Read, Glob, Grep, Bash` (`shared/agents/accessibility-engineer.md:4`). `Bash` *is* the write channel.

`L3.36` is about detecting the divergence after the fact. `A6` is about the capability that permits
it. They are complements, and `A6` is by far the cheaper of the two — it may shrink `L3.36`'s
candidate list, so **look at `A6` before costing `L3.36`**.

---

## Part 1 — Audit findings (`A1`–`A10`), smallest first

Trivial corrections first, deliberately: they are cheap, they remove the contradictions a later
reader would trip on, and three of them are in one file.

> **Status 2026-09-16**: `A1`–`A4` shipped on `fix/gate-nine-reconciliation` as `2859f59` and
> `f7aaf29`. `A1`+`A2`+`A3` landed as **one** commit, not the three this file originally planned —
> fixing the prose to "six" while the constant still held five would only have moved the
> contradiction. Everything from `A5` down is untouched.

### Tier 0 — text corrections, no code (one commit, ~15 min total)

- [x] **`A1` — `approval-gates.md` contradicts its own table.** — `2859f59`
      Prose says *"those five gates"*; the table marks six Always Human (#1, #3, #4, #5, #8, #9).
      **Fix**: five → six.
      **File**: `shared/rules/approval-gates.md`
      **After**: `generate-configs.sh` + `check-parity.sh` (it is a core rule — watch the
      **206-byte** bundle headroom; this change is net-zero characters).

- [x] **`A2` — stale comment in `policy/gate.go`.** — `2859f59`
      *"mirror the eight gates in shared/rules/approval-gates.md"* — there are nine.
      **File**: `internal/policy/gate.go:26`

### Tier 1 — small, mechanical, each with its own check (S)

- [x] **`A3` — gate #9 has no `GateID`, and no always-human classification.** — `2859f59` *(L3.47 · S ·
      **unblocks L2.19**)*
      **Fix**: add `GateTestRemoval GateID = "test-removal"` and an `alwaysHuman()` entry carrying
      the reason from the rule ("the judgement is whether losing *this* signal is acceptable").
      Do **not** add it to `EligibleGates()`.
      **Files**: `internal/policy/gate.go`, `internal/policy/gate_test.go`
      **Shipped**: red was proven by removing the `alwaysHuman()` entry; both tests failed with
      `unknown gate "test-removal"` — the audit's finding verbatim. Two tests hold it:
      `TestAlwaysHumanGatesAreRejectedAtLoad/test-removal` and
      `TestRemovingTestCoverageIsRejectedAsAlwaysHumanNotAsUnknown`, the latter asserting the
      rejection is *not* `unknown gate`, which is the distinction that matters.

- [x] **`A4` — nothing ties the rule's gate list to the Go constants.** — `f7aaf29` *(L3.48 · S)*
      This is the check whose absence let `A1`–`A3` drift in silence, and it is the highest-leverage
      item on this list.
      **Fix**: a `health-check.sh` section asserting every gate in `approval-gates.md` has a matching
      `GateID`, and that each gate's Policy-eligible **No — Always Human** line matches
      `alwaysHuman()` membership.
      **Shipped** as health-check section `7d` (330 passed / 0 failed, up from 321). Gate #7 was
      requested and granted explicitly. The gate-number -> `GateID` map is **pinned** in the script
      rather than parsed: only the three policy-eligible gates carry a `Policy gate ID:` line, and
      adding one to the other six costs ~180 bytes of a core rule with 206 left. Same trade, and the
      same rationale, as `TEST_WRITING_AGENTS` directly above it.
      **Proved red four ways**, each restored: drop #9 from `alwaysHuman()`; add #9 to
      `EligibleGates()`; append an unpinned gate #10 to the rule; drop `git-commit` from
      `EligibleGates()`. Each failed with its own message naming the gate and the disagreement.
      **New failure mode worth knowing**: adding a tenth gate to `approval-gates.md` now fails the
      build until it is pinned. Deliberate — pinning is only a control while widening it is a
      conscious act — but it is a new way for a rule edit to break CI.

- [x] **`A5` — a characterization net says what it does not cover.** — `e4862ba` *(L3.49 · S)*
      A characterization net that does not say what it *didn't* pin reads as broader than it is. The
      curriculum requires naming untried inputs, env/locale/time dependencies, unobserved side
      effects, and that correctness was never asserted.
      **Shipped**, with the item's own fix corrected. As written this said "add a `## Not Covered`
      section to the report" — but `.claude/feature-workspace/` is **gitignored** (`.gitignore:14`), so
      a note written only to the report dies with the workspace and the net ships recording nothing.
      The note goes in the **test file**, which is where the curriculum puts it and why.
      **Second defect, which the audit missed**: step 8 already required naming three
      behavior-carrying lines while the output template had **no section for them** — an orphaned
      instruction. They now live in the scope note, which is also what `backfill-unit-tests` step 6
      mutates.
      **Enforcement**: `judgment-only` with a reason, marked under PRECONDITION / ENFORCED-BY —
      `health-check` runs against this repo and cannot see test files in a user's project.
      `unit-tester` 1.5.0 -> 1.6.0; `check-agent-versions-ci.sh` OK; `health-check` 331 / 0 failed.
      **Follow-up**: `tests/agents/unit-tester/actual-output.md` is a *recording* of 1.5.0 output and
      now predates the requirement. It was deliberately **not** hand-edited — editing a golden is the
      drift the agent-goldens discipline forbids. Regenerate it in a live session, then add a
      scope-note line to `expected-patterns.txt` so the fixture actually tests this.

- [x] **`A6` — `tools` describes what an agent can actually do.** — `b6823f3` + `0255eb7` *(L3.50 · S · **closes L3.36's open question**)*
      **The item as written was wrong in two ways, and the real finding is bigger.**
      It said six agents; it is **seven** — `data-engineer` was missed. And it framed `Bash` as an
      over-permission on read-only agents. It is the opposite: **all seven are required to produce a
      markdown artifact and none declared `Write`**, so `Bash` was the undeclared write channel the
      markdown pipeline runs on. Dropping it would have broken the pipeline.
      Three go further and are instructed to modify source with tools they do not hold:
      `accessibility-engineer` ("Fix violations directly whenever possible"), `security-reviewer`
      ("Write fixes … **using the Write/Edit tools**"), `data-engineer` ("rewrite the migration").
      Both reviewers have carried exactly `Read, Glob, Grep, Bash` since their **first commit** —
      `Write` was never removed, it was never there.
      **This closes `L3.36`'s open question.** How did a stage that "declares write tools? no" change
      production source? It was instructed to, `Bash` was the only channel, and that channel is
      invisible to anything reading the `tools` field. `L3.36`'s remaining question is the *policy*
      one — should a post-review stage modify an approved tree — not the mechanism.
      **Shipped**: `Write` declared on all seven; `Edit` on the three told to fix source (removing
      their write channel would have decided `L3.36` by the back door); `Bash` dropped from the five
      with no execution use, kept for `code-reviewer` (complexity linter) and `security-reviewer`
      (`pnpm audit`). All seven -> **2.0.0** (tool access is a Major bump). **No prompt behavior
      changed.** The false "enforces capability boundaries" claim corrected in the contract, the
      pattern doc and the schema.
      **Fitness function** (`0255eb7`, gate #7 granted): health-check section `7e` — an agent whose
      prompt obliges it to write must declare the tool. Proved red against `b6823f3^`: seven FAILs
      with the right reason each; green at HEAD, 19 agents carrying an obligation.
      **Its limit is measured, not assumed**: stripping `Write` from `spec-writer`, whose obligation
      matches none of the patterns, **passes**. It is a tripwire on the commonest way this breaks,
      not the invariant. Narrow on purpose; a miss fails open.

### Tier 2 — needs a design decision before code (S–M)

- [x] **`A7` — a review must answer whether its tests would fail.** — `634b4ba` *(L3.51 · S)*
      `shared/contracts/review-contract.md:26` makes `## Test Design Review` a **required** section
      and `validate-artifact` checks it. The executor does not:
      `internal/state/review_state.go:79` is `TestDesignReview []string \`json:"...,omitempty"\`` and
      it is **absent from `Validate()`** (l.91-101). A `loom run` review can therefore omit the
      would-this-test-fail answer entirely — the thing `L3.41` was built to add.
      **Open question resolved — my earlier lean was wrong.** Requiring it only when the diff touched
      tests is not available: `Validate()` is a pure method on `ReviewState` with no access to
      `ImplementationState`, and `FilesModified` is agent self-reported rather than measured. Required
      **unconditionally**, which is what the contract already said; a diff with no tests answers in a
      sentence.
      **Worse than recorded**: an empty list renders as the word **"None"** under the heading
      (`render.go:110-118`), so the artifact passed `validate-artifact`'s presence check while carrying
      no judgment at all.
      **Shipped**: `requireItems` in `Validate()`, `required` in `review.schema.json` so the provider is
      asked for it, `code-reviewer` 2.0.0 -> 2.1.0, and the rule stated under the contract's Validation
      Rules. Verified by deleting the check and watching the new test fail.
      **Near-miss worth remembering**: the first attempt put the "never empty" note on the contract's
      required-section bullet, which silently broke `test-agents.sh`'s parser (anchored
      `^- \`(##...)\`$`) and removed the `## Test Design Review` section check — 281 passes quietly
      became 280. Caught by diffing the check lists, not by a failure. Adding a check while removing
      one is exactly what this audit exists to catch.
      **Files**: `internal/state/review_state.go`, `internal/state/review_state_test.go`

- [x] **`A8` — inner layers import only inward.** — `8613861` *(L3.52 · S)*
      Direction currently holds — verified with `go list -deps`: `internal/state` imports no internal
      package; `internal/orchestrator` reaches only `state` and `policy`. But only the **OTel slice**
      is asserted (`internal/telemetry/boundary_test.go`). Nothing would catch `internal/state`
      growing an import of `internal/orchestrator` tomorrow.
      **Shipped** as `internal/orchestrator/layers_test.go`, generalizing `boundary_test.go`'s
      transitive `go list` pattern. Gate #7 requested and granted.
      Pins each inner package to the **complete** set it may reach: `state`, `policy`, `worktree`
      reach nothing; `orchestrator` reaches `state` and `policy` only. Adapters are deliberately not
      pinned — they legitimately import inward and each other.
      **Equality, not a ceiling**: an extra entry is a violation, a missing one means the pin is
      stale and no longer describes the code. Both directions fail.
      **Proved red three ways**, each restored: `state` -> `worktree`; a stale pin claiming
      `orchestrator` reaches `internal/memory`; and — the one that justifies the test existing —
      `policy` -> `worktree`, where `orchestrator` fails **transitively** without itself changing.
      **Scope, honestly**: the Go compiler already rejects any inversion that closes an import cycle
      (my first mutant, `state` -> `telemetry`, was caught by the compiler, not by this test). What
      it cannot catch is an inner layer importing a leaf-shaped adapter, or reaching one through a
      helper. That is the gap this covers.

- [ ] **`A9` — egress is not distinguished from data privilege.** *(proposed L3.53 · M)*
      `grep -ri "egress|allowlist|exfiltrat" shared/ docs/` returns nothing substantive. Loom has
      least privilege on **data** only. `WebFetch`, `WebSearch` and `Bash` are unbounded egress with
      no allowlist concept anywhere in the framework. This is Level 4's central lesson and the
      largest single gap the audit found.
      **Fix**: an egress section in `docs/patterns/security-patterns.md` + a STRIDE-**I** line in
      `shared/agents/security-reviewer.md`. If no fitness function is possible, say so with a reason
      per guardrail #7 — do not ship it unenforced and unflagged.
      **Related**: overlaps `A6`, since `Bash` is the same channel.

- [ ] **`A10` — quarantine expiry is not made real.** *(proposed L3.54 · M)*
      `shared/knowledge/flake-triage-taxonomy.md:74` names the missing piece itself: *"a scheduled
      job that fails the build when a quarantine passes its date. Without that, everything above is a
      naming convention."* Gate #9 is prose-only — no executor barrier — so the discipline rests
      entirely on the agent proposing rather than applying.
      **Fix**: a `health-check.sh` section (or a `scheduled-monthly.yaml` entry, which would land
      `disabled: false`-by-default questions — see loose thread 5) that fails on a passed expiry.
      **Gate**: **#7**.

### Not scheduled — resolve, then decide

- [ ] **`C1` — `go-conventions.md` bans `any`/`interface{}`; Loom's own Go uses them 68 times.**
      Across 15 non-test files: `provider/mock/typed_scripts.go` (10), `orchestrator/executor.go` (4),
      `orchestrator/approval_binding.go` (4), `policy/decode.go`, `telemetry/tool.go`, …
      These are JSON payload maps, jsonschema plumbing, OTel attribute values, and the typed-provider
      dispatch table — places Go offers no alternative. The TypeScript rule has an escape hatch
      (`unknown` + Zod narrowing); the Go rule has none.
      **The audit's read: the code is right and the rule is over-absolute.** A rule violated 68 times
      knowingly is worse than no rule — it teaches that rules in this repo are aspirational.
      **Decision**: bound the rule (marshalling boundaries, reflection plumbing, provider dispatch),
      or accept and record the violation. Human call; the audit does not resolve it.

- [ ] **`C2` — tool scoping is only half Loom's to enforce.** The curriculum says scope a tool to
      values, not languages. Loom does not define `Bash` — it consumes a host platform's fixed tool
      vocabulary. The actionable half is `A6`; the rest is a limitation worth stating in the
      curriculum rather than a Loom defect.

- [ ] **`P10` — "unreproducible is not a closure state" has no Loom counterpart.** The *evidence*
      exists and is good (`shape.go`, `timeline.go`, `loom memory runs/retries/corrections`). The
      *discipline* does not. Either add a short Attribution & Closure section to
      `observability-patterns.md`, or decide it is judgment-only and say why.

---

## Part 2 — What the audit found already correct (do not "fix" these)

Recorded so a later session does not mistake a deliberate decision for a gap.

- **`gen_ai.request.model` is deliberately not emitted** (`internal/telemetry/tracer.go:52-56`) and a
  test asserts the **absence**. Loom passes no `--model`, so it has no honest source; a fabricated
  value would make a reader comparing against a pin conclude there was no substitution. Correct, and
  ahead of the curriculum.
- **`loom.loop.<id>.terminated_by`**, not `app.trajectory.terminated_by`. The curriculum says
  "namespace your own"; Loom is compliant in spirit. Low stakes, but do not let anyone "fix" it.
- **The four numbers are judgment-only, with a documented reason**
  (`docs/patterns/test-suite-health-metrics.md`). Three of four live in systems Loom does not read.
  This *satisfies* guardrail #7 rather than violating it.
- **Exemplars are deliberately not gated** — `L3.46`, decision recorded with measurement.
- **Gate un-bypassability (`P12`) is best-in-class.** `TestProviderClaimingApprovalCannotUnlockAGate`,
  the compiled `alwaysHuman()` kill-switch with load-time rejection, and the L2.14 digest binding
  (nine tests) are the strongest alignment in the framework. `A3`/`A4` repair the *edges* of this —
  they do not question it.

---

## Part 3 — The handoff's own items, re-sequenced

Unchanged in substance; only the ordering below reflects the audit.

**First (the handoff's own recommendation, still right):** reconcile the SHIPPED markers. ~49 items
`grep` as open and that number is wrong. Cheap, and every later session benefits. The audit re-confirms
the warning: `L3.38`–`L3.46` were all verified in code for this audit and all genuinely shipped, but
`A1`–`A3` show the converse hazard: a *rule* shipping does not mean its *code counterpart* did.

| ID | What | Effort | Audit note |
|---|---|---|---|
| L3.16 | `run.started` fires once per invocation | S | — |
| L2.26 | Keep the payload a stage was rejected for | S | — |
| L3.17 | Carry the run's provider across resume | S | — |
| L3.18 | Route on what the analysis says | S | — |
| L3.20 | Papercuts from the second real run | S | — |
| L3.25 | `context-engineer` budget ~7x under | S | — |
| **L2.19** | Honour a policy decision at a gate | S | **Unblocked** by `A3` (`2859f59`) — was not, before |
| L3.24 | Two UI-only stages non-skippable | M | — |
| **L3.36** | Nothing re-reviews post-review stages | M | **Mechanism found** by `A6` (`b6823f3`): instructed to fix + `Bash`. Only the policy question remains |
| L3.13 | Derive agent quality metrics from execution | M | — |
| L3.19 | Cut the per-stage prompt tax | L | branch `measure/l3-19c-growth` may have context |

### Loose threads (carried forward verbatim, with status)

- [ ] `check-agent-versions-ci.sh` has never run on a real PR — `pull_request`-only, everything since
      has gone to `main`. **`A5` and `A6` both touch agents**, so the first of them to go through a PR
      is the natural place to find out.
- [ ] Mutation testing needs its mutants checked — a mutant that fails to apply reads exactly like an
      uncovered line. If `backfill-unit-tests` step 6 is ever automated, the non-empty-diff guard must
      come with it.
- [ ] Comments that overclaim their enforcer. PRECONDITION/ENFORCED-BY catches marked cases only.
      **`A2` is an instance of this class** — a comment asserting a count the code no longer matched.
- [ ] Two standing `health-check` warnings: doc-audit now **42 days** old; `CODEMAP.md` stale. Both
      one command (`documentation-auditor`; `bash scripts/generate-codemap.sh`).
- [ ] Exemplar audits scheduled but disabled — `exemplar-auditor-monthly` is `enabled: false`, like
      every entry in `shared/hooks/scheduled-monthly.yaml`. **`A10` would add a sixth**; decide whether
      that file is a real mechanism or a parking lot before adding to it.

---

## Verification discipline (unchanged — run before every commit)

```bash
go build ./... && go test ./... && golangci-lint run ./...   # the build gate IS golangci-lint
bash scripts/health-check.sh                                  # 0 failed, 8 pre-existing warnings
bash scripts/generate-configs.sh && bash scripts/check-parity.sh   # after ANY shared/rules change
bash scripts/ci-check.sh                                      # after scripts/ or shared/ changes (needs Docker)
bash scripts/check-agent-versions-ci.sh HEAD~1 HEAD            # after any shared/agents change
```

- Agent edit ⇒ `version:` bump **and** a CHANGELOG row, same commit.
- New rule file ⇒ `collect_rules()` + a Cursor `generate_mdc()` call in `generate-configs.sh`.
- Core-rules bundle headroom: **206 bytes** (30,794 / 31,000). `A1` is net-zero; anything larger
  trims first.
- Prove every fitness function red before green. Commit per unit of work. Explain *why*.

---

## Feeds back into Training, not Loom

Five places the audit found Loom **ahead** of the curriculum. These are training backlog — no Loom
work, and worth noting against the level so the curriculum stays honest about what the framework does.

| # | Practice | Level |
|---|---|---|
| 1 | Mutation-proof the net in a throwaway `git worktree`, so "never modify source" needs no exception | 2B — characterization |
| 2 | An honest **absent** attribute beats a fabricated present one (`gen_ai.request.model`) | 3C — attributes |
| 3 | The always-human list as a **compiled constant**, and silence is the wrong answer to a request that will never be honoured | 4 — Module 09 |
| 4 | Approval binds to a **digest set**, not a file; an identical re-run survives, any edit invalidates, the record is kept | 4 — Module 09 |
| 5 | Safe-argument allowlists pinned by a test that forces justification to widen | 3C — what never goes in a span |
