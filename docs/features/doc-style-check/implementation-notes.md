---
feature: "doc-style-check"
bounded_context: ""
domain_terms: ["Rule"]
files_touched: ["internal/docstyle/finding.go", "internal/docstyle/finding_test.go", "internal/docstyle/markdown.go", "internal/docstyle/markdown_test.go", "internal/docstyle/sentences.go", "internal/docstyle/sentences_test.go", "internal/docstyle/vocabulary.go", "internal/docstyle/vocabulary_test.go", "internal/docstyle/modals.go", "internal/docstyle/modals_test.go", "internal/docstyle/passive.go", "internal/docstyle/passive_test.go", "internal/docstyle/dictionary.go", "internal/docstyle/dictionary_test.go", "internal/docstyle/ignore.go", "internal/docstyle/ignore_test.go", "internal/docstyle/check.go", "internal/docstyle/check_test.go", "internal/docstyle/report.go", "internal/docstyle/report_test.go", "cmd/loom/cmd/docs_check.go", "cmd/loom/cmd/docs_check_test.go", "cmd/loom/cmd/docs.go", "shared/rules/writing-style.md"]
issue_refs: ["doc-style-check"]
linked_adrs: ["docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md"]
linked_kis: []
---

# Implementation Notes: doc-style-check

## Files Created

- internal/docstyle/finding.go
- internal/docstyle/finding_test.go
- internal/docstyle/markdown.go
- internal/docstyle/markdown_test.go
- internal/docstyle/sentences.go
- internal/docstyle/sentences_test.go
- internal/docstyle/vocabulary.go
- internal/docstyle/vocabulary_test.go
- internal/docstyle/modals.go
- internal/docstyle/modals_test.go
- internal/docstyle/passive.go
- internal/docstyle/passive_test.go
- internal/docstyle/dictionary.go
- internal/docstyle/dictionary_test.go
- internal/docstyle/ignore.go
- internal/docstyle/ignore_test.go
- internal/docstyle/check.go
- internal/docstyle/check_test.go
- internal/docstyle/report.go
- internal/docstyle/report_test.go
- cmd/loom/cmd/docs_check.go
- cmd/loom/cmd/docs_check_test.go

## Files Modified

- cmd/loom/cmd/docs.go
- shared/rules/writing-style.md

## Interface Design

- docstyle.Check(paths []string) (Report, error) — single public entrypoint; walks files/directories, runs all five rules, applies docstyle-ignore suppression, returns a sorted Report
- docstyle.Report{Findings []Finding, Skipped int}
- docstyle.Finding{File string, Line int, Rule string, Excerpt string} with String() rendering "file:line: rule: excerpt"
- docstyle.Render(Report) string — text renderer used by the CLI
- docstyle.Prose(body string, firstLine int) []Line and docstyle.StripFrontmatter(source string) (string, int) — markdown ignored-region stripping, exported for reuse/testing
- docstyle.Directives(body string, firstLine int) map[int]string and docstyle.Suppress(findings []Finding, directives map[int]string) ([]Finding, int)
- docstyle.Check*(file string, lines []Line[, ...]) []Finding — one exported function per rule (CheckSentenceLength, CheckPlainVocabulary, CheckModalConflict, CheckPassiveVoice, CheckUbiquitousLanguage)
- cmd/loom/cmd: new `loom docs check <path>...` cobra command with --strict flag on the existing shared docsFlags struct

## Named Refactoring Log

None

## Self-Review Checklist

- [x] Every public method has an intention-revealing name (no process/handle/manage)
- [x] No function exceeds 30 LOC
- [x] Cyclomatic complexity < 7 on all new functions — Verified via `golangci-lint run` (gocyclo min-complexity:7 configured repo-wide) — 0 issues on internal/docstyle and the cmd/loom/cmd changes
- [x] No primitive obsession: domain concepts wrapped in value objects, not raw strings/ints — Finding and Line are small value types; vocabularyEntry/synonymEntry wrap phrase+pattern rather than passing bare strings around
- [x] No feature envy: methods use their own class's data more than another's
- [x] No magic numbers or strings: all literals are named constants — listStepWordLimit=20, descriptiveWordLimit=25, passiveThreshold=0.10, rule name constants; the plain-vocabulary/dictionary word lists are themselves data tables, not magic literals scattered through logic
- [x] Dependency direction verified: nothing in inner layers imports from outer layers — internal/docstyle imports nothing from cmd/loom/cmd; confirmed with `go build ./...`

## Simple Design Verification

