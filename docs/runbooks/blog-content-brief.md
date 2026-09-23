# Runbook: Blog Content Handoff — Context Engineering, Memory Engineering, and What We Learned Building This

## When to use
Once the current epic cycle is genuinely wrapped (no open items in
`docs/features/context-engineering-framework/TODO.md`), hand this prompt to a fresh session — a
different agent than whichever one did the engineering work — to draft blog post(s) about this
framework's approach to context engineering, memory engineering, and cross-platform AI tooling. A fresh
session write about it more like a reader encountering it for the first time than a session that's been
heads-down in the implementation for weeks.

## Prerequisites
The target agent needs read access to this repo. No special setup — this can run against any checkout,
including a fresh clone.

## The Prompt

Copy everything below the line into a new session:

---

```
You are drafting blog post content about "ai-assistant-dot-files" -- a Context Engineering Framework
that defines a canonical set of AI agents, skills, and rules once in shared/, then deploys them across
six different AI coding tools (Claude Code, Cursor, Windsurf, GitHub Copilot, Gemini/Antigravity, OpenAI
Codex) without duplicating content per tool. It also ships a 14-agent feature-delivery pipeline with
inter-agent contracts, a memory system with a real promotion lifecycle, and governance modeled on
irreversibility rather than blanket caution.

Read these first, in this order, to ground everything you write in what's actually built (not what
sounds impressive) -- this codebase has a strong internal norm against unverified claims, carry that
into the writing:
1. README.md -- overall architecture, agent roster, skill catalog
2. docs/ARCHITECTURE.md -- the shared/ canonical layer, the Capability Tier system
3. docs/runbooks/context-engineering.md -- the Context/Memory/Learning mental model
4. docs/runbooks/memory-engineering.md -- the memory lifecycle in full
5. docs/AGENT_REFERENCE.md -- every agent's role and its counterbalance (contract, reviewer, gate, or
   metric) -- this is the best evidence for "governance by design, not by vibes"
6. docs/features/context-engineering-framework/TODO.md -- the full epic history, Phases 1-9, especially
   Epic 30 (Cursor) and Epic 5/Epic 22 (contracts and memory) for concrete before/after detail

## Candidate topics (pick what has the strongest narrative, don't try to cover everything in one post)

### 1. Context Engineering: treating the context window as a budget, not an afterthought
The core mechanism: a dedicated context-engineer agent produces a context-manifest.md before real work
starts -- scoped file lists, pruned by bounded-context mapping, with token budget estimates per pipeline
agent tier (Analyst/Architect up to 60%, Developer up to 80%, Reviewers up to 40%). "Context decay":
artifacts older than 2 pipeline phases get read as a ~200-word summary instead of in full. The
distinction the framework insists on: Context (this turn's window) vs. Memory (durable, outlives the
run) vs. Learning (feedback loops that change future behavior) -- three different problems people
conflate under "RAG."

### 2. Memory Engineering: a promotion pipeline, not a pile of notes
The lifecycle: Capture -> Candidate -> Audit -> Approve -> Index -> Retrieve -> Expire. The key design
choice worth explaining: nothing gets written to the knowledge base directly -- every capture becomes a
structured Candidate Record (Source, Type, Evidence, Tags, Expiration condition) that a human approves,
same discipline as a git commit. Promotion Rules exist specifically to reject things (one-off, already
covered, too speculative) -- the framework treats "zero candidates promoted this cycle" as a healthy
outcome, not a failure. Worth contrasting with the LightRAG decision: a whole runbook
(docs/runbooks/lightrag-integration.md) exists for it, with zero code written, on purpose -- a real
example of YAGNI as a documented decision rather than just an absence.

### 3. The forgotten symlink: a story about drift and rediscovery
A genuinely good narrative hook. While building native Cursor subagent/skill support (Epic 30), we
discovered .cursor/agents and .cursor/skills already existed in the repo as symlinks -- committed
2026-04-09, three months before the Tier system that would have explained why they existed. Someone
(an earlier pass) had already solved this exact problem, and it got silently lost because nothing
validated it, documented it, or referenced it anywhere. The fix wasn't just wiring it up properly --
it was adding it to check-parity.sh so it can't get silently lost again. Good example for a post about
why "it works" isn't the same as "it's maintained."

### 4. Governance by irreversibility, not by caution
shared/rules/approval-gates.md gates exactly 8 actions -- all of them genuinely irreversible (commits,
deploys, migrations, external API calls) -- not "anything that feels risky." docs/AGENT_REFERENCE.md
categorizes every agent's counterbalance into one of four kinds (structural contract, downstream agent
review, human approval gate, or aggregate/delayed metric) and is explicit about where a gap is real vs.
where it's a conscious, accepted tradeoff (a standalone TDD agent once bypassed the whole review
chain on purpose, for speed, and that was written down rather than hidden — until ADR-009 retired it).

### 5. Auditing the auditor
Three independent AI tools (Antigravity, Codex, Copilot) each ran a structural self-audit against this
repo at different points, cross-checked against the actual files rather than trusted at face value --
several findings turned out to be false positives, some were real and fixed. The self-audit prompt
itself (docs/runbooks/self-audit-prompt.md) evolved as a result, including a "Known Judgment Calls"
section added specifically so a future audit doesn't re-litigate a decision that was already reviewed
and confirmed correct. Also worth mentioning: a CI-breaking bash bug (`((var++))` evaluating to a
"false" exit status under `set -e` when the pre-increment value is 0) that was invisible on macOS's
bundled bash but broke on Ubuntu's -- caught by reproducing the exact CI environment in Docker
(scripts/ci-check.sh) rather than trusting a local run.

### 6. Ideas we're deliberately not building yet
- AOS (docs/aos/ on the v3-aos branch): a bigger "AI Operating System" concept -- OS-metaphor
  subsystems (Kernel, Scheduler, Security Manager), a governance checks-and-balances pattern (every
  builder role paired with an auditor role). Deliberately kept separate from this framework's shipped
  v2 scope, and deliberately not yet decided whether it's worth building at all -- the first job for
  that branch is figuring out which of it is a real gap versus already covered under a different name.
- Model routing: considered and explicitly rejected for the interactive pipeline (no evidence of a
  real cost problem yet), but flagged as worth revisiting for a possible future headless/autonomous
  execution mode, where nobody's watching cost accumulate in real time.
- Exposing the framework through MCP to give other tools (like Cursor) programmatic access to search-ki,
  query-memory, and agent personas -- with an explicit design principle worth including if this becomes
  its own post: expose retrieval live from shared/, never duplicate the content into a second language
  or repo, because that duplication is exactly what causes the drift bugs documented above.

## Constraints
- Don't invent statistics or claims not traceable to the docs above -- if you want a number (agent
  count, skill count, epic count), get it from the actual current repo state, not from memory of what
  the docs said an hour ago (they change).
- This framework's own voice avoids marketing language ("revolutionary," "game-changing") in favor of
  specific, falsifiable claims -- match that register.
- Flag which post(s) you're actually drafting before writing full prose, so scope can be confirmed
  before you invest in a full draft.
```

---

## Verification
A good draft cites specific file paths, specific epic numbers, and specific before/after facts (file
counts, dates, commit-worthy details) rather than generic "we built a comprehensive system" language.
If a draft reads like it could describe any AI tooling repo, it hasn't actually used the source material
-- send it back with a request to ground every claim in a specific file or epic.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
