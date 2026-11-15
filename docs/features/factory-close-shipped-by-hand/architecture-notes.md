---
feature: "factory-close-shipped-by-hand"
bounded_context: "Factory supervisor — order lifecycle and ledger (ADR-010)"
domain_terms: ["order", "ledger", "phase", "shipped", "parked", "stopped", "worktree", "supervisor", "approver"]
files_touched: ["internal/factory/control.go", "internal/factory/finish.go", "internal/factory/replay.go", "cmd/loom/cmd/factory.go", "internal/factory/control_test.go", "cmd/loom/cmd/factory_inprocess_test.go", "templates/claude-feature-team/intake/README.md"]
issue_refs: []
linked_adrs: ["ADR-010"]
linked_kis: []
---

# Architecture Notes: factory-close-shipped-by-hand

## Structural Decisions

### Add Close as a method on a new small Closer{Layout, Repository} type in internal/factory/control.go, called as Closer{Layout: layout, Repository: repo}.Close(ledger, orders, request), where request is a new ShippedByHand{Order, URL, Approver} value — not a 7-parameter free function.
- **Rationale**: Close needs four things RequestStop/RequestResume/RequestRerun do not (layout and repo, to finish the order the way ship does) on top of the three every control function needs (ledger, orders, id), plus the URL and approver. Passed flat that is 7 parameters, over design-principles.md's 4-parameter hard limit. Bundling layout+repo as Closer's receiver fields and id/URL/approver as one ShippedByHand parameter object brings the method itself to 3 parameters (ledger, orders, request), using the same 'bundle dependencies into a receiver, let call-specific data flow as method arguments' shape Supervisor/Config/Dependencies already establishes in this package — not a new pattern, a smaller instance of one already present.
- **Trade-offs**:
  - Two new small types (Closer, ShippedByHand) for a single call site — more elements than a flat function, accepted only because the alternative (RequestStop-style flat params) breaks the parameter-count limit once layout+repo are added.
  - Close's shape now visibly differs from its three siblings in the same file (they stay flat functions); a reader has to know why. Documented here and in a doc comment on Closer, not resolved by also restructuring the siblings.
- **Fitness Function**: Close (the method) takes at most 4 parameters — verified by golangci-lint's existing complexity/style config, or inspection in code-reviewer — no dedicated param-count linter is confirmed to be wired, so this is read off the diff
### Do not touch RequestStop, RequestResume, or RequestRerun's signatures to bundle their own ledger+orders data clump, even though Close's addition makes the clump's repetition more visible across the file.
- **Rationale**: ledger+orders traveling together across every function in control.go is a genuine Fowler data clump, but those three functions are already shipped and tested; folding them into a shared parameter object is a refactor with no behavior change, and CLAUDE.md's Legacy Code & Refactoring rule requires refactoring and behavior changes to ship in separate commits. Bundling them now would mix an unrelated refactor into this feature's diff and risks exceeding scope.paths review expectations for this delivery.
- **Trade-offs**:
  - The data clump remains unaddressed for three functions; a future refactor commit should revisit it once this feature is not also in flight.
  - Close ends up differently shaped from its siblings (see decision above) rather than all four converging on one shared shape.
