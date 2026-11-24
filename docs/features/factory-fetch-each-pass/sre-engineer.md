---
feature: "factory-fetch-each-pass"
bounded_context: "Factory supervisor — order lifecycle and ledger (ADR-010)"
domain_terms: ["Intake", "Candidate", "Claim", "Repository", "sourceCommit", "remote-tracking ref"]
files_touched: []
issue_refs: []
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md"]
linked_kis: []
---

# Observability & SRE Report: factory-fetch-each-pass

## 1. Service Level Indicators (SLIs)
*These metrics define the health of the newly added feature.*
- **Freshness SLI**: An order promoted to `ready` on the remote is claimed within one supervisor pass of being pushed — at the default 30 s interval, within 60 s of push. This is the business metric the feature exists to fix (a merged order previously went unseen for up to a day).
- **Availability SLI**: A pass completes `Tick()` (settle, advance, read intake, start work, report) on 100% of invocations regardless of fetch outcome — a fetch failure MUST never surface as a `Tick()` error. Verified structurally by `TestAFailedFetchContinuesThePassAndLogsOneLine`.
- **Network budget SLI**: At most 1 `git fetch` call per pass (enforced by `TestAPassFetchesTheRemoteOnceAndClaimsOrdersPromotedThere`), i.e. ≤ 2 outbound calls/minute at the default interval — matches the spec's own non-functional requirement.
- **Fetch failure rate** (operational, not unit-testable): fraction of passes in a given window where `fetchIntakeRef` logged a failure line. A sustained non-zero rate signals a credential or network problem with the configured remote and should page the operator, not the pipeline.

## 2. OpenTelemetry & Tracing
- **Analysis**: Pass (no regression) — but with a pre-existing gap this feature extends rather than closes.
- **Spans Added**: None. `internal/factory` carries zero OTel instrumentation anywhere in the package, before or after this change — `Supervisor.Tick`, `settleEnded`, `advanceFinishLine`, and the new `fetchIntakeRef`/`Git.Fetch` call all run unspanned. This is the established state of the package (OTel is only wired in `internal/provider/claude` today), so adding spans solely to the new fetch call would be an inconsistent, one-off instrumentation pattern rather than closing a real gap — not done here per the standing rule against introducing a new telemetry pattern piecemeal.
- **Missing Telemetry**: The new outbound network call (`git fetch`) is exactly the kind of call `architecture-guardrails.md` #8 expects wrapped in an adapter-layer span once this package is instrumented at all. Flagged as a gap for a future, package-wide OTel pass over `internal/factory` and `internal/worktree` — not a blocker for this change, which is consistent with the rest of its own file.

## 3. Log Quality & Cardinality
- **Status**: Pass
- **Findings**:
  - The new line — `s.say("fetch %s %s failed, continuing from the last fetch: %v", s.config.Remote, s.config.IntakeBranch, err)` — has a stable, low-cardinality message shape (fixed prefix, `remote`/`branch` as bounded config-driven values), matching the existing `s.say`/`record()` convention already used throughout `supervisor.go`. No fix was needed or made; rewriting this into a structured key-value logger would introduce a new logging library inconsistent with every other line in this file, which this review's own rules forbid.
  - **Noted, not fixed**: `fetchIntakeRef` reprints this line on every pass for as long as the fetch keeps failing, with no repeat-suppression — unlike `announce()` in `idle.go`, which already dedups identical state across ticks (`lastReported`). A revoked credential or a renamed remote would print one line every 30 s indefinitely. AC5 only requires one line per pass, which the current code satisfies exactly as specified, so this is not a defect against the acceptance criteria and was left alone rather than quietly broadening the change's behavior beyond what was reviewed and approved. Recorded here as an operational note (see Notes for DevOps Engineer) rather than an edit, since suppressing repeats would be a behavior change outside this feature's approved scope.
  - `err`'s `%v` carries git's raw `CombinedOutput` text (via `Git.Fetch` in `internal/worktree/manage.go`), which is unbounded and not groupable by itself — this is an existing, accepted pattern (identical to `Push`'s error wrapping) and was already reviewed by security-reviewer as Low/tracked tech debt. No new action taken here; see PII/secrets note below for the observability-specific framing of the same finding.

## 4. PII Data Hygiene
- **Status**: Clean
- **Notes**: No user data, email, or other PII flows through any log line touched by this feature — only remote names, branch names, commit SHAs, and git's own command output. One adjacent, non-PII secrets note carried over from security review for completeness: `Git.Fetch`'s error wrapping embeds git's raw stderr, which would echo a credential if one were ever embedded directly in a remote URL (not the case for standard credential-helper or SSH setups). Security-reviewer already tracked this as Low, pre-existing-pattern tech debt on `Push`, now extended to `Fetch`; this review concurs it is not a PII violation and does not block, but flags it below for DevOps' log-handling guidance since it is the kind of value `architecture-guardrails.md` #9 says telemetry must never carry as a raw payload.

## Notes for DevOps Engineer
- If this supervisor's stdout is ever forwarded to a centralized log sink, treat the `fetch %s %s failed...` line's trailing `%v` as potentially sensitive (raw git command output) and scrub before indexing or long-term retention, rather than assuming it is safe because the rest of the line is low-cardinality.
- Consider an alert on 3+ consecutive fetch-failure lines from one supervisor process — today nothing dedups or escalates a sustained failure beyond printing it every pass, so a credential expiry would otherwise only surface as log noise someone has to notice.
- No new dashboard panels are needed for this change alone; if `internal/factory` is instrumented with OTel in a future pass, wire a counter on fetch attempts/failures and a histogram on pass-to-claim latency (the Freshness SLI above) rather than inventing a bespoke metric just for this feature.