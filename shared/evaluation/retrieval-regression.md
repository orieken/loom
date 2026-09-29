# Retrieval Regression Set

Format for capturing real-query retrieval regression cases for the lexical tier
(`search-ki`, `query-memory`). Cases are proposed by `retrieval-evaluator` from
telemetry and approved by a human before entering this file — same discipline as
`learning-engine`'s draft-KI flow.

This graduates ADR-002's judgment-only retrieval fitness function toward mechanical
verification: using queries people actually asked, not synthetic benchmarks.

---

## Telemetry: `retrieval.queried` (emitted since roadmap L3.4)

Every corpus search through `loom mcp serve` — `search_docs` and `search_features` — is traced as a
`loom.retrieval <corpus>` span beneath its tool call, carrying a `retrieval.queried` event. It is a
span event rather than an entry in the executor's event vocabulary because the searches run in the MCP
server's process, not the executor's; `TRACEPARENT` ties it back to the run.

| Attribute | On | Meaning |
|---|---|---|
| `loom.retrieval.corpus` | span | `project-docs`, `project-features` |
| `loom.retrieval.backend` | span | `bm25`, `hybrid` |
| `loom.retrieval.query.length` | span | the query's length in characters |
| `loom.retrieval.query.hash` | span | salted digest, **only** when `LOOM_TELEMETRY_SALT` is set |
| `loom.retrieval.hits` | event | how many references came back |
| `loom.retrieval.outcome` | event | `hit`, or `miss` for zero hits |
| `loom.retrieval.top_hit` | event | the best reference, relative to the corpus root |

**The query text is never recorded.** The schema first proposed here carried `metadata.query` as the
literal string; guardrail #9 (`architecture-guardrails.md`, a hard constraint) forbids recording free
text a caller composed, and a query is exactly that. What survives is enough to find the pattern — the
same salted hash missing again and again, which corpus, which backend — but not what was asked. A case
therefore still starts with a person: the hash says *that* a query keeps missing, and whoever asked it
writes the case below. `chosen` (the result the agent actually loaded) is not emitted: the server cannot
see which reference an agent went on to read.

## Case format

```markdown
### Case: <short-slug>

**Query**: "<exact query string>"
**Corpus**: ki | adr | feature-archive | feature-archive-summaries | domain-dictionary
**Must appear in top-5**: <file path>
**Acceptable alternatives**: <file path>, <file path>  (optional)
**Source**: telemetry:<ISO-date> | manual:<ISO-date>
**Notes**: why this case matters (what would break if it regressed)
```

---

## Running the regression set

`retrieval-evaluator` uses these steps when asked to "run the retrieval regression set":

1. Read this file and load all approved cases (those with a `### Case:` heading).
2. For each case, invoke the appropriate retrieval skill (`search-ki` or `query-memory`)
   with the recorded query.
3. Check whether the "Must appear in top-5" reference is in the result set.
4. Record PASS / FAIL per case.
5. Report results in the standard output format below.

The evaluator never modifies this file — all case additions require human approval.

---

## Output format

```markdown
# Retrieval Regression Run: [YYYY-MM-DD]

## Summary
- Cases run: N
- Passed: N
- Failed: N

## Failures
| Case | Query | Expected | Got (top-5) |
|---|---|---|---|
| <slug> | "<query>" | <expected-file> | <actual top-5 list, or "no hits"> |

## Passes
(omit if all pass — keep output short)
| Case | Query | Expected |
|---|---|---|

## Proposed new cases (from telemetry misses)
(from `retrieval.queried` events with `outcome: miss`; the query is known only by its hash)
- Query hash `sha256:<16 hex>` missed <N> times in `<corpus>` — ask whoever ran the query what it was; candidate for create-ki or tag update
```

---

## Seed cases (manually added — telemetry points at misses, people write the cases)

No seed cases yet. `retrieval.queried` is emitted now (see above), so `retrieval-evaluator`
can find repeated misses in a traced run's spans — a salted query hash with `outcome: miss`
more than once. The case itself is still written by whoever asked the query, since the span
holds its hash and never its text. Cases can also be added directly when a known retrieval
failure is identified.

### Example (reference — not a real case, shows format)

```markdown
### Case: context-engineer-wiring

**Query**: "why isn't context-engineer running automatically before analyst"
**Corpus**: ki
**Must appear in top-5**: shared/knowledge/context-engineer-must-be-wired-into-pipeline.md
**Source**: manual:2026-08-04
**Notes**: This KI was written specifically because the pipeline had a dead-capability
  bug. Regression here would mean the KI became unretrievable, defeating its purpose.
```

---

## Governance

- Cases in this file are approved — `retrieval-evaluator` runs them as-is.
- Cases under "Proposed new cases" in an evaluator report are drafts — a human must
  copy them here (adding source/notes) before they count as regression tests.
- Removing a case requires a commit with a comment explaining why the case is no
  longer relevant (e.g., the KI was deprecated and removed).

---

*Part of the [ai-assistant-dot-files](https://github.com/orieken/loom) Context Engineering Framework by Oscar Rieken — licensed under [CC BY 4.0](https://github.com/orieken/loom/blob/main/LICENSE-CONTENT.md). If you copy or adapt this file, please keep this attribution.*