- **Fitness Function**: judgment-only — ledger+orders traveling together across every function in control.go is a genuine Fowler data clump, but those three functions are already shipped and tested; folding them into a shared parameter object is a refactor with no behavior change, and CLAUDE.md's Legacy Code & Refactoring rule requires refactoring and behavior changes to ship in separate commits. Bundling them now would mix an unrelated refactor into this feature's diff and risks exceeding scope.paths review expectations for this delivery.
### Extract the archive-then-remove-worktree sequence out of Supervisor.ship (internal/factory/finish.go) into a shared function archiveAndRemove(order *OrderState, layout Layout, repo Repository) string, returning the warning message (empty string on success) in the exact wording ship already uses. Supervisor.ship calls it after its own ledger write; Close calls it after its own. Add this extraction to finish.go in this change, alongside control.go/replay.go — one additional file beyond the analyst's list.
- **Rationale**: The analyst's task for Close asks it to repeat Supervisor.ship's archive-then-remove-worktree steps and its exact warning strings inline. That is the real duplication in this feature, not the parameter count — two call sites with the same three-step control flow and the same two error-message templates. Kent Beck's 'no duplication' and the Boy Scout Rule (finish.go is being touched to add the helper, so clean it up now) both point to extracting it once, used by both.
- **Trade-offs**:
  - finish.go moves from analyst's affected-components list of 5 files to 6; the extra touch is to a file the analyst did not flag, which review should expect and not read as scope creep.
  - archiveAndRemove drops ship's error into a Supervisor.say call indirectly (ship still does that), while Close returns the string for the CLI to print instead of writing to an io.Writer — the two callers format identically but report differently, which is correct (Close has no Output stream, the supervisor does) but worth a code comment so it isn't 'fixed' into uniformity later.
- **Fitness Function**: Supervisor.ship and Close call one shared archiveAndRemove function rather than each inlining archiveRun + RemoveWorktree + warning formatting — verified by code-reviewer reads finish.go and control.go and confirms no duplicated archive/remove/warning-formatting block exists between the two call sites
### closeable(orders, id, rawURL) (*OrderState, error) in control.go validates both id-existence (via the existing orderIn) and rawURL (via isHTTPURL) before checking phase, and refuses with its own fmt.Errorf using the same wording RequestRerun already uses ('%s is %s; stop it first, or it has already shipped') — as a duplicated literal string, not a shared helper function.
- **Rationale**: The wording match is an acceptance criterion (consistent human-facing refusal text), but extracting a shared helper would mean touching RequestRerun's body for a one-line format string shared by exactly two call sites — not worth a new abstraction, and it would again mix a RequestRerun-touching refactor into this feature's diff (see the ledger+orders decision above for the same reasoning).
- **Trade-offs**:
  - One format string is now written twice. Acceptable: duplicating a single English sentence is not the kind of duplication Kent Beck's rule is aimed at, and the alternative costs more than it saves for two call sites.
