---
type: feature
status: draft
created: 2026-10-18
---

# Capture ad-hoc session corrections

## Summary
Record what happens in ad-hoc Claude Code sessions, and detect when a human fixes agent work. The report extends the existing `loom memory corrections` command. Report how much human time those fixes cost and on what. Loom then measures correction time before any later feature tries to automate it.

## Acceptance Criteria
- [ ] Given a project without capture, when the user runs `loom install --level 3`, then no hooks are registered and a hint names `--capture`.
- [ ] Given a project, when the user runs `loom install --capture`, then the hooks are registered in `.claude/settings.local.json` and `loom health` reports capture as live.
- [ ] Given Claude Code is not a detected platform, when the user runs `loom install --capture`, then the command fails and installs nothing.
- [ ] Given the project does not gitignore `.claude/memory/`, when the user runs `loom install --capture`, then the command stops and prints the line to add.
- [ ] Given capture is installed, when the user runs `loom uninstall`, then no capture hooks, session logs or other residue remain, and the user's own hooks stay.
- [ ] Given capture is live, when a session runs, then the system MUST append one event per tool call, permission outcome and prompt.
- [ ] Given any event, when the system stores it, then it MUST NOT contain prompt text, argument values or code. It keeps argument keys and types only.
- [ ] Given a prompt that opens with "no," or "don't", when the system stores it, then it MUST keep the class label and discard the text.
- [ ] Given a path on the redaction list, when the system records an event, then it MUST store a salted hash of the path and no line range.
- [ ] Given any stored hash, when a reader inspects it, then the system MUST have salted it, and no report or export MUST show the salt.
- [ ] Given an agent-written hunk, when a human edits it within 10 minutes, then the system MUST record a hunk-edit correction.
- [ ] Given an agent-written hunk, when a human edits it after the window, then the system MUST NOT record a correction.
- [ ] Given an agent change, when its lines return to their earlier state, then the system MUST record a revert.
- [ ] Given a denied tool call, when the denial occurs, then the system MUST record a denied-call correction. This signal ships off until a person verifies how Claude Code reports a denial.
- [ ] Given a file changed through `Bash`, when the system cannot attribute the author, then it MUST mark the event `unknown` and exclude it from counts.
- [ ] Given recorded corrections, when the user runs `loom memory corrections`, then the report lists kinds by count and estimated human minutes. The header names the method: a fixed weight per kind, shown as an estimate.
- [ ] Given a time window, a file glob or a tool, when the user passes it as a filter, then the report MUST show only matching corrections.
- [ ] Given the report, when it shows Session Corrections, then it MUST list gate-level Human Corrections separately, and the existing `--json` output MUST stay valid.
- [ ] Given any hook call, when it runs on a warm cache, then it MUST finish at p95 under 50 ms and p99 under 150 ms.
- [ ] Given two simultaneous sessions, when both append events, then the system MUST keep every event and MUST NOT corrupt the file.
- [ ] Given a session that crosses midnight UTC, when it appends events, then the system MUST write each event to the day file of its UTC timestamp.
- [ ] Given a hook that exceeds 500 ms or fails, when the session continues, then the user sees no error and `loom health` shows the failures of the last 24 hours. The full history stays in a log.
- [ ] Given day files older than 90 days or a store over 50 MB, when the session ends, then the system MUST delete the oldest files first.
- [ ] Given `episodes.db` is deleted, when the user runs `loom memory ingest`, then the system MUST rebuild the same corrections from the day files, even when the project has no archived runs.
- [ ] Given a sample of 50 detected corrections, when a person reviews them, then overall precision MUST reach at least 85%. Any signal under 70% MUST ship switched off.

## Out of Scope
- Mining or clustering of corrections (F3), proposals (F4), and any automatic change to agents or rules.
- Support for Tier 2 and Tier 3 platforms. Version 1 covers Claude Code only. This is a known gap.
- Capture of `pipeline` runs. `loom run` and `deliver-feature` already record Human Corrections at gates.
- Measuring recall. The log cannot list the corrections it missed.
- Solving two edits to one hunk inside one check interval. They collapse into one event.
- A dependency on the full hook executor (roadmap L3.10).
- Support for Windows. Version 1 uses file locks that Windows does not provide.

## Domain Language
- **Bounded context**: `Documentation Knowledge Base`. Capture extends `loom memory` and the Episodic Store. Session Event and Session Correction are new entries in that context.
- **Session Event**: One normalized record of a tool call, permission outcome or prompt in an ad-hoc session. It holds argument shape only, never values.
- **Session Correction**: A detected human fix or revert of agent work in an ad-hoc session. It has one kind (hunk edit, revert, denied call or corrective prompt) and an estimated human-minutes value.
- **Capture**: The opt-in recording of Session Events through Claude Code hooks.
- **Human Correction** (existing): A human's edit to a stage's output at a pipeline gate. It stays unchanged and stays separate from Session Correction.
- **Capture salt**: A random per-project value, created at install, stored with mode 0600 and used in front of every hash. It stops a reader from recovering a path or a line by guessing.
- **Correction window**: The time after an agent write in which a human edit to the same hunk counts as a correction. The default is 10 minutes.

