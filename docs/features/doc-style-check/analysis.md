---
feature: "doc-style-check"
bounded_context: "Documentation Knowledge Base"
domain_terms: ["Rule"]
files_touched: ["internal/docstyle/markdown.go", "internal/docstyle/finding.go", "internal/docstyle/sentences.go", "internal/docstyle/vocabulary.go", "internal/docstyle/modals.go", "internal/docstyle/passive.go", "internal/docstyle/dictionary.go", "internal/docstyle/ignore.go", "internal/docstyle/check.go", "internal/docstyle/report.go", "internal/docstyle/*_test.go and internal/docstyle/testdata/*.md", "cmd/loom/cmd/docs.go", "cmd/loom/cmd/docs_check.go", "cmd/loom/cmd/docs_check_test.go", "shared/rules/writing-style.md"]
issue_refs: ["doc-style-check"]
linked_adrs: []
linked_kis: []
---

# Feature Analysis: doc-style-check

## Summary

Add `loom docs check <path>...`, a new Go package `internal/docstyle` plus a `cmd/loom/cmd/docs_check.go` cobra subcommand under the existing `docs` command family, that mechanically reports where a Markdown file breaks the deterministic parts of `shared/rules/writing-style.md` (Simplified Technical English). It strips frontmatter, fenced/inline code, tables, headings, link targets and HTML comments before scanning prose, then reports over-length sentences and list steps, banned plain-vocabulary words, RFC-2119 modal conflicts, an estimated passive-voice share per file, and `DOMAIN_DICTIONARY.md` 'Synonyms to AVOID' violations — each finding rendered as `file:line: rule: excerpt`. `<!-- docstyle-ignore: <rule> -->` suppresses the next finding and is counted. The command exits 0 by default and 1 under `--strict`. It is a report, not a gate: it finds candidates for a human or agent to fix, and today only `tech-writer`, `spec-writer` and `analyst` even read the rule it mechanizes. No new dependency is added; the noun-cluster rule (more than 3 consecutive nouns) is deliberately left unimplemented because reliable detection needs part-of-speech information unavailable without one, and `shared/rules/writing-style.md` is updated to name the command and say plainly which rules it checks and which stay with a reviewer.

### Acceptance Criteria

- Given `loom docs check <path>...`, with a file or a directory, it reports each finding as `file:line: rule: excerpt` and exits 0 by default; given `--strict`, it exits 1 when there is a finding.
  - Example: `loom docs check README.md` with one finding prints the finding and exits 0.
  - Example: `loom docs check README.md --strict` with the same finding exits 1; with zero findings it exits 0.
- Given a Markdown file, the check ignores frontmatter, fenced and inline code, tables, headings, links' targets and HTML comments, and does not count those words toward any rule.
  - Example: A fenced code block containing `utilize` is never reported.
  - Example: A heading of 30 words is never reported for sentence length.
  - Example: A link `[utilize this](https://example.com/utilize)` reports nothing for the URL and nothing for 'utilize' inside the URL, but the visible link text is still scanned like ordinary prose.
- Given prose, the check reports a descriptive sentence over 25 words and a numbered-list step over 20 words.
  - Example: A 26-word paragraph sentence is reported as a sentence-length finding.
  - Example: A numbered-list item of 21 words is reported; per the spec's own Open Question assumption, 'list step' means an item in a numbered list, so a bulleted item of 21 words is judged against the 25-word descriptive threshold instead.
- Given prose, the check reports a run of more than three consecutive nouns only if a deterministic check can do it without a new dependency; this feature decides it cannot, and records that decision — in the tool's own output/help text and in `shared/rules/writing-style.md` — rather than shipping a guess.
  - Example: Decision recorded: left out. Reliable consecutive-noun detection needs part-of-speech tagging, which is not in the Go standard library or anywhere already in go.mod, and a crude capitalization/word-list heuristic would not be a deterministic check of the grammatical property the rule names.
- Given prose, the check reports words the rule replaces (`utilize`, `prior to`, `with regard to`), naming the preferred plain-vocabulary replacement.
  - Example: 'prior to shipping' reports a finding naming 'before' as the replacement for 'prior to'.
  - Example: 'utilize the API' reports a finding naming 'use' as the replacement for 'utilize'.
