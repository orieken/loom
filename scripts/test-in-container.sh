#!/usr/bin/env bash
# Steps are dispatched by name (run_step calls "step_$1"), which shellcheck cannot see.
# shellcheck disable=SC2329
# Runs loom's full verification (roadmap L3.72). It is the pinned test image's
# entrypoint, so the same toolchain, modules and linters run it everywhere;
# it also runs directly on any machine that has those installed.
#
#   scripts/test-in-container.sh [step...]
#
# Steps, in order (default: all of them):
#   toolchain  Go matches go.mod's go line; prints tool versions
#   build      the loom module and the out-of-repo embedding example
#   test       go test ./... with a coverage profile
#   coverage   the CI ratchet floor, then changed-line coverage (ADR-008)
#   lint       golangci-lint with the repo's .golangci.yml
#   mutation   changed-line mutation score (report-only, as in CI)
#   smoke      build the binary, install it into a scratch project, check it
#
# Every step reports PASS, FAIL, or NOT MEASURED. NOT MEASURED means the step
# could not run — usually no diff base — and is never counted as a pass.
#
# Environment:
#   LOOM_TEST_OUT   where profiles and the scratch project go (default /tmp/loom-test)
#   LOOM_DIFF_BASE  commit to diff changed lines against (default: merge-base with origin/main)
#   PR_BASE, PUSH_BEFORE  as in CI; when either is set, scripts/ci-diff-base.sh picks the base
set -euo pipefail

readonly ALL_STEPS=(toolchain build test coverage lint mutation smoke)
readonly NOT_MEASURED=3
readonly OUT_DIR="${LOOM_TEST_OUT:-/tmp/loom-test}"
readonly COVERAGE_PROFILE="$OUT_DIR/coverage.out"
readonly TESTS_PASSED="$OUT_DIR/tests-passed"
readonly WORKFLOW=".github/workflows/framework-ci.yml"

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly REPO_DIR
cd "$REPO_DIR"
mkdir -p "$OUT_DIR"

# --- helpers -------------------------------------------------------------------

# diff_base prints the commit changed lines are measured against, or nothing.
diff_base() {
  if [[ -n "${LOOM_DIFF_BASE:-}" ]]; then
    echo "$LOOM_DIFF_BASE"
  elif [[ -n "${PR_BASE:-}${PUSH_BEFORE:-}" ]]; then
    bash scripts/ci-diff-base.sh
  elif git rev-parse --verify -q origin/main >/dev/null; then
    git merge-base HEAD origin/main
  fi
}

# coverage_floor reads the ratchet from the workflow, so the number has one home.
coverage_floor() {
  grep -oE 'COVERAGE_FLOOR: "[0-9.]+"' "$WORKFLOW" | grep -oE '[0-9]+(\.[0-9]+)?'
}

not_measured() {
  echo "NOT MEASURED: $1"
  return "$NOT_MEASURED"
}

# --- steps ---------------------------------------------------------------------

step_toolchain() {
  local want have
  want="$(awk '/^go [0-9]/ { print $2; exit }' go.mod)"
  have="$(go env GOVERSION)"
  have="${have#go}"
  if [[ "$have" != "$want" ]]; then
    echo "go.mod requires go $want but this environment has go $have — rebuild the test image"
    return 1
  fi
  echo "go $have"
  golangci-lint version --short 2>/dev/null || golangci-lint --version
  command -v gremlins >/dev/null || { echo "gremlins is not installed"; return 1; }
}

step_build() {
  go build ./...
  (cd examples/embedding && go build ./...)
}

# step_test clears the last run's results first, so a stale profile can never
# stand in for this one.
step_test() {
  rm -f "$COVERAGE_PROFILE" "$TESTS_PASSED"
  go test ./... -coverprofile="$COVERAGE_PROFILE"
  touch "$TESTS_PASSED"
}

