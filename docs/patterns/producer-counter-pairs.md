# Producer / Counter Pairs

Every role that produces something is paired with an independent role whose only job is to check it.
The counter role reports findings. It never edits the producer's work. A human, or the producer on a
retry, acts on the findings.

This is the maker/checker principle from finance and security, applied to agents. The inventory of
pairs is `docs/aos/governance-pairs.md`. The design origin is
`docs/aos/AOS_Governance_Design_Pack/01-Governance-Checks-and-Balances.md`. This file is the pattern.

Terms follow `DOMAIN_DICTIONARY.md`: **Producer / Counter Pair** and **Counter Agent**. Older docs say
"counter-auditor" and "opposing-force pair". Those mean the same thing, and new text uses the
dictionary terms.

---

## Context

An agent that checks its own work shares its own blind spots and its own incentive to finish. A model
that wrote a spec will read that spec the way it meant it. A reader with fresh context reads it the way
the next agent will. The check is only worth anything if it is separate from the thing it checks.

## Structure

| Property | Rule |
|---|---|
| **Independent** | The counter agent has its own prompt and its own context. It does not inherit the producer's reasoning |
| **Read-only** | The counter agent holds no write tool and no egress tool. In this repo, `Read`, `Glob` and `Grep` (`repo-doc-auditor` holds only `Read`) |
| **Report-only** | It outputs findings. It does not fix, rewrite or approve |
| **Acted on by someone else** | A human, or the producer on a retry with the specific findings attached |

### Two kinds of pair

| Kind | What the counter does | Pairs in `governance-pairs.md` |
|---|---|---|
| **Audit pair** | Inspects one artifact the producer made, and reports defects | 1–11: `context-engineer` / `context-auditor`, `create-ki` / `knowledge-auditor`, rule author / `rule-auditor`, `security-reviewer` / `privacy-auditor`, and so on |
| **Opposing-force pair** | Pushes the other way on the same resource, so neither grows without limit | 12–15: `memory-expansion` / `memory-compression`, `learning-engine` / `forgetting-engine`, `cost-optimizer` / `quality-optimizer`, orchestrator / `scheduler` |

An audit pair checks quality. An opposing-force pair balances a trade-off, such as cost against
quality, or remembering against forgetting. Keep them apart when you add a new one, because they fail
differently.

### Layered checks

A counter agent is the middle layer, not the only one:

1. **Code checks what code can check.** `validate-artifact` checks headings and content rules against a
   contract. The `repo-doc` Verifier checks that every cited name exists.
2. **A counter agent checks what code cannot.** Does the cited evidence support the claim? Is the
   manifest pruned? Is there personal data in the artifact?
3. **A human decides.** Approval gates sit after the findings (`approval-gates.md`).

## Example

`repo-documenter` writes a directory's purpose from a fact sheet. The Go Verifier confirms every tag,
entry point and citation exists. `repo-doc-auditor` then reads the fact sheet and the claim and judges
whether the cited facts support it. It holds only `Read` and returns a verdict with findings. On
CHANGES_REQUESTED the documenter retries with the findings. A human approves what survives.

`validate-artifact` can chain the same way for pipeline artifacts. After the structural check passes, it
can invoke the producer's counter agent and attach the findings. This is opt-in. With no configuration
it runs the structural check only (`.claude/validate-artifact.yaml` or `invokeAuditors: true` in
`.claude/delivery-policy.yaml`).

## Adding a pair

1. Name the producer, the artifact it makes and the failure you want caught.
2. Check that code cannot catch it. If code can, write the check and skip the agent.
3. Write the counter agent as read-only: `tools: Read, Glob, Grep` and no more. Use the `light` model
   tier unless the judgement needs more.
4. Say what it returns: findings, not a rewrite.
5. Add the row to `docs/aos/governance-pairs.md` and the agent to `docs/AGENT_REFERENCE.md`.
6. Decide how it runs: standalone, through `validate-artifact`, or on a hook. Nothing dispatches hooks
   today (`shared/hooks/README.md`), so a hook needs a runner you supply.
7. Name the failure it will not catch.

## Trade-offs

**What it buys.** Separate eyes on every artifact that matters. Findings are cheap to produce and cheap
to read, and the producer's work is never silently changed.

**What it costs.** Another model call per artifact, and findings someone must triage. Most counter agents
here run on the `light` tier to keep that small.

**What it does not give you.**

- **Independence is partial.** A counter agent built on the same model family can share the producer's
  blind spots. A second opinion from the same mind is weaker than a second opinion from a different
  one. Layer 1 (code) is what breaks that correlation.
- **A counter agent can become a rubber stamp.** If it approves almost everything, it adds cost and no
  safety. Measure it: its escape rate (defects found after it passed) belongs in the weekly report
  (`engineering-metrics.md`), and its agreement rate belongs on the `agent-scorecard`. Both are **not
  collected today**.
- **Read-only is only enforced for one property.** `TestCounterAgentsDeclareNoWriteOrEgress` asserts the
  counter agents declare no write or egress tool (`security-patterns.md`). That the counter agent
  stays independent of the producer is judgment-only.
- **The default is not "audits always run".** `deliver-feature-workflow.md` describes the audit step as
  the default composition. `validate-artifact` runs it only when configured. Check which applies to your
  project before you rely on it.

## Related

- `docs/aos/governance-pairs.md`: the inventory of the 15 pairs.
- `docs/patterns/deliver-feature-workflow.md`: where the counter agents sit in the pipeline (see "The
  audit-after-producer pattern").
- `docs/patterns/agent-skill-pair-convention.md`: a different pairing. Same name for an agent and a
  skill. Do not confuse the two.
- `shared/skills/validate-artifact/SKILL.md`: the structural layer and the opt-in counter invocation.
- `docs/patterns/security-patterns.md`: the least-privilege reasoning behind read-only counter agents.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
