#!/usr/bin/env bash
# Fails when a committed platform config no longer matches what generate-configs.sh produces
# from shared/ (roadmap C.1, audit H11).
#
# Why this and not check-parity.sh: parity checks that each generated file *mentions* the core
# concepts. It passed while .cursorrules and .windsurfrules — which must be byte-identical — could
# have diverged, and while AGENTS.md and .openai.md did drift apart. This regenerates into a
# scratch directory and compares bytes, so a hand-edit to a generated file, or a shared/ change
# committed without regenerating, fails here instead of shipping.
#
# Two directions are checked:
#   drift   — a file the generator emits differs from the committed copy (or is not committed)
#   orphan  — a tracked file in a generator-owned directory that the generator no longer emits
#
# The working tree is never written: output goes to a mktemp directory.
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRATCH_DIR="$(mktemp -d)"
trap 'rm -rf "$SCRATCH_DIR"' EXIT

problem_count=0

report() { echo "  $1  $2"; problem_count=$((problem_count + 1)); }

generate_into_scratch() {
  bash "$REPO_DIR/scripts/generate-configs.sh" --output "$SCRATCH_DIR" > /dev/null
}

generated_paths() {
  (cd "$SCRATCH_DIR" && find . -type f | sed 's|^\./||' | sort)
}

check_drift() {
  local path
  while IFS= read -r path; do
    if [[ ! -f "$REPO_DIR/$path" ]]; then
      report "MISSING" "$path"
    elif ! cmp -s "$SCRATCH_DIR/$path" "$REPO_DIR/$path"; then
      report "DRIFT  " "$path"
    fi
  done
}

# A directory is generator-owned when the generator writes into it — except the repo root and
# .github/, which also hold hand-written files (workflows, README).
owned_directories() {
  generated_paths | xargs -n1 dirname | sort -u | grep -vxE '\.|\.github'
}

check_orphans() {
  local generated directory tracked
  generated="$(generated_paths)"
  while IFS= read -r directory; do
    while IFS= read -r tracked; do
      grep -qxF "$tracked" <<< "$generated" || report "ORPHAN " "$tracked"
    done < <(git -C "$REPO_DIR" ls-files -- "$directory")
  done < <(owned_directories)
}

main() {
  echo ""
  echo "=== Generated Config Drift Check ==="
  generate_into_scratch
  echo "  $(generated_paths | wc -l | tr -d ' ') generated files compared against the committed copies"
  # Process substitution, not a pipe: a pipe runs check_drift in a subshell and its problem_count
  # is lost — the first version of this script printed DRIFT and then exited 0.
  check_drift < <(generated_paths)
  check_orphans
  echo ""
  if [[ $problem_count -gt 0 ]]; then
    echo "Result: $problem_count problem(s). Run 'bash scripts/generate-configs.sh' and commit the result;"
    echo "        delete ORPHAN files the generator no longer produces."
    exit 1
  fi
  echo "Result: committed configs match generate-configs.sh output."
}

main