- Given prose, the check reports `can`, `might` or `ought to` in a sentence that also contains `MUST`, `SHOULD` or `MAY`.
  - Example: 'The service MUST retry, but it might also log the failure.' is reported.
  - Example: A sentence with only 'might' and no RFC-2119 keyword is not reported.
- Given prose, the check reports each file's share of likely passive sentences (a form of 'to be' followed by a past participle) when that share is over 10%, labelled as an estimate.
  - Example: A file where 3 of 20 prose sentences (15%) match the 'to be' + past-participle pattern reports one file-level finding labelled 'estimate: ~15% passive'.
  - Example: A file at or below 10% reports nothing.
- Given a project with a `DOMAIN_DICTIONARY.md`, the check reports a term from any table's 'Synonyms to AVOID' column used in prose, naming the canonical term to use; a listed word that is also ordinary English counts only in its exact listed surface form, and the check has a tested, explicit way to skip a listed word entirely.
  - Example: Dictionary lists `bot` as a synonym to avoid for 'Agent'; prose using 'bot' is reported, naming 'Agent'.
  - Example: Dictionary lists `data` (ordinary English, flagged in `design-principles.md` itself as too generic to enforce) on an explicit, tested skip-list, so prose using 'data' is never reported for that entry even though it is in the Synonyms column.
- Given `<!-- docstyle-ignore: <rule> -->` on the line before a finding, the finding is skipped, and the report counts how many findings were skipped this way.
  - Example: A 30-word sentence preceded on the prior line by `<!-- docstyle-ignore: sentence-length -->` produces no finding, and the report's skipped count increases by one.
- Given `shared/rules/writing-style.md`, it names the `loom docs check` command and states plainly which of its rules the command checks mechanically and which rules remain for a human or agent reviewer.
- Given tests, each rule has a fixture that triggers it and a fixture that does not, and each ignored region (frontmatter, fenced code, inline code, tables, headings, link targets, HTML comments) has a test showing a rule violation inside it is not reported.
- Given the change is complete, every new or changed function has cyclomatic complexity below 7, every changed line is covered by a test that can fail, and no file outside `scope.paths` changes except this run's own delivery record under `docs/features/doc-style-check/`.

### Non-Functional Requirements

- **security**: The check reads only the paths it is given on the command line and the project's `DOMAIN_DICTIONARY.md`; it makes no network call and has no trust boundary beyond local filesystem paths.

## Proposed Fitness Functions

- **Every new or changed function has cyclomatic complexity below 7**
  - **Verification**: golangci-lint run — gocyclo min-complexity: 7 is already configured in .golangci.yml
  - **Owner**: architect
- **Every changed line is covered by a test that can fail (ADR-008)**
  - **Verification**: go test ./internal/docstyle/... ./cmd/loom/cmd/... — a trigger fixture and a non-trigger fixture per rule, plus one test per ignored-region type showing a violation inside it is suppressed (AC covering test fixtures)
  - **Owner**: qa-engineer
- **No new go.mod/go.sum dependency is introduced**
  - **Verification**: git diff go.mod go.sum is empty for this change
  - **Owner**: architect