step_coverage() {
  [[ -f "$COVERAGE_PROFILE" ]] || not_measured "no coverage profile — run the test step first"
  local floor total base
  floor="$(coverage_floor)"
  total="$(go tool cover -func="$COVERAGE_PROFILE" | awk '/^total:/ { sub(/%/, "", $3); print $3 }')"
  echo "combined statement coverage: ${total}% (ratchet floor: ${floor}%)"
  # As in CI: a profile from a failing run is reported, never gated on.
  [[ -f "$TESTS_PASSED" ]] || not_measured "tests failed, so coverage is not comparable to a clean run"
  awk -v t="$total" -v f="$floor" 'BEGIN { exit (t + 0 < f + 0) ? 1 : 0 }' \
    || { echo "coverage ${total}% is below the ratchet floor ${floor}%"; return 1; }
  base="$(diff_base)"
  [[ -n "$base" ]] || not_measured "changed-line coverage: no diff base (set LOOM_DIFF_BASE)"
  go run ./cmd/diff-coverage --base "$base" --profile "$COVERAGE_PROFILE"
}

step_lint() {
  golangci-lint run ./...
}

step_mutation() {
  local base
  base="$(diff_base)"
  [[ -n "$base" ]] || not_measured "no diff base (set LOOM_DIFF_BASE)"
  go run ./cmd/diff-mutation --base "$base" --gremlins "$(command -v gremlins)"
}

# step_smoke proves the binary a user would install: it installs, the install
# is healthy, the intake template arrives, and `loom intake check` both accepts
# a valid order and rejects an invalid one — so a pass is not a vacuous one.
step_smoke() {
  local binary="$OUT_DIR/loom" project="$OUT_DIR/scratch-project"
  rm -rf "$project"
  mkdir -p "$project"
  go build -o "$binary" ./cmd/loom
  git -C "$project" init -q
  "$binary" install --target "$project" --platform claude-code --copy
  "$binary" health --target "$project"
  [[ -f "$project/intake/TEMPLATE.md" ]] || { echo "install did not create intake/TEMPLATE.md"; return 1; }
  write_smoke_order "$project/intake/smoke-order.md" "smoke-order"
  (cd "$project" && "$binary" intake check) | tee "$OUT_DIR/intake-valid.txt"
  grep -q "valid: 1 order(s)" "$OUT_DIR/intake-valid.txt"
  write_smoke_order "$project/intake/misnamed.md" "smoke-order"
  if (cd "$project" && "$binary" intake check >/dev/null); then
    echo "loom intake check accepted an order whose id does not match its file name"
    return 1
  fi
  echo "intake check rejected the misnamed order, as it must"
}

write_smoke_order() {
  cat >"$1" <<ORDER
---
intake: 1
id: $2
type: chore
plan: deliver-feature
status: ready
priority: 3
created: 2026-10-23
requested-by: test-in-container
source: manual
trust: internal
base-branch: main
finish: branch
autonomy: strict-human
budget:
  max-attempts: 1
  max-usd: 1
  max-hours: 1
---
# Smoke order written by scripts/test-in-container.sh
ORDER
}

# --- runner --------------------------------------------------------------------

is_known_step() {
  local candidate="$1" step
  for step in "${ALL_STEPS[@]}"; do
    [[ "$step" == "$candidate" ]] && return 0
  done
  return 1
}

# run_step runs one step in a subshell and returns its status. The caller must
# have `set +e`: the subshell sits outside any conditional, so the `set -e`
# inside it still stops the step at its first failing command (inside `if`,
# bash would silently ignore it), while a failing step cannot end the run.
run_step() {
  echo ""
  echo "=== $1 ==="
  ( set -euo pipefail; "step_$1" )
}

main() {
  local steps=("$@") step status failed=0
  [[ ${#steps[@]} -gt 0 ]] || steps=("${ALL_STEPS[@]}")
  for step in "${steps[@]}"; do
    is_known_step "$step" || { echo "unknown step '$step' (known: ${ALL_STEPS[*]})"; exit 2; }
  done
  local results=()
  for step in "${steps[@]}"; do
    set +e
    run_step "$step"
    status=$?
    set -e
    case "$status" in
      0) results+=("PASS          $step") ;;
      "$NOT_MEASURED") results+=("NOT MEASURED  $step") ;;
      *) results+=("FAIL          $step"); failed=1 ;;
    esac
  done
  echo ""
  echo "=== summary ==="
  printf '%s\n' "${results[@]}"
  exit "$failed"
}

main "$@"
