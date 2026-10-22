---
type: feature
status: draft
created: 2026-10-22
---

# Mine repeated corrections and workflows

## Summary
Find the corrections and workflows that repeat in captured sessions, and decide what kind of fix each repeat needs. A repeated fix points to a rule the agent lacked; a repeated multi-step procedure points to a workflow worth packaging. Loom then proposes the right fix (F4) instead of turning every repeat into a new skill.

## Acceptance Criteria
- [ ] Given captured sessions, when the user runs `loom memory mine`, then the system MUST write cluster records and report how many of each kind it found.
- [ ] Given the same day files and configuration, when the user runs `loom memory mine` twice, then both runs MUST give identical clusters, identical ids and identical draft specs.
- [ ] Given 3 corrections with one signature across 2 sessions, when mining runs, then the system MUST report one correction cluster.
- [ ] Given 3 corrections with one signature in a single session, when mining runs, then the system MUST NOT report a cluster.
- [ ] Given 2 corrections with one signature, when mining runs, then the system MUST NOT report a cluster.
- [ ] Given corrections that differ only in hunk-size bucket, when mining runs, then the system MUST report them in separate clusters.
- [ ] Given corrections on redacted paths, when mining runs, then the system MUST group them under one redacted extension bucket and MUST NOT store a path.
- [ ] Given a correction the detector marked `unknown`, when mining runs, then the system MUST exclude it from every cluster.
- [ ] Given a run of 3 or more normalized tool events that recurs in 3 sessions spread over at least 14 days, with consistent outcomes, when mining runs, then the system MUST report one workflow cluster.
- [ ] Given a recurring run whose sessions span fewer than 14 days, when mining runs, then the system MUST NOT report a workflow cluster.
- [ ] Given a recurring run where one instance's hunk later received a Session Correction, when mining runs, then the system MUST NOT report a workflow cluster.
- [ ] Given a recurring run that is part of a longer run with the same instances, when mining runs, then the system MUST report only the longer run.
- [ ] Given a correction cluster whose instances fall inside a workflow cluster's instances, when mining classifies it, then the system MUST classify it as a rule gap and record the overlapping workflow cluster as evidence.
- [ ] Given a cluster, when the system classifies it, then the record MUST name the evidence that decided its kind.
- [ ] Given a workflow cluster, when mining runs, then the system MUST write a draft pattern spec with status `candidate` to `.claude/patterns/`.
- [ ] Given any draft spec the miner writes, when `loom health` runs, then the draft MUST pass validation.
- [ ] Given a draft spec a human has edited, when mining runs again, then the system MUST NOT overwrite it and MUST print the file it skipped.
- [ ] Given a workflow cluster whose instances touch different paths at one step, when the system drafts the spec, then that step MUST consume a `path` input.
- [ ] Given clusters, when the user runs `loom memory clusters`, then the system MUST list each cluster with its id, kind, signature, instances, sessions, first and last seen, and estimated human minutes.
- [ ] Given a cluster id, when the user runs `loom memory clusters <id>`, then the system MUST show the cluster's instances and its classification evidence.
- [ ] Given `--json`, when the user runs `loom memory clusters`, then the output MUST be valid JSON with the same fields.
- [ ] Given thresholds in the project configuration, when mining runs, then the system MUST use them, and the report header MUST name the thresholds in effect.
- [ ] Given an invalid mining configuration, when the user runs `loom memory mine`, then the command MUST fail and name the invalid field.
- [ ] Given a store without session data, when the user runs `loom memory mine`, then the command MUST say there is nothing to mine and name `loom memory ingest`.
- [ ] Given a synthetic corpus with planted patterns and noise, when mining runs, then the system MUST find every planted pattern and MUST NOT report noise below threshold.

## Out of Scope
- Naming, describing or approving clusters, and any model call (F4).
- Rendering a spec into a skill, tool or runbook (F5).
- Measuring whether a fix reduced corrections (F6).
- Mining `loom run` pipeline data (F7).
- Gapped sequence mining. Version 1 mines contiguous runs only.
- Comparing clusters across repositories (F8). One store holds one project.
- Any write to `shared/`, rules, skills or agents.

## Hypothesis and Target Metric
- We believe deterministic clustering will surface repeated human fixes worth a rule or a workflow. At least half of the clusters a human reviews at F4 SHOULD be accepted, measured by F4's approval records. The date is an open question.

## Edge Cases and Failure Modes
- Empty store or no session rows: the command says there is nothing to mine and names `loom memory ingest`.
- Day files purged by retention since the last run: their clusters disappear from the next run. An id returns if its signature reaches the threshold again.
- A draft spec file that is no longer the miner's (edited by a human): the miner skips it and prints the path.
- A draft spec whose cluster no longer exists: the miner leaves the file alone and prints it as orphaned. It never deletes a file.
- A session that crosses midnight UTC: its events stay one session, ordered by timestamp.
- Events with equal timestamps: ordered by event id, so the order is stable.
- A corpus over the runtime budget: the command stops at the budget, writes nothing, and says so.
- An unwritable `.claude/patterns/`: cluster records are kept, and the command fails and names the directory.

