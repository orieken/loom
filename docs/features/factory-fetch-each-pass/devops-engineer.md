# DevOps Report: factory-fetch-each-pass

## Files Created

None.

## Files Modified

None. No CI/CD pipeline file, Dockerfile, IaC, or deployment script requires a change for this feature.

Verified rather than assumed:
- `.github/workflows/framework-ci.yml` already runs `go test ./... -coverprofile=coverage.out` (line 145) and `golangci-lint` with `gocyclo min-complexity 7` via `.golangci.yml` at the repo root (lines 273–286) — both gates already enforce AC9 (`go test ./...` and lint pass, cyclomatic complexity < 7) with no new job, step, or config needed. `implementation-notes.md` confirms every new/changed function (`Tick`=6, `fetchIntakeRef`=6, `hasRemote`=3, `withDefaults`=6, `readIntake`=2) is already under the existing gate.
- `pre-ship-checks.md` confirms `go test ./internal/factory/...` passes at 95.7% coverage and changed-line coverage is 100% (63 of 63 executable changed lines; threshold 85%) — no coverage-tooling change needed.
- No new CLI flag value, config key, or code path in this feature touches a secret, credential, or token — `--remote` and `--no-fetch` are plain strings/booleans read from the CLI and the repository's already-configured git remote. There is nothing for `.env.example` or a secrets manager to carry.

## New Environment Variables Required

None. The feature reuses the repository's already-configured git remote and the operator's ambient git credential helper (spec's own Trust Boundaries section); it sends and stores nothing new.

## Migration Steps

None required — no database or schema change in this feature.

## Deployment Notes

- This is a behavior change inside `loom factory run`'s supervisor loop (`internal/factory/supervisor.go`), shipped as part of the `loom` binary — there is no separate service deploy, container image, or infra resource to update.
- **Manual restart required**: per the spec's own "Infrastructure / Deploy Notes" and `implementation-notes.md`'s "Notes for DevOps," any `loom factory run` supervisor process already running against this repository must be restarted with the new build after this ships. The running process holds the old binary in memory; a restart is the only way it picks up fetch-before-read. This matches the project's **Immutable Infrastructure** convention (`shared/rules/iac-conventions.md` / `devops-engineer` checklist) — restart with a new build, not a hot patch.
- **Rollback**: low risk. `--no-fetch` (or simply running the previous binary) restores the exact prior behavior — reading `intake/` off the local branch with zero fetch calls. If the new fetch behavior causes an unexpected problem in production, restarting the affected supervisor with `--no-fetch` is a safe, immediate mitigation that requires no code rollback.
- **Observed operational gap** (from `sre-engineer.md`'s SRE report, carried here since it is the kind of thing an operator running the supervisor needs, not a defect blocking ship): a fetch failure prints one line per pass with no de-duplication or escalation. A revoked credential or renamed remote will print that line every pass (every 30 s at the default interval) indefinitely rather than alerting once. This is in-scope behavior per AC5, not a bug — flagging it here as an operational note for whoever watches these supervisors' logs, not a blocker.

## Manual Steps Required

- Restart every currently running `loom factory run` supervisor with the new build after this change ships, so the fetch-before-read behavior takes effect. Nothing else is required: no secret to set, no vault entry to create, no dashboard to provision, and no CI configuration to touch.

🤖 Generated with [Claude Code](https://claude.com/claude-code)