# Handoff: next roadmap items after the test-evidence & trace-fidelity stream

Self-contained prompt for a fresh Claude Code chat in
`/Users/oscarrieken/Projects/Rieken/ai-assistant-dot-files`.

**State at handoff**: `main` @ `2a22047`, green, pushed. Framework v3.3.14 · 40 agents · 69 skills ·
`health-check` 321 passed / 0 failed / 8 warned · combined Go coverage **66.5%** against a **66.3%**
floor.

---

## What just finished

Nine items, all shipped and marked: **L3.38–L3.46**. They came from reviewing two training levels
(AI adoption in existing test suites; GenAI observability) against what loom actually enforced.

| | |
|---|---|
| L3.38 | Guardrail #9 — tool spans record properties, not payloads; allowlist + `tools.SafeArguments` |
| L3.39 | `test-repair-contract.md`, approval gate #9, nine agents bound |
| L3.40 | Exemplar tests — contract, manifest, registry source, `exemplar-auditor`, +schedule and freshness check |
| L3.41 | `code-reviewer` asks whether a test would fail; mutation stopping condition in `backfill-unit-tests` |
| L3.42 | `test-suite-health-metrics.md` + flake-triage KI |
| L3.43 | `gen_ai.response.model` (was mislabelled), finish reasons, `loom.loop.<id>.terminated_by` |
| L3.44 | Trace-shape assertions with a committed baseline |
| L3.45 | Coverage 64.7% → 66.5%; CI now reports coverage even when tests fail |
| L3.46 | Exemplars deliberately **not** gated — decision recorded with measurement |

Four pre-existing defects surfaced and were fixed along the way: a stage timeout that waited on the
agent's orphaned children, a CI check that false-FAILed every agent PR (SIGPIPE under `pipefail`), a
stale `examples/embedding/go.mod`, and a load-sensitive flaky probe test.

---

## Read these first

- `docs/roadmaps/BUILD-ROADMAP.md` — the authoritative work list. **Read its header before trusting
  item status.**
- `shared/rules/architecture-guardrails.md` — nine hard constraints, #9 is new
- `shared/rules/approval-gates.md` — nine gates, #9 is new
- `shared/rules/test-repair-contract.md` — new this stream
- `docs/patterns/framework-meta-patterns.md` — the PRECONDITION / ENFORCED-BY convention
- `docs/CONTRIBUTING.md` — rule changes need `generate-configs.sh` + `check-parity.sh`

---

## Trap: "open" items are not all open

`grep`ing for items without a **SHIPPED** marker returns ~49. That number is wrong and the roadmap
header says why: *"Absence of a SHIPPED line means only that no one has reconciled it, not that the
work is unbuilt."*

M0.1–M0.3 and L2.1–L2.8 are pre-`SHIPPED`-convention and largely built — M0.2 is "Put the Go in CI",
and Go has been in CI for weeks. **Verify against the code before starting anything that looks
open.** Two items in this very stream (L3.38, L3.39) shipped days before their markers were added.

**Reconciling those markers is itself a good first task** and is cheap: for each unmarked item, check
whether the target files show the work, and either add a SHIPPED line with the commit or leave it
alone. It would make the roadmap trustworthy again, which every later session benefits from.

---

## Credible next items (verified unblocked, small to medium)

| ID | What | Effort |
|---|---|---|
| **L3.16** | `run.started` fires once per invocation, not once per run | S |
| **L2.26** | Keep the payload a stage was rejected for | S |
| **L3.17** | Carry the run's provider across resume | S |
| **L3.18** | Route on what the analysis says, not how many items it has | S |
| **L3.20** | Papercuts from the second real run | S |
| **L3.25** | `context-engineer` reports a token budget ~7x under, with arithmetic | S |
| **L2.19** | Honour a policy decision at a gate | S |
| **L3.24** | Two UI-only stages non-skippable; one boilerplate NFR routes in two more | M |
| **L3.36** | Nothing re-reviews what the post-review stages write | M |
| **L3.13** | Derive agent quality metrics from execution | M |
| **L3.19** | Cut the per-stage prompt tax | L |

