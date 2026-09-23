#!/usr/bin/env bash
set -euo pipefail

# Headless agent-eval runner (Epic 61).
#
# Runs the agent-eval harness against one or more agent fixtures in tests/agents/:
#   1. Pattern check (deterministic, always — no API key required)
#   2. LLM generation via `claude --bare -p` (requires ANTHROPIC_API_KEY)
#   3. Rubric judge via a second cheaper claude --bare -p call (light tier)
#   4. Regression comparison against the previous eval in shared/evaluation/agent-evals/
#   5. Eval record written to shared/evaluation/agent-evals/<agent>-eval-<date>.md
#
# Usage: bash scripts/run-agent-evals.sh [--agents <a,b,...>] [--pattern-only] [--no-judge]
#
# Flags:
#   --agents <list>   Comma-separated agent names to evaluate (default: all with fixtures)
#   --pattern-only    Skip LLM calls; run pattern + contract checks only (zero cost)
#   --no-judge        Run generation but skip rubric grading (~$2 instead of ~$2.10)
#
# No API key → LLM steps SKIP (exit 0). Pattern checks always run.
# Written for bash 3.2 (macOS default) — no associative arrays.

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TESTS_DIR="$REPO_DIR/tests/agents"
AGENTS_DIR="$REPO_DIR/shared/agents"
CONTRACTS_DIR="$REPO_DIR/shared/contracts"
EVALS_DIR="$REPO_DIR/shared/evaluation/agent-evals"
DEFAULTS_YAML="$REPO_DIR/shared/model-defaults.yaml"
TODAY="$(date -u +%Y-%m-%d)"
JUDGE_MODEL="claude-haiku-4-5-20251001"

PATTERN_ONLY=false
NO_JUDGE=false
AGENTS_FILTER=""

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0
REGRESSION_COUNT=0

pass()       { echo "  PASS  $1"; ((PASS_COUNT++)) || true; }
fail()       { echo "  FAIL  $1"; ((FAIL_COUNT++)) || true; }
skip()       { echo "  SKIP  $1"; ((SKIP_COUNT++)) || true; }
regression() { echo "  REGR  $1"; ((REGRESSION_COUNT++)) || true; }

# --- Argument parsing -------------------------------------------------------

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --agents)       AGENTS_FILTER="$2"; shift 2 ;;
      --pattern-only) PATTERN_ONLY=true; shift ;;
      --no-judge)     NO_JUDGE=true; shift ;;
      -h|--help)      usage; exit 0 ;;
      *) echo "Unknown flag: $1"; usage; exit 1 ;;
    esac
  done
}

usage() {
  echo "Usage: bash scripts/run-agent-evals.sh [--agents <a,b,...>] [--pattern-only] [--no-judge]"
}

# --- Model resolution -------------------------------------------------------

resolve_model() {
  local agent_file="$1"
  local tier
  tier=$(grep '^model_tier:' "$agent_file" 2>/dev/null | head -1 | sed 's/model_tier:[[:space:]]*//')
  case "$tier" in
    light) echo "$JUDGE_MODEL" ;;
    heavy) echo "claude-opus-5" ;;
    *)     echo "inherit" ;;
  esac
}

model_flag() {
  local model="$1"
  [[ "$model" == "inherit" ]] && echo "" || echo "--model $model"
}

# --- Contract helpers (mirrors test-agents.sh) ------------------------------

