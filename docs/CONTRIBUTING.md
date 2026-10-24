# Contributing

How to add a new agent, skill, rule, or platform to this framework. For the overall design, see
[ARCHITECTURE.md](ARCHITECTURE.md).

Everything you add lives in `shared/` and only `shared/` — never edit a generated platform config directly
(anything in `.cursor/`, `.github/`, `.gemini/`, `.openai.md`, `.windsurfrules`), it'll just get overwritten
by the next `scripts/generate-configs.sh` run.

---

## Adding a new agent

1. Create `shared/agents/<name>.md` following an existing agent as a template. The file **must start with
   `---` on line 1** — no preamble text before the frontmatter block. Claude Code tolerates a preamble (it
   scans for the block wherever it is), but Cursor's stricter parser doesn't, and `.cursor/agents/` is a
   direct symlink to this same file (see `install.sh`'s `install_cursor()`), so anything that breaks
   Cursor's parsing breaks there too. Required frontmatter:
   ```yaml
   ---
   name: your-agent-name
   description: When to use it — be specific enough that an orchestrator or human can decide without guessing.
   tools: Read, Write, Edit, Bash, Glob, Grep   # only what it actually needs -- ignored by Cursor's
                                                # subagent parser (no per-agent tools allowlist there;
                                                # it inherits all parent tools), but still enforced by
                                                # Claude Code
   model: inherit   # matches whatever model the operator's own session is running, on both Claude Code
                     # and Cursor -- only hardcode a specific model (sonnet/opus/haiku/etc.) if this agent
                     # genuinely needs a specific capability regardless of session model
   version: 1.0.0
   ---
   ```
2. Write the persona, process, output format, and rules sections — see any existing agent for the expected
   shape. Keep the process numbered and concrete; vague steps produce vague output. If the agent needs to
   read `shared/rules/*.md` before starting, say so as the first line of the body (not before the
   frontmatter) — see `shared/agents/analyst.md` for the pattern.
3. If this agent participates in `deliver-feature`, decide where in the pipeline it runs and update
   `shared/skills/deliver-feature/SKILL.md`'s numbered steps accordingly.
4. If its output is consumed by other agents, consider adding a contract in `shared/contracts/` (see
   "Adding a contract" below) and wiring `validate-artifact` into `deliver-feature` after its step.
5. Add a `## 2026-MM-DD — <reason>` entry to `shared/agents/CHANGELOG.md` in the same commit — the
   pre-commit hook (`scripts/hooks/pre-commit`, opt-in) checks for this if it's enabled.
6. Consider a golden-file fixture in `tests/agents/<name>/` plus an `eval-rubric.md` (see
   [editing-agent-prompts.md](runbooks/editing-agent-prompts.md) and the `agent-eval` skill) if this agent is
   likely to regress silently on future edits.
7. Run `scripts/check-parity.sh` and `scripts/generate-configs.sh` — the new agent needs to show up in
   every platform's persona roster.

## Adding a new skill

1. Copy `shared/skills/SKILL_TEMPLATE.md` into `shared/skills/<name>/SKILL.md`.
2. Fill in `triggers.keywords` and `triggers.intentPatterns` — be specific; overly broad keywords cause a
   skill to fire when the user meant something else.
3. Write "When To Use" with explicit "Do NOT use when" cases — this is what lets Claude (or a human)
   disambiguate between similar skills (e.g. `retrospective` vs. `pipeline-retrospective` vs.
   `agent-scorecard` all sound similar but analyze different things).
4. If the skill is meant to run inside `deliver-feature`, wire it into the numbered pipeline steps. If it's
   standalone, it needs no wiring — it's just discoverable by its trigger patterns.
5. Run `scripts/check-parity.sh` — skills aren't part of the persona roster check, but confirm nothing else
   broke.

## Adding a new rule

1. Add or edit a file in `shared/rules/` (`architecture-guardrails.md`, `design-principles.md`,
   `approval-gates.md`, or a new one).
2. Treat rule changes as high-impact because agents load them as session-long law:
   - Without CI wiring, Gate #7 does not apply; obtain normal human or PR review.
   - If the change also modifies CI/CD to enforce a new architectural property, Gate #7 applies. Before
     committing that wiring, the user must say "approve fitness function" or "add to CI."
3. If you added a new rule *file* (not just edited an existing one), update `generate-configs.sh`'s
   `collect_rules()` and, for Cursor, add a new `generate_mdc()` call in `generate_cursor()` so it gets its
   own `.mdc` file (Cursor needs focused, small rule files, not one giant one).
4. Run `scripts/generate-configs.sh` then `scripts/check-parity.sh`.

## Adding a contract (`shared/contracts/`)

Contracts define the required section headings for a pipeline agent's output, enforced structurally by
`validate-artifact` between handoffs (see Epic 5 in
`docs/features/context-engineering-framework/TODO.md` for the original design rationale).

1. Create `shared/contracts/<agent>-contract.md` — list every required heading **exactly** as the producing
   agent's own Output Format template writes it, including heading level (`##` vs `###`).
2. Add any content-level rules beyond "heading exists" (e.g. `qa-contract.md` requires `Failed: 0`,
   `review-contract.md` requires the literal `APPROVED`/`CHANGES REQUESTED` string).
3. Add the agent to `validate-artifact`'s Contract Mapping table and to `scripts/test-agents.sh`'s
   `contract_for_agent()` function.
4. Wire a `validate-artifact` step into `deliver-feature` immediately after the producing agent's step.

## Adding a new platform

See [docs/runbooks/adding-a-new-platform.md](runbooks/adding-a-new-platform.md) — it's involved enough to
warrant its own step-by-step guide (registry entry, generator function, parity check, tier/terminology
decision).

## Editing an existing agent prompt

See [docs/runbooks/editing-agent-prompts.md](runbooks/editing-agent-prompts.md) — versioning, changelog, and
testing requirements.

## Before you push

- `scripts/ci-check.sh` — runs `check-parity.sh`, `test-agents.sh`, `health-check.sh`, and
  `test-install.sh` inside a container matching the actual CI runner (ubuntu-latest, modern bash), not
  just your local shell. `test-install.sh` (2026-07-08) is an install verification matrix -- it runs a
  real `install.sh --project <scratch-dir> --platform <name>` for every platform and asserts the
  expected symlinks/files actually exist, complementing `check-parity.sh` (which only checks this
  repo's own already-generated output, not what a fresh install produces elsewhere). Requires Docker.
  This exists because macOS's default bash (3.2) silently tolerates some `set -e` + arithmetic bugs that
  modern bash treats as failures — a local pass on macOS proves nothing about whether CI will pass (see the
  2026-07-04 incident where this exact gap broke CI for several commits before anyone noticed). Run this
  before every push that touches `scripts/`, `shared/`, or `.github/workflows/` — a plain local run of the
  individual scripts below isn't sufficient for those.
- `make test-in-container` — the Go side of the same idea (roadmap L3.72). It builds
  `docker/test.Dockerfile`, a pinned image with the Go toolchain `go.mod` requires, both modules'
  dependencies, and the linters CI pins, then runs `scripts/test-in-container.sh` against your working
  tree mounted read-only: build, tests, the coverage ratchet and changed-line coverage, golangci-lint,
  changed-line mutation, and a smoke test that installs the built binary into a scratch project.
  `make test-in-container-quick` skips mutation; `STEPS="build test"` runs a subset. Each step reports
  PASS, FAIL or NOT MEASURED, and NOT MEASURED — usually no diff base, so fetch `origin/main` or set
  `LOOM_DIFF_BASE` — is never a pass. Requires Docker. Run it before pushing any change under `cmd/`,
  `internal/` or `shared/mcp/`: CI's `test-image` job runs the same image.

## Before you commit

- `scripts/check-parity.sh` — no drift between `shared/` and generated configs
- `scripts/test-agents.sh` — if you touched an agent with a `tests/agents/` fixture
- the `agent-eval` skill — if that fixture also has an `eval-rubric.md`, for the qualitative regression check
- `scripts/check-context-budget.sh` — if you touched `context-engineer` or its output format
- A `shared/agents/CHANGELOG.md` entry — if you touched any `shared/agents/*.md` behavior
