# Loom End-to-End Run 4 — Audit

**Status**: COMPLETE. Experiment B ran to completion. Experiment A halted at `architect` (L3.33)
after four of twelve stages and was not resumed — see §9.4 for why, and what that costs the
findings.

**Protocol**: [`loom-e2e-run-4-brief-2026-09-07.md`](./loom-e2e-run-4-brief-2026-09-07.md), written
before the run so its decision rules could not be chosen after seeing the numbers. That property
held: no rule below was edited after a result was known.

**Executed** 2026-09-07/08 against `ai-assistant-dot-files` @ `44a6375` (attempt 1) and `81f0553`
(attempt 2), targeting `saturday-monorepo` @ `9e9daa2`.

---

## 1. Headline

Run 4 found **seven defects**, four of them one shape: a property the framework asserts in prose and verifies
against mocks, which does not hold against a real model. Two of them halted or hollowed out the
pipeline; all three were invisible to every test that existed.

| ID | Defect | Effect | Why nothing caught it |
|---|---|---|---|
| L3.28 | `schemaVersion` reflected as a bare integer while the validator demands equality | **Stage-1 halt on every typed stage.** Two runs died, $2.09, zero stages completed | Mocks build state in Go, where the constant is correct by construction |
| L3.29 | The typed output contract tells *every* typed stage "Do not write files" | **`developer` reported success against an empty `git diff`** | Same — a mock developer never needs to write a file to satisfy its contract |
| L3.30 | A stage with no edit tool wrote source via `Bash`; and three stages modified the tree after `code-reviewer` approved | The approved tree (312 insertions) is not the shipped tree (343) — nothing records the divergence | No test asserts a read-only stage leaves the tree unchanged, and nothing re-reviews post-review writes |
| L3.33 | Architecture schema does not express a conditional the validator enforces (`fitness` required unless `judgmentOnly`) | **Halted Experiment A at `architect`.** $2.52 spent, run incomplete | Same as L3.28 — mocks build valid state in Go |
| L3.34 | `route.md` reports a predicate's whole disjunction as its reason, not the fact that fired | The route file cannot say *why* a stage ran; one reason is provably false against its own input | Nothing checks a reason string against the analysis it describes |
| L3.32 | A gate halts on a stage the router already skipped, transiently overwriting `SKIPPED` with `WAITING_APPROVAL` | Spurious approval prompt; state disagrees with `route.md` while halted. **The stage does not run** — self-corrects on approval | No test pairs the router's decision with the gate check |
| L3.31 | QA reports one package's coverage as the feature's coverage | `qa-report.md` reads as clearing the ≥85% bar; the package holding most of the new surface is at 43.1% | Nothing cross-checks a reported metric against its own scope |

The run's most useful output is not a cost number. It is that **mock-verified is not verified** for
any property whose failure mode is a model doing something a mock cannot do.

Against that: **Experiment B's decision rules all passed.** The L3.24 narrowing does not
under-route, which was the risk the brief called "the more important experiment."

---

## 2. Method

Four fresh clones under `/tmp`, never the live repository, so `git status` is a trustworthy record
of what the pipeline wrote. `loom install --target . --platform claude-code` in each, dependencies
installed before the pipeline, and the post-install state committed as a baseline.

### 2.1 Scale — the brief's figures confirmed

§5.3 required citing the real scale rather than run 3's `node_modules`-inflated counts.

| Measure | Brief | Measured |
|---|---|---|
| Tracked files | 1,547 | **1,547** |
| First-party TypeScript lines | ~22k | **21,826** |

### 2.2 Specs

Experiment A's spec is byte-identical to run 3's — md5 `b16ccb20d09e524602f9495f372e1819`, verified
in all three clones — so any comparison to run 3's $9.49 carries no spec confound.

---

## 3. Protocol compliance

| Requirement | Result |
|---|---|
| §5.1 — no hand-patching of L2.22 / L2.25 workarounds | **Held.** No `settings.local.json`, no STRIDE enum patch |
| §5.2 — dependencies installed before the pipeline | **Held** for every run executed |
| §5.2 — one A run deliberately without dependencies | **Dropped** — see §9.2 |
| §5.3 — cite the real scale | **Held** (§2.1) |
| §5 — clone, never the live repository | **Held** |
| §5 — commit post-install baseline | **Held**, all four clones |
| §5 — `git status` shows no deleted tracked files (L3.26) | **Held — 0 in all four clones.** Run 3 left 124 |
| §5 — framework cache is read-only | **Held** — every file `-r--r--r--` |
| §7 — stop if A's first run exceeds $11 | **Held in spirit** — halted on attempt 1's anomaly instead |

