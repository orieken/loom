#!/usr/bin/env bash
# Prints the commit a CI run should diff against, for the changed-line checks
# (cmd/diff-coverage, cmd/diff-mutation — roadmap L3.58, L3.59).
#
# PR_BASE     the pull request's base SHA (empty on a push)
# PUSH_BEFORE the branch head before this push (empty on a pull request)
#
# A push is measured against its pre-push head, so every commit in the push is
# covered, not only the last. When that commit is absent — a new branch reports
# all zeros, a force push can name one no longer in history — the fallback is
# HEAD~1, announced on stderr so nobody mistakes it for the whole push.
set -euo pipefail

base="${PR_BASE:-${PUSH_BEFORE:-}}"
if [[ -z "$base" || "$base" =~ ^0+$ ]] || ! git cat-file -e "${base}^{commit}" 2>/dev/null; then
  echo "base '${base}' is not available (new branch or rewritten history) — measuring against HEAD~1" >&2
  base="HEAD~1"
fi
echo "$base"