- **Fitness Function**: judgment-only — The wording match is an acceptance criterion (consistent human-facing refusal text), but extracting a shared helper would mean touching RequestRerun's body for a one-line format string shared by exactly two call sites — not worth a new abstraction, and it would again mix a RequestRerun-touching refactor into this feature's diff (see the ledger+orders decision above for the same reasoning).
### isHTTPURL(raw string) bool in control.go uses only net/url.Parse — true iff scheme is http or https and Host is non-empty. No network call, ever.
- **Rationale**: Satisfies the NFR that close must never fetch, open, or otherwise reach the --shipped-by URL. net/url.Parse is purely syntactic.
- **Fitness Function**: isHTTPURL and everything it calls perform no network I/O — verified by code review confirms the only import isHTTPURL needs is net/url; no CI check exists for the absence of a network call, so this is judgment-only until/unless a static-analysis rule for forbidden imports in this function is added
### Add entryUpdates[EventShipped] in internal/factory/replay.go: func(o *OrderState, e Entry) { if e.URL != "" { o.URL = e.URL } }. Leave Supervisor.ship's own EventShipped entry (which carries no URL) unaffected, and do not fold ApprovedBy onto OrderState.
- **Rationale**: status only needs to display the URL (AC2); the approver is preserved in the ledger entry itself, which is where provenance belongs — OrderState is a display fold, not an audit log. The guard on non-empty URL means this handler is a no-op for every EventShipped line the supervisor itself already writes, so no existing behavior changes.
- **Fitness Function**: An EventShipped entry with a non-empty URL sets OrderState.URL; an EventShipped entry with no URL leaves OrderState.URL unchanged — verified by control_test.go or a replay-focused test asserts both cases against Replay(), per ADR-008's changed-line coverage requirement
### closeable only ever permits order.Phase == PhaseParked or order.Phase == PhaseStopped; every other phase (including PhaseShipped) is refused. closeable reads orders fresh from openLedger()'s Replay() each CLI invocation, exactly like RequestStop/RequestResume/RequestRerun, rather than caching state across calls — the same TOCTOU-tolerant pattern, no new lock.
- **Rationale**: This is ADR-010's existing concurrency answer for every human-issued control: read the ledger fresh, validate current phase, append one line. Close needs no new mechanism because the existing one already covers 'close races the supervisor' the same way it covers 'stop races the supervisor'.
- **Fitness Function**: Two concurrent close/supervisor actions on the same order never both succeed in appending conflicting terminal ledger lines — verified by control_test.go (or factory_inprocess_test.go) exercises closeable against a phase that changed between an initial read and the append, the same test shape already used for approve/stop/rerun's TOCTOU handling
### Close never receives a PullRequests/GHPullRequests dependency and never imports net/http or os/exec for a GitHub call. Its only external dependencies are Ledger, the in-memory orders map, Layout, and Repository (reused unchanged from internal/factory/intake.go).
- **Rationale**: Structurally guarantees the 'close never calls GitHub' acceptance criterion by omission rather than by a runtime check: there is no reachable code path to gh or the GitHub API because Close's dependency set does not include one. Reuses the existing Repository interface (already defined in its consumer package, internal/factory, per the project's interface-placement convention) — no new interface needed.
- **Fitness Function**: control.go contains no import of net/http, os/exec, or the GHPullRequests type — verified by grep over control.go in code review; no CI lint rule currently forbids these imports per-file, so this is judgment-only until such a rule exists
### Wire the CLI as factoryCloseCmd (Use: "close <order>", a required --shipped-by string flag) in cmd/loom/cmd/factory.go, registered beside factoryStopCmd/factoryResumeCmd/factoryRerunCmd, with runFactoryClose calling openLedger() then Closer{Layout: layout, Repository: worktree.New(layout.Root)}.Close(ledger, orders, factory.ShippedByHand{Order: args[0], URL: shippedByFlagValue, Approver: approverName()}), printing the returned warning or a success line.
- **Rationale**: Matches every existing control subcommand's wiring exactly (same openLedger() helper, same worktree.New(layout.Root) construction already used in newFactorySupervisor, same approverName() source AC7 requires). No new CLI pattern introduced.
- **Fitness Function**: runFactoryClose exits 1 on any error Close/closeable returns, prints the order id and URL on success, and never calls any gh/GitHub code path — verified by cmd/loom/cmd/factory_inprocess_test.go asserts exit code and output for the success path and each refusal path named in the acceptance criteria
### No on-disk ledger schema change. Entry.URL and Entry.ApprovedBy are reused exactly as approve/pr-opened already write them; Close's ledger line is Entry{Event: EventShipped, Order: request.Order, URL: request.URL, ApprovedBy: request.Approver}.
- **Rationale**: The ledger format is already general enough — a 'shipped' line gains two fields it already supports, written by a new caller. No migration, no versioning concern.
- **Fitness Function**: judgment-only — The ledger format is already general enough — a 'shipped' line gains two fields it already supports, written by a new caller. No migration, no versioning concern.

## Component Placement