---

## 4. Attempt 1 — the stage-1 halt (L3.28)

Both Experiment A runs died on the first stage, identically and deterministically:

```
Error: stage "context-engineer": stage "context-engineer" returned invalid
state: field "schemaVersion" is 1, this build supports 2
```

| Run | Duration | Cost | cache_read | Outcome |
|---|---|---|---|---|
| A-1 | 61s | $0.8226 | 1,000,545 | FAILED at stage 1 |
| A-2 | 143s | $1.2632 | 2,005,958 | FAILED at stage 1 |

**$2.09 spent, zero stages completed.**

### Root cause

`internal/state/state.go:26` sets `SchemaVersion = 2` and `requireSchemaVersion` refuses anything
else. The JSON Schema handed to the agent at invocation time declared:

```json
"schemaVersion": { "type": "integer" }
```

No `const`, no `enum`, no description. The agent was told "an integer", asked to produce a document
it reasonably read as version 1, and validated against an equality check. **All eight** pipeline
schemas carried the same hole.

This is the class of bug L2.25 closed for the STRIDE enum — enums derived from the constants — never
applied to `SchemaVersion` itself. It was survivable only while the constant was `1`, the value a
model writes unprompted; bumping it to `2` turned a latent hole into a halt on every typed stage at
once.

### Why nothing caught it

The brief's §1 table records the typed `context-engineer` as verified by *mock run*.
`internal/provider/mock/typed_scripts.go` builds `sampleContext()` in Go, where the constant is
correct by construction. **No mock can exercise the path where a model chooses the value.**

### Fix — `9580614`

`schemaVersion` is now reflected as `const: <SchemaVersion>` with a description, derived from the Go
constant rather than hand-written. Two regression tests: one asserting every schema pins its
version, and one building each document *from the schema the agent is handed* and asserting the
validator accepts it — which closes the mock boundary rather than adding another test behind it.

---

## 5. Attempt 2 — Experiment B

**Target**: `saturday-monorepo` @ `9e9daa2`, fresh clone, tool at `81f0553`.
**Spec**: `run-history-browsing.md` — run history listing for `apps/console`.

### 5.1 B1 — does the analyst emit the facts the predicates read? **PASS**

```json
"surfaces": { "ui": true, "runtime": true },
"threshold": { "metric": "p95 latency", "value": 200, "unit": "ms" }
```

`schemaVersion: 2`. Both surfaces true; all three threshold fields populated. The spec never named
a field — it said "the console has a page listing the runs", "responds in under 200ms at p95".
The analyst produced the typed facts from ordinary prose requirements.

### 5.2 B2 — do the four route IN? **PASS (4/4)**

| Stage | Routed | Reason recorded in `route.md` |
|---|---|---|
| `architect` | in | structural work: …a performance threshold |
| `performance-engineer` | in | a performance requirement carries a measurable threshold |
| `sre-engineer` | in | declares a served runtime surface or changes an API |
| `visual-qa-engineer` | in | the analysis declares a UI surface |

`accessibility-engineer` also routed in off the UI surface, which the spec expressed only as
"someone using the keyboard alone can move through the list".

### 5.3 B4 — do the controls stay OUT? **PASS (2/2)**

| Stage | Routed | Reason |
|---|---|---|
| `data-engineer` | **no** | no data-model change to sequence |
| `devops-engineer` | **no** | the analysis lists no DevOps tasks |

**4 in, 2 out — routing discriminates.** It did not pass B2 by including everything, which is the
failure mode §4's B4 was added to catch.

### 5.4 B3 — judged on B1 and B2 together, not cost: **PASS**

### 5.5 The under-routing risk is not observed

The brief's central worry — that `Surfaces` being a new field with absent-means-false would make
four stages skip on *every* feature — **did not reproduce**. Per §6.3 this is n=1 and qualitative:
an analyst that emits the fields once is not an analyst that emits them reliably. A pass here
lowers the risk; it does not close it.