## Domain Language
- **Bounded context**: `Documentation Knowledge Base`. Mining reads the Episodic Store that capture (F1) fills.
- **Correction Signature**: The normalized form that groups Session Corrections: tool, correction kind, file extension (or `redacted`) and hunk-size bucket (`1-3`, `4-20`, `21+` lines).
- **Workflow Sequence**: A contiguous run of 2 to 6 normalized tool events in one session. A normalized event is the tool name plus its sorted argument keys.
- **Pattern Cluster**: A group of instances that share a Correction Signature or a Workflow Sequence and meet the thresholds. Its id is a hash of its signature.
- **Rule Gap**: A Pattern Cluster that shows the agent lacked knowledge. The fix is a rule, a Knowledge Item or a prompt edit (F4).
- **Workflow Gap**: A Pattern Cluster that shows a repeated multi-step procedure. The fix is a pattern spec (F2), rendered by F5.
- **Consistent Outcome**: A Workflow Sequence instance whose written hunks received no Session Correction.

## Non-Functional Requirements
- Determinism: the same day files and configuration MUST give byte-identical `loom memory clusters --json` output.
- Runtime: mining 200,000 events MUST finish in under 10 seconds on a warm cache. The budget is configurable, and a run over it writes nothing.
- Memory: mining MUST NOT hold more than one pass of normalized events in memory at a time.
- Network: none. Mining MUST work offline and MUST make no model call.
- Complexity: every function under cyclomatic complexity 7. Changed-line coverage of at least 85%.

## Trust Boundaries
- Day files → miner: treat events as untrusted data, as F1 does. The miner reads argument keys, tool names, paths and hashes, never values.
- Paths: signatures hold extensions only. A redacted path joins the `redacted` bucket and is never stored in plain form (`architecture-guardrails.md` #9).
- Draft specs → `.claude/patterns/`: the miner writes only files it can prove it wrote. It records each draft's content hash and compares before every overwrite.
- Evidence in drafts: cluster id, counts, dates and the project identifier only. No prompt text, code or path.

## Test Approach
Go unit and integration tests. No Saturday or Sunday.
- Golden-file tests over a synthetic corpus in `testdata/`, with planted correction clusters, planted workflows and noise just under each threshold.
- Table-driven tests for signature normalization, hunk-size buckets, the 14-day span, consistent outcomes and maximal-sequence pruning.
- A determinism test: mine the corpus twice and compare the JSON byte for byte.
- An integration test: day files → `loom memory ingest` → `loom memory mine` → `loom health` validates every draft.
- A benchmark of 200,000 events for the runtime budget.

## Open Questions
- The F1 precision checkpoint: the initiative asks for a review of about a week of real correction log before F3 builds on it. Assumption: build F3 only after that review, and tune F1 first if the log is not trustworthy.
- Where thresholds live: F1 settled on one settings file, `.claude/capture.yaml`. Assumption: a `mining:` section in that file, with defaults compiled in.
- The project identifier in draft evidence: F2 requires at least one repository identifier. Assumption: the repository directory name. It stays local, and F8 decides how to share it.
- The executor table: version 1 maps `Edit`, `Write` and `MultiEdit` (model-authored content) to `skill`, a corrective prompt or human edit inside a run to `human` with reason `judgment-out-of-scope`, and every other tool to `tool`. A person SHOULD confirm this table against real sessions.
- The target date for the hypothesis: F4 has no date yet.

## Decisions
- A Correction Signature is tool + correction kind + file extension (`redacted` for a redacted path) + hunk-size bucket.
- Workflow mining uses contiguous runs of 2 to 6 normalized events, counted by distinct sessions. Only maximal runs are reported.
- Defaults: a correction cluster needs 3 corrections across 2 or more sessions. A workflow cluster needs 3 or more steps, 3 sessions, a span of at least 14 days, and a consistent outcome on every instance. All of these are configurable.
- A correction cluster is a rule gap by default. A cluster that qualifies as both is a rule gap, and its record names the overlapping workflow cluster.
- A cluster id is a hash of its normalized signature. Clusters never merge or split: a different signature is a different cluster.
- Cluster records are a projection in `episodes.db`, rebuilt on every mining run, like the session tables. F4 keeps its own approval records by cluster id.
- `session_corrections` gains a nullable `session_ref` column (`ALTER TABLE ... ADD COLUMN`, no drop or rename). `loom memory ingest` fills it from the day files.
- Draft specs are named `mined-<first 8 characters of the id>.pattern.yaml`. Their intent, step descriptions and inputs are generated deterministically from the cluster; F4 renames and rewrites them.
- Estimated human minutes reuse F1's fixed weights per correction kind. A workflow cluster reports minutes as not measured.

## Infrastructure / Deploy Notes
- New commands: `loom memory mine`, `loom memory clusters [--kind rule-gap|workflow-gap] [--json]` and `loom memory clusters <id>`.
- `episodes.db` gains two tables with `CREATE TABLE IF NOT EXISTS` (clusters and their instances), and `session_corrections` gains `session_ref`. The schema version stays the same.
- `.claude/capture.yaml` gains an optional `mining:` section. A missing section means the defaults.
- New files under `.claude/patterns/` only, for workflow gaps. No environment variables. Standard deploy.
