# Loom End-to-End Run Audit — 2026-09-07

Third real end-to-end run of the `loom` executor against a production codebase. Runs 1 and 2 targeted
toy repositories; everything above M0.4 in `docs/roadmaps/BUILD-ROADMAP.md` had been verified against
`--provider mock` only.

**Purpose of this document**: to be audited. Every quantitative claim below carries its source and a
command that reproduces it. Section 9 lists the claims most likely to be wrong and says what would
falsify them. An auditor should start there.

| | |
|---|---|
| Run date | 2026-09-07 (pipeline 04:57–05:19 UTC) |
| Executor | `loom` built from `ai-assistant-dot-files` @ `5b0e9fb` |
| Framework version | v3.7.0 (installed fresh over committed v3.1.0) |
| Target | `saturday-monorepo`, cloned to `/tmp/loom-e2e3` |
| Spec | `docs/features/console-log-filtering.md` — add `getLogsByType()` to `ConsoleLogger` |
| Result | Completed all 12 routed stages; **halted once** at `qa-engineer` |
| Billed | **$9.49** across 12 provider calls, 9.05M tokens |
| Wall clock | ~22 min pipeline + gate inspection |
| Outcome | Feature delivered, builds, 169/169 tests pass |

---

## 1. Method

Run on a clone, never the live repository, so that `git status` is a trustworthy record of what the
pipeline wrote.

```bash
cd ~/Projects/Rieken/ai-assistant-dot-files && go build -o /tmp/loom ./cmd/loom
rm -rf /tmp/loom-e2e3 && git clone -q ~/Projects/Rieken/saturday-monorepo /tmp/loom-e2e3
cd /tmp/loom-e2e3 && /tmp/loom install --target . --platform claude-code
```

The install left 124 tracked files showing as deleted (see §6, L3.23). That state was committed as
`baseline: loom install v3.7.0` so subsequent `git status` output isolates pipeline-authored changes.

Two known blockers were worked around **before** the run, which means neither was exercised and
neither can be reported on:

- **L2.22** — `Edit`/`Write`/`MultiEdit`/`Read`/`Glob`/`Grep`/`Bash` merged into
  `.claude/settings.local.json`, then verified with a single-file write probe (which succeeded, and
  cost $0.46 on its own).
- **L2.25** — the STRIDE uppercase enum appended to the clone's `CLAUDE.md`.

Gates were approved one at a time (`--resume --approve <gate>`) after reading the pending artifacts.

**Deviation from plan.** The run design called for `pnpm install` only *after* the pipeline, so that
no stage could run tests mid-run. The `developer` stage ran `pnpm install` itself at 00:07:18,
creating 758 `node_modules` trees. This was not prevented and changes the interpretation of §4 — see
§9.2.

---

## 2. Scale — the premise was wrong, and it matters

The run brief described the target as ~1.4M lines across 5,130 TypeScript files. That count included
`node_modules`.

```bash
cd /tmp/loom-e2e3
git ls-files | wc -l                                                     # 1547
find . -name '*.ts' -not -path '*/node_modules/*' | wc -l                # 245 (clone)
find . -name '*.ts' -not -path '*/node_modules/*' -exec cat {} + | wc -l # 21826
```

| Measure | Live repo | Clone (measured) | Brief claimed |
|---|---|---|---|
| Tracked files | 1,547 | 1,547 | — |
| TS files, excl. `node_modules` | 544 | 245 | 5,130 |
| TS lines, excl. `node_modules` | 48,159 | 21,826 | ~1,400,000 |
| TS lines, incl. `node_modules` | 2,077,330 | — | — |

The clone is smaller than the live repo because the `apps/saturday-go-cli` submodule is
uninitialized and ~49 uncommitted files are absent.

**Consequence**: this run tested **~770×** run 2's source (1,547 files vs 2; ~22k lines vs 26), not
54,000×. The primary question's decision rule is unaffected — the rule was about the *direction and
magnitude* of cache_read growth, and 770× is more than enough to detect a linear source term. But
any statement of the form "verified at 1.4M lines" would be false, and 770× is arguably the more
honest axis regardless: `node_modules` is gitignored and no stage ever reads it.

---

## 3. Primary question — does cost scale with codebase size?

