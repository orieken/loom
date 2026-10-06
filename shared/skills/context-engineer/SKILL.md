---
name: context-engineer
description: Use PROACTIVELY before starting any task that touches 3+ files, a new feature area, or unfamiliar code — invoke this yourself, don't wait for the user to explicitly ask. Optimizes the agent's context window by pruning unrelated files, identifying relevant Knowledge Items (KIs), and compiling a precise context manifest.
triggers:
  keywords: ["optimize-context", "prune-context", "context-engineering", "context", "manifest"]
  intentPatterns: ["/optimize-context", "Optimize my context", "Help me build context for *", "Clean up context"]
standalone: true
---

## When To Use
**Use PROACTIVELY, not just on request** — self-invoke before:
- Starting a new feature, a complex bug fix, or any task touching 3+ files.
- Working in a codebase area you haven't already scoped this session.
- The active context is crowded with unrelated files or terminal outputs.

This applies outside `deliver-feature` too — an ad-hoc session ("fix this bug," "add this endpoint") gets
no context engineering at all unless this skill decides to run itself. Don't wait for the user to say
"optimize my context" — that phrasing is a fallback trigger for humans who know this skill exists, not the
primary way it's supposed to fire.

Do NOT use when:
- Performing simple, single-file edits that do not require architectural planning.

