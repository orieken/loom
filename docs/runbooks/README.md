# Operational Runbooks

This directory contains runbooks for setup, troubleshooting, agent operations, and pipeline execution. Each runbook is a self-contained guide that can be followed step-by-step.

---

## Categories

### Installation and Setup

Guides for setting up the development environment, installing dependencies, and configuring agents and skills.

- Initial repository setup and dependency installation.
- Shell configuration and dotfile integration.
- Agent and skill registration in `.claude/` directories.

### Troubleshooting

Diagnostic procedures for common issues encountered during development and pipeline execution.

- Environment and PATH debugging.
- Agent configuration resolution failures.
- Pipeline stage errors and recovery steps.
- Dependency conflicts and version mismatches.

### Skill-driven remediation

- **Environment remediation (`debug-environment`)** — use when commands, PATH, or shell configuration
  breaks. It inspects the relevant configuration, diagnoses before editing, and verifies the result. Do
  not commit global environment-profile changes without reviewing their consequences with the user.
- **Test suite remediation (`debug-tests`)** — use for cascading or widespread failures that may come
  from runner, dependency, database, or environment state. Diagnose the shared cause before individual
  assertions; never disable tests or comment out assertions to manufacture a passing suite.

### Agent and Skill Reference

Operational guides for the agent ecosystem.

- Agent invocation patterns and context loading.
- Skill configuration and registration.
- Pipeline orchestration and stage sequencing.
- Approval gate procedures (commit, deploy, external API calls).

### Pipeline Execution

Step-by-step guides for running the delivery pipeline and its individual stages.

- Full pipeline execution from analysis through delivery summary.
- Running individual stages (code review, security scan, QA).
- Persisting artifacts to `/docs/features/<feature-name>/`.
- Reviewing and shipping delivery summaries.

---

## Contributing a Runbook

Each runbook follows this structure:

1. **Title** -- What this runbook covers.
2. **When to use** -- The scenario or symptom that triggers this runbook.
3. **Prerequisites** -- Tools, access, or configuration required.
4. **Steps** -- Numbered, sequential actions. Each step is atomic and verifiable.
5. **Verification** -- How to confirm the runbook completed successfully.
6. **Escalation** -- What to do if the runbook does not resolve the issue.

---

## Existing Runbooks

- [context-engineering.md](context-engineering.md) — context taxonomy and principles, plus the full map of every Memory (KIs, ADRs, TEAM_TOPOLOGY.md, feature archive) and Learning (retrospective, agent-scorecard, agent-eval, extract-lessons) mechanism and how they differ
- [adding-a-new-platform.md](adding-a-new-platform.md) — wiring a new AI tool into the `shared/` -> generated-config pipeline
- [editing-agent-prompts.md](editing-agent-prompts.md) — versioning, changelog, and testing workflow for `shared/agents/` edits
- [scaling-cross-feature-learning.md](scaling-cross-feature-learning.md) — when and how to replace `context-engineer`'s grep-based same-bounded-context lookup with a real index, once the feature archive has grown enough to need one
- [self-audit-prompt.md](self-audit-prompt.md) — a reusable prompt for having a *different* agent independently audit this framework's internal consistency (twin-file drift, missing contracts, stale docs, dead references)
- [memory-engineering.md](memory-engineering.md) — the Capture→Candidate→Audit→Approve→Index→Retrieve→Expire lifecycle for durable memory (mainly Knowledge Items), promotion rules, and expiration criteria
- [lightrag-integration.md](lightrag-integration.md) — when and how to add an optional LightRAG retrieval backend later; not built yet, documented so the "not now" decision is deliberate and reversible
- [blog-content-brief.md](blog-content-brief.md) — a reusable prompt for handing off blog-post drafting (context engineering, memory engineering, and other framework highlights) to a fresh agent once an epic cycle wraps, with candidate topics and grounding-source pointers so drafts stay evidence-based
- [mcp-server-integration.md](mcp-server-integration.md) — a reusable prompt for adding this framework's agents (as MCP Resources) and skills (as MCP Prompts, plus Tools where a `check.sh`/`run.sh` exists) to a team's *existing* MCP server, without asking them to adopt this repo or run `install.sh`
- [repo-context-indexing.md](repo-context-indexing.md) — indexing a repository (`loom index`), having the documenter write what each directory is for (`loom document`), reviewing and applying the changeset (`loom docs`), and handling stale entries

As new individual runbooks are created, add them to this directory and link them here.