`BUILD-ROADMAP.md` L3.19 claimed loom's cost scales with agent count, not codebase size, and used
that to argue a repo map / source graph would optimize the wrong axis. The claim rested on one run
against a 26-line repo.

**Decision rule, fixed in advance:**

- `context-engineer` cache_read within ~2× of 964,590 → source discovery is not the cost driver;
  L3.19 holds and the repo-map idea closes out.
- Growth ≥ 3× → source scale is a real cost axis; `aider-repo-map` / `repomix-codebase-packing`
  become the next item.

### Result

**`context-engineer` cache_read = 729,694 — 0.76× the baseline. It decreased.**

| Metric | Run 2 (26 lines) | Run 3 (this) | Δ |
|---|---|---|---|
| context-engineer cache_read | 964,590 | **729,694** | **0.76×** |
| context-engineer cache_write | 90,846 | 83,901 | 0.92× |
| context-engineer cost | $0.99 | $0.88 | 0.89× |
| per-stage cache_write range | 74k – 97k | 73k – 106k | flat |
| mean cost per call | $0.71 | $0.79 | 1.11× |
| cache_read share of all tokens | 84.5% | 89.0% | flat |
| first-party source | 26 lines | ~22k lines | ~770× |

The outcome lands inside even the *no-growth* case, not merely inside the 2× band.

### Separating source-driven from framework-driven growth

The brief required these be reported separately, and that any inability to separate them be stated
rather than papered over. They separate cleanly here, because they moved in opposite directions.

```bash
for d in .claude/agents .claude/skills .claude/rules; do
  n=$(find -L $d -type f 2>/dev/null | wc -l)
  b=$(find -L $d -type f -exec cat {} + 2>/dev/null | wc -c)
  echo "$d: $n files, $b bytes (~$((b/4)) tok)"
done
```

| Framework surface | v3.1.0 (committed) | v3.7.0 (installed) | Δ |
|---|---|---|---|
| `.claude/agents` | 39 files, ~55,772 tok | 40 files, ~61,625 tok | +10% |
| `.claude/skills` | 72 files, ~78,732 tok | 76 files, ~98,798 tok | +25% |
| `.claude/rules` | 13 files, ~13,746 tok | 14 files, ~16,676 tok | +21% |
| `CLAUDE.md` | ~1,983 tok | ~1,983 tok | — |
| `ARCHITECTURE_RULES.md` | ~3,737 tok | ~3,737 tok | — |
| `DOMAIN_DICTIONARY.md` | ~3,340 tok | ~4,632 tok | +39% |

- **Source-driven growth: none detectable.** Source grew ~770×; cache_read fell 24%.
- **Framework-driven growth: real, and the only input that increased.** The framework surface grew
  10–39% between the two runs, and cache_read *still* fell.

The confound therefore cannot rescue the alternative hypothesis: it points the wrong way. If
framework surface is the dominant term and it grew while the total fell, the residual source term is
at best negligible and at worst negative.

**Mechanism**: no stage reads the repository broadly. It reads the files the context manifest pins —
four files and three rules documents in this run. Repository size changes what *could* be read, not
what is.

**Verdict (applying the rule as written, unsoftened): do not build `aider-repo-map` or
`repomix-codebase-packing`.** They optimize source-discovery cost, measured here as near zero, and
would add a per-run indexing pass to a system that spends ~90% of its tokens re-caching a prompt
prefix. The two levers L3.19 already names — sharing the cached prefix across stages, and making a
stage's decline cheap — remain the only ones this evidence supports. L3.24 (below) adds a third: not
invoking the stage at all.

L3.19 has been updated in place with a `RESOLVED 2026-09-07` block recording this.

---

## 4. Per-stage cost and cache accounting

Summed from `generate_content` spans in
`.claude/feature-workspace/console-log-filtering/traces.jsonl`.