- `Closer, ShippedByHand, Close, closeable, isHTTPURL` → Application / orchestration (factory supervisor control) layer, `internal/factory/control.go` — Beside RequestStop/RequestResume/RequestRerun — the existing home for every human-issued control that reads the ledger, validates the order's current phase, and appends one line.
- `archiveAndRemove` → Application / orchestration (factory supervisor control) layer, `internal/factory/finish.go` — Extracted from Supervisor.ship's existing body so Close and ship share one archive-then-remove implementation instead of two copies; stays where archiveRun already lives.
- `entryUpdates[EventShipped] handler` → Domain fold (ledger replay) layer, `internal/factory/replay.go` — Every other ledger-to-OrderState projection rule already lives in this map; EventShipped's URL fold belongs beside EventPROpened's identical URL fold.
- `factoryCloseCmd, runFactoryClose` → Adapter (CLI) layer, `cmd/loom/cmd/factory.go` — The outer layer that turns a human's command line into a call on the factory package's Closer — same placement as every other factory subcommand.
- `Close/closeable/isHTTPURL/archiveAndRemove/replay unit tests` → Test layer, `internal/factory/control_test.go`
- ``close` subcommand in-process tests` → Test layer, `cmd/loom/cmd/factory_inprocess_test.go`
- `Lifecycle documentation for `close`` → Docs layer, `templates/claude-feature-team/intake/README.md`

## Bounded Context

- **Owning Context**: Factory supervisor — order lifecycle and ledger (ADR-010)
- **Context Crossings**: cmd/loom/cmd — CLI adapter layer wiring the new human control onto internal/factory, templates/claude-feature-team/intake/README.md — operator-facing lifecycle documentation, read by humans, not by the supervisor

## Stability Design

- **Optimistic concurrency via fresh-read-then-validate (TOCTOU-tolerant), not a lock** on closeable, reading orders freshly folded from the ledger on every CLI invocation before appending — Matches RequestStop/RequestResume/RequestRerun exactly; the append-only ledger plus a pre-append phase check is this codebase's established substitute for locking a human-issued, low-frequency control action. supervisor.lock exists only to guard a second `loom factory run`, not this.
- **Fail-open on the finishing step, fail-closed on the ledger write** on archiveAndRemove inside Close — If archiving fails, the worktree is deliberately kept (data preserved over cleanliness) and the failure is reported, never silently retried or swallowed — same stability posture Supervisor.ship already has.

## Observability Design

- CLI success/warning line printed by runFactoryClose emitted from Adapter layer (cmd/loom/cmd/factory.go), from the string Close returns (cardinality: One line per invocation; contains the order id and the (already human-supplied, non-sensitive) --shipped-by URL, no PII)

## Layer Boundary Checks

- [PASS] cmd/loom/cmd (adapter/CLI) may depend on internal/factory (application layer); internal/factory must not depend on cmd/loom/cmd — Read imports directly (internal/factory/control.go, replay.go, finish.go, intake.go, supervisor.go and cmd/loom/cmd/factory.go) rather than querying the repo graph — the mcp__loom__dependencies_of/dependents_of/describe tools were not available in this session's tool set, so this is the stated fallback, not a graph answer. cmd/loom/cmd/factory.go imports internal/factory and internal/worktree only; internal/factory imports nothing from cmd/. The new Close/Closer/ShippedByHand stay inside internal/factory and introduce no new outbound import from that package.
- [PASS] internal/factory must not depend on a GitHub/network adapter for this feature's new code path — Close's dependency set (Ledger, orders, Layout, Repository) carries no PullRequests/GHPullRequests reference; confirmed by reading control.go's and finish.go's existing imports and the Dependencies struct in supervisor.go, which keeps Pulls separate from what Close needs.

## Anti-Pattern Check

- Shotgun surgery: not found — Five-to-six files change, but every one is a direct, necessary consequence of adding one new control (mirrors exactly how stop/resume/rerun each already span control.go + factory.go + their tests) — not a change rippling unpredictably outward.
- God object: not found — control.go gains one more small, single-purpose control alongside three existing ones; still a cohesive 'human-issued order controls' responsibility, not an accretion of unrelated behavior.
- Premature generalization: not found — Considered and rejected: bundling ledger+orders into a shared context type for all four control functions, which would have been generalizing beyond what this feature needs (see the structural decision above declining that refactor).
- Anemic domain model: not found — OrderState keeps behavior (IsActive, apply) alongside its data; the new EventShipped fold is one more small rule in the existing entryUpdates dispatch table, not a new anemic layer.
- Leaky abstraction: not found — Repository and Entry are reused as-is with no new leakage; ShippedByHand carries exactly the three fields Close needs, not a reused but mostly-empty Entry (which would have leaked irrelevant fields to the caller).