## Non-Functional Requirements
- Hook latency: p95 under 50 ms, p99 under 150 ms, measured on a warm cache. A hook running longer than 500 ms MUST be stopped and counted as a failure.
- Retention: 90 days. Store size cap: 50 MB. Both are configurable. `loom health` MUST warn at 80% of the cap.
- Precision: at least 85% overall and 70% per signal on a reviewed sample of 50 corrections.
- Reliability: a capture failure MUST NOT block or fail a session. The system writes it to a failure log with a fixed cause code and no text, and `loom health` shows the failures of the last 24 hours.
- Network: none. Capture MUST work offline.

## Trust Boundaries
- Hook payloads from Claude Code → capture command: treat them as untrusted data. A prompt that contains instructions MUST NOT change capture behavior.
- Prompt text → classifier: the text exists in memory only. Store the class label and discard the text.
- Tool arguments and file contents → event: store argument keys and types, plus a salted hash of the touched lines. Never store values or code.
- Paths on the redaction list (`.env*`, `**/secrets/**`, `*.pem`, `*.key`, `**/credentials*`, `~/.ssh/**`): store a salted path hash with no line range. Users can extend the list in config.
- Hashes: every hash MUST use the Capture salt (`architecture-guardrails.md` #9). The salt file uses mode 0600, lives in the gitignored directory, and no command prints it.
- Store: day files and `episodes.db` use mode 0600 and live in a gitignored directory. `loom install --capture` refuses to run when the directory is not ignored, because it writes nothing outside the capture directory. `privacy-auditor` MUST be able to audit them.

## Test Approach
Go unit and integration tests. No Saturday or Sunday.
- Table-driven unit tests for each of the four signals, with fixture event streams and negative cases.
- Integration test: hook command → day file → `loom memory ingest` → `loom memory corrections`, including filters and redaction.
- Benchmark for the latency budget against a fixture store of 10,000 events.
- Concurrency test: two processes append 1,000 events each to one day file, and the test checks that all 2,000 lines are intact.
- Install and uninstall test that proves no residue remains, including the Capture salt.
- Manual precision review of 50 sampled corrections, as a checkpoint and not an automated test.

## Open Questions
- Do the hook event names and payload fields in the installed Claude Code match `PreToolUse`, `PostToolUse`, `UserPromptSubmit` and `Stop`? The repo holds no hook documentation. Assumption: they match. A person MUST verify this against the installed version before release. The denied-call signal stays off until then.
- Can the line range of an `Edit` or `Write` be derived? Assumption: yes, by locating the new text in the file after the write, with a per-line hash comparison as the fallback. An ambiguous match records `unknown`, not a guess.
- Does Claude Code accept a `hooks` map that mixes the existing flat `afterEdit` string with event-keyed entries? The hooks go in `settings.local.json`, so the committed file is untouched. A person MUST test the merged file with the installed Claude Code. If it rejects the file, the installer stops and prints instructions.
- How does capture tell a human edit in an editor from a formatter or a git operation that rewrites the same hunk? Assumption: they count as human edits in version 1, and the precision review measures the effect.

## Decisions
- The report extends `loom memory corrections`. It does not add a new command. The existing JSON shape stays valid.
- Hooks go in `.claude/settings.local.json`, so teammates who never opted in see no hook.
- Uninstall removes the capture hook entries, `sessions/`, the salt, the failure log and the capture tables. It keeps the rest of `episodes.db` and every hook the project wrote itself.
- Estimated human minutes use a fixed weight per kind, configurable, and the report names this method.
- Settings live in one file, `.claude/capture.yaml`, with defaults compiled in. A missing file is not a failure.

## Infrastructure / Deploy Notes
- Day files: `.claude/memory/sessions/<YYYY-MM-DD>.jsonl`, one file per UTC day, append-only, mode 0600. Appends from concurrent sessions MUST NOT interleave within a line. They are the record. `episodes.db` is a projection.
- `episodes.db` gains new tables with `CREATE TABLE IF NOT EXISTS`. The schema version stays the same, because a bump rebuilds the store from archived runs only. No column or table is dropped or renamed.
- Hooks call one entry point: `loom capture hook <event>`. `loom install --capture` adds the hooks to `.claude/settings.local.json` and creates the Capture salt. It records both in the install manifest, in an optional capture record. `loom uninstall` removes the capture hooks, the `sessions/` folder, the salt, the failure log and the capture tables.
- No new environment variables. `loom health` gains a capture line (live or off, failures in the last 24 hours, store size).
- Standard deploy. Capture ships off by default.