| Stage | Output | cache_read | cache_write | Cost |
|---|---:|---:|---:|---:|
| context-engineer | 10,429 | 729,694 | 83,901 | $0.88 |
| analyst | 4,700 | 416,453 | 77,621 | $0.66 |
| architect | 7,351 | 517,500 | 81,046 | $0.75 |
| performance-engineer | 2,573 | 672,193 | 76,914 | $0.70 |
| developer | 7,877 | 2,592,156 | 106,057 | $1.53 |
| code-reviewer | 6,234 | 743,259 | 87,929 | $0.84 |
| security-reviewer | 2,829 | 412,723 | 77,403 | $0.63 |
| qa-engineer *(failed to parse)* | 4,159 | 517,665 | 79,466 | $0.69 |
| qa-engineer *(retry)* | 4,185 | 621,621 | 82,345 | $0.74 |
| visual-qa-engineer | 3,068 | 208,449 | 72,933 | $0.55 |
| sre-engineer | 2,857 | 399,802 | 77,813 | $0.63 |
| tech-writer | 4,858 | 755,927 | 96,356 | $0.88 |
| **Total — 12 calls** | **61,120** | **8,587,442** | **999,784** | **$9.49** |

cache_read was **89.0%** of all tokens moved.

**Per-stage floor**: $0.55–0.88, against run 2's $0.50–0.74. Essentially flat despite 770× the
source. The one outlier, `developer` at 2.59M cache_read and $1.53, is explained by tool-use
iterations — it ran `pnpm install` and the 169-test suite — not by source discovery. This is
corroborating evidence for §3's conclusion: the expensive stage is the one that *did the most
tool calls*, not the one that saw the most code.

For calibration: a single trivial one-file write probe against this repo, outside loom entirely,
cost **$0.46**. That is the ambient floor before loom does anything.

---

## 5. Delivery verification

The developer wrote real files, and — unlike run 2 — the qa-engineer's claims are true.

```
packages/saturday-core/src/utils/console-logger.ts        |  4 ++
packages/saturday-core/tests/utils/console-logger.spec.ts | 59 +++++
packages/saturday-core/README.md                          | 13 +++
```

Implementation:

```typescript
public getLogsByType(type: string) {
  return this.logs.filter((log) => log.type === type);
}
```

**L2.24 check (run 2's worst finding: qa-engineer reported passing tests for a file it never
created).** qa-engineer's state document claimed `testResults.passed: 169` and
`statementCoveragePercent: 86.08`. Independently verified:

```bash
cd /tmp/loom-e2e3
pnpm --filter @orieken/saturday-core build   # Build success
pnpm --filter @orieken/saturday-core test    # Test Files 26 passed / Tests 169 passed
                                             # All files 86.08 % Stmts
grep -c getLogsByType packages/saturday-core/dist/index.d.ts   # 1
```

**Exact match on both numbers.** `console-logger.ts` reports 100% coverage;
`console-logger.spec.ts` holds 8 tests.

Note the causal chain, because it limits how far this result generalizes: qa-engineer could report
real numbers **because the developer had already installed `node_modules` and run the suite**. This
is evidence that qa-engineer reports honestly *when it has real data*, not evidence that it declines
to fabricate when it does not. Run 2 tested the latter; this run did not. See §9.2.

**Analyst file resolution (L2.21)**: the analyst resolved exact paths in a 1,547-file repo —
`packages/saturday-core/src/utils/console-logger.ts`, the existing spec file, and the correct line
in `src/index.ts` — and correctly identified that `type` is an open `string` rather than a closed
union. No hedging. L2.21 does not reproduce at this scale.

---

## 6. New findings

Filed as roadmap items L3.21–L3.25 in `docs/roadmaps/BUILD-ROADMAP.md`.

### L3.21 — `extractJSON` rejects a valid state document preceded by one sentence *(halted the run)*

The run died at `qa-engineer`:

```
Error: stage "qa-engineer": agent did not return a JSON state document
— got: Now producing the final QA state JSON.
```

