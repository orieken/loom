---
feature: "factory-idle-message"
bounded_context: "Agent Orchestration"
domain_terms: ["Idle", "Waiting on a human"]
files_touched: ["internal/factory/idle.go", "internal/factory/supervisor.go", "internal/factory/idle_test.go", "internal/factory/fixture_test.go"]
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Observability & SRE Report: factory-idle-message

## 1. Service Level Indicators (SLIs)
*These metrics define the health of the newly added feature.*
- **Availability SLI**: Not applicable in the request/response sense — this feature has no endpoint, no network call, and no success/failure outcome of its own; it only observes the ledger state `Tick()` already computed. The closest analogue is a **correctness SLI**: every tick whose classified state (`tickIdle` / `tickWaiting` / `tickBusy`) differs from `lastReported` MUST produce exactly the line the spec defines, and every tick whose state is unchanged MUST produce none. This is enforced at test time by `idle_test.go`, not by a runtime metric — there is no metrics backend wired into this CLI, so a live percentage cannot be reported; the test suite is the enforcement mechanism for this SLI today.
- **Latency SLI**: **Announcement latency MUST be ≤ one tick.** `reportTickState()` runs unconditionally as the last step of every `Tick()` (`internal/factory/supervisor.go:103`), after `startWork` returns successfully, so an idle or waiting transition is announced on the same tick it occurs — bounded by `Config.Interval` (default `DefaultInterval = 30s`) under `Run`, or immediately under `Once`. No additional delay is introduced. This is a structural guarantee, not a measured one; there is nothing to instrument because the bound is enforced by call order, not by I/O that could stall.

## 2. OpenTelemetry & Tracing
- **Analysis**: Pass — not applicable, correctly.
- **Spans Added**: None, and none are warranted. There is no OTel instrumentation anywhere in `internal/factory` (confirmed: no `otel`, `trace.`, `span`, or logging-framework import in the package), so this feature introduces no new convention and breaks no existing one. The feature adds no asynchronous boundary and no external network call — `reportTickState()` calls `s.Orders()`, which is the same local ledger replay `Tick()` already computed no additional time in this change (`Orders()` is called independently by `settleEnded`, `advanceFinishLine`, and `startWork` within the same `Tick()` today — precedent the implementation notes correctly cite). Per guardrail #8, spans belong at the adapter/interceptor layer, not in domain logic; `classify`, `waitingCount`, and `hasUnshippedOrder` are pure functions over `map[string]*OrderState` and are exactly the kind of inner-layer domain logic that must stay free of tracing concerns.
- **Missing Telemetry**: None for this feature's scope. If this CLI gains a metrics backend in the future, the natural metric is a counter on state transitions (`idle_enter`, `waiting_enter{reason=gate|commit|pr|parked}` would require the per-order reason, which `waitingCount`'s current aggregate-only count does not retain — out of scope here, flagged for awareness only, not a blocking gap for this feature).

## 3. Log Quality & Cardinality
- **Status**: Pass
- **Findings**: Both new lines are stable, low-cardinality message strings with no interpolation hygiene issues:
  - `"idle — no ready orders; commit one to intake/ on %s"` — the one variable is `s.config.IntakeBranch`, an operator-configured value from a small, fixed set of branch names (not user input, not unbounded).
  - `"waiting on a human — %d order(s) in the inbox: loom factory inbox"` — the one variable is an integer count.
  No order ID, phase name, gate name, or free-text reason is interpolated into either line, so neither can explode in cardinality the way a per-order log line would. This matches the package's existing convention: `say()` (`internal/factory/supervisor.go:191`) is plain `fmt.Fprintf` to an `io.Writer` for direct human-facing console text, not a structured logging pipeline (no Pino/Logrus/`slog` is used anywhere in `internal/factory`), so there is no existing structured-logging implementation this change should have used instead. Introducing one here would violate this agent's own rule against adding new telemetry libraries — `say()` is the established mechanism and this feature correctly reuses it unchanged.
  - **Adjacent observation, not a blocking finding**: `record()` (`internal/factory/supervisor.go:173-179`) prints `"%s %s%s", entry.Order, entry.Event, detail(entry)` — unbounded order IDs and free-text gate/reason strings glued into one line. This is pre-existing, untouched by this change, and out of this feature's scope; noted for awareness only, since a future SRE pass over that call site would have a real cardinality finding to fix.

## 4. PII Data Hygiene
- **Status**: Clean
- **Notes**: The two new lines carry only a configured branch name and an integer count — no user identity, email, token, credential, or free-text order content. `classify`, `waitingCount`, and `hasUnshippedOrder` take only `map[string]*OrderState` and never touch `Repository`, `TreeOpener`, `PullRequests`, or re-read intake (confirmed against the NFR and architecture-notes.md's structural decision), so there is no path by which this feature could pull in anything beyond phase counts. No fix was required.

## Notes for DevOps Engineer
- No dashboards, alerts, or panels are needed for this change — there is no metrics backend in this CLI's path, and the feature's entire observable surface is a console line a human reads directly while `loom factory run` is attached to a terminal. Nothing here should be wired into Grafana/Datadog.
- If a future feature adds a metrics backend to `loom factory`, the idle/waiting state transition this feature now exposes as text (`classify()`'s `tickState`) is the natural source for a state-duration gauge or a transition counter — worth revisiting at that point, not now.