- **No file outside scope.paths (internal/docstyle/**, cmd/loom/cmd/docs*.go, shared/rules/writing-style.md, docs/features/doc-style-check/**) changes**
  - **Verification**: git diff --stat reviewed against the claim's scope.paths before merge
  - **Owner**: qa-engineer

## Out of Scope

- Rewriting any existing doc that the check flags — running it over the repository and fixing findings is a later piece of work.
- Making `loom docs check` a pre-ship check or an agent pipeline step — that decision waits until its false-positive rate is known.
- The STE approved-word dictionary — the writing-style rule's exception for domain and engineering terms (DOMAIN_DICTIONARY.md terms, words like `payload`, `idempotent`, `middleware`) stands and is not re-litigated by this check.
- Implementing the consecutive-noun-cluster rule — left out per AC3's own escape hatch, with the reason recorded rather than guessed at with a new dependency.
- Verifying active voice as a whole, or one-term-one-meaning across documents — both need judgement this mechanical check cannot deterministically make; the passive-voice-share estimate and the domain-dictionary synonym check are the two proxies this feature ships instead.

## Technical Breakdown

None

### Bounded Context

- **Owning Context**: Documentation Knowledge Base
- **Context Crossings**: Craftsmanship Governance

### Domain Events (Event Storming Lite)

- **Commands**: Check(paths []string, strict bool)
- **Events Produced**: None
- **Owning Aggregates**: None
- **Read Models / Projections**: None

### Affected Components

- `internal/docstyle/markdown.go` — New: strips frontmatter (reimplementing the fence-splitting pattern from internal/intake/parse.go, since that package's Parse decodes an unrelated Order schema) and classifies each line/span as ignored (fenced/inline code, tables, headings, link targets, HTML comments) so every other rule only ever sees prose.
- `internal/docstyle/finding.go` — New: Finding{File, Line, Rule, Excerpt} with a String() method rendering `file:line: rule: excerpt`, the closest in-repo precedent being internal/patternspec/issue.go's Issue.String().
- `internal/docstyle/sentences.go` — New: splits prose into sentences, tags each as a numbered-list step or descriptive text, and reports AC3's 20/25-word thresholds.
- `internal/docstyle/vocabulary.go` — New: AC's plain-vocabulary word/phrase list (`utilize`, `prior to`, `with regard to`) and its replacement suggestions.
- `internal/docstyle/modals.go` — New: flags a sentence containing both an RFC-2119 keyword (MUST/SHOULD/MAY) and a weak modal (can/might/ought to).
- `internal/docstyle/passive.go` — New: heuristic 'to be' + past-participle passive-sentence detector and the per-file >10% share estimate.
- `internal/docstyle/dictionary.go` — New: parses DOMAIN_DICTIONARY.md's 'Synonyms to AVOID' table column into a synonym-to-canonical map and an explicit, tested skip-list for ordinary-English entries. Reimplements (cannot import, due to Go's internal/ visibility rules) the table-row/backtick/comma-split parsing pattern already proven in shared/mcp/internal/analyzers/ubiquitous_language_analyzer.go.
- `internal/docstyle/ignore.go` — New: recognizes `<!-- docstyle-ignore: <rule> -->` on the line before a finding, suppresses that finding, and counts the suppression.
- `internal/docstyle/check.go` — New: Check(paths []string) (Report, error) — walks a file or directory (filepath.WalkDir, .md files, skipping dotdirs), runs every rule per file, and aggregates findings and the skipped count.
- `internal/docstyle/report.go` — New: renders a Report to the `file:line: rule: excerpt` text format plus the skipped-findings summary line.
- `internal/docstyle/*_test.go and internal/docstyle/testdata/*.md` — New: per AC7, one trigger and one non-trigger fixture per rule, and one fixture per ignored-region type showing a violation inside it is not reported.
- `cmd/loom/cmd/docs.go` — Modify: register the new `docsCheckCmd` in init()'s docsCmd.AddCommand(...) call, and add a `strict bool` field to the shared docsFlags struct, following the existing docsPlanCmd/docsApproveCmd/docsApplyCmd pattern.
- `cmd/loom/cmd/docs_check.go` — New: the `loom docs check <path>...` cobra.Command — Args: cobra.MinimumNArgs(1), a --strict flag, calls docstyle.Check, prints the report, and returns a sentinel error (exit 1) only when --strict and findings exist.
- `cmd/loom/cmd/docs_check_test.go` — New: command-level test with captured IO (matching docs_test.go's runDocsCommand convention), covering default exit 0, --strict exit 1 on findings, and --strict exit 0 on none.
- `shared/rules/writing-style.md` — Modify: AC6 — add a short note naming `loom docs check` and listing which of this file's rules it checks mechanically versus which stay with a reviewer (active voice as a whole, noun clusters, one-term-one-meaning).

### Data Model Changes

- No persisted data model. The check is stateless: it reads markdown files and DOMAIN_DICTIONARY.md from disk, holds findings in memory, and prints a report. (**none** phase)

### API Changes

None

### New Dependencies

None

## Task List

None

### Developer Tasks

1. Create internal/docstyle/markdown.go: strip a leading --- frontmatter fence (reimplementing the fence-splitting shape of internal/intake/parse.go's splitFrontmatter, not importing it), then classify each remaining line/span as ignored (fenced code via ``` tracking, inline code via backtick spans, table rows via leading |, headings via leading #, link targets via markdown [text](url) syntax keeping text and dropping the URL, and <!-- --> HTML comments single- or multi-line) versus prose.
2. Create internal/docstyle/finding.go: Finding{File string; Line int; Rule string; Excerpt string} with String() rendering `file:line: rule: excerpt`.
3. Create internal/docstyle/sentences.go: split prose into sentences, tag each as a numbered-list step or descriptive sentence, strip markdown emphasis markers before counting words, and report sentences over 20 words (list step) or 25 words (descriptive) as rule "sentence-length".
4. Create internal/docstyle/vocabulary.go: a word/phrase table {utilize: use, prior to: before, with regard to: about}; report a case-insensitive, word-boundary match in prose as rule "plain-vocabulary", naming the replacement.
5. Create internal/docstyle/modals.go: for each sentence, detect the presence of an RFC-2119 keyword (MUST, SHOULD, MAY) together with a weak modal (can, might, ought to) and report as rule "modal-conflict".
6. Create internal/docstyle/passive.go: detect a 'to be' form (is, are, was, were, be, being, been, am) immediately or near-immediately followed by a past participle (regex word ending "ed", plus a small irregular-participle list); compute the per-file ratio of flagged to total prose sentences and report one file-level finding labelled "estimate" as rule "passive-voice" when the ratio exceeds 0.10.
7. Create internal/docstyle/dictionary.go: parse DOMAIN_DICTIONARY.md's `| **Term** | Domain | Description | Synonyms to AVOID |` table rows, splitting the last column on backtick-quoted terms if present, else on commas, trimming parenthetical notes, into a synonym-to-canonical map; apply an explicit, tested skip-list (e.g. data, info, handler, manager, process, result) for ordinary-English listed words; match prose with case-sensitive, word-boundary exact-form matching and report as rule "ubiquitous-language", naming the canonical term.
8. Create internal/docstyle/ignore.go: recognize `<!-- docstyle-ignore: <rule> -->` on the line immediately before a reported finding's line, matching the <rule> name against the finding's Rule, and suppress it; track and return a skipped-findings count.
9. Create internal/docstyle/check.go: Check(paths []string) (Report, error) — for each path, if a directory, filepath.WalkDir skipping dot-directories and non-.md files; for each .md file, strip ignored regions via markdown.go, run every rule (sentences, vocabulary, modals, passive, dictionary), apply ignore.go's suppression, and aggregate into a Report{Findings []Finding, Skipped int} sorted by file then line.
10. Create internal/docstyle/report.go: Render(Report) string producing one `file:line: rule: excerpt` line per finding plus a trailing "N finding(s) skipped via docstyle-ignore" summary line.
11. In cmd/loom/cmd/docs.go: add a `strict bool` field to docsFlags and wire a new `docsCheckCmd` into docsCmd.AddCommand(...) inside init(), following the existing docsPlanCmd/docsApproveCmd/docsApplyCmd registration pattern.
12. Create cmd/loom/cmd/docs_check.go: docsCheckCmd := &cobra.Command{Use: "check <path>...", Args: cobra.MinimumNArgs(1), RunE: runDocsCheck}; register a `--strict` bool flag; runDocsCheck calls docstyle.Check(args), prints docstyle.Render(report) to cmd.OutOrStdout(), and returns a sentinel error only when docsArgs.strict is true and len(report.Findings) > 0 (nil otherwise, giving exit 0 by default per AC1).
13. In shared/rules/writing-style.md, add a short paragraph naming `loom docs check <path>...` and stating plainly which rules it checks mechanically (sentence length, plain vocabulary, modal conflicts, an estimated passive-voice share, and DOMAIN_DICTIONARY.md synonym violations) and which rules remain for a human or agent reviewer (active voice as a whole, noun clusters, one-term-one-meaning) — written in the file's own Simplified Technical English style.

### QA Tasks

1. In internal/docstyle, add a trigger fixture and a non-trigger fixture for each rule (sentence-length, plain-vocabulary, modal-conflict, passive-voice, ubiquitous-language) and assert the exact expected Finding set, covering AC3–AC5 and AC7's first clause.
2. Add one fixture per ignored-region type (frontmatter, fenced code block, inline code span, a table row, a heading, a link target, an HTML comment) each containing a rule violation inside the ignored region, and assert zero findings from that region — covering AC2 and AC7's second clause.
3. Add a test for `<!-- docstyle-ignore: <rule> -->`: a fixture with a finding immediately preceded by the comment naming that rule asserts the finding is absent and Report.Skipped increased by one; a second fixture where the comment names a different rule than the actual finding asserts the finding is still reported — covering AC6/ignore semantics.
4. Add a dictionary test using a small fixture DOMAIN_DICTIONARY.md (not the real ~500-line project one): one entry whose synonym is reported when used in prose in its exact surface form but not when stemmed/pluralized, and one entry on the skip-list that is never reported even though it is a listed synonym — covering AC5's exact-form and skip-list requirements.
5. Add cmd/loom/cmd/docs_check_test.go: run `loom docs check` against a fixture repo with one known finding — assert default invocation exits 0 and prints the finding; assert --strict exits non-nil (1); assert --strict against a fixture with zero findings exits nil (0) — covering AC1.
6. Add a test asserting `loom docs check` against a directory argument recurses into subdirectories and reports findings from nested .md files, and a test for a path that does not exist returning a clear error.
7. Run go test ./internal/docstyle/... ./cmd/loom/cmd/... and confirm it passes.
8. Run golangci-lint run (or confirm CI's existing gocyclo check) against every new function in internal/docstyle and cmd/loom/cmd/docs_check.go and confirm complexity stays below 7.
9. Confirm with git diff --stat and git diff go.mod go.sum that only internal/docstyle/**, cmd/loom/cmd/docs*.go, shared/rules/writing-style.md, and docs/features/doc-style-check/** changed, and that go.mod/go.sum are untouched.

### Tech Writer Tasks

1. Confirm shared/rules/writing-style.md's new paragraph (AC6) itself follows the file's own Simplified Technical English rules — active voice, sentences under 25 words, MUST/SHOULD/MAY used correctly — and accurately matches the rules the shipped check actually implements versus leaves to a reviewer.

### DevOps Tasks

None

## Edge Cases and Risks

- An empty .md file, or a path with no .md files at all (file or directory).: Report zero findings and exit 0; never error on the absence of findings.
- A path argument that does not exist on disk.: Return a clear error naming the missing path; this is a usage error, not a finding, and is not affected by --strict.
- A sentence spans across a fenced code block boundary (prose, then a code fence, then more prose on the next line).: Treat an ignored region as a hard sentence boundary so prose before and after a fence is never concatenated into one sentence.
- Unclosed/malformed frontmatter (an opening --- with no closing ---).: Fail open: treat the whole file as body/prose rather than erroring the entire check, since this is a report, not a build gate.
- No DOMAIN_DICTIONARY.md exists in the project.: Skip the AC5 dictionary check entirely for that run; every other rule still applies, per AC5's own framing ('given a project with a DOMAIN_DICTIONARY.md').
- A `<!-- docstyle-ignore: <rule> -->` comment with no finding on the following line (nothing to suppress).: Not counted in the skipped total — the count reflects findings actually suppressed, not comment occurrences.
- A DOMAIN_DICTIONARY.md Synonyms column mixes backtick-quoted terms, plain comma-separated terms, and parenthetical notes (e.g. `role` (too generic)) in the same cell.: Prefer backtick-quoted terms when present in a cell, else split on commas, and always strip a trailing parenthetical before matching — the same precedence proven in shared/mcp's existing (non-importable) parser.
- A bulleted (unordered) list item versus a numbered (ordered) list item.: Per the spec's own Open Question assumption, only a numbered list item is judged against the 20-word 'list step' threshold; a bulleted item is judged against the 25-word descriptive threshold.

## Definition of Done

- go test ./internal/docstyle/... ./cmd/loom/cmd/... passes.
- A fixture (trigger and non-trigger) exists and passes for each rule, and a fixture exists and passes for each ignored-region type, per AC7.
- `loom docs check <path>...` exits 0 by default regardless of findings, and exits 1 under --strict when findings exist, verified by a command-level test.
- shared/rules/writing-style.md names `loom docs check` and states which rules it checks mechanically and which remain for a reviewer (AC6).
- No new entry is added to go.mod or go.sum.
- Every new or changed function has cyclomatic complexity below 7, verified by golangci-lint's configured gocyclo rule.
- git diff touches only internal/docstyle/**, cmd/loom/cmd/docs*.go, shared/rules/writing-style.md, and docs/features/doc-style-check/** (this run's own delivery record) — no other file changes.
- code-reviewer approval recorded; no security-sensitive surface is touched (local file reads only, no network, no auth).