contract_for_agent() {
  case "$1" in
    analyst)              echo "analysis-contract.md" ;;
    architect)            echo "architecture-contract.md" ;;
    code-reviewer)        echo "review-contract.md" ;;
    security-reviewer)    echo "security-contract.md" ;;
    qa-engineer)          echo "qa-contract.md" ;;
    tech-writer)          echo "docs-contract.md" ;;
    devops-engineer)      echo "devops-contract.md" ;;
    sre-engineer)         echo "observability-contract.md" ;;
    data-engineer)        echo "data-contract.md" ;;
    performance-engineer) echo "performance-contract.md" ;;
    accessibility-engineer) echo "accessibility-contract.md" ;;
    visual-qa-engineer)   echo "visual-qa-contract.md" ;;
    developer)            echo "implementation-contract.md" ;;
    context-engineer)     echo "context-manifest-contract.md" ;;
    spec-writer)          echo "spec-contract.md" ;;
    product-owner)        echo "product-review-contract.md" ;;
    release-manager)      echo "release-plan-contract.md" ;;
    unit-tester)          echo "unit-test-contract.md" ;;
    refactor-engineer)    echo "refactoring-contract.md" ;;
    *)                    echo "" ;;
  esac
}

# --- Pattern and contract checks (deterministic) ----------------------------

