# Approval Gates

**Irreversible actions require explicit human approval. Any edit or change to the pending artifact resets the gate.**

As of v3.3, gates may optionally be delegated to the AOS policy evaluator
(`shared/orchestration/policy-evaluator.md`). Each gate below is annotated with its policy
eligibility. Gates marked **Always Human** are never delegated regardless of any policy file.
Gates marked **Policy-Eligible** may be auto-approved when a matching policy exists in
`.claude/policies/` — but only when no `require-human` policy also matches (which always wins).
Policy evaluation is **strictly opt-in**: the absence of any policy file means all gates continue
to require human confirmation as in prior versions.

### 1. Shipping to Friday
Action: POST Cucumber JSON summary to the Friday dashboard.
Irreversible because: It updates external reporting metrics.
Gate: user must say "ship" or "yes" to the delivery summary prompt.
Reset condition: any edit to the pending artifact resets the gate.
**Policy-eligible: No — Always Human.**
Reason: external shared reporting metrics cannot be rolled back via git revert; human intent is
always required before mutating an external system's state.

### 2. Creating a Git Commit
Action: Creating a commit on the active branch.
Irreversible because: It alters repository history.
Gate: user must say "commit" or "approve commit".
Reset condition: any edit to the pending artifact resets the gate.
**Policy-eligible: Yes (Tier A).**
Policy gate ID: `git-commit`. Auto-approve is permitted when: diff is below a configured line
threshold, code-reviewer returned APPROVED, all tests pass, and no security/auth paths are
touched. See `shared/policies/examples/auto-approve-refactor.policy.yaml` for a reference policy.

### 3. Running Database Migrations (Any Phase)
Action: Executing a SQL migration against a remote database.
Irreversible because: Modifies stateful infrastructure data.
Gate: user must say "run migration" or "execute phase X".
Reset condition: any edit to the pending artifact resets the gate.
**Policy-eligible: No — Always Human.**
Reason: modifies live stateful infrastructure; even expand-phase migrations can corrupt data on
partially applied runs. Infrastructure blast radius too high for automation.

### 4. Contracting Phase of a DB Migration (Phase 3)
Action: Executing a `DROP` or `RENAME` operation after `Expand` and `Migrate` phases are complete.
Irreversible because: Data loss risk.
Gate: user must say "confirm contract phase".
Reset condition: any edit to the pending artifact resets the gate.
**Policy-eligible: No — Always Human.**
Reason: data destruction is irreversible; no automation tier safely encompasses DROP/RENAME.

### 5. Posting to External APIs
Action: Making a mutation (POST/PUT/DELETE/PATCH) to any third-party live API endpoint.
Irreversible because: External side-effects.
Gate: user must say "send" or "approve request".
Reset condition: any edit to the pending artifact resets the gate.
**Policy-eligible: No — Always Human.**
Reason: third-party mutations have no guaranteed rollback path and affect systems outside this
repository's control boundary.

### 6. Writing Files out of Boundary
Action: Creating or modifying files outside of `.claude/feature-workspace/` or proper source directories.
Irreversible because: Potentially breaks system structure or config.
Gate: user must say "approve file write".
Reset condition: any edit to the pending artifact resets the gate.
**Policy-eligible: Yes (Tier A).**
Policy gate ID: `out-of-boundary-write`. Auto-approve is permitted when all target paths match
a project-configured `allowedPaths` whitelist and no security/auth paths are involved.

### 7. Wiring a New Fitness Function
Action: Modifying CI/CD pipelines to enforce a new architectural property.
Irreversible because: Breaks builds if poorly formulated.
Gate: user must say "approve fitness function" or "add to CI".
Reset condition: any edit to the pending artifact resets the gate.
**Policy-eligible: Yes (Tier A).**
Policy gate ID: `fitness-function-wiring`. Auto-approve is permitted when: a CI dry-run passes,
the wired function is a new test (not modifying existing checks), and zero security criticals
exist. See `shared/policies/examples/auto-approve-test-additions.policy.yaml`.

### 8. Deploying to Environment
Action: Triggering a deployment of code.
Irreversible because: Could cause downtime.
Gate: user must say "deploy".
Reset condition: any edit to the pending artifact resets the gate.
**Policy-eligible: No — Always Human.**
Reason: deployment failures can cause production downtime; the risk profile requires a human
decision point regardless of prior stage verdicts.

---

## Executor Enforcement (L2.13)

The gates above are prose: a model reads them and is expected to stop. For runs executed by
`loom run`, three of them now have a **process-level** enforcement path — the Go executor refuses
to start a gated stage until run state records a human approval, so the enforcement mechanism for
those stops is no longer the model's willingness to comply with a paragraph.