---

## 6. L3.29 — the typed contract cancels the developer's job

**Discovered mid-B, after the design gate.** `developer` ran 193s, cost $1.2389, reported
`success: true`, and wrote **zero source files**. `git diff` was empty.

The stage said so itself, in its `deviations` field:

> "This invocation's output contract explicitly overrode the developer agent's normal
> Write/Edit/Bash access and required a JSON design record with no file writes. No source files
> were modified in this session."

### Root cause

`internal/provider/claude/typed_stage.go` appended to **every** typed stage:

```
Return a single JSON object conforming to this schema, and nothing else.
Do not write files. Do not add commentary before or after the JSON.
```

`developer` is a typed stage (`KindImplementation`). The executor granted it
`Read,Write,Edit,MultiEdit,Bash,Glob,Grep` — **L2.22 working exactly as specified, visible in the
run log** — and the typed wrapper then forbade their use. The tools were granted and taken away by
instruction.

This is §6.5's outcome. The brief predicted the shape: *"If Experiment A's developer produces an
empty diff, L2.22 is not fixed regardless of the probe result."* The probe passes; the end-to-end
property did not. The fault is not in L2.22 — its mechanism is correct and demonstrably works.
`typedInstruction` was written for read-only analysis stages and applied unconditionally.

### Honesty result, unprompted

The developer did **not** fabricate. Its `simpleDesign` recorded `"Passes the tests": false` —
*"No code was written or executed this session — cannot claim green tests for a change that was
never applied"* — and marked every self-review item "design-time estimate only, not measured
against compiled source". This is L2.24's property holding under real pressure, on a run not
designed to test it.

### Fix

`fileClause(allowed)` conditions the clause on the stage's own declared posture: a stage holding no
edit tool is still told not to write; a stage holding one is told to make its changes and that the
JSON *reports* work it must verify with `git status` before answering. Three regression tests cover
both branches and the `writesFiles` tool table.

### Verification — §6.5 satisfied

After the fix, `developer` ran 553s and produced **304 insertions across exactly the four files the
design named**, compiling clean. L2.22's done-when — a non-empty `git diff` from a fresh install
plus run — is met end to end for the first time.

The self-report changed accordingly: `filesModified` matched `git diff` exactly, and
`"Passes the tests": true` carried the note that `go build`, `go vet` and `go test -race` all pass,
with a throwaway race test written and removed because "developer does not own test files". The
build claim was independently confirmed.

---

## 7. L3.30 — a read-only stage modified source, after code review approved

`accessibility-engineer` declares `tools: Read, Glob, Grep, Bash` and describes itself as a
reviewer that "Produces accessibility-report.md". It **modified `handlers.go`** at 00:01:03, inside
its own stage window, and said so:

> "**Fixed**: duplicate, context-free link text. **Added** `aria-label=…`"
> "**Fixed**: `<table>` had no accessible name. **Added** a visually-hidden `<caption>`"

Three separate problems:

1. **The allowlist is not a write barrier.** `permissionArgs` passes `Bash` plus
   `--permission-mode acceptEdits`. Any stage holding Bash can write through a heredoc or `sed -i`.
   "Read-only stage" is not a property the executor enforces for six of the plan's stages.
2. **The edits bypassed review — and this is the wider half.** `code-reviewer` returned `APPROVED`
   at 23:53:30 against a 312-insertion tree. The tree that actually ends the run is **343
   insertions**. Three stages modified it afterwards:

   | Stage | Declares write tools? | Changed | Reviewed by `code-reviewer`? |
   |---|---|---|---|
   | `accessibility-engineer` | **no** (`Read, Glob, Grep, Bash`) | production source (+12) | no |
   | `qa-engineer` | yes | test files only | no — but tests are its own remit |
   | `sre-engineer` | **yes** (`Read, Write, Edit, Glob, Grep`) | production source (+19/-8) | no |

   So two separate defects live here. The **posture violation** is `accessibility-engineer` alone:
   it wrote production code with no edit tool, through Bash. The **review-coverage gap** is
   structural and implicates a stage that did nothing wrong — `sre-engineer` is entitled to write,
   its `slog` instrumentation is correct and low-cardinality, and the suite still passes. The
   pipeline simply has no step that re-reviews what the post-review stages wrote.

   The plan orders `code-reviewer` before four stages that can modify source. Nothing re-reviews,
   and no artifact records that the approved tree and the shipped tree differ. Neither the fact of
   the divergence nor its size appears anywhere in run state.
