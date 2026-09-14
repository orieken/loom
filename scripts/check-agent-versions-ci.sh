#!/usr/bin/env bash
set -euo pipefail

# CI equivalent of scripts/hooks/pre-commit, for a PR/branch comparison instead of staged-vs-HEAD.
# Checks that every shared/agents/*.md file that differs between BASE_REF and HEAD_REF has a bumped
# `version:` field, and that shared/agents/CHANGELOG.md mentions that agent's name somewhere.
#
# Usage: check-agent-versions-ci.sh [base-ref] [head-ref]
#   Defaults: base-ref = origin/main (or $GITHUB_BASE_REF if set), head-ref = HEAD
#
# Written for bash 3.2 — see scripts/test-agents.sh for why (no associative arrays), and every
# grep pipeline ends with `|| true` where a legitimate zero-match result would otherwise abort the
# script under `set -e` + `pipefail` (the same bug found and fixed three times already in this repo).

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_DIR"

BASE_REF="${1:-${GITHUB_BASE_REF:-origin/main}}"
HEAD_REF="${2:-HEAD}"
CHANGELOG="shared/agents/CHANGELOG.md"

echo ""
echo "=== Agent Version Check (CI) ==="
echo "Comparing: $BASE_REF..$HEAD_REF"
echo ""

CHANGED_FILES=$( (git diff --name-only --diff-filter=ACM "$BASE_REF...$HEAD_REF" -- 'shared/agents/*.md' 2>/dev/null || true) | (grep -v "^${CHANGELOG}$" || true) )

if [[ -z "$CHANGED_FILES" ]]; then
  echo "No shared/agents/*.md changes in this range — nothing to check."
  exit 0
fi

CHANGELOG_CHANGED=false
if (git diff --name-only --diff-filter=ACM "$BASE_REF...$HEAD_REF" 2>/dev/null || true) | grep -qF "$CHANGELOG"; then
  CHANGELOG_CHANGED=true
fi

FAILED=0

# changelog_mentions avoids `git show | grep -q`, which returns 141 under
# `set -o pipefail`: grep -q exits on the first match, git show is killed
# writing to the closed pipe, and the pipeline's status becomes SIGPIPE even
# though the pattern was found. That false-FAILed every agent change whose
# name appeared early in a CHANGELOG long enough for git show to still be
# writing. Buffer first, match second — no pipe, no signal.
changelog_mentions() {
  local name="$1" body
  body=$(git show "$HEAD_REF:$CHANGELOG" 2>/dev/null || true)
  [[ -n "$body" ]] && grep -qF -- "$name" <<< "$body"
}

get_version() {
  local ref="$1" file="$2"
  git show "$ref:$file" 2>/dev/null | grep '^version:' | head -1 | sed 's/version: *//' || true
}

while IFS= read -r file; do
  [[ -z "$file" ]] && continue

  agent_name=$(git show "$HEAD_REF:$file" 2>/dev/null | grep '^name:' | head -1 | sed 's/name: *//' || true)
  new_version=$(get_version "$HEAD_REF" "$file")
  old_version=$(get_version "$BASE_REF" "$file")

  echo "--- $file ($agent_name) ---"

  if [[ -z "$new_version" ]]; then
    echo "  FAIL: no 'version:' frontmatter field at $HEAD_REF."
    FAILED=1
    continue
  fi

  if [[ -n "$old_version" && "$old_version" == "$new_version" ]]; then
    echo "  FAIL: version unchanged ($old_version) between $BASE_REF and $HEAD_REF."
    FAILED=1
  else
    echo "  OK: version ${old_version:-<new agent>} -> $new_version"
  fi

  if ! $CHANGELOG_CHANGED; then
    echo "  FAIL: $CHANGELOG was not touched in this range. Add a dated entry for '$agent_name'."
    FAILED=1
  elif [[ -n "$agent_name" ]] && ! changelog_mentions "$agent_name"; then
    echo "  FAIL: $CHANGELOG doesn't mention '$agent_name' at $HEAD_REF. Add a row for it."
    FAILED=1
  fi
done <<< "$CHANGED_FILES"

echo ""

if [[ $FAILED -ne 0 ]]; then
  echo "Agent version check failed — see FAIL lines above."
  exit 1
fi

echo "Agent version check passed."