## Context To Load First
1. [context-engineering.md](file:///Users/oscarrieken/Projects/Rieken/ai-assistant-dot-files/docs/runbooks/context-engineering.md)
2. [ARCHITECTURE_RULES.md](file:///Users/oscarrieken/Projects/Rieken/ai-assistant-dot-files/ARCHITECTURE_RULES.md)
3. [DOMAIN_DICTIONARY.md](file:///Users/oscarrieken/Projects/Rieken/ai-assistant-dot-files/DOMAIN_DICTIONARY.md)

## Process

### 1. Identify Target Component Scope
Identify which layers, files, and domains are relevant to the requested task. Classify them using the Sunday/Saturday Clean Architecture boundary guidelines:
- Domain/Entity (inner-most)
- Application/Use Case
- Adapter/Presenter/Controller
- Infrastructure/Framework/UI (outer-most)

### 2. Auto-Prune by Bounded Context and Change Surface
Apply these by default, not just as a manual suggestion:
- **Bounded context exclusion**: exclude files from a different Bounded Context than the one identified in
  step 1 (e.g. task is `billing`, exclude `auth` files) unless the spec/analysis explicitly documents a
  Context Crossing — in which case include only the specific crossing files, not the whole other context.
- **Change-surface exclusion**: if the task is UI-only (no data model or API changes), exclude
  infrastructure/migration files by default.
- List all open documents in the session and flag any violating either rule above for closing.

### 3. Retrieve Domain Knowledge (Proactive RAG)
Invoke `search-ki` with the target component/domain, **before** doing independent analysis — it already
searches `shared/knowledge/`, `.claude/knowledge/`, and `docs/adrs/` and ranks results, so don't duplicate
that scan here. Separately, identify key interfaces/types that define the contract of the target component
(that's a codebase lookup, not a KI search). If the task's question isn't specifically KI/ADR-shaped (e.g. it
might be answered by a past feature's retrospective or a `DOMAIN_DICTIONARY.md` term instead), invoke
`query-memory` in place of `search-ki` — check `shared/memory-registry.json` if unsure which memory sources
exist. This is additive: the default, proven path (`search-ki` for KI/ADR questions) is unchanged.

### 4. Search Prior Deliveries in the Same Bounded Context (Recency-Independent)
Grep (case-insensitive — older archived analyses use `**Owning context**`, newer ones `**Owning Context**`)
`docs/features/*/analysis.md` for an Owning Context entry matching the Bounded Context from step 1.
For every match with a `retrospective.md`, pull its "What Went Poorly"/"What To Improve" sections. This is
deliberately independent of recency — a same-area feature from 20 deliveries ago still surfaces here, unlike
`analyst`'s own feedback loop (which only reads the 3 *most recent* deliveries regardless of area). At small
scale a direct grep is fine; once the archive grows past ~15-20 features, see
`docs/runbooks/scaling-cross-feature-learning.md`.

### 4a. Pin by Blast Radius (when the repo graph is available)
Once the files the task changes are known, ask the graph rather than guessing who depends on them. If the
`loom` MCP server is registered (`loom mcp serve`) and the repo was indexed (`loom index`), call
`mcp__loom__impact` with those paths and a depth of 2–3: it returns every directory that transitively imports
them, nearest first, each labelled `exact` or `approximate`. Pin the changed files, then their distance-1
`exact` dependents, then tests; treat `approximate` as something to verify. In the typed manifest record only
`blastRadius.seeds` and `depth` — `loom` computes the rest from the graph and replaces anything else, so never
transcribe the result. A `fresh: false` answer predates the working tree: confirm importers with Grep. With no
graph, pin by grep over imports, omit `blastRadius`, and say the radius was not graph-derived. Full guidance:
`shared/agents/context-engineer.md` step 7.

### 4b. Prefer High-Fidelity References
When choosing what to pin, prefer references Claude can inspect directly over prose paraphrases:
- Interfaces, schemas, tests, fixtures, generated golden outputs, and prior implementation files.
- HTML/design mockups or artifact files when visual fidelity matters.
- Rubrics/checklists when the task depends on taste or review judgment.

If a prose spec summarizes a code/test/schema reference, pin the underlying reference too — or replace the
prose pin with the underlying reference when the summary adds no unique information.

### 5. Estimate the Token Budget
For each pinned file, estimate tokens (~line count × 8 chars/line ÷ 4 chars/token — a rough heuristic). Sum the total and compare against the consuming agent's tier budget (of a 200k-token context window): Analyst/Architect ≤60%, Developer ≤80%, Reviewer agents ≤40%. Flag `WARNING` if over budget and recommend specific cuts.

### 6. Compile a Context Manifest
Generate a concise `context-manifest.md` in the active feature workspace (e.g., `.claude/feature-workspace/<feature-name>/context-manifest.md`, where `<feature-name>` is provided by the orchestrator; or output directly when invoked standalone). 

## Output Format

The skill produces a `context-manifest.md` matching this structure:

```markdown
# Context Manifest: [Feature/Task Name]

## 1. Scope and Boundaries
- **Target Component**: [e.g., billing-service, user-auth]
- **Relevant Layers**: [e.g., Domain Entities, Application Use Cases]
- **Bounded Context**: [e.g., Identity & Access]

## 2. Pinpoint Files (To Keep Open)
List specific files and line ranges that MUST be kept in active memory. Prefer high-fidelity references
(interfaces, tests, schemas, fixtures, mockups, rubrics) over prose paraphrases when both exist:
- [File Basename](file:///absolute/path/to/file#L1-L50) -- [Reference type] -- [Purpose/Contract]
- [File Basename](file:///absolute/path/to/file#L100-L150) -- [Reference type] -- [Implementation details]

## 3. Global Rules and Constraints
List reference files that establish the patterns:
- [ARCHITECTURE_RULES.md](file:///absolute/path/to/ARCHITECTURE_RULES.md)
- [DOMAIN_DICTIONARY.md](file:///absolute/path/to/DOMAIN_DICTIONARY.md)

## 4. Knowledge Items & ADRs (To Load)
- [KI Name](file:///absolute/path/to/ki/artifact) -- [Summary of relevance]
- [ADR Name](file:///absolute/path/to/adr) -- [Decision context]

## 5. Prior Deliveries in This Bounded Context
- [Feature Name](docs/features/<name>/) -- [key lesson from its retrospective.md, or "no retrospective.md exists" if none]
— or "No prior deliveries found in this bounded context" if none match

## 6. Prune Recommendations (To Close)
List files currently open that should be closed immediately:
- [ ] [File Basename](file:///absolute/path/to/file) -- [Unrelated context / different architecture layer]
- [ ] [File Basename](file:///absolute/path/to/file) -- [Unrelated context / different architecture layer]

## 7. Token Budget
- **Estimated total tokens for pinned files**: ~<N>
- **Target agent tier**: [Analyst/Architect: ≤60% | Developer: ≤80% | Reviewer: ≤40%] of a 200k-token context window
- **Status**: OK | WARNING (exceeds tier budget — see cut recommendations below)
- **Cut recommendations (if WARNING)**: [file] -- [reason it's the lowest-signal pin]
```

## Guardrails
- **No directory dumps**: Do not include entire directories in the manifest. Specify files explicitly.
- **Limit line count**: Files over 500 lines must be referenced with specific line ranges.
- **Prefer reference fidelity**: Do not pin a prose summary when a smaller, more precise code/test/schema/
  rubric/mockup reference carries the same information.
- **Rule alignment**: Never recommend files that violate Clean Architecture dependency boundaries (e.g. loading infrastructure database models in the Domain context manifest).
- **Never** report a token budget as OK without having actually estimated it.
- **Never** skip the "Prior Deliveries" section silently — state "none found" explicitly so it's clear the check ran.
- **Never** fabricate a lesson from a retrospective that doesn't actually say it.

---
*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