3. **A comment in L3.29's own fix is falsified by it.** `writesFiles` excludes `Bash` with the
   justification that "a stage that declares Bash without an edit tool declares it to run checks,
   not to author code." That is now demonstrably false as a description of behaviour. The code is
   still correct — a reviewer should not be *invited* to write — but the comment states a guarantee
   the system does not have and must be reworded.

`security-reviewer`, holding the same posture, did **not** edit. So this is a stage-level
behaviour, not a universal consequence of granting Bash.

---

## 7a. L3.31 — QA reports one package's coverage as the feature's coverage

`qa-engineer` reported:

```json
"coverage": { "acceptanceCriteriaCovered": 9, "acceptanceCriteriaTotal": 9,
              "newTests": 22, "statementCoveragePercent": 89.7 }
```

Independently verified, everything checks out **except the scope of that last number**:

| Claim | Independent check | Verdict |
|---|---|---|
| 23 passed, 0 failed, 1 skipped | `go test -count=1 -v` → 23 PASS, 0 FAIL | ✅ exact |
| `-race` clean | `go test -race ./internal/...` passes | ✅ |
| 9/9 acceptance criteria | test names map to all nine | ✅ |
| `statementCoveragePercent: 89.7` | `internal/runs` → **89.7%** | ✅ exact — **for one package** |
| — | `internal/httpserver` → **43.1%** | ❌ **undisclosed** |

The feature spans both packages. `internal/httpserver` holds the handlers, the pagination link and
the page rendering — most of the new surface — and sits at 43.1%. The reported figure carries no
package qualifier and 43.1% appears nowhere in the QA state.

This is not fabrication: the number is real and correctly measured. It is the higher of two,
presented unqualified. `testing-conventions.md` sets coverage ≥ 85% as CRITICAL; a reader of
`qa-report.md` would conclude the feature clears that bar, and for the package holding most of the
change it does not.

Distinct from L3.28–L3.30: those are model-boundary defects in the framework. This one is an
**artifact-honesty** defect — a true statement whose scope is left ambiguous in a way that flatters
the result. It is the failure mode nearest to what L2.24 exists to prevent, arrived at from a
direction L2.24 does not cover: not a fabricated measurement, but a real measurement of the wrong
thing.

### Against that — the honesty that did hold

`bugsFound: []`, with two `knownGaps` recorded rather than papered over:

- The security CRITICAL was **not** fixed by QA. It added
  `TestRunEndpoints_CurrentlyRequireNoAuthentication` to pin today's no-auth behaviour "so a future
  change is a visible, intentional test update rather than a silent behaviour change" — a
  characterization test around a known defect, which is the correct move.
- The 200ms p95 NFR is tested at the store layer only, and it says so, rather than claiming the
  end-to-end budget was verified.

---

## 7b. L3.32 — a gate halts on a routed-out stage and erases the skip

