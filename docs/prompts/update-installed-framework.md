# Update the Framework in an Installed Project

> Compatibility notice: this prompt and its linked historical execution sequence include copy-based MCP assumptions.
> For Pattern B, use [the current MCP adoption prompt](adopt-framework-mcp-tools.md) and inspect the existing integration first.
> The deleted `shared/mcp-patterns/go/` directory is not an update source. Prefer the supported Go registry for Go integrations;
> compare native ports with the compiled implementation and preserve local adaptations.
> Inspect current CLI help and install metadata before following legacy install or generation commands below.
> A commit-per-operation plan does not authorize commits: obtain explicit approval for each pending changeset under the project's gates.

Propagate framework updates from `ai-assistant-dot-files` into a target project, detecting which
install pattern the project uses and running the correct update sequence for that pattern.

This is an updated version of `docs/prompts/done/update-installed-framework.md` that treats the
`framework-install.json` version marker (written by install.sh v3.3+ / Epic 68) as detection input #1.
Legacy installs without a marker fall back to the filesystem forensic path documented in the done/ version.

## When to use

- Framework has changed (new agents, updated skills, new templates, new tools, etc.)
- Target project needs to pick up those changes
- Unsure which install pattern the project uses

## Prerequisites

- Absolute path to the target project
- Absolute path to the framework clone (typically `/path/to/ai-assistant-dot-files`)
- Framework clone at the version you want to propagate (pull latest first if that's the intent)
- **Both** target project and framework at clean working trees

If either working tree is dirty in a way you don't want carried into the update, halt before starting.

## The four install patterns

| Pattern | How to detect | Update mechanism |
|---|---|---|
| **A. Traditional install** — symlinks | `.claude/agents/`, `.claude/skills/`, `.claude/rules/` in target project are symlinks pointing into the framework clone | `install.sh` re-run (refreshes symlinks; usually no-op since symlinks track the framework live) |
| **B. Bridge install** — copied tool source in project's MCP | Target project has an MCP server with source files that descend from `shared/mcp-patterns/go/tools/*` | Manual re-copy of updated tool files + regen tests + rebuild MCP server |
| **C. Adopted saturday-mcp** — used as-is | Target project uses `saturday-mcp` as a submodule, dependency, or installed binary | Update the saturday-mcp version pin; restart MCP server |
| **D. Generated platform configs** — non-symlinkable | `.cursor/rules/*.mdc`, `.cursorrules`, `.windsurfrules`, `.github/copilot-instructions.md`, `.openai.md`, `AGENTS.md` exist as regenerated content | Re-run `scripts/generate-configs.sh` |

Most projects hit A + D at minimum.

## Scope

### Phase A — Detect install patterns

**Detection input #1 (marker-based — reliable):**

Check `<target>/.claude/framework-install.json`. If it exists:
```bash
cat <target>/.claude/framework-install.json
```
The marker records `source_repo`, `git_tag`, `commit_sha`, `installed_at`, `mode`, `framework_level`,
and `platforms`. Compare `git_tag` against the framework clone's current HEAD tag to quantify drift.

**Detection input #2 (filesystem forensics — fallback for pre-marker installs):**

If no marker exists (install predates Epic 68 / v3.3), fall back to the forensic path from the
`done/` version:
1. Check `.claude/agents/`, `.claude/skills/`, `.claude/rules/` for symlinks (Pattern A)
2. Check `mcp/` for copied tool source (Pattern B)
3. Check for saturday-mcp dependency (Pattern C)
4. Check `.cursor/`, `.github/`, etc. for generated configs (Pattern D)

Run `bash <framework>/scripts/health-check.sh` from the target directory — if the marker is present,
the "Install Version Marker" section will report drift automatically; if not, use the manual forensic
checks above.

Record findings in `<target>/docs/framework-update-<YYYY-MM-DD>.md`. Include:
- Marker contents (or "no marker — pre-v3.3 install, forensic detection used")
- Detected patterns (A/B/C/D)
- Drift summary: installed tag vs. current framework tag
- Changed upstream files since installed commit (if marker has `commit_sha`):
  ```bash
  cd <framework> && git log --oneline <installed_commit>..HEAD -- shared/
  ```
- Local modifications in target
- Estimated update ops

Commit (in target repo): `docs(framework): investigate update needs <YYYY-MM-DD>`.

**Pause. Get user approval on the plan before Phase B.**

### Phase B — Execute the update sequence

Identical to `docs/prompts/done/update-installed-framework.md` Phase B. After completing the update,
run `install.sh` to refresh the marker:
```bash
cd <framework> && ./install.sh --project <target>
```
This writes a new `framework-install.json` with the current git_tag and commit_sha, marking the
install as up to date.

### Phase C — Contract-drift check

Identical to `docs/prompts/done/update-installed-framework.md` Phase C.

## Discipline (non-negotiable)

- One commit per op.
- Conventional Commits.
- **NEVER `git add -A`** in the target project.
- `git status --short` after staging, before every commit.
- Build + test green per commit for any pattern that touches code.
- Do NOT push in the target project — human step.

## Escalation criteria

Same as `docs/prompts/done/update-installed-framework.md`, plus:
- If `framework-install.json` is present but `source_repo` no longer resolves: note in the
  investigation log, proceed with forensic detection for the source path.
- If `framework-install.json` has a `mode: copy` entry: the install was a copy-based install;
  symlinks won't exist under `.claude/` — skip Pattern A symlink refresh, use direct file copy
  update instead.

## Report (under 100 words per phase)

```
Marker: <git_tag> @ <commit_sha[:8]>, installed <installed_at>, mode <mode>
Drift: <installed_tag> → <current_tag> (<n> commits behind)
Patterns detected: <A/B/C/D>
Upstream changes since installed commit: <n> files in shared/
Local modifications: <list or "none">
Estimated ops: <n>
Phase B commits: <sha list>
New marker after update: <new_git_tag> @ <new_commit_sha[:8]>
```
