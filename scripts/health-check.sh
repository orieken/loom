#!/usr/bin/env bash
set -euo pipefail

# Deterministic backbone for the `health-check` skill (shared/skills/health-check/SKILL.md).
# Covers everything that's actually scriptable; the skill itself adds AI judgment for anything
# that requires reading prose (e.g. whether an orphaned-looking domain term is really unused).
#
# Written for bash 3.2 (macOS default has no associative arrays) — see scripts/test-agents.sh for the
# same constraint, and every grep pipeline ends with `|| true` where a legitimate zero-match result
# would otherwise abort the script under `set -e` + `pipefail` (a real bug found and fixed twice
# already in this repo's other scripts).

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SHARED_DIR="$REPO_DIR/shared"

VERBOSE=false
FIX=false

for arg in "$@"; do
  case "$arg" in
    --verbose) VERBOSE=true ;;
    --fix) FIX=true ;;
  esac
done

PASS_COUNT=0
FAIL_COUNT=0
WARN_COUNT=0

pass() { if $VERBOSE; then echo "  PASS  $1"; fi; ((PASS_COUNT++)) || true; }
fail() { echo "  FAIL  $1"; ((FAIL_COUNT++)) || true; }
warn() { echo "  WARN  $1"; ((WARN_COUNT++)) || true; }

echo ""
echo "=== Framework Health Check ==="
echo "Repository: $REPO_DIR"
if $FIX; then echo "Mode: --fix (will attempt repairs)"; fi
echo ""

# --- 1. Symlinks resolve ---------------------------------------------------
echo "--- Symlinks ---"
for name in agents skills rules; do
  link=".claude/$name"
  expected="../shared/$name"
  if [[ -L "$link" ]]; then
    target=$(readlink "$link")
    if [[ "$target" == "$expected" && -e "$link" ]]; then
      pass "$link -> $target"
    else
      fail "$link -> $target (expected $expected, or target missing)"
      if $FIX; then
        rm -f "$link"
        ln -s "$expected" "$link"
        echo "        [fix] recreated $link -> $expected"
      fi
    fi
  else
    fail "$link is not a symlink"
    if $FIX; then
      rm -rf "$link"
      ln -s "$expected" "$link"
      echo "        [fix] created $link -> $expected"
    fi
  fi
done
echo ""

# --- 2. Agent frontmatter ---------------------------------------------------
echo "--- Agent Frontmatter (name, description, tools, model_tier, version) ---"
for agent_file in "$SHARED_DIR/agents/"*.md; do
  base="$(basename "$agent_file")"
  [[ "$base" == "CHANGELOG.md" ]] && continue

  missing=""
  for field in name description tools version; do
    if ! grep -q "^${field}:" "$agent_file"; then
      missing="$missing $field"
    fi
  done

  tier_val=$(grep '^model_tier:' "$agent_file" | head -1 | awk '{print $2}' || true)
  if [[ -z "$tier_val" ]]; then
    warn "$base — missing model_tier: (WARN cadence for 1 release cycle; will upgrade to FAIL)"
  elif [[ "$tier_val" != "light" && "$tier_val" != "default" && "$tier_val" != "heavy" ]]; then
    fail "$base — invalid model_tier '$tier_val' (must be light, default, or heavy)"
  fi

  if grep -q "^model:" "$agent_file"; then
    warn "$base — has explicit 'model:' override alongside model_tier: (Claude Code will use 'model:', other platforms will use 'model_tier:')"
  fi

  if [[ -z "$missing" ]]; then
    pass "$base"
  else
    fail "$base — missing frontmatter:$missing"
  fi
done
echo ""

# --- 3. Skill frontmatter ---------------------------------------------------
echo "--- Skill Frontmatter (name, description, triggers, standalone) ---"
for skill_dir in "$SHARED_DIR/skills/"*/; do
  skill_name="$(basename "$skill_dir")"
  skill_file="${skill_dir}SKILL.md"

  if [[ ! -f "$skill_file" ]]; then
    fail "$skill_name — no SKILL.md"
    continue
  fi

  missing=""
  for field in name description triggers standalone; do
    if ! grep -q "^${field}:" "$skill_file"; then
      missing="$missing $field"
    fi
  done

  if [[ -z "$missing" ]]; then
    pass "$skill_name"
  else
    fail "$skill_name — missing frontmatter:$missing"
  fi
done
echo ""

# --- 3a. Capability deprecation validation -----------------------------------
# WARN: deprecated with no superseded_by (no migration pointer).
# FAIL: superseded_by points at a name that doesn't exist in agent or skill inventory.
echo "--- Capability Deprecation (status/superseded_by) ---"

# Build name inventories (agents + skills) for cross-reference lookups.
_all_cap_names=""
for _cap_file in "$SHARED_DIR/agents/"*.md; do
  _base="$(basename "$_cap_file")"
  [[ "$_base" == "CHANGELOG.md" ]] && continue
  _n=$(grep '^name:' "$_cap_file" | head -1 | sed 's/name: *//' || true)
  [[ -n "$_n" ]] && _all_cap_names="$_all_cap_names $_n"
done
for _cap_dir in "$SHARED_DIR/skills/"*/; do
  _cap_skill="${_cap_dir}SKILL.md"
  [[ -f "$_cap_skill" ]] || continue
  _n=$(grep '^name:' "$_cap_skill" | head -1 | sed 's/name: *//' || true)
  [[ -n "$_n" ]] && _all_cap_names="$_all_cap_names $_n"
done

_dep_found=0
_dep_issues=0

# Check agents
for _cap_file in "$SHARED_DIR/agents/"*.md; do
  _base="$(basename "$_cap_file")"
  [[ "$_base" == "CHANGELOG.md" ]] && continue
  _status=$(grep '^status:' "$_cap_file" | head -1 | sed 's/status: *//' || true)
  [[ "$_status" == "deprecated" ]] || continue
  ((_dep_found++)) || true
  _aname=$(grep '^name:' "$_cap_file" | head -1 | sed 's/name: *//' || true)
  _succ=$(grep '^superseded_by:' "$_cap_file" | head -1 | sed 's/superseded_by: *//' || true)
  if [[ -z "$_succ" ]]; then
    warn "agent $_aname — deprecated with no superseded_by (add superseded_by: <name>)"
    ((_dep_issues++)) || true
  elif ! echo "$_all_cap_names" | tr ' ' '\n' | grep -qxF "$_succ"; then
    fail "agent $_aname — superseded_by: '$_succ' does not name an existing agent or skill"
    ((_dep_issues++)) || true
  else
    pass "agent $_aname — deprecated, superseded by '$_succ' (exists)"
  fi
done

# Check skills
for _cap_dir in "$SHARED_DIR/skills/"*/; do
  _skill_name="$(basename "$_cap_dir")"
  _cap_skill="${_cap_dir}SKILL.md"
  [[ -f "$_cap_skill" ]] || continue
  _status=$(grep '^status:' "$_cap_skill" | head -1 | sed 's/status: *//' || true)
  [[ "$_status" == "deprecated" ]] || continue
  ((_dep_found++)) || true
  _succ=$(grep '^superseded_by:' "$_cap_skill" | head -1 | sed 's/superseded_by: *//' || true)
  if [[ -z "$_succ" ]]; then
    warn "skill $_skill_name — deprecated with no superseded_by (add superseded_by: <name>)"
    ((_dep_issues++)) || true
  elif ! echo "$_all_cap_names" | tr ' ' '\n' | grep -qxF "$_succ"; then
    fail "skill $_skill_name — superseded_by: '$_succ' does not name an existing agent or skill"
    ((_dep_issues++)) || true
  else
    pass "skill $_skill_name — deprecated, superseded by '$_succ' (exists)"
  fi
done

if [[ "$_dep_found" -eq 0 ]]; then
  pass "no deprecated capabilities"