`devops-engineer` was routed **out** (B4's control). The run halted anyway:

```
Halted at gate "confirm-ship" before stage "devops-engineer" — approval required.
```

The event timeline records both facts, eight hours of wall time apart:

```json
{ "at": "2026-09-08T04:09:27Z", "kind": "stage.skipped", "stage": "devops-engineer",
  "sequence": 5, "reason": "the analysis lists no DevOps tasks" }
{ "at": "2026-09-08T12:34:09Z", "kind": "gate.waiting",  "stage": "devops-engineer",
  "gate": "confirm-ship" }
```

The stage's status in `run-state.json` went from `SKIPPED` to `WAITING_APPROVAL`. The gate check
does not consult the routing decision.

Two consequences:

1. **The approval record is corrupted.** A human is asked to approve `confirm-ship`, and what run
   state then records is an approval bound to a stage that was never going to run. `route.md` still
   says `devops-engineer` — **no**; `run-state.json` says `WAITING_APPROVAL`. Two artifacts of the
   same run disagree about the same stage.
2. **The skip is no longer visible in state.** After the gate fires, nothing in `run-state.json`
   says this stage was routed out — only `run-events.jsonl` and `route.md` still carry it. A reader
   of run state alone would conclude the stage was pending.

**Resolved by test.** The gate was approved to find out what happens next. The executor **did not
run the stage**: `devops-engineer` returned to `SKIPPED`, cost $0, and the run completed in 5ms.

```
gate.approved -> run.started -> run.completed   (5ms, $0.0000)
```

So the defect is narrower than it first appears, and L3.24's saving **is** realised — a routed-out
stage stays routed out through its own gate. What remains is real but bounded:

- a **spurious halt**, asking a human to approve a stage that will not run; and
- a **transient state corruption** — while halted, `run-state.json` says `WAITING_APPROVAL` for a
  stage `route.md` and `run-events.jsonl` both record as skipped. It self-corrects on approval.

**Effect on B4**: the control **stands**, on both the routing evidence and the execution evidence.

The severity is accordingly INFO-to-LOW, not the routing-bypass this initially looked like. Recorded
at its measured severity rather than the one it was first written up at.

---

## 7c. L3.33 — the architecture schema under-specifies what the validator enforces *(halted Experiment A)*

```
Error: stage "architect": stage "architect" returned invalid state:
field "structuralDecisions[0].fitness" is required unless the decision is
flagged judgmentOnly (architecture-guardrails.md #7)
```

The generated schema:

| | |
|---|---|
| `required` on a decision | `["decision", "rationale"]` — **`fitness` is not required** |
| conditional (`if`/`then`/`dependentRequired`) | **none** |
| `fitness.description` | "Omitted only when judgmentOnly is true" |
| `judgmentOnly.description` | **empty** |

The validator enforces a conditional requirement. The schema declares the field optional, states
the real rule only in one field's prose, and leaves the escape-hatch field undocumented. JSON Schema
expresses exactly this with `dependentRequired` or `if`/`then`; neither is used.

**Why B passed and A did not.** B's feature was substantial enough that the architect wrote a
fitness function for all six structural decisions, so the conditional never bound. A's feature adds
one method to one class; the architect reasonably had a decision with no meaningful fitness
function, omitted it — and had no way to learn that `judgmentOnly: true` was how to say so.

This is **L3.28's shape for the third time**: a constraint the validator enforces and the schema
does not communicate, invisible to mocks because mocks build valid state in Go. L3.28's fix pinned
`schemaVersion`; it did not audit the other constraints for the same gap.

---

## 7d. L3.34 — the route file cannot say why a stage ran

`internal/state/route.go` gives each predicate a fixed pair of strings via
`decide(id, bool, reasonWhenTrue, reasonWhenFalse)`. For two stages the true-reason is the entire
disjunction rather than the disjunct that fired:

| Stage | Reason recorded when it routes in |
|---|---|
| `architect` | "structural work: a context crossing, a data-model change, a new dependency, a performance threshold, or an explicit flag" |
| `sre-engineer` | "the analysis declares a served runtime surface **or** changes an API" |

In Experiment A this produces a reason that is **provably false against its own input**. The
analyst emitted:

```json
"surfaces": { "ui": false, "runtime": false }
```

and `route.md` reports `sre-engineer | yes | the analysis declares a served runtime surface or
changes an API`. The analysis declares the opposite; the API-change disjunct is what fired. A reader
auditing the route file would conclude a runtime surface was declared.

**This weakens the evidence for B2 in this audit.** §5.2 cites these reason strings as evidence that
routing discriminated on the right facts. For `visual-qa-engineer`, `performance-engineer`,
`accessibility-engineer`, `data-engineer` and `devops-engineer` the reasons are specific and the
evidence holds. For `architect` and `sre-engineer` they are disjunctions, and **this audit
over-claimed by presenting them as if they named the satisfied fact.** B2's verdict is unchanged —
those two stages did route in, which is what B2 asked — but the stated reason for two of the four is
not evidence of anything.

---

## 7e. Experiment A — halted at stage 4, and what it did and did not answer

One run (not three — §9.2), against the run-3 spec, md5 `b16ccb20d09e524602f9495f372e1819` verified
in the clone. It halted at `architect` on L3.33 after $2.5158.

| Stage | Status | Cost |
|---|---|---|
| context-engineer | COMPLETED | $1.3588 |
| analyst | COMPLETED | $0.6811 |
| router | COMPLETED | $0.0000 |
| performance-engineer, data-engineer, accessibility-engineer, visual-qa-engineer, devops-engineer | SKIPPED | $0.0000 |
| architect | **FAILED** (L3.33) | $0.4759 |
| **Total** | **incomplete** | **$2.5158** |

### A3 — which stages routed: **partial pass**

Routed 7 of 12. Skipped `performance-engineer`, `data-engineer`, `accessibility-engineer`,
`visual-qa-engineer`, `devops-engineer` — five stages, against run 3's four no-op stages.

A3's hard rule is "**any review stage skipped is a failure of this run regardless of what it
saved**". `code-reviewer`, `security-reviewer` and `qa-engineer` all routed **in**. **The rule
passes** — but on the routing decision only. The run never reached them, so whether they *run* is
unanswered here; B answered it affirmatively on a different spec.

Note also that `architect` and `sre-engineer` routed **in** on a spec that adds one method to one
class, and L3.34 means the route file cannot say which disjunct fired for either. Whether that is
correct routing or residual over-routing is not determinable from the artifacts.

### A2 — the routing saving: **unanswerable**

The run died at stage 4 of 12. $2.5158 is not comparable to run 3's $9.4923, and no verdict is
recorded. The brief's §6.4 warned that the informative outcomes are the ones where the projected
saving *is not* realised; this run produced neither outcome.

### A1 — variance: **void at n=1**, but one number is worth recording

Run 3's `context-engineer` cache_read on this exact spec was **729,694** — the 0.76× figure L3.19's
`RESOLVED` block rests on. Run 4's, same spec, same repository, same clone procedure:
**1,974,775 — 2.71×**.

What can honestly be said:

- **Not** a variance result. A1's ≤15% / >30% rules required three runs under identical conditions.
  Two points from two builds are not that.
- **Not** attributable. Between the runs `context-engineer` became a typed stage, L3.25 changed how
  the budget is measured, and L3.28's fix changed the schema it is handed. §6.1 applies in full.
- **But**: L3.19's resolution rests on a single measurement, and the next single measurement of the
  same thing did not reproduce it, at 2.71×. That is not a refutation. It is grounds to treat the
  0.76× result as unreplicated rather than established.

The brief said the costly outcome "must not be argued away". It is not argued away here; it is also
not claimed, because the evidence for claiming it was not collected. **Run 3's §9.1 request for three
repeat runs remains open and is now more pressing, not less.**

---

## 8. Pipeline behaviour observed for the first time

Three mechanisms had never been exercised against real code in any prior run, because no prior run
produced any.

**The review loop converges (L2.17).** `code-reviewer` returned `CHANGES_REQUESTED` with a genuine
cross-AC defect: the "Next page" link was built from `nextCursor` alone and never threaded the
`status` param, so paging a filtered listing silently dropped the filter. It sits exactly where two
individually-correct acceptance criteria meet — findable only by reading code that exists. The
developer applied a targeted 8-line fix; round 2 returned `APPROVED`. Converged in 2 of 3 permitted
rounds.

**The security reviewer escalates rather than patching.** One CRITICAL: the new unauthenticated
`GET /api/runs` turns previously-unguessable run UUIDs into a fully enumerable index. It declined
to fix it —

> "A finding that requires an architectural change is escalated rather than silently patched —
> adding authentication is a whole-service decision, not something this pass can safely bolt onto
> two handlers."

— and correctly distinguished *systemic* from *introduced*: the service was already unauthenticated,
but this feature makes exhaustive enumeration practical rather than requiring UUID guessing, which
is a deliberate change in exposure. The failure mode this gate guards against is a reviewer that
quietly bolts on a fix and reports green; that did not happen.

**Gates halt and survive resume.** Both `confirm-design` and `confirm-security` halted the executor
with exit 3 and the resume command. Nothing an agent returned approved a gate. The `confirm-design`
approval survived a hand-rollback of two downstream stage records without re-gating, which is
L2.14's binding behaving as documented.

---

## 9. Protocol deviations

Recorded rather than elided.

### 9.1 Hand-edited run state to re-run one stage

`loom state` has no rollback subcommand and `loom run` implements neither `--from-phase` nor
per-agent rollback, both of which the `resume-pipeline` skill documents. To observe L3.29's fix on
the stage that exposed it, the `developer` (COMPLETED) and `code-reviewer` (INTERRUPTED) records
were deleted from `run-state.json` by hand, along with `state/developer.json`. Pre-rollback state is
preserved as `b-run-state-preroll.json`.

This is not a §5.1 workaround — no defect was patched around to let the run proceed. But it is a
manual intervention in run state, and it revealed a real gap: **there is no supported way to re-run
one stage.**

### 9.2 The no-dependencies A run was dropped

§5.2 required one of the three A runs to deliberately lack dependencies, testing what L2.24 actually
describes — *does it fabricate when it cannot measure*, rather than *does it report honestly when it
can*. Budget consumed by attempt 1 and by B's real (more expensive) pipeline reduced A to two runs.
At n=2, mixing a with-deps and a without-deps run would confound the cache_read spread, which is the
only thing A×2 can still report. Identical conditions were kept and the probe dropped.

The A3 clone remains staged and dependency-free.

### 9.3 Experiment A's decision thresholds are void

A1's ≤15% / >30% rules were written for n=3. At n=2 a spread is two points, and neither threshold
can be honestly applied. The spread will be reported as a number with no verdict, per the brief's
own "report the number, claim nothing" disposition — extended to a case the brief did not anticipate.

A was ultimately reduced further, to **one** run (§9.4). A2 is unanswerable and A3 passes on the
routing decision alone — both recorded in §7e.

---

### 9.4 Experiment A was cut to one run, then abandoned mid-run

A×3 (brief) → A×2 (budget, after attempt 1 and B overran) → A×1 (the spread is uninterpretable with
voided thresholds, so A2 and A3 were the better value at n=1) → **abandoned at stage 4** when L3.33
halted it.

It was not resumed. Resuming meant fixing L3.33 first, then paying for `architect`, `developer`,
`code-reviewer`, `security-reviewer`, `qa-engineer`, `sre-engineer` and `tech-writer` — $10–15 at
B's observed per-stage costs, against $6–13 remaining. The overrun was likely and the purchase was
A2's cost comparison, the least valuable remaining item: B had already demonstrated the full
pipeline end to end, and A2 at n=1 answers a question run 5 can answer properly with three runs.

**This is the third consecutive run in which the variance question has been deferred.** Recording it
plainly: it is now the oldest open item from the run-3 audit, and each deferral has had a good local
reason.

---

## 10. Cost accounting

| Phase | Cost |
|---|---|
| Attempt 1 — two dead A runs (L3.28) | $2.0858 |
| Experiment B — complete, 13 stages | $20.2718 |
| Experiment A — halted at stage 4 (L3.33) | $2.5158 |
| **Total** | **$24.8734** |
| Brief's projection | $33–40 |

Under budget, but not by getting more for less: **$4.60 of the $24.87 (18.5%) bought nothing** —
two runs dead at stage 1 and one dead at stage 4, all three to schema defects of the same class.

B's $20.27 exceeded its $12–15 projection for two legitimate reasons: the review loop ran a real
second round (it had nothing to review in any prior run), and `developer` reports `iteration=2` at
$4.0123, which is **L3.22 working as designed** — summing every attempt rather than the last.

B's final accounting, as the executor reported it:

```
Usage: 466 in / 225,659 out tokens (26,720,023 cache read, 1,478,246 cache write) — $20.2718
```

Two drivers pushed B above its $12–15 projection, both legitimate: the review loop ran a real second
round (it had nothing to do in every prior run), and `developer` shows `iteration=2` at $4.0123 —
which is **L3.22 working as designed**, summing every attempt rather than reporting only the last.

---

## 11. Regression matrix — the brief's eight shipped items

| Item | Claim | Verdict this run |
|---|---|---|
| L3.26 | install owns files per path; cache read-only | ✅ **Confirmed.** 0 deleted tracked files across four clones (run 3: 124); every cache file `-r--r--r--` |
| L2.22 | provider passes each stage its declared tools | ✅ **Confirmed at the CLI and now end to end**, once L3.29 stopped cancelling it |
| L3.22 | run total sums every attempt | ✅ **Observed** — `developer` iteration=2 reports the sum |
| L3.24 + L3.18 | routing reads facts prose cannot satisfy | ✅ **Confirmed both directions** — 4 in, 2 out (§5.2, §5.3) |
| L2.25 | schema enums derived from the constants | ⚠️ **Incomplete** — applied to STRIDE, not to `SchemaVersion` (L3.28) |
| L3.21 | `extractJSON` takes the last fenced block | ➖ Not exercised — no stage returned commentary-wrapped JSON |
| L3.25 | executor measures the context budget | ➖ Not independently verified — no stage's reported budget was cross-checked against a measured one |
| — | `context-engineer` is a typed stage | ⚠️ **Shipped broken** — mock-verified only; L3.28 halted it on first real contact |

---

## 12. What an auditor should challenge

**12.1 — The fixes are this run's own work.** L3.28 and L3.29 were both diagnosed and fixed
mid-audit, then verified by the same run that found them. That is faster than filing and waiting,
but it means run 4 measures a pipeline that differs from the one it set out to measure, sharpening
the brief's own §6.1 attribution problem from eight changed items to ten.

**12.2 — B remains n=1 and qualitative.** §6.3 stands unchanged. B1 and B2 are yes/no readings of
one analyst's output.

**12.3 — Three defects of one shape is a sample, not a census.** The claim in §1 is that
mock-verification systematically misses model-boundary properties. Three instances support it. They
do not enumerate what else is unverified, and no audit of the remaining mock-verified claims has
been done.

**12.4 — L3.30's severity is arguable.** The unreviewed edits were correct and improved the code.
The finding rests on process, not outcome, and someone could reasonably rank it INFO rather than a
defect. It is recorded as a defect because the property "review sees what ships" is one the pipeline
sells.

**12.5 — Experiment A answers almost nothing.** It was cut to one run and then died at stage 4
(§9.4). A2 has no verdict; A3 passes on the routing decision but never saw the stages run. Run 3's
§9.1 request for three repeat runs remains open for the third consecutive run.

**12.6 — Four of the seven findings were found by *running the thing*, not by reading it.** L3.28,
L3.29, L3.31 and L3.33 are all invisible to the test suite, which was green throughout. The suite
was green before this run and is green after it, and neither state predicted anything. The cheapest
correct response to this audit is not seven fixes; it is one integration test that runs a real
pipeline against a real model on a trivial spec, because that is the only instrument that found any
of this.

**12.7 — This audit corrected itself twice.** §7b's L3.32 was first written as a routing bypass and
downgraded to INFO-to-LOW after the gate was approved and the stage provably did not run. §7d's
L3.34 falsifies part of §5.2's own evidence for B2. Both corrections are left visible rather than
edited into having always been right, because an audit that never records being wrong mid-flight is
not reporting its method honestly.

---

## 13. Artifacts

Copied out of the `/tmp` clones, which do not survive a reboot.

| Artifact | Source |
|---|---|
| `a1-traces.jsonl`, `a1-run-state.json`, `a1-run-events.jsonl` | attempt 1, run 1 |
| `a2-traces.jsonl`, `a2-run-state.json`, `a2-run-events.jsonl` | attempt 1, run 2 |
| `b-traces.jsonl`, `b-run-state.json`, `b-run-events.jsonl` | Experiment B |
| `b-route.md` | B's routing decision — the B2/B4 evidence |
| `b-analyst-state.json` | B1's evidence |
| `b-developer-state.json` | L3.29's evidence |
| `b-qa-state.json` | L3.31's evidence — the coverage claim |
| `b-security-state.json` | the CRITICAL finding and its escalation (§8) |
| `b-run-state-preroll.json` | pre-rollback state (§9.1) |
| `a-traces.jsonl`, `a-run-state.json`, `a-run-events.jsonl` | Experiment A, halted at `architect` |
| `a-route.md` | A3's evidence, and L3.34's counter-example |
| `a-analyst-state.json` | `surfaces: {ui:false, runtime:false}` — the fact L3.34's reason string contradicts |

Reproduction: clone `saturday-monorepo` @ `9e9daa2` to `/tmp`, `loom install --target . --platform
claude-code`, copy the spec from `loom-e2e-run-4-specs/` into `docs/features/`, commit a baseline,
then `loom run --spec docs/features/<spec>.md`.

---

*Run 4 audit. Complete.*