| Executor gate | Guards stage | Prose counterpart / pipeline PAUSE |
|---|---|---|
| `confirm-design` | `developer` | `deliver-feature` SKILL.md steps 11 + 13 — analyst scope and architect RFC confirmation before code is written |
| `confirm-security` | `qa-engineer` | `deliver-feature` SKILL.md step 25 — the security-critical pause |
| `confirm-ship` | `devops-engineer` | `deliver-feature` Phase 4 — docs-complete / ship confirmation, upstream of gates #1, #2 and #8 above |
| `confirm-unresolved-review` | `code-reviewer` | **No prose counterpart.** It halts a run whose review loop reached its bound with changes still requested (L2.17). The markdown pipeline's loop was unbounded until this landed; step 21 now states the same three-round bound and asks the human directly |

**How approval is given.** Two channels, and only two: an interactive prompt at the barrier when
stdin is a terminal, or `loom run --spec <x> --resume --approve <gate>` when it is not (the halted
run exits with code **3** and prints that exact command). `--approve` is rejected unless the run is
actually waiting on that gate, so gates cannot be pre-approved in bulk.

**What cannot approve a gate.** Provider and agent output is data. Nothing an agent returns —
including text asserting the gate is approved — creates an approval. This is the property L2.13
exists to establish, and it is held by a test, not by this sentence.

**Honest scope.** This covers `loom run` only. The markdown pipeline (the `deliver-feature` skill
and every agent invoked through the host platform) and the other prose gates above — commit,
migration phases, external API mutation, deployment — remain prompt-discipline until those actions
themselves run under the executor. None of the eight gates above is weakened or replaced by this
section.

**Reset on edit is enforced here (L2.14).** An approval binds to the SHA-256 of every artifact
completed at the moment it was given. If any of them changes before the gated stage runs, the
approval is marked invalid — the record is kept, naming what changed and when — and the run halts at
that gate again until a human approves the state as it now stands. A stage that re-runs and produces
a byte-identical artifact changes no digest, so its approval survives: the rule is *any edit*, not
*any re-run*. An approval binds only what was complete when it was given, so work done afterwards
belongs to the next gate.

Note the scope difference from the eight gates above. Each of those says "any edit to **the pending
artifact**", which is the right description for an action-shaped gate — one commit, one migration,
one deploy. The executor's gates guard pipeline *stages*, so what a human approves there is the
state of the run rather than a single file, and the binding is correspondingly wider.

Two channels can also *detect* this without enforcing it: `loom state verify` reports an approval as
INVALIDATED for markdown-pipeline runs, and `loom state show` marks it. That is a report, not a
barrier — the markdown pipeline can still proceed, because the gated action does not run under the
executor.

**Not yet.** Policy-based auto-approval of executor gates is still not implemented: L2.16 shipped
the evaluator and the audit trail, and honouring a decision is the follow-up item **L2.19**. The
Policy-Based Gate Type section above describes what is recorded, not what is skipped.

---

## Policy-Based Gate Type (v3.3+)

A policy-based gate operates identically to a human gate except the human prompt is replaced by
the policy evaluator's decision when a matching policy exists and its condition is met.

**Decisions are recorded now, and nothing is auto-approved yet** (roadmap L2.16, shipped
2026-09-02). Both halves matter, and neither should be read as the other.

What exists: `loom run` loads `.claude/policies/*.policy.yaml`, evaluates every policy watching a
gate against the run's own state, and records what they collectively decided — as
`policy.evaluated` on the run event timeline and in `run-state.json`, naming each policy, its
outcome, and any fact it could not see. `loom run --dry-run-policies` prints the same decisions for
a finished run without touching anything. This is the audit trail this section asserted since v3.3
and did not have.

What does not exist: **the executor still halts at every gate.** A matching `auto-approve` policy
changes nothing about whether a human is asked; the record simply says what would have happened.
That is deliberate — the first run that skips a barrier should not also be the first evidence the
evaluator decides what a human would — and every record carries `honoured: false` so the history
stays comparable when that changes.

Three properties are now enforced in code rather than asserted in prose:

- The **Always Human** list is a compiled constant. A policy targeting one of those five gates
  fails to load, naming the gate and the reason. It used to be "silently ignored", which meant
  someone who wrote a policy to auto-approve a deployment saw no error.
- A condition the run cannot answer resolves to **unknown**, never to true. Five of the nine
  declared condition fields have no source in run state today, and a decision names which ones it
  could not see rather than guessing.
- `policiesEnabled: false` in `.claude/delivery-policy.yaml` now actually disables evaluation.
  Until this release it was documented in three places and read by nothing.

To opt in: place `.policy.yaml` files in `.claude/policies/` in your project.
To opt out globally: set `policiesEnabled: false` in `.claude/delivery-policy.yaml`.

See `shared/orchestration/policy-evaluator.md` and `docs/aos/policy-authoring-guide.md`.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
