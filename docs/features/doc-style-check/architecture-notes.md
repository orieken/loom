---
feature: "doc-style-check"
bounded_context: "Documentation Knowledge Base"
domain_terms: ["Rule", "Simplified Technical English"]
files_touched: ["internal/docstyle/finding.go", "internal/docstyle/markdown.go", "internal/docstyle/sentences.go", "internal/docstyle/vocabulary.go", "internal/docstyle/modals.go", "internal/docstyle/passive.go", "internal/docstyle/dictionary.go", "internal/docstyle/ignore.go", "internal/docstyle/check.go", "internal/docstyle/report.go", "cmd/loom/cmd/docs_check.go", "cmd/loom/cmd/docs.go", "shared/rules/writing-style.md"]
issue_refs: []
linked_adrs: ["docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md"]
linked_kis: []
---

# Architecture Notes: doc-style-check

## Structural Decisions

### internal/docstyle is a new, flat utility package — Finding, markdown classification, and each rule (sentences, vocabulary, modals, passive, dictionary, ignore) live as sibling files in one package, with Check(paths []string) (Report, error) as the single public entrypoint. No Entities/UseCases/Adapters sub-packages.
- **Rationale**: Verified against existing sibling packages (internal/patternspec, internal/testlint, internal/diffcover): each is a flat, single-purpose package with one exported entrypoint consumed directly by a cmd/loom/cmd/*.go file — none splits into Clean Architecture sub-layers. go-conventions.md's Entities→UseCases→Adapters→Frameworks layering governs services with real domain/use-case/adapter separation and multiple consumers; a stateless, file-in/report-out lint utility with one consumer does not have that shape, and pre-splitting it would be the Premature Generalization anti-pattern named in design-principles.md. Reversibility: Cheap — splitting later, if a second consumer appears, is a mechanical extraction.
- **Trade-offs**:
  - If docstyle later gains a second consumer with different formatting needs, Render/Report may need an interface extracted from report.go at that point — deliberately deferred, not pre-built.
- **Fitness Function**: internal/docstyle is imported only by cmd/loom/cmd/docs_check.go and its own tests; no reverse dependency from internal/docstyle back into cmd/loom/cmd exists — verified by go build ./... — Go's own import-cycle rules and the existing cmd→internal dependency direction make the reverse import impossible without an explicit, visible new import; no new lint rule needed beyond the existing build
### Add a `strict bool` field to the existing shared docsFlags struct in cmd/loom/cmd/docs.go (not a separate flags struct for docs_check.go), and register docsCheckCmd in the same init() alongside docsPlanCmd/docsApproveCmd/docsApplyCmd.
- **Rationale**: Confirmed in cmd/loom/cmd/docs_test.go: runDocsCommand's helper does `docsArgs = flags` wholesale and every existing docs subcommand test reuses that one package-level docsFlags struct, even for fields scoped to a single subcommand (inTreeReadmes, docComments, noCodemap, codemapOnly are all plan-only, yet live in the shared struct, not a plan-specific one). Matching that precedent lets docs_check_test.go reuse the exact same test harness rather than introducing a second, inconsistent flags convention for one new subcommand. Reversibility: Cheap.
- **Trade-offs**:
  - docsFlags keeps accumulating fields that only one subcommand reads — accepted existing debt (see refactoring opportunity below), not newly introduced by this feature, and not this feature's job to fix.
- **Fitness Function**: docs_check_test.go compiles against and calls the existing runDocsCommand(t, run, flags, input, args...) helper, exactly like the plan/approve/apply tests — verified by go test ./cmd/loom/cmd/...
### Finding is a new type in internal/docstyle/finding.go — Finding{File, Line, Rule, Excerpt} with String() rendering `file:line: rule: excerpt`. It does not reuse or extend internal/patternspec.Issue.
- **Rationale**: patternspec.Issue.String() renders 'file: field: message' (no Line field, a Severity field this feature has no use for) for a different bounded context — Pattern Spec validation, per DOMAIN_DICTIONARY.md — not doc style checking. Bending Issue to a second shape and a second meaning would be a Leaky Abstraction; a new four-field value type is cheaper and clearer. Reversibility: Cheap.
- **Fitness Function**: Finding.String() renders exactly `file:line: rule: excerpt` — verified by internal/docstyle/finding_test.go asserts the exact rendered string for a representative Finding
### internal/docstyle/dictionary.go reimplements the table-row/backtick/comma-split parsing approach already proven in shared/mcp/internal/analyzers/ubiquitous_language_analyzer.go (tableRowRe-equivalent regex, synonym comma/backtick splitting, parenthetical-note trimming) rather than importing that package.
- **Rationale**: Confirmed via go.mod: this repo's module root is github.com/orieken/loom; shared/mcp/internal/analyzers sits under a different internal/ visibility root (shared/mcp/), so Go's compiler refuses the import outright from internal/docstyle. There is no refactor that preserves both packages' internal/ privacy and shares the code without first exporting it from a non-internal location, which is out of this feature's scope. Verified DOMAIN_DICTIONARY.md's live table format (`| **Term** | Domain | Description | Synonyms to AVOID |`, backtick-quoted comma lists, trailing parenthetical notes like '(too generic)') matches what the existing analyzer already parses, so copying its proven regex shape is lower-risk than inventing a new one.
- **Trade-offs**:
  - A future change to DOMAIN_DICTIONARY.md's table format must be hand-applied to both parsers; nothing mechanically keeps them in sync.
- **Fitness Function**: judgment-only — Confirmed via go.mod: this repo's module root is github.com/orieken/loom; shared/mcp/internal/analyzers sits under a different internal/ visibility root (shared/mcp/), so Go's compiler refuses the import outright from internal/docstyle. There is no refactor that preserves both packages' internal/ privacy and shares the code without first exporting it from a non-internal location, which is out of this feature's scope. Verified DOMAIN_DICTIONARY.md's live table format (`| **Term** | Domain | Description | Synonyms to AVOID |`, backtick-quoted comma lists, trailing parenthetical notes like '(too generic)') matches what the existing analyzer already parses, so copying its proven regex shape is lower-risk than inventing a new one.
### The noun-cluster rule (writing-style.md's '>3 consecutive nouns') is explicitly not implemented, and that omission is itself a checked, testable fact — stated in `loom docs check`'s own help/Long text and in shared/rules/writing-style.md, in closely matching wording.
- **Rationale**: Per AC3's escape hatch: reliable consecutive-noun detection needs part-of-speech tagging unavailable in go.mod or the Go standard library; a crude capitalization/word-list heuristic would not deterministically check the grammatical property the rule names and would generate findings a human then has to triage, which is worse than silence. Recording the decision turns an invisible gap into a visible, verified one rather than an implied-but-false claim of full coverage. Reversibility: Cheap — reverses the moment a POS tagger is deliberately adopted, which would be its own decision given the 'no new dependency' constraint.
- **Fitness Function**: `loom docs check --help` and shared/rules/writing-style.md both state that the noun-cluster rule is not mechanically checked by this command — verified by cmd/loom/cmd/docs_check_test.go asserts the command's help/Long text contains the noun-cluster exclusion note; a second test greps shared/rules/writing-style.md for the matching note
### Check() returns Findings sorted by file then line; docs_check.go signals a --strict failure with a package-level sentinel error (e.g. errDocStyleFindingsFound) and relies entirely on cmd/loom/cmd/root.go's existing cobra.CheckErr(err) fallthrough for the exit(1) — root.go itself is not touched.
- **Rationale**: Confirmed in root.go's Execute(): a short list of named sentinel errors (errHealthFailed, errStateVerifyFailed, orchestrator.ErrWaitingApproval, orchestrator.ErrBudgetExhausted, context.Canceled) get special exit codes, then any other non-nil error falls through to cobra.CheckErr(err), which exits 1. AC1 only needs plain 0/1, no special code, so a plain sentinel reaching that fallthrough is correct and needs zero root.go changes. Reversibility: Cheap.
- **Fitness Function**: `loom docs check --strict` exits 1 when Findings is non-empty and 0 otherwise; default mode always exits 0 — verified by cmd/loom/cmd/docs_check_test.go — three cases: default exit 0 with findings present, --strict exit 1 with findings present, --strict exit 0 with zero findings
### No stability patterns (CircuitBreaker, retries, timeouts) are introduced for internal/docstyle.
- **Rationale**: architecture-guardrails.md #5 governs network calls; this feature's own NFR states it makes none — it reads only the file paths given on the command line and DOMAIN_DICTIONARY.md from local disk. Adding resilience machinery here would be unjustified ceremony for synchronous local file reads.
- **Fitness Function**: internal/docstyle imports no net/http, database/sql, or queue client package — verified by go list -deps ./internal/docstyle/... reviewed in code review; no CI gate is proposed since nothing here currently imports any network package

## Component Placement

- `docstyle.Finding` → Domain (flat package — see structural decision 1) layer, `internal/docstyle/finding.go` — Value type carrying File, Line, Rule, Excerpt; no behavior beyond String().
- `markdown region classifier (frontmatter/fence/table/heading/link/comment stripping)` → Domain (flat package) layer, `internal/docstyle/markdown.go` — Pure text classification every rule depends on; reimplements (does not import) internal/intake/parse.go's frontmatter-fence-splitting shape, since that package's Parse decodes an unrelated Order schema.
- `sentence-length rule` → Domain (flat package) layer, `internal/docstyle/sentences.go` — AC3's 20/25-word thresholds, list-step vs descriptive-sentence tagging.
- `plain-vocabulary rule` → Domain (flat package) layer, `internal/docstyle/vocabulary.go` — AC4's word/phrase replacement table.
- `modal-conflict rule` → Domain (flat package) layer, `internal/docstyle/modals.go` — AC's RFC-2119-vs-weak-modal detection.
- `passive-voice estimate` → Domain (flat package) layer, `internal/docstyle/passive.go` — AC's per-file 'to be' + past-participle share heuristic, labelled as an estimate.
- `DOMAIN_DICTIONARY.md synonym rule` → Domain (flat package) layer, `internal/docstyle/dictionary.go` — AC5's Synonyms-to-AVOID table parsing and skip-list; see structural decision on duplication across the Go internal/ visibility boundary.
- `docstyle-ignore suppression` → Domain (flat package) layer, `internal/docstyle/ignore.go` — Recognizes the ignore comment, suppresses the next finding, counts suppressions.
- `Check orchestrator` → Use Case layer, `internal/docstyle/check.go` — Single public entrypoint: walks paths, strips ignored regions, runs every rule, applies suppression, aggregates and sorts the Report. The one place that knows the full rule list.
- `Report renderer` → Adapter layer, `internal/docstyle/report.go` — Converts the in-memory Report into the `file:line: rule: excerpt` text format plus the skipped-count summary line — the output-formatting boundary.
- `docsCheckCmd` → Framework/CLI layer, `cmd/loom/cmd/docs_check.go` — The outermost cobra.Command layer: parses --strict, calls docstyle.Check, prints via cmd.OutOrStdout(), returns the sentinel error on --strict failure.
- `docsFlags.strict field + docsCheckCmd registration` → Framework/CLI layer, `cmd/loom/cmd/docs.go` — Extends the existing shared flags struct and init() wiring — see structural decision 2.
- `writing-style.md command note` → Documentation layer, `shared/rules/writing-style.md` — AC6: names the command and states which rules are mechanized vs reviewer-only.

## Bounded Context

- **Owning Context**: Documentation Knowledge Base
- **Context Crossings**: Craftsmanship Governance

## Stability Design

None

## Observability Design

None

## Layer Boundary Checks

- [PASS] internal/docstyle (Use Case/Domain) must not import cmd/loom/cmd/* (Framework layer) — Confirmed no internal/* package in this repo imports cmd/loom/cmd; the dependency direction is always cmd → internal, matching Clean Architecture's inward-pointing rule. No docstyle code needs anything cmd-layer provides.
- [PASS] internal/docstyle must not import shared/mcp/internal/analyzers — Go's internal/ visibility refuses this import outright — confirmed the two packages sit under different module-relative internal/ roots (shared/mcp/ vs the repo module root). This is why dictionary.go reimplements rather than imports (structural decision above).
- [PASS] cmd/loom/cmd/docs_check.go (Framework/CLI) may import internal/docstyle (Use Case) — correct inward-to-outward direction — Outer layer depending on inner layer is the Clean Architecture direction; no violation.

## Anti-Pattern Check

- Duplication (pattern, not bug) across an internal/ visibility boundary: FOUND — dictionary.go knowingly duplicates ubiquitous_language_analyzer.go's table-parsing regex approach because Go's internal/ visibility makes importing it impossible. Accepted and documented, not a defect — see the dedicated structural decision.
- Premature generalization: not found — Considered and rejected a Strategy-pattern per-rule plugin interface. design-principles.md reserves Strategy for behavior that varies by context; here the rule set is fixed and small (5 mechanized rules), so a dispatch-map/interface is speculative. Plain functions called in sequence from check.go is the simplest design that satisfies AC7.
- Shotgun surgery: not found — Adding a 6th rule later touches exactly two places — a new <rule>.go file and one call from check.go — not shotgun surgery.
- God object: not found — check.go orchestrates but each rule's logic lives in its own file; no single file does sentence-splitting, vocabulary matching, modal detection, passive detection, and dictionary parsing.
- Anemic domain model: not found — Not applicable in the DDD sense — this is a stateless report generator, not a persisted-entity system; Finding is a plain value type by design, not an entity missing behavior.

## Fitness Functions

- **Every new or changed function has cyclomatic complexity below 7**
  - **Verification**: golangci-lint run — gocyclo min-complexity: 7 is already configured in .golangci.yml at the repo root and applies to internal/ and cmd/ by default (only third_party$/builtin$/examples$ are excluded)
  - **Owner**: architect
- **Every changed line is covered by a test that can fail (ADR-008)**
  - **Verification**: go test ./internal/docstyle/... ./cmd/loom/cmd/... — one trigger and one non-trigger fixture per rule, plus one fixture per ignored-region type showing an in-region violation is suppressed
  - **Owner**: qa-engineer
- **No new go.mod/go.sum dependency is introduced**
  - **Verification**: git diff go.mod go.sum is empty for this change
  - **Owner**: architect
- **No file outside scope.paths (internal/docstyle/**, cmd/loom/cmd/docs*.go, shared/rules/writing-style.md, docs/features/doc-style-check/**) changes**
  - **Verification**: git diff --stat reviewed against the claim's scope.paths before merge
  - **Owner**: qa-engineer
- **internal/docstyle is imported only by cmd/loom/cmd/docs_check.go and its own tests**
  - **Verification**: go build ./... — Go's import-cycle rules and the existing cmd→internal direction make a reverse import structurally visible if ever attempted
  - **Owner**: architect
- **Finding.String() renders exactly `file:line: rule: excerpt`**
  - **Verification**: internal/docstyle/finding_test.go
  - **Owner**: qa-engineer
- **`loom docs check --help` and shared/rules/writing-style.md both state the noun-cluster rule is not mechanically checked**
  - **Verification**: cmd/loom/cmd/docs_check_test.go asserts help text content; a second test greps shared/rules/writing-style.md for the same note
  - **Owner**: qa-engineer
- **`loom docs check --strict` exits 1 only when Findings is non-empty; default mode always exits 0**
  - **Verification**: cmd/loom/cmd/docs_check_test.go — default/strict-with-findings/strict-without-findings cases
  - **Owner**: qa-engineer

## Refactoring Opportunities

- Introduce Parameter Object on `cmd/loom/cmd/docs.go docsFlags struct` — docsFlags will carry 8 fields spanning 4 unrelated subcommands' flags after this feature (root is persistent; run/inTreeReadmes/docComments/noCodemap/codemapOnly/yes are plan/approve-specific; strict is check-specific) — a Data Clump per subcommand. Not pursued now: it would touch the existing plan/approve/apply tests for no behavior change, which is out of this feature's scope. Flagged for a future pass if a 5th or 6th subcommand makes the shared struct noisier.

## Developer Handoff Notes

- internal/docstyle stays flat — one package, Check(paths []string) (Report, error) is the only symbol cmd/loom/cmd/docs_check.go calls. No sub-packages.
- Add `strict bool` to docsFlags in cmd/loom/cmd/docs.go and register docsCheckCmd via docsCmd.AddCommand(docsPlanCmd, docsApproveCmd, docsApplyCmd, docsCheckCmd) in the existing init().
- Print all report output through cmd.OutOrStdout() (matches docs.go:101,135,215), never fmt.Println/os.Stdout directly, so docs_check_test.go can capture it the same way docs_test.go's runDocsCommand helper does.
- Define one package-level sentinel error (e.g. errDocStyleFindingsFound) in docs_check.go. Return nil unless --strict && len(report.Findings) > 0. Do not touch root.go — its existing cobra.CheckErr(err) fallthrough already turns any other non-nil error into exit 1.
- dictionary.go must not try to import shared/mcp/internal/analyzers — Go will refuse it. Copy the regex approach (table-row match, backtick/comma synonym split, parenthetical-note trimming), don't attempt to share code across that internal/ boundary.
- Put the noun-cluster omission note in both docsCheckCmd's help/Long text and shared/rules/writing-style.md in closely matching wording — a test checks both.
- Sort Report.Findings by file then line in Check() so output is deterministic across test runs and across OS directory-walk orderings.

## Open Architectural Questions

None