## Fitness Functions

- **Cyclomatic complexity of every new or changed function (Close, closeable, isHTTPURL, archiveAndRemove, runFactoryClose, the EventShipped closure) stays below 7**
  - **Verification**: the project's existing complexity lint rule (golangci-lint / gocyclo) in CI
  - **Owner**: devops-engineer
- **Every changed line in this delivery is covered by a test that fails when the behavior it checks is broken**
  - **Verification**: go test ./... -coverprofile plus a manual check that flipping each new branch fails its test (ADR-008)
  - **Owner**: developer
- **No file outside scope.paths changes, except this feature's own delivery record**
  - **Verification**: the run executor's existing scope.paths check against the committed diff
  - **Owner**: devops-engineer

## Refactoring Opportunities

- Extract Function on `internal/factory/finish.go: Supervisor.ship's archive-then-remove-worktree block` — Pulled into archiveAndRemove(order, layout, repo) string so Close does not duplicate ship's control flow and warning-message wording — see the structural decision above.
- Introduce Parameter Object on `internal/factory/control.go: Close's id/rawURL/approver inputs` — Grouped into ShippedByHand{Order, URL, Approver} to keep Close's own parameter count at the Sandi Metz limit without touching RequestStop/RequestResume/RequestRerun.

## Developer Handoff Notes

- finish.go needs a small edit beyond the analyst's file list: extract archiveAndRemove(order *OrderState, layout Layout, repo Repository) string out of Supervisor.ship's existing body (same warning wording, verbatim), then have ship call it. This is a pure extraction — ship's behavior and output must not change; characterize with the existing ship-focused tests before and after if none currently pin that wording.
- Define Closer{Layout Layout; Repository Repository} and ShippedByHand{Order, URL, Approver string} in control.go. Close is a method: func (c Closer) Close(ledger *Ledger, orders map[string]*OrderState, request ShippedByHand) (string, error) — not a flat 7-parameter function.
- closeable(orders, id, rawURL string) (*OrderState, error): validate existence via the existing orderIn, then isHTTPURL(rawURL), then phase (only PhaseParked or PhaseStopped pass); refusal wording duplicates RequestRerun's string literally — do not refactor RequestRerun to share it.
- Close builds Entry{Event: EventShipped, Order: request.Order, URL: request.URL, ApprovedBy: request.Approver}, appends it, then returns archiveAndRemove(order, c.Layout, c.Repository) as its string result (empty string = full success, printed by the CLI as-is).
- replay.go: add entryUpdates[EventShipped] = func(o *OrderState, e Entry) { if e.URL != "" { o.URL = e.URL } } — guarded on non-empty URL so Supervisor.ship's own EventShipped lines (no URL) are unaffected.
- factory.go: factoryCloseCmd mirrors factoryStopCmd/factoryResumeCmd/factoryRerunCmd's registration; runFactoryClose builds Closer{Layout: layout, Repository: worktree.New(layout.Root)} and the ShippedByHand request from args[0], the --shipped-by flag, and approverName().
- Test both closeable refusal paths (wrong phase, missing order, invalid/missing URL) and the success path (ledger line shape, archiveAndRemove's two failure branches via a faulty Repository double like faults_test.go's faultyRepository, and the status/inbox-visible effect of the new entryUpdates rule) to meet ADR-008's changed-line coverage.

## Open Architectural Questions

- Should the ledger+orders data clump across all four control.go functions be addressed in a dedicated follow-up refactor commit/PR? Flagged here as out of this feature's scope, not resolved.