fi
echo ""

# --- 4. Platform config drift (delegates to check-parity.sh) ---------------
echo "--- Platform Config Drift ---"
if bash "$REPO_DIR/scripts/check-parity.sh" > /tmp/health-check-parity.$$ 2>&1; then
  pass "check-parity.sh — no drift"
else
  fail "check-parity.sh reported drift — see below"
  grep -E "DRIFT|MISS" /tmp/health-check-parity.$$ || true
  if $FIX; then
    echo "        [fix] running scripts/generate-configs.sh"
    bash "$REPO_DIR/scripts/generate-configs.sh" > /dev/null
    echo "        [fix] re-run scripts/check-parity.sh to confirm"
  fi
fi
rm -f /tmp/health-check-parity.$$
echo ""

# --- 4a. Token Footprint Budget ---------------------------------------------
# Monolithic platform configs (AGENTS.md, .cursorrules, etc.) inject static
# prompt overhead on every request. Warn when a file exceeds ~5,000 tokens
# (20 KB at 4 chars/token). Fix: scripts/generate-configs.sh --stack <stacks>
echo "--- Token Footprint Budget (static baseline overhead) ---"
TOKEN_BUDGET_BYTES=20480  # 20 KB ≈ 5,000 tokens
token_files_checked=0
for config_file in \
  "$REPO_DIR/AGENTS.md" \
  "$REPO_DIR/.cursorrules" \
  "$REPO_DIR/.windsurfrules" \
  "$REPO_DIR/.openai.md" \
  "$REPO_DIR/.junie/guidelines.md" \
  "$REPO_DIR/.roomodes"; do
  [[ -f "$config_file" ]] || continue
  rel="${config_file#"$REPO_DIR/"}"
  file_bytes=$(wc -c < "$config_file" | tr -d ' ')
  approx_tokens=$((file_bytes / 4))
  ((token_files_checked++)) || true
  if [[ "$file_bytes" -gt "$TOKEN_BUDGET_BYTES" ]]; then
    warn "$rel — ~${approx_tokens} tokens (${file_bytes} bytes) exceeds 5k budget; run: scripts/generate-configs.sh --stack <stacks>"
  else
    pass "$rel — ~${approx_tokens} tokens (${file_bytes} bytes)"
  fi
done
if [[ "$token_files_checked" -eq 0 ]]; then
  pass "no generated platform configs present — token budget check skipped"
fi
echo ""

# --- 5. Domain dictionary orphaned terms (best-effort) ----------------------
echo "--- Domain Dictionary Orphaned Terms (best-effort) ---"
DICT="$REPO_DIR/DOMAIN_DICTIONARY.md"
if [[ -f "$DICT" ]]; then
  terms=$(grep -oE '^\| \*\*[A-Za-z][^*]*\*\*' "$DICT" | sed 's/^| \*\*//; s/\*\*$//' || true)
  while IFS= read -r term; do
    [[ -z "$term" ]] && continue
    # Case-insensitive, and also tries the kebab-case form: several dictionary terms are defined
    # Title Case but only ever appear as a path/slug in prose (e.g. "Pipeline Trace" -> only shows up
    # as pipeline-trace.json / the pipeline-trace skill). Also checks root-level *.md files in addition
    # to shared/ and docs/; retained blueprint prompts now live under docs/blueprints/. Still best-effort:
    # a term embedded inside markdown code-formatting broken across words (like `` `api` fixture ``,
    # where a backtick sits between "api" and "fixture") won't match either variant.
    hyphenated=$(echo "$term" | tr '[:upper:]' '[:lower:]' | tr ' ' '-')
    hits=$( { grep -rliF "$term" "$SHARED_DIR" "$REPO_DIR/docs" "$REPO_DIR"/*.md 2>/dev/null || true; \
              grep -rliF "$hyphenated" "$SHARED_DIR" "$REPO_DIR/docs" "$REPO_DIR"/*.md 2>/dev/null || true; } \
            | (grep -v "DOMAIN_DICTIONARY.md" || true) | sort -u | wc -l | tr -d ' ')
    if [[ "$hits" -eq 0 ]]; then
      warn "\"$term\" — defined but not referenced anywhere in shared/, docs/, or root *.md files (may be a framework-level term used only in generated project code, not this repo — verify before removing)"
    else
      pass "\"$term\" — referenced in $hits file(s)"
    fi
  done <<< "$terms"
else
  fail "DOMAIN_DICTIONARY.md not found"
fi
echo ""

# --- 6. Inter-agent contracts exist for pipeline agents ---------------------
# Parsed directly from validate-artifact/SKILL.md's Contract Mapping table rather than a second
# hardcoded list here -- this list drifted out of sync with that table twice already (missed
# context-engineer, then the 5 agents added in Epic 5) before switching to a single source of truth.
echo "--- Inter-Agent Contracts ---"
VALIDATE_ARTIFACT_SKILL="$SHARED_DIR/skills/validate-artifact/SKILL.md"
if [[ -f "$VALIDATE_ARTIFACT_SKILL" ]]; then
  mapping_rows=$(grep -E '^\| [a-z-]+ \| .*shared/contracts/[a-z-]+\.md' "$VALIDATE_ARTIFACT_SKILL" || true)
  while IFS= read -r row; do
    [[ -z "$row" ]] && continue
    agent=$(echo "$row" | awk -F'|' '{print $2}' | tr -d ' ')
    contract=$(echo "$row" | grep -oE 'shared/contracts/[a-z-]+\.md' | xargs basename)
    if [[ -f "$SHARED_DIR/contracts/$contract" ]]; then
      pass "$agent -> $contract"
    else
      fail "$agent has no contract at shared/contracts/$contract"
    fi
  done <<< "$mapping_rows"
else
  fail "shared/skills/validate-artifact/SKILL.md not found — cannot verify inter-agent contracts"
fi
echo ""

# --- 7. Agent changelog up to date (no version mismatches) -----------------
echo "--- Changelog / Version Consistency ---"
CHANGELOG="$SHARED_DIR/agents/CHANGELOG.md"
if [[ -f "$CHANGELOG" ]]; then
  for agent_file in "$SHARED_DIR/agents/"*.md; do
    base="$(basename "$agent_file")"
    [[ "$base" == "CHANGELOG.md" ]] && continue
    name=$(grep '^name:' "$agent_file" | head -1 | sed 's/name: *//' || true)
    version=$(grep '^version:' "$agent_file" | head -1 | sed 's/version: *//' || true)
    [[ -z "$name" || -z "$version" ]] && continue

    if grep -qF "$version" "$CHANGELOG" && grep -qF "$name" "$CHANGELOG"; then
      pass "$name $version — mentioned in CHANGELOG.md"
    else
      warn "$name $version — not found together in CHANGELOG.md (current version may be undocumented)"
    fi
  done
else
  fail "shared/agents/CHANGELOG.md not found"
fi
echo ""

# --- 7a. Test-repair contract is carried by every test-writing agent --------
# Fitness function for shared/rules/test-repair-contract.md (roadmap L3.39).
#
# The set is pinned rather than computed. Deriving it from "has Write or Edit"
# binds 20 agents, 12 of which never touch a test file — a rule that most of
# its audience cannot violate is a rule people skim. Deriving it from "mentions
# tests in its description" catches 7 and misses `developer`, which writes
# tests in every TDD cycle and is the agent most likely to delete an assertion
# under deadline. So: an explicit list, and adding to it is a deliberate edit.
echo "--- Test Repair Contract (agents that can write a test file) ---"
CONTRACT_RULE="shared/rules/test-repair-contract.md"
TEST_WRITING_AGENTS=(
  developer qa-engineer dx-engineer refactor-engineer
  test-driven-developer unit-tester api-test-generator visual-qa-engineer
)
if [[ -f "$SHARED_DIR/rules/test-repair-contract.md" ]]; then
  pass "$CONTRACT_RULE exists"
  for agent in "${TEST_WRITING_AGENTS[@]}"; do
    agent_file="$SHARED_DIR/agents/${agent}.md"
    if [[ ! -f "$agent_file" ]]; then
      fail "pinned test-writing agent '$agent' has no file — update TEST_WRITING_AGENTS or restore it"
    elif grep -qF "test-repair-contract.md" "$agent_file"; then
      pass "$agent references the test repair contract"
    else
      fail "$agent can write test files but does not reference $CONTRACT_RULE"
    fi
  done
else
  fail "$CONTRACT_RULE is missing, but agents are bound by it"
fi
echo ""

# --- 7b. Exemplar manifest and annotations agree ----------------------------
# Fitness function for shared/contracts/exemplar-contract.md (roadmap L3.40).
#
# An exemplar is marked twice — an annotation on the test, an entry in the
# manifest — and the contract's answer to "that is two sources of truth" is
# that this asserts them equal in BOTH directions. A manifest entry whose test
# lost its annotation is a stale pointer; an annotated test missing from the
# manifest is invisible to every retrieval path the framework has.
#
# Absent manifest is a PASS, not a skip-with-warning: most projects have none,
# and exemplars are opt-in.
#
# This check is the whole enforcement story for exemplars: they are deliberately
# NOT gated, so nothing stops an agent editing one. That decision was measured
# rather than assumed — see the contract's "Exemplars are not gated" section.
# The digest warning below is the signal that would reverse it.
echo "--- Exemplar Tests (.claude/exemplars.yaml) ---"
EXEMPLARS="$REPO_DIR/.claude/exemplars.yaml"
if [[ ! -f "$EXEMPLARS" ]]; then
  pass "no exemplars declared (opt-in; nothing to check)"
elif ! command -v python3 >/dev/null 2>&1; then
  warn "python3 unavailable — cannot validate .claude/exemplars.yaml"
else
  exemplar_report=$(python3 - "$EXEMPLARS" "$REPO_DIR" <<'PYEOF'
import sys, os, re, hashlib
try:
    import yaml
except ImportError:
    print("SKIP:PyYAML not installed")
    sys.exit(0)

manifest_path, repo = sys.argv[1], sys.argv[2]
data = yaml.safe_load(open(manifest_path)) or {}
entries = data.get("exemplars") or []
if not entries:
    print("FAIL:manifest declares no exemplars — delete it or declare one")
    sys.exit(0)

MARKS = ["@exemplar", "pytest.mark.exemplar", 'Tag("exemplar")', '[Trait("Exemplar"', "// exemplar:"]
declared_files = set()
for entry in entries:
    name = entry.get("test", "<unnamed>")
    rel = entry.get("file", "")
    for field in ("test", "file", "language", "level", "demonstrates"):
        if not entry.get(field):
            print("FAIL:%s — manifest entry missing required field '%s'" % (name, field))
    if not rel:
        continue
    declared_files.add(rel)
    full = os.path.join(repo, rel)
    if not os.path.isfile(full):
        print("FAIL:%s — declared file does not exist: %s" % (name, rel))
        continue
    body = open(full, encoding="utf-8", errors="replace").read()
    if name not in body:
        print("FAIL:%s — not found in %s" % (name, rel))
    if not any(mark in body for mark in MARKS):
        print("FAIL:%s — %s carries no exemplar annotation" % (name, rel))
    else:
        print("PASS:%s (%s/%s)" % (name, entry.get("language"), entry.get("level")))
    recorded = (entry.get("digest") or "").replace("sha256:", "")
    if recorded:
        actual = hashlib.sha256(open(full, "rb").read()).hexdigest()[:len(recorded)]
        if actual != recorded:
            print("WARN:%s — %s changed since its digest was recorded. A file digest cannot tell "
                  "an edit to this exemplar from an edit to a neighbouring test in the same file; "
                  "run exemplar-auditor to confirm it still holds" % (name, rel))

# The other direction: an annotated test the manifest never mentions.
for root, dirs, files in os.walk(repo):
    dirs[:] = [d for d in dirs if d not in {".git", "node_modules", "archive", "tests"}]
    for fname in files:
        if not re.search(r"(_test\.go|\.spec\.[jt]s|\.test\.[jt]s|test_.*\.py|Tests?\.(java|cs|kt|swift))$", fname):
            continue
        full = os.path.join(root, fname)
        rel = os.path.relpath(full, repo)
        if rel in declared_files:
            continue
        try:
            body = open(full, encoding="utf-8", errors="replace").read()
        except OSError:
            continue
        if any(mark in body for mark in MARKS):
            print("FAIL:%s carries an exemplar annotation but is not in the manifest" % rel)
PYEOF
)
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    case "$line" in
      PASS:*) pass "exemplar ${line#PASS:}" ;;
      WARN:*) warn "exemplar ${line#WARN:}" ;;
      SKIP:*) warn "exemplar check skipped — ${line#SKIP:}" ;;
      *)      fail "exemplar ${line#FAIL:}" ;;
    esac
  done <<< "$exemplar_report"
