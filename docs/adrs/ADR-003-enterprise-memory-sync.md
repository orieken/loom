# ADR-003: Enterprise Memory Sync Protocol

## Status

Proposed

## Date

2026-07-31

## Context

Knowledge Items (KIs) in this framework reside in two locations per `shared/memory-registry.json`:

- `shared/knowledge/` — portable KIs shipped with the framework itself, applicable across any project
- `.claude/knowledge/` — project-specific KIs local to a single codebase, not portable

Today, every team that installs the framework learns in isolation. When team A discovers a durable insight (a performance pattern, a security invariant, a domain-modeling decision) and promotes it to a KI, that knowledge is unreachable by team B working in a different repository. The only path to sharing is manual copy-paste, which creates drift, stale copies, and no visibility into where knowledge originated.

At organization scale, the absence of a sync mechanism makes the framework's memory capabilities local-only — which defeats the purpose of having portable KIs at all.

### Considered options

| Option | Pros | Cons |
|---|---|---|
| **Separate git repo (`<org>/knowledge-hub`)** | Git-native conflict resolution, familiar tooling, SSH/HTTPS auth already solved, auditability via git log, no new infrastructure | Requires org to create + maintain a repo; PR flow adds latency |
| Git submodule inside framework install | Co-located with framework | Submodule complexity notorious for footguns; breaks on shallow clones; poor UX |
| Org-scoped npm/pip registry | Package-style versioning | Heavy ceremony for markdown files; requires registry infrastructure |
| Cloud sync (S3, GCS) | No git required | Requires cloud credentials per developer; non-obvious conflict resolution |

## Decision

Adopt the **separate git repo** model. An organization creates a repository (conventional name: `<org>/knowledge-hub`) that serves as the authoritative source for org-promoted KIs.

### Sync mechanics

**Pull** (`install.sh --sync-memory` or `install.sh --sync-memory pull`):

1. Clone or fetch the org repo into a local cache dir (`~/.claude/sync-cache/<org-repo-slug>/`)
2. Diff incoming KIs against local `shared/knowledge/` and `.claude/knowledge/`
3. Print a human-readable diff of what will change — **no mutations without `--confirm`**
4. On `--confirm`: copy new/updated org KIs into `shared/knowledge/`; never overwrite `.claude/knowledge/` (project-local wins on divergence)
5. Validate every merged KI against `shared/schemas/ki-frontmatter.schema.json` — reject any that fail validation
6. **Scan every incoming KI's body for instruction-override patterns before anything is written, dry run included** — `loom ki scan`. A flagged KI is not pulled; the pull names it and exits non-zero, and only a person copies it in after reading it. If no scanner can run, nothing is pulled. *(Added 2026-09-24, roadmap L3.7: until then only the frontmatter was validated, and a synced body entered agent context unaudited.)*

**Push** (`install.sh --sync-memory push`):

1. Collect local KIs in `.claude/knowledge/` that are candidates for promotion (heuristic: created more than 30 days ago, referenced by at least one other KI or ADR)
2. Open a PR against the org repo — **never a direct commit to the org repo**
3. The PR description lists each KI being promoted and its source project

### Conflict resolution

| Location | Rule |
|---|---|
| `shared/knowledge/` on pull | Org repo wins — org KIs are the authoritative promoted corpus |
| `.claude/knowledge/` on pull | Local wins — project KIs are never overwritten by sync |
| Name collision (org KI and project KI share the same `name:` slug) | Halt, report the collision, require manual resolution before `--confirm` is accepted |

### Auth

**M1 path**: SSH (GitHub org membership). The sync script calls `git clone git@github.com:<org>/<repo>.git` — if SSH is configured, it just works.

**Fallback**: HTTPS + personal access token. Set `MEMORY_SYNC_TOKEN=<pat>` in the environment; the script substitutes `https://<token>@github.com/<org>/<repo>.git` **for git's clone only**. Documented in README; never hardcoded. *(Corrected 2026-09-24, roadmap L3.7: the substituted URL was also printed and stamped into every pulled KI's `sync_source`, writing the token into files the script told you to commit. It no longer is, and a test holds that.)*

### Configuration

A new stanza in `install.sh` reads from `.claude/sync-config.yaml` (committed per project, no secrets):

```yaml
memory_sync:
  org_repo: git@github.com:<org>/knowledge-hub.git
  cache_dir: ~/.claude/sync-cache
  push_pr_base: main
```

If `.claude/sync-config.yaml` is absent, `--sync-memory` halts with an actionable error describing how to create the file.

### Health reporting

`scripts/health-check.sh` gains a **Memory Sync** section reporting:
- Last-sync timestamp (read from `~/.claude/sync-cache/<slug>/.last-sync`)
- Number of KIs pulled at last sync
- Any pending conflicts from last run
- WARN if last sync is > 30 days ago; INFO if sync-config is absent (not an error — org sync is opt-in)

## Consequences

### What becomes easier

- KIs promoted from one project become searchable in every project that syncs with the org repo.
- Conflict resolution is explicit and human-approved, not silent.
- The org repo is a clean audit trail of which teams contributed which knowledge.
- Auth is git-native — no new credentials infrastructure.

### What becomes harder / trade-offs

- Each project must set `.claude/sync-config.yaml` to participate — opt-in, not automatic.
- Push flow requires a PR review in the org repo — this is intentional (prevents unreviewed KIs polluting the corpus) but adds friction.
- Name collision resolution is manual — no automatic merge strategy for slug conflicts.
- SSH-disabled enterprises must configure HTTPS + token; this is documented but adds setup steps.

### Phase B gating

Phase B implementation (the actual `install.sh` flag, `scripts/sync-memory.sh`, registry updates, README) is gated on explicit user approval of this ADR.

## References

- `shared/memory-registry.json` — KI source registry
- `shared/schemas/ki-frontmatter.schema.json` — validation schema for KIs
- `shared/knowledge/README.md` — portable KI authoring guide
- `docs/adrs/ADR-001-adopt-rag-friendly-docs-structure.md` — establishes docs layout
- `docs/adrs/ADR-002-corpus-aware-retrieval-strategy.md` — retrieval strategy this sync feeds into
