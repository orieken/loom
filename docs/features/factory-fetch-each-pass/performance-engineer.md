---
feature: "factory-fetch-each-pass"
bounded_context: "Factory supervisor — order lifecycle and ledger (ADR-010)"
domain_terms: ["Intake", "Candidate", "Claim", "Repository", "Order", "base-branch", "remote-tracking ref"]
files_touched: ["internal/worktree/manage.go", "internal/factory/intake.go", "internal/factory/supervisor.go", "internal/factory/start.go", "cmd/loom/cmd/factory.go"]
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Performance & Reliability Report: factory-fetch-each-pass

## 1. Idempotency Guarantees
- **Status**: N/A
- **Notes**: `Fetch` only advances the local remote-tracking ref (`<remote>/<branch>`); it never mutates the person's checkout (HEAD, index, working tree) and never mutates shared state on a server. Running it zero, one, or many times in a row converges to the same remote-tracking ref state, so it is naturally idempotent with no key or dedup logic required. Order-claiming itself (the one real mutation touched by this change) keeps its existing idempotency behavior untouched — `claimFor` still only widens *which* ref a fresh claim defaults its `base-branch` to (`committed.Ref` instead of the bare branch); it does not change whether or how an already-claimed order can be claimed again. No new idempotency guarantee is required by this feature.

## 2. Timeout & Circuit Breaker Mandates
- **Status**: Pass
- **Mandates**:
  - `Git.Fetch` MUST wrap its network call in `context.WithTimeout(fetchTimeout)` with `GIT_TERMINAL_PROMPT=0`, exactly as planned (mirroring `Push`'s existing pattern) — architecture-guardrails #5 requires an explicit timeout on every network call, and this is the only new network call this feature introduces.
  - `Git.Remotes` MUST stay a local-only call with **no** `context.WithTimeout` and **no** network I/O (matches `hasBranch`'s existing pattern). A `CircuitBreaker` here would be mitigating a failure mode that cannot occur — reserve breakers for calls that actually cross the network.
  - A `CircuitBreaker` or `ExponentialBackoffStrategy` is **not** required around `Repository.Fetch` itself: it runs once per ~30 s pass, not in a tight retry loop, so there is nothing for a breaker to protect against cascading. The architecture notes' "fail fast + log one line + continue from the last-fetched ref" (AC5) is the correct shape for this cadence — do not add a hidden retry loop inside `fetchIntakeRef` that would turn one bounded network call into several.
  - **Flag for the developer to verify, not a blocker**: `fetchTimeout` is planned at `5*time.Minute`, copied from `pushTimeout`. `Push` is triggered by an infrequent, human-waited-for action; `Fetch` now runs on *every* ~30 s supervisor pass per this feature's own NFR ("at most one network call per pass... two a minute"). If `Tick()` is synchronous end-to-end (fetch blocks before any candidate is processed that pass), a remote that is reachable but slow could make a single pass take up to 5 minutes before any order in that pass is claimed or continued — ten passes' worth of cadence lost to one stall. Confirm whether `Tick()`'s callers tolerate an occasional 5-minute pass; if not, use a shorter, fetch-specific timeout (e.g. 30–60 s) rather than reusing `pushTimeout` by default. This does not block the implementation — AC5 already accepts that a failed fetch degrades gracefully — but the timeout *value* should be a deliberate choice, not an inherited one.

## 3. N+1 Query Prevention
- **Status**: Pass
- **Findings**: The one loop risk this feature could introduce is calling `Repository.Fetch` (or even `Repository.Remotes`) once per candidate while iterating claimable/continuing orders in a pass, instead of once per `Tick`. The design already closes this off correctly: `fetchIntakeRef()` is the single call site, invoked exactly once before `ReadIntake`, and its resolved ref (the `Intake` struct, carrying `Commit` + `Ref`) is threaded down through `claimWork`/`claim`/`claimFor` as a parameter rather than re-fetched per order. This is already named as a fitness function with its own test (a fake `Repository.Fetch` call-counter across a `Tick` with several claimable and continuing orders) — keep that test in place; it is the regression guard for exactly this N+1 shape. No additional DataLoader or batching is needed since there is nothing to batch: one fetch serves every order in the pass by construction.

## 4. Hot Path Caching
- **Analysis**: The hot path is `Tick()`, which runs on an indefinite ~30 s loop for the lifetime of the supervisor process. This feature adds two calls to that path: `git remote` (local, no network) and `git fetch <remote> <branch>` (network, explicitly bounded to once per pass by this feature's own NFR and fitness function).
- **Strategy**: No caching layer is needed for either call, and adding one would work against the feature's purpose:
  - `Fetch` exists specifically to make each pass see freshly-merged work; caching its result across passes would reintroduce the staleness this feature is built to remove. The "cache" here is correctly the once-per-pass bound already designed in, not a TTL store.
  - `Remotes()` is a local, in-process git metadata read with no measurable cost (confirmed in architecture-notes as negligible next to one network fetch). Caching its result across ticks (remotes rarely change mid-run) would trade a free call for cache-invalidation complexity with no observable benefit — Kent Beck's "fewest elements" rules this out. Leave it uncached.

## Notes for Developer
- Keep exactly one call site for `Repository.Fetch` — inside `fetchIntakeRef()`, invoked once per `Tick()` before `ReadIntake`. Never call it inside the per-candidate loop in `pendingWork`; the planned `Intake`-as-parameter change to `claimWork`/`claim`/`claimFor` is what lets every order in a pass reuse one resolved ref instead of re-fetching.
- Verify whether `Tick()` is fully synchronous before accepting `fetchTimeout = 5*time.Minute` unmodified. If a stalled-but-reachable remote blocking the whole pass for up to 5 minutes is undesirable, give `Fetch` its own shorter timeout rather than inheriting `pushTimeout`'s value for a different calling pattern.
- `Remotes()` must remain local-only: no `context.WithTimeout`, no network I/O. Treat any future change that adds network behavior to `Remotes()` as a performance regression, not just an abstraction leak — it would silently double this feature's network-call budget per pass with no caller-visible signal.
- Do not add retry/backoff around `Fetch`. One attempt per pass, log-and-continue on failure (AC5), is the correct reliability shape at this call frequency — a retry loop would risk exceeding the "at most one network call per pass" NFR.
- No caching work is required by this report. If a future change proposes caching `Fetch` results across passes, that proposal works against this feature's stated purpose and should be challenged at design time, not implemented.