fi
echo ""

# --- 7c. Marked preconditions name an enforcer that exists -------------------
# Fitness function for architecture-guardrails.md #7's third clause and
# docs/patterns/framework-meta-patterns.md.
#
# A comment stating a condition the code depends on must name what holds it.
# This does NOT detect stale prose — nothing mechanical can. It makes a
# deliberately marked precondition impossible to leave unenforced, and makes
# an unenforceable one say so out loud.
echo "--- Marked Preconditions (PRECONDITION / ENFORCED-BY) ---"
if ! command -v python3 >/dev/null 2>&1; then
  warn "python3 unavailable — cannot validate marked preconditions"
else
  precondition_report=$(python3 - "$REPO_DIR" <<'PYEOF'
import os, re, sys

repo = sys.argv[1]
SKIP_DIRS = {".git", "node_modules", "archive", ".claude", "scratch", "templates"}
TEXT_SUFFIXES = (".go", ".yaml", ".yml", ".md", ".sh", ".py", ".ts", ".js")

marks, enforcers = [], set()
for root, dirs, files in os.walk(repo):
    dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
    for name in files:
        if not name.endswith(TEXT_SUFFIXES):
            continue
        path = os.path.join(root, name)
        rel = os.path.relpath(path, repo)
        try:
            lines = open(path, encoding="utf-8", errors="replace").read().splitlines()
        except OSError:
            continue
        # Every Go test function in the tree is a candidate enforcer.
        if name.endswith("_test.go"):
            enforcers.update(re.findall(r"^func (Test\w+)", "\n".join(lines), re.M))
        for number, line in enumerate(lines):
            if "PRECONDITION:" not in line:
                continue
            # The pattern doc and the guardrail describe the convention; they
            # are not themselves preconditions.
            # The convention's own definition, and this checker's source,
            # both contain the marker without stating a precondition.
            if rel in ("docs/patterns/framework-meta-patterns.md",
                       "shared/rules/architecture-guardrails.md",
                       "scripts/health-check.sh"):
                continue
            window = "\n".join(lines[number:number + 6])
            found = re.search(r"ENFORCED-BY:\s*(.+)", window)
            if not found:
                marks.append(("MISSING", rel, number + 1, ""))
            else:
                marks.append(("OK", rel, number + 1, found.group(1).strip()))

if not marks:
    print("NONE")
for status, rel, number, enforcer in marks:
    if status == "MISSING":
        print("FAIL:%s:%d states a PRECONDITION with no ENFORCED-BY" % (rel, number))
    elif enforcer.startswith("judgment-only"):
        if len(enforcer) <= len("judgment-only") + 3:
            print("FAIL:%s:%d is judgment-only with no reason" % (rel, number))
        else:
            print("PASS:%s:%d judgment-only, with a reason" % (rel, number))
    else:
        named = enforcer.split()[0].rstrip(".,")
        if named.startswith("Test") and named not in enforcers:
            print("FAIL:%s:%d names enforcer %s, which does not exist" % (rel, number, named))
        else:
            print("PASS:%s:%d enforced by %s" % (rel, number, named))
PYEOF
)
  if [[ "$precondition_report" == "NONE" ]]; then
    pass "no marked preconditions (the convention is opt-in)"
  else
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      case "$line" in
        PASS:*) pass "precondition ${line#PASS:}" ;;
        *)      fail "precondition ${line#FAIL:}" ;;
      esac
    done <<< "$precondition_report"
  fi