`L3.19` is where the branch at the start of the last session came from (`measure/l3-19c-growth`), so
there may be context worth recovering there.

---

## How the last session worked, and why it is worth repeating

This method produced nine shipped items and four incidental bug fixes with no rework. It is the most
transferable thing in this handoff.

1. **Verify the item's premise before building it.** Four of nine items were wrong about specifics:
   L3.41's section already existed and specified nothing; L3.43's attribute was *mislabelled*, not
   missing; L3.44's proposed token assertion would have been vacuous; L3.40's cost cascaded past its
   estimate. The items were written after reading the repo carefully and were still wrong about half
   the time. **Checking the premise is where the value is.**

2. **Ask before building, with real options.** Each item began with 3–4 questions naming genuine
   trade-offs and a recommendation. Several answers changed the design materially — the allowlist
   inversion in L3.38, the scope split in L3.40, dropping the token assertion in L3.44.

3. **Measure instead of assuming.** L3.46 was closed by measuring how often exemplar files actually
   changed (10 commits, 0 exemplar-body changes) rather than waiting for data to accumulate. The
   dash-vs-macOS `/bin/sh` difference behind the timeout bug was verified in a container, not
   reasoned about.

4. **Prove red before green.** Every fitness function was verified by breaking it deliberately and
   watching it fail, then restoring. Every new test was mutation-verified.

5. **Respect the gates.** Gate #7 (wiring a fitness function) was requested explicitly four times and
   never assumed. `architecture-guardrails.md` is amendable by a reviewed commit — its "cannot be
   overridden" banner governs runtime override during agent execution, not amendment.

---

## Verification discipline for this repo

Run before every commit that touches the relevant surface:

```bash
go build ./... && go test ./... && golangci-lint run ./...   # the build gate IS golangci-lint
bash scripts/health-check.sh                                  # 0 failed, 8 pre-existing warnings
bash scripts/generate-configs.sh && bash scripts/check-parity.sh   # after ANY shared/rules change
bash scripts/ci-check.sh                                      # after scripts/ or shared/ changes (needs Docker)
bash scripts/check-agent-versions-ci.sh HEAD~1 HEAD            # after any shared/agents change
```

- An agent edit needs a `version:` bump **and** a `shared/agents/CHANGELOG.md` row in the same commit.
- A **new rule file** needs `collect_rules()` and a Cursor `generate_mdc()` call in
  `generate-configs.sh` (CONTRIBUTING step 3).
- The core-rules bundle has **206 bytes of headroom** (30,794 / 31,000). Adding to a core rule will
  trip `TestCoreRulesBundleStaysUnderCeiling`. That is deliberate — trim before raising, and prefer
  putting detail in a pattern doc with a pointer from the rule.
- Commit per unit of work, not batched. Conventional Commits. Explain *why*.

---

## Loose threads

1. **`check-agent-versions-ci.sh` has never run on a real PR.** Its SIGPIPE fix (`617f20c`) is
   verified by worktree tests only; the job is `pull_request`-only and everything since has gone
   straight to `main`.
2. **Mutation testing needs its mutants checked.** Twice a mutant failed to apply and the passing
   test read exactly like an uncovered line. `backfill-unit-tests` step 6 now requires confirming a
   non-empty diff — if that step is ever automated, that guard must come with it.
3. **Comments that overclaim their enforcer.** Twice a comment asserted a property broader than the
   test actually held (`cloneForInspection`, the `levels.yaml` measurement). The
   PRECONDITION/ENFORCED-BY convention catches marked cases only.
4. **Two standing `health-check` warnings**: the doc-audit is 41 days old, and `CODEMAP.md` is stale
   (`bash scripts/generate-codemap.sh`). Both are one-command fixes nobody has chosen to run.
5. **Exemplar audits are scheduled but disabled** — `exemplar-auditor-monthly` in
   `shared/hooks/scheduled-monthly.yaml` is `enabled: false` like every entry there. Nothing runs it
   until a project opts in.

---

## Suggested opening move

Pick one item. Before writing code: read its target files, state plainly whether its premise still
holds, and ask the questions whose answers would change the design. Then build, prove the check fails
when it should, and commit.