The JSON was complete, valid and schema-conformant. It was preceded by one sentence of prose before
the fence. `extractJSON` accepts raw JSON, and `unfence` accepts a response that is *entirely* one
fenced block (`strings.HasPrefix(text, "```")`), so a leading sentence fails the prefix test.

- **Source**: `internal/provider/claude/typed_stage.go:66-92`
- **Nondeterministic**: an unmodified `--resume` re-ran the identical stage and it parsed first try.
- **Billed anyway**: $0.69 for the failed attempt.
- **The prompt already forbids it**: `typed_stage.go:29` writes *"Do not write files. Do not add
  commentary before or after the JSON."* The model disregarded both halves in the same run, so the
  instruction is not load-bearing and the parser cannot treat it as a guarantee.
- **Fix**: take the *last* fenced block. This preserves the property the current strictness exists
  for (not adopting a schema example the agent quoted back — a quote precedes the real answer, never
  follows it).

### L3.22 — The run summary under-reports what the run cost

`loom`'s completion line and `loom memory runs` both report **$8.7978**. Summing every
`generate_content` span in `traces.jsonl` gives **$9.49**. The $0.69 delta is exactly the failed
qa-engineer attempt.

The error is silent, always in the same direction, and grows with how badly a run goes — which is
when the figure is most likely to be read. It propagates into the persisted memory store, so it will
contaminate any future analysis built on run records (L3.13 explicitly wants to derive quality
metrics from execution).

- **Source**: `internal/orchestrator/executor.go`, `internal/memory/`

### L3.23 — Install replaces committed files with writable symlinks into a shared cache

`loom install --target .` replaced **124 committed files** (`.claude/agents`, `.claude/skills`,
`.claude/rules`, plus `ARCHITECTURE_RULES.md` and `DOMAIN_DICTIONARY.md`) with symlinks into
`~/Library/Caches/loom/v3.7.0/shared/`. It backed each up and printed that it had — **after the
fact**. No prompt, no warning that the targets were tracked files with local content.

```
lrwxr-xr-x  DOMAIN_DICTIONARY.md -> ~/Library/Caches/loom/v3.7.0/shared/DOMAIN_DICTIONARY.md
-rw-r--r--  ~/Library/Caches/loom/v3.7.0/shared/DOMAIN_DICTIONARY.md    # writable
```

Two consequences:

1. **Latent, and nearly realised.** The symlink target is writable and shared by every project
   installed from that cache. The analyst emitted Developer Task 4: *"Add 'ConsoleLogger', 'captured
   log entry', and 'log entry type' to DOMAIN_DICTIONARY.md."* Had the developer complied, it would
   have written into the shared cache and corrupted the framework for every project on the machine.
   The developer declined. **The cache survived on one agent's judgment, not on any property of the
   system.** Verified intact by checksum before and after (`/tmp/loom-cache-baseline.md5`).
2. **Immediate.** The project's own 13,363-byte `DOMAIN_DICTIONARY.md` was shadowed by the
   framework's generic 18,530-byte default — while `design-principles.md` §6 requires every domain
   term to match that file. The install silently changed what the rules are checked against.

`install` already knows how to decline (`skipped CLAUDE.md (already exists)`); it does not apply
that logic to these two files.

- **Source**: `internal/install/`, `cmd/loom/install.go`

### L3.24 — Two UI-only stages are non-skippable, and one boilerplate NFR routes in two more

The feature was a three-line synchronous array filter with no UI, no I/O, no network. The router
skipped `data-engineer`, `accessibility-engineer` and `devops-engineer` correctly, then ran four
stages that reported having nothing to do, for **$2.63**:

| Stage | Cost | Routed in because | Reported |
|---|---:|---|---|
| `visual-qa-engineer` | $0.55 | "always runs; not skippable by routing" | `UNCONFIGURED` — "no visual QA surface exists" |
| `sre-engineer` | $0.63 | "always runs; not skippable by routing" | no availability or latency SLI applies |
| `architect` | $0.75 | "a performance threshold" | no structural decision needed |
| `performance-engineer` | $0.70 | "a performance requirement carries a measurable threshold" | "Not applicable" ×4 |

Two distinct defects:

- **Inconsistency.** `accessibility-engineer` was skipped with the reason *"no accessibility
  requirement, so the analysis describes no UI surface"*. The same fact skips one UI-only agent and
  cannot skip `visual-qa-engineer`, which is hard-coded as always-runs.
- **Over-routing on boilerplate.** `architect` and `performance-engineer` both triggered on a single
  NFR line the analyst writes every time (*"O(n) over the captured logs array... no I/O"*). This is
  L3.18's failure mode with a different trigger: L3.18 counts list items; this counts the presence
  of an NFR heading. Route on an actually-measurable threshold (a number, a budget, an SLO).

- **Source**: `internal/state/` (routing predicates), `internal/orchestrator/plan.go`

### L3.25 — `context-engineer` reports a token budget ~7× under, with arithmetic

`context-engineer.md` §7 states:

> Recomputed precisely: 84 + 164 + 34 + 10 + 376 + 238 ≈ **906 tokens** [...] ≈ **1,350 tokens
> total** [...] Status: OK (≈1,350 tokens is ~1.1% of the Analyst tier budget — no cuts needed).

Every term derives from a ~2-tokens-per-line rule that holds for nothing in the list:

| File | Lines | Bytes | Real (~bytes/4) | Manifest claimed |
|---|---:|---:|---:|---:|
| `ARCHITECTURE_RULES.md` | 188 | 14,949 | ~3,737 | 376 |
| `DOMAIN_DICTIONARY.md` | 119 | 18,530 | ~4,632 | 238 |
| `console-logger.ts` | 42 | 916 | ~229 | 84 |

Real total ≈ **9,100 tokens** against a claimed 1,350 — roughly **7× under**.

The presentation is as much the defect as the number: a per-file breakdown, a recomputation, a
percentage and a Status line, all resting on a per-line rate that is wrong by an order of magnitude
for prose. A budget 7× under reports `OK` right up to the point it overflows. The estimate should be
computed by the executor from the pinned files (`bytes/4`), not asserted by the agent — it is
arithmetic over known quantities.

- **Source**: `shared/agents/context-engineer.md`, `internal/orchestrator/executor.go`

---

## 7. Known-issue regression matrix

| Item | Status | Evidence |
|---|---|---|
| L2.21 — analyst hedges on file paths | **no repro** | Exact paths and line numbers resolved in a 1,547-file repo. |
| L2.22 — `claude -p` denies writes | *untested* | Worked around in setup before the run. |
| L2.23 — typed stages told "do not write files" | **reproduces** | `typed_stage.go:29` sends it; developer wrote 4 files anyway. Harmless — it wrote the right ones. |
| L2.24 — nothing verifies a claimed file exists | **no repro** | qa-engineer's 169 passed / 86.08% verified exact. See §9.2 for the caveat. |
| L2.25 — STRIDE enum mismatch | *untested* | Worked around in setup before the run. |
| L2.26 — rejected payload is discarded | **partial** | Payload is now echoed in the error, but truncated at 800 chars; the tail (`knownGaps`) was lost. |
| L3.16 — `run.started` once per invocation | **reproduces** | Appears 4× across the run in `loom state timeline`. |
| L3.17 — `--provider` not persisted across resume | *untested* | `--provider` never passed, per the run plan. |
| L3.18 — routing predicates count list items | **no repro** | "DevOps Tasks: None" read correctly, stage skipped. A variant fired elsewhere — see L3.24. |
| L3.20 #1 — failing CLI reports empty stderr | **no repro** | The failing stage produced a real, actionable error. |
| L3.20 #2 — `route.md` titled from payload | **fixed** | Titled `# Delivery Route: console-log-filtering`, correct. |
| L3.20 #3 — `confirm-ship` gates a skipped stage | **reproduces** | Halted before `devops-engineer`, which the router had skipped. |
| L3.20 #4 — untyped artifacts keep their fence | **reproduces** | `tech-writer.md` persisted wrapped in a ```` ```markdown ```` fence. |
| L3.20 #5 — feature archive holds no artifacts | **reproduces** | `docs/features/console-log-filtering/` holds only `run-state.json`, `run-events.jsonl`. |

---

## 8. Artifact quality — subjective assessment

Recorded separately from the measured findings above because it is judgment, not measurement.

Artifact quality was **high**, and higher than the cost analysis might suggest. The context manifest
correctly identified the bounded context, noted that `src/index.ts` already wildcard-re-exports the
class (so no export change was needed), and flagged that `type` is an open string. The security
review's STRIDE walk was accurate and appropriately dismissive. The code reviewer's `APPROVED` was
justified. The performance and SRE reports were correct — they were simply unnecessary.

The recurring defect is not wrongness but **misplaced confidence in the one number nobody checks**
(L3.25), and **stages producing excellent reasoning about why they had nothing to do** (L3.24).

One further contract defect worth noting, below roadmap threshold: `security-report.md` frontmatter
carries `bounded_context: ""` and `files_touched: []` despite the body naming both correctly. Any
downstream consumer reading frontmatter rather than prose sees nothing.

---

## 9. What an auditor should challenge

The claims below are the weakest in this document. Each is stated with what would falsify it.

**9.1 — The primary result rests on a single run per condition.** n=1 vs n=1. cache_read is not
deterministic across identical runs, and no variance estimate exists for either baseline. A 0.76×
ratio is comfortably inside the decision band, but if run-to-run variance is itself ±30% the
measurement discriminates less than it appears to. *Falsified by*: three repeat runs on the same
spec showing a spread wide enough to cross the 2× line. **This is the single most load-bearing
untested assumption in the report.**

**9.2 — The L2.24 "no repro" is weaker than it looks.** qa-engineer reported true numbers because
the developer had already installed dependencies and run the suite, so real data was available. The
run therefore tested "does qa-engineer report honestly when it can measure", not "does qa-engineer
fabricate when it cannot" — which is the behaviour L2.24 actually describes. *Falsified by*: a run
where dependencies are genuinely absent at qa time. Do not treat L2.24 as closed on this evidence.

**9.3 — One spec, one shape of work.** A ~3-line pure function added to an existing file following
an established local pattern. Deliberately matched to run 2 for comparability, which is the right
call for the cost question, but it exercises none of: multi-file changes, new packages, schema
changes, cross-context work, or anything the architect or data-engineer would meaningfully engage.
The §3 conclusion is about *source-discovery* cost and should not be read as covering feature
complexity, which was never varied.

**9.4 — Framework and source growth were separated by direction, not by controlled isolation.** The
argument in §3 is that the confound points the wrong way, not that it was held constant. A run at
v3.1.0 against the large repo would isolate the source term properly; it was not performed.

**9.5 — Three known issues are untested, not passed.** L2.22, L2.25 and L3.17 were worked around or
never exercised. They must not be scored as fixed on this run's evidence.

**9.6 — The scale figure is smaller than the brief assumed.** ~770×, not 54,000× (§2). Anyone citing
this run should cite 1,547 files / ~22k lines.

**9.7 — L3.23's severity is argued, not demonstrated.** The shared cache was *not* corrupted. The
claim is that the system permits it and an agent was instructed to do it, which is a near-miss
rather than an incident. An auditor may reasonably rate it lower. The counter-argument is that the
only thing that prevented it was a model's discretionary choice.

---

## 10. Artifacts and reproduction

The run's telemetry is committed alongside this report in
[`loom-e2e-run-2026-09-07/`](./loom-e2e-run-2026-09-07/), so §3 and §4 remain checkable after the
clone is gone. Scanned for credentials before committing; none present.

| Item | Location |
|---|---|
| Raw telemetry (span-level cost/usage) | [`loom-e2e-run-2026-09-07/traces.jsonl`](./loom-e2e-run-2026-09-07/traces.jsonl) |
| Run event timeline | [`loom-e2e-run-2026-09-07/run-events.jsonl`](./loom-e2e-run-2026-09-07/run-events.jsonl) |
| Run state (stages, approvals, digests) | [`loom-e2e-run-2026-09-07/run-state.json`](./loom-e2e-run-2026-09-07/run-state.json) |
| Routing decisions | [`loom-e2e-run-2026-09-07/route.md`](./loom-e2e-run-2026-09-07/route.md) |
| Original workspace (ephemeral) | `/tmp/loom-e2e3/.claude/feature-workspace/console-log-filtering/` |
| Roadmap changes | branch `roadmap/third-real-run`, commit `1ffc842` |

Markdown artifacts (`analysis.md`, `context-engineer.md`, `security-report.md`, etc.) were **not**
copied — they are large, and every claim this report makes about them is quoted inline. An auditor
wanting the full text must re-run; the quoted fragments in §5–§8 are the evidence of record.

Per-stage accounting is reproduced by iterating `resourceSpans[].scopeSpans[].spans[]` in
`traces.jsonl`, selecting spans whose `name` starts with `generate_content` and which carry
`loom.usage.cost_usd`, then reading `loom.stage.id`, `gen_ai.usage.output_tokens`,
`loom.usage.cache_read_tokens`, `loom.usage.cache_creation_tokens` and `loom.usage.cost_usd`.

The clone itself is under `/tmp` and will not survive a reboot; the telemetry above was copied into
this repository for that reason.

---

*Run and compiled 2026-09-07 against `ai-assistant-dot-files` @ `5b0e9fb`, framework v3.7.0.*