fi
echo ""

# --- 7d. Every gate in the rule exists in code, classified the same way -----
# Fitness function for shared/rules/approval-gates.md.
#
# This check exists because the drift it catches already happened. Gate #9
# was added to the rule and marked Always Human; nothing in Go followed. A
# policy targeting it was rejected as `unknown gate` — the message a typo
# gets — so the obvious repair was to add it to eligible(), which is the one
# change the gate exists to prevent. Nothing would have reported that.
#
# The gate-number -> GateID mapping below is PINNED rather than parsed. Only
# the three policy-eligible gates carry a `Policy gate ID:` line in the rule;
# adding one to the other six costs ~180 bytes of a core rule that has 206
# left, which would spend the entire remaining bundle budget on punctuation.
# The same trade is documented at TEST_WRITING_AGENTS above: an explicit list
# whose edit is deliberate beats a derivation that is clever and wrong.
#
# A gate added to the rule without a line here FAILS, which is the point —
# pinning is only a control while someone has to widen it on purpose.
echo "--- Approval Gates (rule and code agree) ---"
if ! command -v python3 >/dev/null 2>&1; then
  warn "python3 unavailable — cannot cross-check approval gates"
else
  gates_report=$(python3 - "$REPO_DIR" <<'PYEOF'
import re, sys

repo = sys.argv[1]
rule_path = f"{repo}/shared/rules/approval-gates.md"
code_path = f"{repo}/internal/policy/gate.go"

# Pinned: gate number in approval-gates.md -> GateID declared in gate.go.
PINNED = {
    1: "ship-to-friday",      2: "git-commit",             3: "db-migration",
    4: "db-contract-phase",   5: "external-api",           6: "out-of-boundary-write",
    7: "fitness-function-wiring", 8: "deploy",             9: "test-removal",
}

try:
    rule = open(rule_path, encoding="utf-8").read()
    code = open(code_path, encoding="utf-8").read()
except OSError as exc:
    print(f"FAIL:cannot read {exc.filename}")
    sys.exit(0)

# Each gate's number, title, and whether the rule calls it always-human.
declared = {}
for match in re.finditer(r"^### (\d+)\. (.+)$", rule, re.M):
    number, title = int(match.group(1)), match.group(2).strip()
    body = rule[match.end():]
    nxt = re.search(r"^### \d+\. ", body, re.M)
    body = body[:nxt.start()] if nxt else body
    eligibility = re.search(r"\*\*Policy-eligible: (No|Yes)", body)
    if not eligibility:
        print(f"FAIL:gate #{number} ({title}) states no Policy-eligible line")
        continue
    declared[number] = (title, eligibility.group(1) == "No")

if not declared:
    print("FAIL:no gates parsed from approval-gates.md — the heading format changed")
    sys.exit(0)

# GateID constants, and membership of the two classifying lists, by identifier.
constants = dict(re.findall(r"(\w+)\s+GateID = \"([^\"]+)\"", code))

def ids_in(function):
    block = re.search(r"func " + function + r"\(\).*?\n}", code, re.S)
    if not block:
        return None
    return {constants[name] for name in re.findall(r"\b(Gate\w+)\b", block.group(0))
            if name in constants}

always_human = ids_in("alwaysHuman")
eligible = ids_in("EligibleGates")
if always_human is None or eligible is None:
    print("FAIL:cannot locate alwaysHuman() or EligibleGates() in internal/policy/gate.go")
    sys.exit(0)

for number in sorted(declared):
    title, is_always_human = declared[number]
    label = f"gate #{number} ({title})"
    gate_id = PINNED.get(number)
    if gate_id is None:
        print(f"FAIL:{label} is not pinned in health-check.sh — add it deliberately")
        continue
    if gate_id not in constants.values():
        print(f"FAIL:{label} has no GateID {gate_id!r} in internal/policy/gate.go")
        continue
    if is_always_human and gate_id not in always_human:
        print(f"FAIL:{label} is Always Human in the rule but absent from alwaysHuman()")
    elif is_always_human and gate_id in eligible:
        print(f"FAIL:{label} is Always Human in the rule but policy-targetable in code")
    elif not is_always_human and gate_id not in eligible:
        print(f"FAIL:{label} is policy-eligible in the rule but absent from EligibleGates()")
    else:
        classification = "always human" if is_always_human else "policy-eligible"
        print(f"PASS:{label} -> {gate_id}, {classification} in both")

for stale in sorted(set(PINNED) - set(declared)):
    print(f"FAIL:gate #{stale} is pinned in health-check.sh but no longer in the rule")
PYEOF
  )
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    case "$line" in
      PASS:*) pass "${line#PASS:}" ;;
      *)      fail "${line#FAIL:}" ;;
    esac
  done <<< "$gates_report"
fi
echo ""

# --- 7e. An agent obliged to write declares a tool that can ------------------
# Fitness function for shared/contracts/agent-frontmatter-contract.md.
#
# Seven agents once declared `Read, Glob, Grep, Bash` while every one of them
# was required to produce a markdown artifact, and three were told to modify
# source. `Bash` was doing the writing — undeclared, and invisible to anything
# reading the `tools` field. That is the mechanism behind roadmap L3.36.
#
# BE HONEST ABOUT WHAT THIS IS. It greps English prose for an obligation and
# compares it against `tools`. It is a tripwire on the commonest way the
# "tools describes real capability" invariant breaks, not the invariant
# itself. Verified on the tree before the fix: it flags all seven, with the
# right reason for each, and flags nothing at HEAD.
#
# Its blind spot is measured, not assumed: stripping `Write` from
# `spec-writer` — whose obligation is phrased in none of these patterns —
# passes this check. The patterns are deliberately narrow because widening
# them buys false positives on agents that merely mention producing
# something, and a check that cries wolf gets skimmed. A miss fails OPEN: the
# agent keeps whatever it declared, and nothing is blocked.
echo "--- Agent Write Obligations (tools can do what the prompt says) ---"
if ! command -v python3 >/dev/null 2>&1; then
  warn "python3 unavailable — cannot cross-check agent write obligations"
