#!/usr/bin/env bash
set -uo pipefail

# Runs the same checks Framework CI runs (.github/workflows/framework-ci.yml), inside a container
# matching the actual runner (ubuntu-latest -> ubuntu:24.04, bash 5.x) instead of trusting a local run.
#
# Why this exists: macOS ships bash 3.2 by default, which silently tolerates some `set -e` + arithmetic
# gotchas that modern bash treats as command failures (see the 2026-07-04 CI break: ((var++)) evaluating
# to 0 on the first iteration aborted check-parity.sh under bash 5.x, but never locally). A local script
# passing on macOS's bash proves nothing about whether it'll pass in CI. Run this before every push that
# touches scripts/, shared/, or .github/workflows/ — not just after.
#
# Covers the CI jobs that are reproducible from a standalone checkout: check-parity, test-agents,
# health-check, and test-install. The agent-versions job only runs on pull requests and needs a real
# base-ref + head SHA from the PR event -- not reproducible standalone without a PR to compare against.
# If you want to check it locally, run
# scripts/check-agent-versions-ci.sh <base-ref> <head-sha> directly against a real branch comparison.
#
# test-install.sh writes to scratch dirs under the container's own /tmp, not to the read-only /repo
# mount, so it's safe to run here alongside the static checks.

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="ubuntu:24.04"  # matches ubuntu-latest at the time this script was written; bump if GitHub moves LTS

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required (this script runs checks inside the same OS/bash CI uses) — not found in PATH."
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  echo "docker is installed but not running — start Docker and try again."
  exit 1
fi

echo ""
echo "=== CI Parity Check (${IMAGE}, matching GitHub Actions ubuntu-latest) ==="
echo "Repository: $REPO_DIR"
echo ""

PASS_COUNT=0
FAIL_COUNT=0

run_check() {
  local label="$1"
  local cmd="$2"

  echo "--- $label ---"
  # These must match what the workflow installs, or this script stops being a
  # parity check and becomes a different one. python3-yaml covers
  # check-parity.sh's .roomodes validation; python3-jsonschema covers
  # health-check.sh's frontmatter validation, which is opt-in and skips with a
  # warning when the import is missing — so an absent package here shows up as
  # three checks quietly not running rather than as a failure.
  if docker run --rm -v "$REPO_DIR:/repo:ro" -w /repo "$IMAGE" bash -c "
    apt-get update -qq >/dev/null 2>&1
    apt-get install -y -qq python3 python3-yaml python3-jsonschema git >/dev/null 2>&1
    # git: check-generated-drift.sh lists tracked files for its orphan pass. The runner's checkout
    # has it; this image does not. safe.directory because the mount is owned by the host user.
    git config --global --add safe.directory /repo
    $cmd
  "; then
    echo "  PASS  $label"
    ((PASS_COUNT++)) || true
  else
    echo "  FAIL  $label"
    ((FAIL_COUNT++)) || true
  fi
  echo ""
}

run_check "check-parity.sh" "bash scripts/check-parity.sh"
run_check "check-generated-drift.sh" "bash scripts/check-generated-drift.sh"
run_check "test-agents.sh" "bash scripts/test-agents.sh"
run_check "health-check.sh --verbose" "bash scripts/health-check.sh --verbose"
run_check "test-install.sh" "bash scripts/test-install.sh"

echo "==========================================="
echo "Results: $PASS_COUNT passed, $FAIL_COUNT failed"
echo ""

if [[ $FAIL_COUNT -gt 0 ]]; then
  echo "One or more checks would fail in CI. Fix before pushing."
  exit 1
fi

echo "All checks would pass in CI."
exit 0
