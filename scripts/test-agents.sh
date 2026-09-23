#!/usr/bin/env bash
set -euo pipefail

# Golden-file structural tests for pipeline agent prompts (shared/agents/).
#
# This script does NOT invoke any LLM agent itself — generating actual-output.md
# is a manual step (see tests/agents/README.md). It only validates a
# previously-generated actual-output.md against:
#   1. expected-patterns.txt — fuzzy, scenario-specific patterns
#   2. the agent's contract in shared/contracts/ — required section headings, if one exists
#
# Run this after editing any agent prompt in shared/agents/.

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TESTS_DIR="$REPO_DIR/tests/agents"
CONTRACTS_DIR="$REPO_DIR/shared/contracts"

# bash 3.2 (macOS default) has no associative arrays — use a function instead.
contract_for_agent() {
  case "$1" in
    # Pipeline agents
    analyst) echo "analysis-contract.md" ;;
    architect) echo "architecture-contract.md" ;;
    code-reviewer) echo "review-contract.md" ;;
    security-reviewer) echo "security-contract.md" ;;
    qa-engineer) echo "qa-contract.md" ;;
    tech-writer) echo "docs-contract.md" ;;
    devops-engineer) echo "devops-contract.md" ;;
    sre-engineer) echo "observability-contract.md" ;;
    data-engineer) echo "data-contract.md" ;;
    performance-engineer) echo "performance-contract.md" ;;
    accessibility-engineer) echo "accessibility-contract.md" ;;
    visual-qa-engineer) echo "visual-qa-contract.md" ;;
    # Delivery-role agents
    developer) echo "implementation-contract.md" ;;
    context-engineer) echo "context-manifest-contract.md" ;;
    spec-writer) echo "spec-contract.md" ;;
    product-owner) echo "product-review-contract.md" ;;
    release-manager) echo "release-plan-contract.md" ;;
    unit-tester) echo "unit-test-contract.md" ;;
    refactor-engineer) echo "refactoring-contract.md" ;;
    # Counter/auditor agents (no contract files — output is audit findings, no fixed schema)
    agent-evaluator) echo "" ;;
    context-auditor) echo "" ;;
    documentation-auditor) echo "" ;;
    documentation-manager) echo "" ;;
    knowledge-auditor) echo "" ;;
    memory-auditor) echo "" ;;
    model-tier-auditor) echo "" ;;
    pattern-reviewer) echo "" ;;
    privacy-auditor) echo "" ;;
    prompt-evaluator) echo "" ;;
    retrieval-evaluator) echo "" ;;
    rule-auditor) echo "" ;;
    tool-validator) echo "" ;;
    *) echo "" ;;
  esac
}

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0

pass() { echo "  PASS  $1"; ((PASS_COUNT++)) || true; }
fail() { echo "  FAIL  $1"; ((FAIL_COUNT++)) || true; }
skip() { echo "  SKIP  $1"; ((SKIP_COUNT++)) || true; }

echo ""
echo "=== Agent Golden-File Tests ==="
echo "Fixtures: $TESTS_DIR"
echo ""

if [[ ! -d "$TESTS_DIR" ]]; then
  echo "FAIL: no tests/agents/ directory found — the agent suite cannot be green by absence."
  exit 1
fi

# --- Fixture manifest check (roadmap M0.2, audit H9) ---------------------------
# A missing required fixture is a FAIL, not a SKIP: every file listed in
# fixture-manifest.txt must exist, and every committed golden baseline
# (actual-output.md) must be listed there, so deleting either can never pass.
MANIFEST="$TESTS_DIR/fixture-manifest.txt"
MANIFEST_VERIFIED=0
echo "--- fixture manifest ---"
if [[ -f "$MANIFEST" ]]; then
  while IFS= read -r rel || [[ -n "$rel" ]]; do
    [[ -z "$rel" || "$rel" == \#* ]] && continue
    if [[ -f "$TESTS_DIR/$rel" ]]; then
      ((MANIFEST_VERIFIED++)) || true
    else
      fail "required fixture missing: tests/agents/$rel (listed in fixture-manifest.txt)"
    fi
  done < "$MANIFEST"
  while IFS= read -r found; do
    rel="${found#"$TESTS_DIR"/}"
    if ! grep -qxF -- "$rel" "$MANIFEST"; then
      fail "unlisted golden baseline: tests/agents/$rel — add it to fixture-manifest.txt"
    fi
  done < <(find "$TESTS_DIR" -name actual-output.md)
  echo "  $MANIFEST_VERIFIED manifest fixtures verified"
else
  fail "tests/agents/fixture-manifest.txt is missing — required since roadmap M0.2"
fi
echo ""

for agent_dir in "$TESTS_DIR"/*/; do
  agent_name="$(basename "$agent_dir")"
  actual="$agent_dir/actual-output.md"
  expected="$agent_dir/expected-patterns.txt"

  echo "--- $agent_name ---"

  if [[ ! -f "$actual" ]]; then
    # SKIP is legitimate only for agents with no committed baseline; a baseline
    # listed in fixture-manifest.txt that goes missing is caught as FAIL above.
    skip "$agent_name — no golden baseline committed yet. Run this agent against its input-* fixture in a live Claude Code session, save the output as actual-output.md, add it to fixture-manifest.txt, then re-run this script."
    echo ""
    continue
  fi

  if [[ -f "$expected" ]]; then
    while IFS= read -r pattern || [[ -n "$pattern" ]]; do
      [[ -z "$pattern" || "$pattern" == \#* ]] && continue
      if grep -qEi -- "$pattern" "$actual"; then
        pass "pattern: $pattern"
      else
        fail "pattern: $pattern — not found"
      fi
    done < "$expected"
  else
    echo "  (no expected-patterns.txt — skipping pattern checks)"
  fi

  contract_file="$(contract_for_agent "$agent_name")"
  if [[ -n "$contract_file" && -f "$CONTRACTS_DIR/$contract_file" ]]; then
    while IFS= read -r line; do
      # Match only "- `## Heading`" bullets from Required Sections. The capture group
      # forbids backticks so a Validation Rules bullet — which quotes a heading and then
      # continues with more backticked prose — is not mistaken for a required section.
      if [[ "$line" =~ ^-\ \`(#{2,6}[^\`]+)\`$ ]]; then
        section="${BASH_REMATCH[1]}"
        if grep -qF -- "$section" "$actual"; then
          pass "contract section: $section"
        else
          fail "contract section: $section — missing (see $contract_file)"
        fi
      fi
    done < "$CONTRACTS_DIR/$contract_file"
  fi

  echo ""
done

echo "==========================================="
echo "Results: $PASS_COUNT passed, $FAIL_COUNT failed, $SKIP_COUNT skipped"
echo ""

if [[ $FAIL_COUNT -gt 0 ]]; then
  exit 1
fi