else
  obligations_report=$(python3 - "$SHARED_DIR" <<'PYEOF'
import os, re, sys

agents_dir = os.path.join(sys.argv[1], "agents")

# Pinned, and deliberately narrow — see the comment above this heredoc.
#
# BACKTICK is built rather than written: this heredoc sits inside $( ), where
# bash reads a literal backtick as legacy command substitution and the script
# stops parsing. Learned the direct way.
BACKTICK = chr(96)

WRITE_OBLIGATION = [
    (r"Produces\s+" + BACKTICK + r"?[\w./<>-]+\.md",
     "description says it Produces an .md artifact"),
    (r"\*\*Write\*\*\s+" + BACKTICK,
     "a process step says **Write** with a path"),
    (r"\*\*Produce\*\*\s+" + BACKTICK,
     "a process step says **Produce** with a path"),
    (r"produce your artifact at",
     "Output Format says produce your artifact at"),
]
EDIT_OBLIGATION = [
    (r"Fix\s+\w+\s+directly", "instructed to fix findings directly"),
    (r"Write fixes",          "instructed to write fixes"),
    (r"rewrite the \w+",      "instructed to rewrite existing code"),
]

def why(text, rules):
    return [reason for pattern, reason in rules if re.search(pattern, text)]

if not os.path.isdir(agents_dir):
    print("FAIL:%s does not exist" % agents_dir)
    sys.exit(0)

for name in sorted(os.listdir(agents_dir)):
    if not name.endswith(".md") or name == "CHANGELOG.md":
        continue
    text = open(os.path.join(agents_dir, name), encoding="utf-8").read()
    declared = re.search(r"^tools:\s*(.+)$", text, re.M)
    if not declared:
        continue
    tools = {tool.strip() for tool in declared.group(1).split(",")}
    agent = name[:-3]

    needs_write, needs_edit = why(text, WRITE_OBLIGATION), why(text, EDIT_OBLIGATION)
    problems = []
    if needs_write and not tools & {"Write", "MultiEdit"}:
        problems.append("declares no Write but %s" % needs_write[0])
    if needs_edit and not tools & {"Edit", "MultiEdit"}:
        problems.append("declares no Edit but %s" % needs_edit[0])

    if problems:
        print("FAIL:%s %s" % (agent, "; ".join(problems)))
    elif needs_write or needs_edit:
        print("PASS:%s declares the tools its prompt requires" % agent)
PYEOF
  )
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    case "$line" in
      PASS:*) pass "${line#PASS:}" ;;
      *)      fail "${line#FAIL:}" ;;
    esac
  done <<< "$obligations_report"
fi
echo ""

# --- 7f. Counter agents declare neither a write nor an egress tool ----------
# Fitness function for docs/patterns/security-patterns.md's "Least Privilege
# on Egress", and the one property that pattern can actually hold.
#
# Least privilege on data is not least privilege on egress. A counter agent
# is the one class in this framework where BOTH must hold: it audits work it
# must not alter, and it reads the whole corpus, so a channel out is the
# thing that turns a reader into an exfiltration path.
#
# `Bash` is the entire egress surface here — no agent declares WebFetch or
# WebSearch — and it is both axes at once: any file, any host. So a counter
# agent holding it is unconstrained on both, whatever the rest of its tools
# line says.
#
# The set is derived, not pinned, because these agents already declare
# themselves in prose ("read-only counter agent") and deriving it means a
# NEW counter agent is covered the day it is written rather than the day
# someone remembers to add it to a list. That is the opposite trade from
# TEST_WRITING_AGENTS above, and for the opposite reason: there the prose
# signal was unreliable, here it is the agent's own self-description.
#
# What this does NOT cover, per the pattern: egress inside a user's project,
# which loom does not run in, and output-rendered paths like a markdown
# image URL, which no tool allowlist can see.
echo "--- Counter Agent Egress (read-only means no way out) ---"
if ! command -v python3 >/dev/null 2>&1; then
  warn "python3 unavailable — cannot check counter agent tools"
else
  egress_report=$(python3 - "$SHARED_DIR" <<'PYEOF'
import os, re, sys

agents_dir = os.path.join(sys.argv[1], "agents")
WRITE_TOOLS = {"Write", "Edit", "MultiEdit"}
EGRESS_TOOLS = {"Bash", "WebFetch", "WebSearch"}

found = 0
for name in sorted(os.listdir(agents_dir)):
    if not name.endswith(".md") or name == "CHANGELOG.md":
        continue
    text = open(os.path.join(agents_dir, name), encoding="utf-8").read()
    if not re.search(r"read-only counter\b", text, re.I):
        continue
    declared = re.search(r"^tools:\s*(.+)$", text, re.M)
    if not declared:
        print("FAIL:%s declares itself a counter agent but has no tools line" % name[:-3])
        continue

    found += 1
    tools = {tool.strip() for tool in declared.group(1).split(",")}
    agent = name[:-3]
    problems = []
    if tools & WRITE_TOOLS:
        problems.append("can write (%s)" % ", ".join(sorted(tools & WRITE_TOOLS)))
    if tools & EGRESS_TOOLS:
        problems.append("has a way out (%s)" % ", ".join(sorted(tools & EGRESS_TOOLS)))

    if problems:
        print("FAIL:%s is a read-only counter agent but %s — it audits what it must not alter, "
              "and reads the whole corpus" % (agent, " and ".join(problems)))
    else:
        print("PASS:%s declares no write and no egress tool" % agent)

# A derived set shrinks silently when the phrase it derives from drifts, and
# a check covering fewer agents than it did yesterday reports nothing at all.
# This caught its own first version: the regex required "read-only counter
# agent", and memory-auditor says "Read-only counter to the memory-engineer
# skill" — so it was quietly excluded while the section printed all PASS.
EXPECTED_AT_LEAST = 13
if found < EXPECTED_AT_LEAST:
    print("FAIL:only %d agents matched the read-only-counter phrase, expected at least %d — "
          "the phrase has drifted and this check now covers less than it did; widen the match "
          "or lower the floor deliberately" % (found, EXPECTED_AT_LEAST))
PYEOF
  )
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    case "$line" in
      PASS:*) pass "${line#PASS:}" ;;
      *)      fail "${line#FAIL:}" ;;
    esac
  done <<< "$egress_report"
fi
echo ""

