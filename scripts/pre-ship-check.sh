#!/usr/bin/env bash
# Reproduces CI's own changed-line coverage gate (ADR-008, roadmap L3.58),
# so `loom run`'s pre-ship-checks stage can fail before a human is ever
# asked to approve shipping a change CI will reject — see
# .github/workflows/framework-ci.yml's "Changed-line coverage" step, which
# this runs identically against LOOM_BASE_COMMIT instead of a CI-computed
# base (scripts/ci-diff-base.sh).
set -euo pipefail

if [[ -z "${LOOM_BASE_COMMIT:-}" ]]; then
  echo "pre-ship-check.sh: LOOM_BASE_COMMIT is empty — refusing to measure changed-line coverage against nothing" >&2
  exit 1
fi

profile="$(mktemp)"
trap 'rm -f "$profile"' EXIT

go test ./... -coverprofile="$profile"
go run ./cmd/diff-coverage --base "$LOOM_BASE_COMMIT" --profile "$profile"