run_pattern_check() {
  local agent="$1"
  local actual="$2"
  local fixture_dir="$3"
  local p_pass=0
  local p_fail=0

  local expected="$fixture_dir/expected-patterns.txt"
  if [[ -f "$expected" ]]; then
    while IFS= read -r pattern || [[ -n "$pattern" ]]; do
      [[ -z "$pattern" || "$pattern" == \#* ]] && continue
      if grep -qEi -- "$pattern" "$actual" 2>/dev/null; then
        pass "pattern: $pattern"
        ((p_pass++)) || true
      else
        fail "pattern: $pattern — not found"
        ((p_fail++)) || true
      fi
    done < "$expected"
  fi

  local contract_file
  contract_file="$(contract_for_agent "$agent")"
  if [[ -n "$contract_file" && -f "$CONTRACTS_DIR/$contract_file" ]]; then
    while IFS= read -r line; do
      if [[ "$line" =~ ^-\ \`(.+)\`$ ]]; then
        local section="${BASH_REMATCH[1]}"
        if grep -qF -- "$section" "$actual" 2>/dev/null; then
          pass "contract: $section"
          ((p_pass++)) || true
        else
          fail "contract: $section — missing"
          ((p_fail++)) || true
        fi
      fi
    done < "$CONTRACTS_DIR/$contract_file"
  fi

  [[ $p_fail -eq 0 ]] && echo "PASS" || echo "FAIL"
}

# --- LLM generation ---------------------------------------------------------

run_generation() {
  local agent="$1"
  local fixture_dir="$2"
  local actual="$3"
  local agent_file="$AGENTS_DIR/${agent}.md"

  if [[ ! -f "$agent_file" ]]; then
    skip "$agent — no shared/agents/${agent}.md found"
    return 1
  fi

  local input_file
  input_file=$(find "$fixture_dir" -maxdepth 1 -name 'input-*' | head -1)
  if [[ -z "$input_file" ]]; then
    skip "$agent — no input-* fixture found"
    return 1
  fi

  local model
  model=$(resolve_model "$agent_file")
  local mflag
  mflag=$(model_flag "$model")
  local version
  version=$(grep '^version:' "$agent_file" 2>/dev/null | head -1 | sed 's/version:[[:space:]]*//' || echo "unknown")

  local gen_prompt
  gen_prompt="Read shared/agents/${agent}.md in full. Act as that agent. Apply its complete \
Process and Output Format to the content of $(basename "$input_file") in tests/agents/${agent}/. \
Produce the full markdown output only — no preamble, no 'I will now' framing."

  # shellcheck disable=SC2086
  if claude --bare -p $mflag "$gen_prompt" > "$actual" 2>/dev/null; then
    echo "  GEN   $agent (model: $model, version: $version)"
    echo "$model:$version"
    return 0
  else
    skip "$agent — generation failed (claude -p returned non-zero)"
    rm -f "$actual"
    return 1
  fi
}

# --- Rubric judging ---------------------------------------------------------

run_rubric_judge() {
  local agent="$1"
  local fixture_dir="$2"
  local actual="$3"
  local rubric="$fixture_dir/eval-rubric.md"
  local grade_file="/tmp/rubric-grade-${agent}-$$.txt"

  if [[ ! -f "$rubric" ]]; then
    echo "SKIP"
    return
  fi

  local judge_prompt
  judge_prompt="Read tests/agents/${agent}/eval-rubric.md and tests/agents/${agent}/actual-output.md. \
For EACH criterion in eval-rubric.md output exactly one line: \
PASS <criterion-label> | <one-line quote from actual-output.md> \
or FAIL <criterion-label> | <one-line explanation of what is missing>. \
Output nothing else — one line per criterion only."

  if ! claude --bare -p --model "$JUDGE_MODEL" "$judge_prompt" > "$grade_file" 2>/dev/null; then
    echo "SKIP"
    rm -f "$grade_file"
    return
  fi

  local r_pass=0
  local r_fail=0
  while IFS= read -r line; do
    case "$line" in
      PASS\ *) pass "rubric: ${line#PASS }"; ((r_pass++)) || true ;;
      FAIL\ *) fail "rubric: ${line#FAIL }"; ((r_fail++)) || true ;;
    esac
  done < "$grade_file"
  rm -f "$grade_file"

  [[ $r_fail -eq 0 && $r_pass -gt 0 ]] && echo "PASS" || echo "FAIL"
}

# --- Regression comparison --------------------------------------------------

find_previous_eval() {
  local agent="$1"
  find "$EVALS_DIR" -maxdepth 1 -name "${agent}-eval-*.md" 2>/dev/null \
    | sort -r | head -1
}

read_grade_field() {
  local file="$1"
  local field="$2"
  grep "^\*\*${field}\*\*:" "$file" 2>/dev/null | head -1 | sed "s/^\*\*${field}\*\*:[[:space:]]*//"
}

check_regression() {
  local agent="$1"
  local pattern_grade="$2"
  local rubric_grade="$3"
  local prev_file="$4"

  if [[ -z "$prev_file" ]]; then
    echo "BASELINE"
    return
  fi

  local prev_pattern prev_rubric
  prev_pattern=$(read_grade_field "$prev_file" "Pattern overall")
  prev_rubric=$(read_grade_field "$prev_file" "Rubric overall")

  local delta="STABLE"
  if [[ "$prev_pattern" == "PASS" && "$pattern_grade" == "FAIL" ]]; then
    regression "$agent — pattern regressed (was PASS, now FAIL)"
    delta="REGRESSION"
  fi
  if [[ "$prev_rubric" == "PASS" && "$rubric_grade" == "FAIL" ]]; then
    regression "$agent — rubric regressed (was PASS, now FAIL)"
    delta="REGRESSION"
  fi
  echo "$delta"
}

# --- Eval record writer -----------------------------------------------------

write_eval_record() {
  local agent="$1" version="$2" model="$3" input_file="$4"
  local pattern_grade="$5" rubric_grade="$6"
  local prev_file="$7" delta="$8"
  local out_file="$EVALS_DIR/${agent}-eval-${TODAY}.md"
  local prev_label
  prev_label=$(basename "${prev_file:-}" 2>/dev/null || echo "none")
  [[ -z "$prev_file" ]] && prev_label="no baseline — first recorded eval"

  mkdir -p "$EVALS_DIR"
  cat > "$out_file" <<RECORD
# Agent Eval: ${agent} — ${TODAY}

**Agent version**: ${version}
**Model used**: ${model}
**Fixture**: tests/agents/${agent}/${input_file}
**Run mode**: $( $PATTERN_ONLY && echo "pattern-only" || ( $NO_JUDGE && echo "no-judge" || echo "full" ) )

## Pattern Grade

(See terminal output above for per-pattern results.)

**Pattern overall**: ${pattern_grade}

## Rubric Grade

(See terminal output above for per-criterion results.)

**Rubric overall**: ${rubric_grade}

## Regression Delta

Compared against: ${prev_label}

**Overall delta**: ${delta}

---
*Generated by scripts/run-agent-evals.sh on ${TODAY}.*
RECORD

  echo "  REC   wrote $out_file"
}

# --- Single-agent orchestration ---------------------------------------------

eval_one_agent() {
  local agent="$1"
  local fixture_dir="$TESTS_DIR/$agent"
  local actual="$fixture_dir/actual-output.md"

  echo ""
  echo "--- $agent ---"

  if [[ ! -d "$fixture_dir" ]]; then
    skip "$agent — no fixture directory"
    return
  fi

  local pattern_grade="SKIP"
  local rubric_grade="SKIP"
  local model="n/a"
  local version="n/a"
  local input_basename="n/a"

  if $PATTERN_ONLY; then
    [[ ! -f "$actual" ]] && { skip "$agent — no actual-output.md (run generation first)"; return; }
    pattern_grade=$(run_pattern_check "$agent" "$actual" "$fixture_dir")
    rubric_grade="SKIP"
  else
    if [[ -z "${ANTHROPIC_API_KEY:-}" ]]; then
      skip "$agent — ANTHROPIC_API_KEY not set; pattern check only"
      [[ -f "$actual" ]] && pattern_grade=$(run_pattern_check "$agent" "$actual" "$fixture_dir") || true
      return
    fi

    local gen_result
    if gen_result=$(run_generation "$agent" "$fixture_dir" "$actual"); then
      model=$(echo "$gen_result" | cut -d: -f1)
      version=$(echo "$gen_result" | cut -d: -f2)
      input_basename=$(find "$fixture_dir" -maxdepth 1 -name 'input-*' | head -1 | xargs basename 2>/dev/null || echo "n/a")
      pattern_grade=$(run_pattern_check "$agent" "$actual" "$fixture_dir")
      if $NO_JUDGE; then
        rubric_grade="SKIP"
      else
        rubric_grade=$(run_rubric_judge "$agent" "$fixture_dir" "$actual")
      fi
    else
      return
    fi
  fi

  local prev_file
  prev_file=$(find_previous_eval "$agent")
  local delta
  delta=$(check_regression "$agent" "$pattern_grade" "$rubric_grade" "$prev_file")

  if ! $PATTERN_ONLY; then
    write_eval_record "$agent" "$version" "$model" "$input_basename" \
      "$pattern_grade" "$rubric_grade" "$prev_file" "$delta"
  fi
}

# --- Agent list helpers -----------------------------------------------------

all_fixture_agents() {
  find "$TESTS_DIR" -mindepth 1 -maxdepth 1 -type d | sort | xargs -I{} basename {}
}

agents_to_run() {
  if [[ -n "$AGENTS_FILTER" ]]; then
    echo "$AGENTS_FILTER" | tr ',' '\n'
  else
    all_fixture_agents
  fi
}

# --- Main -------------------------------------------------------------------

main() {
  parse_args "$@"

  mkdir -p "$EVALS_DIR"

  echo ""
  echo "=== Agent Eval Runner ==="
  echo "Mode: $(
    $PATTERN_ONLY && echo "pattern-only" ||
    ( $NO_JUDGE && echo "no-judge (generation + patterns)" || echo "full (generation + patterns + rubric)" )
  )"
  [[ -n "$AGENTS_FILTER" ]] && echo "Filter: $AGENTS_FILTER"
  [[ -z "${ANTHROPIC_API_KEY:-}" ]] && \
    echo "WARN  ANTHROPIC_API_KEY not set — LLM steps will SKIP (pattern checks still run)"
  echo ""

  while IFS= read -r agent; do
    [[ -z "$agent" ]] && continue
    eval_one_agent "$agent"
  done < <(agents_to_run)

  echo ""
  echo "==========================================="
  echo "Results: $PASS_COUNT passed, $FAIL_COUNT failed, $SKIP_COUNT skipped"
  [[ $REGRESSION_COUNT -gt 0 ]] && echo "REGRESSIONS: $REGRESSION_COUNT agent(s) regressed"
  echo ""

  [[ $FAIL_COUNT -gt 0 || $REGRESSION_COUNT -gt 0 ]] && exit 1 || exit 0
}

main "$@"
