---
feature: "pre-ship-checks"
bounded_context: ""
domain_terms: ["Pre-ship check"]
files_touched: []
issue_refs: ["PR #18"]
linked_adrs: ["docs/adrs/ADR-010-factory-order-state-and-policy-configuration.md", "docs/adrs/ADR-008-define-tested-by-what-tests-can-catch.md"]
linked_kis: []
---

# Security Review: pre-ship-checks

## Threat Model Summary

This feature adds an executor-internal stage that runs project-declared shell commands (from .claude/delivery-policy.yaml) before the confirm-ship gate, and the config-decoding path that loads them. The only genuinely new attack surface is arbitrary command execution — but the spec explicitly scopes the command source to the project's committed delivery-policy.yaml, the same trust level as its CI config, and never from a work order or any other external/untrusted input. Verified in code: commands are decoded only from policy.ConfigFile (.claude/delivery-policy.yaml, a fixed repo-relative path), ProjectRoot and StartCommit are pre-existing infrastructure values (not work-order-derived), every check runs under an explicit per-check timeout via context.WithTimeout (architecture-guardrails.md #5), a failing check halts the run before confirm-ship with the FailureTerminal class (no supervisor retry), and the full command output never reaches telemetry/events — only the check's name, exit status/timeout, and duration do (record.Error = "pre-ship check %q: exited %d"/"timed out after %s"), which satisfies guardrail #9. No new dependency, no new auth/session surface, no new network client. No Critical or High findings; two Low/Info observations noted below for awareness, no fix required given the stated trust boundary.

## Dependency Audit

- No new dependencies introduced — go.mod/go.sum unchanged. New code uses only stdlib (os/exec, context, time, path/filepath, strings) plus the already-present go.yaml.in/yaml/v4 for node-walk decoding, consistent with decode.go's existing Condition pattern.

## STRIDE Analysis

None

### Spoofing

No auth/session/identity boundary is introduced. Checks execute as the same OS user/process identity that runs `loom run` — no new identity to spoof.

### Tampering

Whether the command set executed can be influenced by anything other than the project's own committed .claude/delivery-policy.yaml. Verified: policy.LoadConfig reads a fixed path (policy.ConfigFile), decodePreShipChecksKey rejects unknown keys per entry (rejectUnknownPreShipCheckKeys), and no work-order, spec, or KI content feeds into PreShipCheck.Command at any point in the call chain (run.go -> loadPolicies -> policy.LoadConfig -> decodeAndValidate -> decodePreShipChecksKey).

### Repudiation

Whether a check's outcome is durably, verifiably recorded. Verified: every outcome (name, command, exit code/timeout, duration, output) is written to pre-ship-checks.md, and both COMPLETED and FAILED stage records carry ArtifactPath + ArtifactSHA256 (attachFailureArtifact mirrors persistCompletion's existing behavior) — a failure's evidence survives even though the stage never reaches COMPLETED.

### Information Disclosure

What can be read, and what can carry it out. CombinedOutput captures stdout+stderr, which per the spec's own NFR may contain secrets from the project's environment (cmd.Env = os.Environ() + LOOM_BASE_COMMIT). Verified the artifact file (workspace-local, 0o644 — the same permission every other workspace artifact in this codebase already uses, e.g. correction.go, typed.go) is the only place full output lands; record.Error and the emitted Event carry only the check name and exit status/duration text, never output, satisfying architecture-guardrails.md #9. No outbound network call is made by loom itself for this feature — a check's own command could make one, but that is the same capability a CI step already has, within the trust boundary the spec declares (committed config, CI-equivalent trust).

### Denial of Service

Whether a check can consume unbounded time or resources. Verified: every check runs under context.WithTimeout bound to its own Timeout (default 10 minutes), a timeout is classified TimedOut and fails the stage as FailureTerminal (no retry loop), and a failing/timed-out check stops the loop before later checks run — bounding total stage time to the sum of checks up to and including the first failure.

### Elevation of Privilege

Whether running declared commands grants any privilege beyond what the loom process already has. Verified: exec.CommandContext("bash", "-c", ...) runs with the invoking process's own user/environment — no setuid, no privilege boundary crossed, no new permission the project's own CI didn't already have running the same command.

## Findings

### [LOW] — Per-check timeout has no upper bound
**Location**: `internal/policy/preship.go:136-145 (preShipCheckTimeout)`
**Threat category**: Denial of Service
**Description**: preShipCheckTimeout accepts any value time.ParseDuration can parse, including an arbitrarily large one (e.g. "999999h"), which would let a committed config entry defeat the intent of architecture-guardrails.md #5 ('every network call MUST have an explicit timeout') by setting a timeout so large it is effectively none. Given the trust boundary the spec declares (same trust level as the project's own CI config, never externally supplied), this is defense-in-depth only, not an exploitable path today.
**Fix applied**: Recommendation only
**Verification**: Not applicable — no fix applied; informational for a future hardening pass (e.g. cap at a sane ceiling such as 1 hour) if desired.
### [INFO] — Full check output (including possible secrets) is written to a world-readable artifact file
**Location**: `internal/orchestrator/preship.go:131-142 (writePreShipCheckReport)`
**Threat category**: Information Disclosure
**Description**: pre-ship-checks.md is written with 0o644, matching every other workspace artifact in this codebase (correction.go, typed.go) — not a new pattern this feature introduced. The spec's own NFR already acknowledges a check's output may contain secrets from the project's environment and scopes the mitigation to telemetry only (guardrail #9), which is correctly satisfied. Flagging only so QA/tech-writer are aware the workspace directory itself is the remaining boundary for this risk, same as it already is for every other artifact.
**Fix applied**: Recommendation only
**Verification**: 

## Files Modified

None

## Security Checklist

- [x] Command source is restricted to the project's own committed .claude/delivery-policy.yaml, never a work order or external input
- [x] Every check runs under an explicit, bounded timeout (architecture-guardrails.md #5)
- [x] A failing/timed-out check halts the run before confirm-ship and is classified terminal (no supervisor retry)
- [x] Raw command output never reaches telemetry/events — only name, exit status, duration (architecture-guardrails.md #9)
- [x] Unknown keys in a preShipChecks entry are rejected rather than silently dropped (ErrInvalidConfig)
- [x] No new third-party dependency introduced
- [ ] Per-check timeout has a sane upper bound — LOW finding above — informational only given the trusted config source; not blocking

## Notes for QA

- No Critical/High findings — nothing blocks QA.
- Worth a QA scenario: a preShipChecks command that writes something secret-shaped to stdout on failure — confirm it lands only in pre-ship-checks.md (workspace file) and never in run-events.jsonl/traces.jsonl.
- Existing test suite (internal/orchestrator, internal/policy, cmd/loom/cmd) already covers the timeout-terminal-failure path, the halt-before-confirm-ship path, and the last-50-lines-vs-full-output split — no gaps observed that need backfilling.

## Notes for Tech Writer

- If documenting preShipChecks for end users, state explicitly that command output may contain secrets from the environment and is retained in the run's workspace artifact (pre-ship-checks.md) — not sent to telemetry — so project owners know where that data lives and for how long.