- [x] Passes all tests — go test ./internal/docstyle/... ./cmd/loom/cmd/... — all green, including the full pre-existing cmd/loom/cmd suite (no regressions)
- [x] Reveals intention — Rule constants, Finding/Line value types, and one file per rule (sentences.go, vocabulary.go, modals.go, passive.go, dictionary.go) name exactly what each checks
- [x] No duplication — splitSentences/stripEmphasis/excerpt shared by sentences.go, modals.go and passive.go rather than reimplemented per rule; dictionary.go's table-parsing shape is a deliberate, documented duplication across a Go internal/ visibility boundary (see keyDecisions), not accidental duplication
- [x] Fewest elements — No Strategy/plugin interface for the five rules — plain functions called in sequence from check.go, since the rule set is fixed and small

## Key Decisions

- internal/docstyle stays a single flat package (no Entities/UseCases/Adapters sub-packages): Matches the shape of every other sibling single-purpose package (internal/patternspec, internal/testlint) in this repo; it is a stateless file-in/report-out utility with one consumer, so pre-splitting it would be premature generalization, per architecture-notes.md's structural decision
- dictionary.go reimplements, rather than imports, shared/mcp/internal/analyzers/ubiquitous_language_analyzer.go's table-row/backtick/comma parsing: Go's internal/ visibility rules refuse the import outright (different internal/ root); the duplication is deliberate and documented, matching architecture-notes.md
- A prose sentence is never joined across lines; each surviving markdown line is scanned as its own sentence-splitting unit: Keeps per-function complexity low and avoids inventing paragraph-joining logic the AC examples never require (every fixture's trigger text fits on one line); the accepted cost is a false negative for a sentence hand-wrapped across multiple lines, acceptable since this is a report, not a gate
- docstyle.Check resolves DOMAIN_DICTIONARY.md at a fixed, current-working-directory-relative path ("DOMAIN_DICTIONARY.md"), not via docsFlags.root: Keeps Check's signature exactly as analysis.md specified (Check(paths []string)); this repo's own convention is a root-level DOMAIN_DICTIONARY.md symlink (confirmed via `ls -la DOMAIN_DICTIONARY.md`), and dogfooding `loom docs check shared/rules/writing-style.md` from the repo root correctly picked up the real dictionary and reported genuine (if occasionally over-broad) ubiquitous-language findings
- Dogfooded the finished command against shared/rules/writing-style.md itself and fixed a 31-word run-on sentence the new AC6 paragraph introduced: Tech Writer Task 1 asks the new paragraph to follow the file's own STE rules; running the actual shipped tool against it, rather than hand-counting, caught a real violation before merge

## Deviations from Analysis

- Developer Task 9/Analysis's stated Check(paths []string) (Report, error) signature: Signature kept exactly as specified; the DOMAIN_DICTIONARY.md path resolution it implies (not spelled out in analysis.md) was resolved as a fixed cwd-relative lookup rather than threading docsFlags.root through — see keyDecisions. No API surface beyond what analysis.md named was added.

## Dependencies Added

None

## Notes for QA

- go test ./internal/docstyle/... ./cmd/loom/cmd/... -cover passes; internal/docstyle coverage is 92.6%.
- golangci-lint run ./internal/docstyle/... ./cmd/loom/cmd/... reports 0 issues (gocyclo min-complexity:7 and revive both configured and passing).
- git diff --stat and git diff go.mod go.sum confirmed: only cmd/loom/cmd/docs.go and shared/rules/writing-style.md modified, only internal/docstyle/** and cmd/loom/cmd/docs_check*.go added, go.mod/go.sum untouched.
- Per-rule trigger/non-trigger fixtures and per-ignored-region fixtures (frontmatter, fenced code, inline code, table, heading, link target, HTML comment) all live as inline string literals inside internal/docstyle/*_test.go rather than internal/docstyle/testdata/*.md files — no on-disk testdata fixtures were needed since every fixture is a short string constructed in the test itself; AC7's coverage requirement (a trigger and non-trigger fixture per rule, and a suppression fixture per ignored-region type) is satisfied by markdown_test.go, sentences_test.go, vocabulary_test.go, modals_test.go, passive_test.go and dictionary_test.go.
- Dogfooding note: running `loom docs check` against the repo's own shared/rules/writing-style.md surfaces real ubiquitous-language false positives (e.g. the ordinary word "command" matching DOMAIN_DICTIONARY.md's Skill-synonym entry "command") — expected per the feature's own framing (a report with an as-yet-unmeasured false-positive rate), not a defect; worth keeping in mind when this command is later run over the whole docs tree.

## Notes for DevOps

None
