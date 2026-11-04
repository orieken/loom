---
intake: 1
id: doc-style-check
type: feature
plan: deliver-feature
status: draft
priority: 2
created: 2026-11-04
requested-by: oscar
source: manual
trust: internal
base-branch: main
finish: pr
autonomy: inherit
depends-on: []
budget:
  max-attempts: 2
  max-usd: 25
  max-hours: 3
scope:
  paths: [internal/docstyle/**, cmd/loom/cmd/docs*.go, shared/rules/writing-style.md, docs/features/doc-style-check/**]
---

# Check that docs follow the writing style

## Request

> I just want to make sure all of our docs are easy to read and consistent with one pattern.

## Summary

`shared/rules/writing-style.md` (Simplified Technical English) is the one pattern for every doc
(decision D-33). Only `tech-writer`, `spec-writer` and `analyst` read it, and nothing checks the
result, so a doc written by hand can drift. `loom docs check` reports where a Markdown file breaks the
rule's mechanical parts.

The check cannot judge whether a doc is clear. It finds candidates for a person or agent to fix, so
its output says "check this", and it is a report before it is ever a gate.

## Acceptance Criteria

- [ ] Given `loom docs check <path>...`, with a file or a directory, then it reports each finding as
      `file:line: rule: excerpt`, and exits 0 by default. Given `--strict`, it exits 1 when there is a finding.
- [ ] Given a Markdown file, then it ignores frontmatter, fenced and inline code, tables, headings, links'
      targets and HTML comments, and it does not count those words.
- [ ] Given prose, then it reports:
      - a descriptive sentence over 25 words, and a list step over 20 words;
      - a run of more than three consecutive nouns, only if a deterministic check can do it without a
        new dependency; otherwise record that it was left out, and why;
      - words the rule replaces (`utilize`, `prior to`, `with regard to`);
      - `can`, `might` or `ought to` in a sentence that also has `MUST`, `SHOULD` or `MAY`.
- [ ] Given prose, then it reports each file's share of likely passive sentences (a form of "to be"
      followed by a past participle) when that share is over 10%, labelled as an estimate.
- [ ] Given a project with a `DOMAIN_DICTIONARY.md`, then it reports a term from any table's "Synonyms to
      AVOID" column used in prose, naming the term to use. A word listed there that is also ordinary
      English (such as `process` or `result`) only counts when it is the listed term's exact form, and
      the check has a tested way to skip a listed word.
- [ ] Given `<!-- docstyle-ignore: <rule> -->` on the line before a finding, then it is skipped, and the
      report counts how many were skipped.
- [ ] Given `shared/rules/writing-style.md`, then it names the command and says which rules it checks and
      which remain for a reviewer.
- [ ] Given tests, then each rule has a fixture that triggers it and a fixture that does not, and each
      ignored region has a test showing a rule violation inside it is not reported.
- [ ] Given the change is complete, then every new or changed function has cyclomatic complexity
      below 7, every changed line is covered by a test that can fail, and no file outside
      `scope.paths` changes, the run's own delivery record under
      `docs/features/doc-style-check/` excepted.

## Out of Scope

- Rewriting any existing doc. Running it over the repository and fixing what it finds is a later order.
- Making it a pre-ship check or an agent step. Decide that after seeing its false-positive rate.
- The STE approved-word dictionary. The rule's exception for domain and engineering terms stands.

## Domain Language

- No new terms introduced.

## Non-Functional Requirements

- It reads only the paths it is given and the project's `DOMAIN_DICTIONARY.md`, and needs no network.

## Trust Boundaries

- None — no auth, no user input beyond local paths, no external calls

## Test Approach

Go unit tests in `internal/docstyle` with Markdown fixtures, and a `cmd/loom/cmd` test running the command.

## Open Questions

- What a "list step" is: Assumption if unresolved: an item in a numbered list.

## Infrastructure / Deploy Notes

- None