# --- 8. Knowledge Item frontmatter valid ------------------------------------
echo "--- Knowledge Item Frontmatter (name, tags, domain, created) ---"
for ki_dir in "$SHARED_DIR/knowledge" "$REPO_DIR/.claude/knowledge"; do
  [[ -d "$ki_dir" ]] || continue
  for ki_file in "$ki_dir"/*.md; do
    [[ -f "$ki_file" ]] || continue
    base="$(basename "$ki_file")"
    [[ "$base" == "README.md" ]] && continue

    missing=""
    for field in name tags domain created; do
      if ! grep -q "^${field}:" "$ki_file"; then
        missing="$missing $field"
      fi
    done

    if [[ -z "$missing" ]]; then
      pass "$base"
    else
      fail "$base — missing frontmatter:$missing"
    fi
  done
done
echo ""

echo "--- Frontmatter JSON Schema Validation (opt-in — requires python3 jsonschema + PyYAML) ---"
# Adds real value-validation on top of the field-presence checks in steps 2, 3, and 8 above:
# `tools: WhatEverRandomName` and `standalone: false` slip past field-presence but fail here.
# Zero hard dependency — if jsonschema/PyYAML aren't installed, this step warns and skips
# rather than fails, so the rest of health-check.sh still runs cleanly on a stock machine.
SCHEMA_VALIDATOR="$REPO_DIR/scripts/validate-frontmatter.py"
SCHEMA_DIR="$SHARED_DIR/schemas"
if [[ ! -f "$SCHEMA_VALIDATOR" ]]; then
  warn "scripts/validate-frontmatter.py not found — skipping schema validation"
elif ! python3 -c "import jsonschema, yaml" 2>/dev/null; then
  warn "python3 jsonschema + PyYAML not available — skipping schema validation (install: pip install jsonschema pyyaml)"
else
  # Agents
  AGENT_SCHEMA="$SCHEMA_DIR/agent-frontmatter.schema.json"
  if [[ -f "$AGENT_SCHEMA" ]]; then
    agent_files=$(find "$SHARED_DIR/agents" -maxdepth 1 -name "*.md" ! -name "CHANGELOG.md" | sort)
    # `|| schema_exit=$?` is load-bearing: under `set -e` a failing command
    # substitution in an assignment aborts the script, so the fail branch
    # below was unreachable and a schema violation killed health-check
    # mid-section with no message and no summary.
    schema_exit=0
    schema_output=$(python3 "$SCHEMA_VALIDATOR" "$AGENT_SCHEMA" $agent_files 2>&1) || schema_exit=$?
    if [[ $schema_exit -eq 0 ]]; then
      pass "agent frontmatter — all files valid against $AGENT_SCHEMA"
    else
      fail "agent frontmatter — schema violations:"
      echo "$schema_output" | grep -E "FAIL|^          -" || true
    fi
  else
    warn "$AGENT_SCHEMA not found — skipping agent schema check"
  fi

  # Skills
  SKILL_SCHEMA="$SCHEMA_DIR/skill-frontmatter.schema.json"
  if [[ -f "$SKILL_SCHEMA" ]]; then
    skill_files=$(find "$SHARED_DIR/skills" -maxdepth 2 -name "SKILL.md" | sort)
    schema_exit=0
    schema_output=$(python3 "$SCHEMA_VALIDATOR" "$SKILL_SCHEMA" $skill_files 2>&1) || schema_exit=$?
    if [[ $schema_exit -eq 0 ]]; then
      pass "skill frontmatter — all files valid against $SKILL_SCHEMA"
    else
      fail "skill frontmatter — schema violations:"
      echo "$schema_output" | grep -E "FAIL|^          -" || true
    fi
  else
    warn "$SKILL_SCHEMA not found — skipping skill schema check"
  fi

  # Knowledge Items — walk shared/knowledge and .claude/knowledge separately so a
  # missing .claude/knowledge (common — it's project-local, not always present in
  # this repo) doesn't trip `set -o pipefail` under `find`.
  KI_SCHEMA="$SCHEMA_DIR/ki-frontmatter.schema.json"
  if [[ -f "$KI_SCHEMA" ]]; then
    ki_files=""
    for ki_root in "$SHARED_DIR/knowledge" "$REPO_DIR/.claude/knowledge"; do
      [[ -d "$ki_root" ]] || continue
      found=$(find "$ki_root" -maxdepth 1 -name "*.md" ! -name "README.md" | sort || true)
      [[ -n "$found" ]] && ki_files="$ki_files $found"
    done
    if [[ -n "$ki_files" ]]; then
      schema_exit=0
      schema_output=$(python3 "$SCHEMA_VALIDATOR" "$KI_SCHEMA" $ki_files 2>&1) || schema_exit=$?
      if [[ $schema_exit -eq 0 ]]; then
        pass "knowledge item frontmatter — all files valid against $KI_SCHEMA"
      else
        fail "knowledge item frontmatter — schema violations:"
        echo "$schema_output" | grep -E "FAIL|^          -" || true
      fi
    else
      pass "no knowledge items found to validate"
    fi
  else
    warn "$KI_SCHEMA not found — skipping KI schema check"
  fi
fi
echo ""

echo "--- Memory Registry (shared/memory-registry.json) ---"
REGISTRY="$SHARED_DIR/memory-registry.json"
if [[ -f "$REGISTRY" ]]; then
  if python3 -c "import json; json.load(open('$REGISTRY'))" 2>/dev/null; then
    pass "memory-registry.json is valid JSON"
  else
    fail "memory-registry.json is not valid JSON"
  fi

  # Every path each source declares must actually exist.
  registry_paths=$(python3 -c "
import json
data = json.load(open('$REGISTRY'))
for s in data.get('sources', []):
    for p in s.get('paths', []):
        print(p)
" 2>/dev/null || true)
  optional_paths=$(python3 -c "
import json
data = json.load(open('$REGISTRY'))
for p in data.get('optionalPaths', []):
    print(p)
" 2>/dev/null || true)
  while IFS= read -r rpath; do
    [[ -z "$rpath" ]] && continue
    full_path="$REPO_DIR/$rpath"
    if [[ -e "$full_path" ]]; then
      pass "registry path exists: $rpath"
    elif echo "$optional_paths" | grep -qxF "$rpath"; then
      pass "registry path missing (marked optional, skipped): $rpath"
    else
      fail "registry path missing: $rpath"
    fi
  done <<< "$registry_paths"

  # ADR-002 mechanical check: every registry source must declare a retrievalBackend from the
  # allowed enum {lexical, llm-as-retriever, bm25, vector}. Note: the actual registry field is
  # "retrievalBackend" (singular), while ADR-002 describes it as "retrievalBackends" (plural) —
  # follow the registry as-is per Epic 60 Op 2 escalation rule; the ADR discrepancy is noted here.
  # Degrades to SKIP (never false-FAIL) if python3 is unavailable.
  if command -v python3 >/dev/null 2>&1; then
    registry_backend_check=$(python3 -c "
import json, sys
VALID = {'lexical', 'llm-as-retriever', 'bm25', 'vector'}
data = json.load(open('$REGISTRY'))
problems = []
for s in data.get('sources', []):
    name = s.get('name', '?')
    backend = s.get('retrievalBackend')
    if backend is None:
        problems.append('MISSING:' + name)
    elif backend not in VALID:
        problems.append('INVALID:' + name + ':' + str(backend))
print('\n'.join(problems))
" 2>/dev/null || true)
    if [[ -z "$registry_backend_check" ]]; then
      pass "all registry sources have a valid retrievalBackend"
    else
      while IFS= read -r problem; do
        [[ -z "$problem" ]] && continue
        kind="${problem%%:*}"
        rest="${problem#*:}"
        if [[ "$kind" == "MISSING" ]]; then
          fail "registry source '$rest' missing retrievalBackend field (must be one of: lexical, llm-as-retriever, bm25, vector)"
        else
          src="${rest%%:*}"; val="${rest#*:}"
          fail "registry source '$src' has invalid retrievalBackend '$val' (must be one of: lexical, llm-as-retriever, bm25, vector)"
        fi
      done <<< "$registry_backend_check"
    fi
  else
    warn "python3 unavailable — skipping retrievalBackend enum check (ADR-002 fitness function)"
  fi

  # No two KIs should share an exact frontmatter name: — a real duplicate, not just an overlap
  # memory-engineer would judge more subtly; this is the cheap, deterministic half of that check.
  ki_names=$( (grep -h '^name:' "$SHARED_DIR"/knowledge/*.md "$REPO_DIR"/.claude/knowledge/*.md 2>/dev/null || true) | sed 's/^name: *//' | sort)
  dupe_names=$(echo "$ki_names" | uniq -d || true)
  if [[ -z "$dupe_names" ]]; then
    pass "no duplicate KI frontmatter names"
  else
    while IFS= read -r dname; do
      [[ -z "$dname" ]] && continue
      fail "duplicate KI frontmatter name: $dname — memory-engineer should audit these for a merge"
    done <<< "$dupe_names"
  fi
else
  warn "shared/memory-registry.json not found — skipping Memory Registry checks"
fi
echo ""

# --- Memory sync health (opt-in — only checked when sync-config exists) -----
echo "--- Memory Sync (enterprise KI sync via ADR-003) ---"
SYNC_CONFIG="$REPO_DIR/.claude/sync-config.yaml"
if [[ ! -f "$SYNC_CONFIG" ]]; then
  pass "memory sync config absent — enterprise sync not configured (opt-in; see ADR-003)"
else
  # Read org_repo and cache_dir from the config using the same no-pyyaml approach as sync-memory.sh
  sync_org_repo=$(grep "^\s*org_repo:" "$SYNC_CONFIG" | head -1 | sed 's/.*org_repo:\s*//' | tr -d '"' || true)
  sync_cache_base=$(grep "^\s*cache_dir:" "$SYNC_CONFIG" | head -1 | sed 's/.*cache_dir:\s*//' | tr -d '"' || true)
  sync_cache_base="${sync_cache_base/#\~/$HOME}"
  sync_slug=$(echo "$sync_org_repo" | sed 's|.*[:/]||; s|\.git$||' || true)
  last_sync_file="${sync_cache_base}/${sync_slug}/.last-sync"

  if [[ -n "$sync_org_repo" ]]; then
    pass "sync config present — org repo: $sync_org_repo"
  else
    fail "sync config found but memory_sync.org_repo is empty"
  fi

  if [[ -f "$last_sync_file" ]]; then
    last_sync=$(cat "$last_sync_file")
    pass "last sync: $last_sync"

    # Warn if last sync is more than 30 days ago
    if command -v python3 &>/dev/null; then
      days_since=$(python3 -c "
import datetime, sys
try:
    ts = '${last_sync}'.rstrip('Z')
    last = datetime.datetime.fromisoformat(ts)
    now = datetime.datetime.utcnow()
    print((now - last).days)
except Exception:
    print(-1)
" 2>/dev/null || echo "-1")
      if [[ "$days_since" -ge 30 ]]; then
        warn "last sync was $days_since day(s) ago — consider running: ./install.sh --sync-memory"
      elif [[ "$days_since" -ge 0 ]]; then
        pass "sync age: $days_since day(s)"
      fi
    fi
  else
    warn "never synced — run: ./install.sh --sync-memory"
  fi

  # Report sync-stamped KIs in shared/knowledge/
  synced_count=$(grep -rl '^sync_source:' "$SHARED_DIR/knowledge" 2>/dev/null | wc -l | tr -d ' ' || echo 0)
  if [[ "$synced_count" -gt 0 ]]; then
    pass "$synced_count KI(s) in shared/knowledge/ pulled from org repo"
  else
    pass "no org-pulled KIs in shared/knowledge/ yet"
  fi
fi
echo ""

# --- Agent golden-file fixture coverage (FAIL if any non-deferred agent lacks one) -----------
echo "--- Agent Golden-File Fixtures (tests/agents/) ---"
TESTS_AGENTS_DIR="$REPO_DIR/tests/agents"
# Specialists deferred in Epic 55 — non-deterministic or large-surface agents documented
# in tests/agents/README.md as "specialist; deferred" or "multi-agent coordinator; deferred"
deferred_agents="api-test-generator chaos-engineer dependency-auditor dx-engineer finops-engineer modernization-supervisor"

if [[ ! -d "$TESTS_AGENTS_DIR" ]]; then
  fail "tests/agents/ directory not found"
else
  for agent_file in "$SHARED_DIR/agents/"*.md; do
    base="$(basename "$agent_file")"
    [[ "$base" == "CHANGELOG.md" ]] && continue
    agent_name=$(grep '^name:' "$agent_file" | head -1 | sed 's/name: *//' | tr -d '"' || true)
    [[ -z "$agent_name" ]] && continue

    # Skip deferred specialists
    if echo "$deferred_agents" | grep -qw "$agent_name"; then
      pass "$agent_name — deferred specialist (documented in tests/agents/README.md)"
      continue
    fi

    fixture_dir="$TESTS_AGENTS_DIR/$agent_name"
    if [[ ! -d "$fixture_dir" ]]; then
      fail "$agent_name — no fixture directory at tests/agents/$agent_name/"
      continue
    fi

    has_input=$(find "$fixture_dir" -maxdepth 1 -name "input-*" | wc -l | tr -d ' ')
    has_patterns=false; [[ -f "$fixture_dir/expected-patterns.txt" ]] && has_patterns=true
    has_rubric=false;   [[ -f "$fixture_dir/eval-rubric.md" ]]       && has_rubric=true

    if [[ "$has_input" -gt 0 ]] && $has_patterns && $has_rubric; then
      pass "$agent_name — fixture complete (input + patterns + rubric)"
    else
      missing_pieces=""
      [[ "$has_input" -eq 0 ]] && missing_pieces="$missing_pieces input-*"
      $has_patterns || missing_pieces="$missing_pieces expected-patterns.txt"
      $has_rubric   || missing_pieces="$missing_pieces eval-rubric.md"
      fail "$agent_name — fixture incomplete — missing:$missing_pieces"
    fi
  done
fi
echo ""

# --- Cap drift (shared/configs/ vs. convention files) -----------------------
echo "--- Linter Config Cap Drift ---"
if [[ -d "$REPO_DIR/shared/configs" ]]; then
  cap_drift_output=$("$REPO_DIR/scripts/check-cap-drift.sh" 2>&1 || true)
  cap_drift_count=$(echo "$cap_drift_output" | grep -cE '^\s+FAIL' || true)
  cap_warn_count=$(echo "$cap_drift_output" | grep -cE '^\s+WARN' || true)
  if [[ "$cap_drift_count" -gt 0 ]]; then
    while IFS= read -r line; do
      fail "cap-drift: $line"
    done < <(echo "$cap_drift_output" | grep -E '^\s+FAIL' || true)
  elif [[ "$cap_warn_count" -gt 0 ]]; then
    while IFS= read -r line; do
      warn "cap-drift: $line"
    done < <(echo "$cap_drift_output" | grep -E '^\s+WARN' || true)
  else
    pass "all config caps match their source conventions"
  fi
else
  pass "shared/configs/ absent — cap-drift check skipped"
fi
echo ""

# --- Hook script-type check (security: THREAT_MODEL.md F-03/F-07) -----------
echo "--- Hook Security (script-type hooks) ---"
hooks_dir="$REPO_DIR/.claude/hooks"
if [[ -d "$hooks_dir" ]]; then
  script_hooks=$(grep -rl 'type:.*"script"\|type: script' "$hooks_dir" 2>/dev/null | sort || true)
  if [[ -n "$script_hooks" ]]; then
    while IFS= read -r hook_file; do
      # Only warn if the hook is enabled (enabled: true or no enabled field, which defaults to true)
      if grep -qE '^\s*enabled:\s*true' "$hook_file" 2>/dev/null || \
         ! grep -qE '^\s*enabled:' "$hook_file" 2>/dev/null; then
        warn "hook script-type enabled: $(basename "$hook_file") — review per shared/hooks/README.md security constraints"
      else
        pass "hook script-type disabled: $(basename "$hook_file") (enabled: false — safe)"
      fi
    done <<< "$script_hooks"
  else
    pass "no script-type hooks in .claude/hooks/"
  fi
else
  pass ".claude/hooks/ absent — hook security check skipped"
fi
echo ""

# --- Inventory drift (Gate #7 approved 2026-08-16; includes living docs) -----
echo "--- Inventory drift ---"
drift_output=$("$REPO_DIR/scripts/check-inventory-drift.sh" 2>&1 || true)
drift_count=$(echo "$drift_output" | grep -cE '^\s+DRIFT' || true)
if [[ "$drift_count" -gt 0 ]]; then
  while IFS= read -r drift_line; do
    warn "inventory: $drift_line"
  done < <(echo "$drift_output" | grep -E '^\s+DRIFT' || true)
else
  pass "no inventory drift in authoritative prose docs"
fi
echo ""

# --- Documentation Auditor freshness (opt-in) --------------------------------
# Silently skipped when docs/audits/ doesn't exist — the convention is opt-in.
# WARN only; never FAIL — running the auditor is a human judgment call.
echo "--- Documentation Auditor Freshness ---"
DOC_AUDIT_DIR="$REPO_DIR/docs/audits"
DOC_AUDIT_MAX_DAYS=14
if [[ ! -d "$DOC_AUDIT_DIR" ]]; then
  pass "docs/audits/ absent — doc-audit freshness check skipped (opt-in)"
else
  newest_audit=$(find "$DOC_AUDIT_DIR" -maxdepth 1 -name "doc-audit-*.md" | sort | tail -1 || true)
  if [[ -z "$newest_audit" ]]; then
    warn "no doc-audit-*.md in docs/audits/ — consider running documentation-auditor"
  else
    audit_date=$(basename "$newest_audit" | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}' || true)
    if [[ -z "$audit_date" ]]; then
      warn "newest doc-audit filename has no parseable date — consider re-running documentation-auditor"
    elif command -v python3 &>/dev/null; then
      days_since=$(python3 -c "
import datetime
try:
    last = datetime.datetime.strptime('${audit_date}', '%Y-%m-%d')
    now = datetime.datetime.utcnow()
    print((now - last).days)
except Exception:
    print(-1)
" 2>/dev/null || echo "-1")
      if [[ "$days_since" -ge $DOC_AUDIT_MAX_DAYS ]]; then
        warn "doc-audit findings are $days_since day(s) old ($(basename "$newest_audit")) — consider re-running documentation-auditor"
      else
        pass "doc-audit findings are $days_since day(s) old ($(basename "$newest_audit"))"
      fi
    else
      pass "doc-audit findings present ($(basename "$newest_audit")) — python3 unavailable, skipping age check"
    fi
  fi
fi
echo ""

# --- Exemplar audit freshness (opt-in) ---------------------------------------
# Approval gate #7 (Wiring a New Fitness Function) — approved by the user.
#
# Exemplars are deliberately NOT gated (roadmap L3.46, measured): nothing stops
# an agent editing one. exemplar-auditor is the only thing that notices a
# degraded exemplar, and an audit nobody runs is the same silence. This warns
# when a project that HAS exemplars has not audited them lately.
#
# Only applies to projects that declare exemplars — no manifest, nothing to
# audit, no warning. WARN only, never FAIL: running the auditor is a judgment
# call, same as the doc-audit check above.
echo "--- Exemplar Audit Freshness ---"
EXEMPLAR_AUDIT_MAX_DAYS=45   # the monthly schedule plus room for a late month
if [[ ! -f "$REPO_DIR/.claude/exemplars.yaml" ]]; then
  pass "no exemplars declared — audit freshness check skipped (opt-in)"
elif [[ ! -d "$REPO_DIR/docs/audits" ]]; then
  warn "exemplars are declared but docs/audits/ has no exemplar audit — run exemplar-auditor, or enable exemplar-auditor-monthly in shared/hooks/scheduled-monthly.yaml"
else
  newest_exemplar_audit=$(find "$REPO_DIR/docs/audits" -maxdepth 1 -name "exemplar-audit-*.md" | sort | tail -1 || true)
  if [[ -z "$newest_exemplar_audit" ]]; then
    warn "exemplars are declared but never audited — run exemplar-auditor, or enable exemplar-auditor-monthly in shared/hooks/scheduled-monthly.yaml"
  else
    exemplar_audit_date=$(basename "$newest_exemplar_audit" | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}' || true)
    if [[ -z "$exemplar_audit_date" ]]; then
      warn "newest exemplar audit filename has no parseable date — consider re-running exemplar-auditor"
    elif command -v python3 &>/dev/null; then
      exemplar_days=$(python3 -c "
import datetime
try:
    last = datetime.datetime.strptime('${exemplar_audit_date}', '%Y-%m-%d')
    print((datetime.datetime.utcnow() - last).days)
except Exception:
    print(-1)
" 2>/dev/null || echo "-1")
      if [[ "$exemplar_days" -ge $EXEMPLAR_AUDIT_MAX_DAYS ]]; then
        warn "exemplar audit is $exemplar_days day(s) old ($(basename "$newest_exemplar_audit")) — a stale exemplar teaches every test written after it"
      else
        pass "exemplar audit is $exemplar_days day(s) old ($(basename "$newest_exemplar_audit"))"
      fi
    else
      pass "exemplar audit present ($(basename "$newest_exemplar_audit")) — python3 unavailable, skipping age check"
    fi
  fi
fi
echo ""

# --- CODEMAP freshness --------------------------------------------------
echo "--- CODEMAP Freshness ---"
CODEMAP_FILE="$REPO_DIR/CODEMAP.md"
if [[ ! -f "$CODEMAP_FILE" ]]; then
  warn "CODEMAP.md not found — run 'bash scripts/generate-codemap.sh' to create it (source-tier retrieval entry point)"
else
  # Uses stat -f %m (macOS) or stat -c %Y (Linux).
  if [[ "$(uname -s)" == "Darwin" ]]; then
    codemap_mtime=$(stat -f %m "$CODEMAP_FILE" 2>/dev/null || echo "0")
  else
    codemap_mtime=$(stat -c %Y "$CODEMAP_FILE" 2>/dev/null || echo "0")
  fi
  newest_dir_mtime=0
  while IFS= read -r dir; do
    dname="$(basename "$dir")"
    [[ "$dname" == .* ]] && continue
    if [[ "$(uname -s)" == "Darwin" ]]; then
      mtime=$(stat -f %m "$dir" 2>/dev/null || echo "0")
    else
      mtime=$(stat -c %Y "$dir" 2>/dev/null || echo "0")
    fi
    [[ "$mtime" -gt "$newest_dir_mtime" ]] && newest_dir_mtime="$mtime"
  done < <(find "$REPO_DIR" -mindepth 1 -maxdepth 1 -type d 2>/dev/null)
  if [[ "$codemap_mtime" -lt "$newest_dir_mtime" ]]; then
    warn "CODEMAP.md is older than the newest directory change — run 'bash scripts/generate-codemap.sh' to refresh"
  else
    pass "CODEMAP.md is current"
  fi
fi
echo ""

# --- Install-vs-upstream drift (installed projects only) --------------------
echo "--- Install Version Marker ---"
MARKER_FILE="$PWD/.claude/framework-install.json"
if [[ "$PWD" == "$REPO_DIR" ]]; then
  pass "running in framework repo — marker check skipped (no installed project drift to report)"
elif [[ -f "$MARKER_FILE" ]]; then
  if ! command -v python3 &>/dev/null; then
    warn "framework-install.json found but python3 unavailable — skipping drift check"
  elif ! python3 -c "import json; json.load(open('$MARKER_FILE'))" 2>/dev/null; then
    fail "framework-install.json found but is not valid JSON"
  else
    installed_tag=$(python3 -c "import json; d=json.load(open('$MARKER_FILE')); print(d.get('git_tag',''))" 2>/dev/null || true)
    installed_sha=$(python3 -c "import json; d=json.load(open('$MARKER_FILE')); print(d.get('commit_sha',''))" 2>/dev/null || true)
    installed_mode=$(python3 -c "import json; d=json.load(open('$MARKER_FILE')); print(d.get('mode',''))" 2>/dev/null || true)
    source_repo=$(python3 -c "import json; d=json.load(open('$MARKER_FILE')); print(d.get('source_repo',''))" 2>/dev/null || true)

    if [[ -z "$source_repo" || ! -d "$source_repo" ]]; then
      pass "framework-install.json present — source repo not found at '$source_repo' (moved or deleted), skipping drift check"
    else
      current_tag=$( (cd "$source_repo" && git describe --tags --abbrev=0 2>/dev/null) || echo "unknown")
      current_sha=$( (cd "$source_repo" && git rev-parse HEAD 2>/dev/null) || echo "unknown")
      if [[ "$installed_tag" == "$current_tag" ]]; then
        pass "framework up to date — installed $installed_tag ($installed_mode) matches source $current_tag"
      else
        warn "framework drift — installed $installed_tag (${installed_sha:0:8}) but source is now $current_tag (${current_sha:0:8}) — re-run install.sh to update"
      fi
    fi
  fi
else
  pass "no framework-install.json in $PWD/.claude/ — marker check skipped (pre-marker install or not an installed project)"
fi
echo ""

echo "==========================================="
echo "Results: $PASS_COUNT passed, $WARN_COUNT warned, $FAIL_COUNT failed"
if ! $VERBOSE; then
  echo "(pass details hidden — re-run with --verbose to see them)"
fi
echo ""

if [[ $FAIL_COUNT -gt 0 ]]; then
  exit 1
fi
