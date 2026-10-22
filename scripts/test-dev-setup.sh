#!/usr/bin/env bash
# test-dev-setup.sh — tests for scripts/dev-setup.sh.
#
# Sources dev-setup.sh for its functions, and stubs the network (curl), the Go toolchain (go) and
# the source build, so every path runs offline and in seconds — including the ones a sandbox
# without network access can never reach for real. Each test runs in a subshell, because the
# functions under test exit on failure.
#
# Usage: bash scripts/test-dev-setup.sh
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PASS_COUNT=0
FAIL_COUNT=0
SCRATCH="$(mktemp -d)"
trap 'rm -rf "$SCRATCH"' EXIT

# run_test NAME FUNCTION — one test, isolated in a subshell with its own LOOM_DEV_HOME.
run_test() {
  local name="$1" body="$2" output
  if output="$(cd "$SCRATCH" && LOOM_DEV_HOME="$SCRATCH/$body" bash -c "
      set -euo pipefail
      source '$SCRIPT_DIR/dev-setup.sh'
      source '$SCRIPT_DIR/test-dev-setup.sh' --definitions-only
      $body" 2>&1)"; then
    echo "  PASS  $name"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo "  FAIL  $name"
    echo "$output" | sed 's/^/        /'
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
}

# --- Helpers available inside every test --------------------------------------------------------

expect_equal() {
  [[ "$1" == "$2" ]] || { echo "want '$2', got '$1'"; return 1; }
}

# expect_failure COMMAND... — the command exits non-zero, and its message contains $EXPECTED.
expect_failure() {
  local message
  if message="$( ("$@") 2>&1)"; then
    echo "want a failure from: $*"
    return 1
  fi
  [[ "$message" == *"$EXPECTED"* ]] || { echo "message lacks '$EXPECTED': $message"; return 1; }
}

write_file() { mkdir -p "$(dirname "$1")" && printf '%s\n' "$2" >"$1"; }

# A release archive holding a `go` binary that reports VERSION, and its correct SHA-256.
make_release() {
  local version="$1" dir="$PWD/release"
  write_file "$dir/go/bin/go" "#!/usr/bin/env bash
echo 'go version go$version linux/amd64'"
  chmod +x "$dir/go/bin/go"
  tar -czf "$PWD/release.tar.gz" -C "$dir" go
  sha256_of "$PWD/release.tar.gz" >"$PWD/release.tar.gz.sha256"
}

# curl stub: serve the archive and its checksum from $PWD, or fail like a refused proxy.
stub_curl_serving() {
  CURL_MODE="$1"
  curl() {
    local output="" url=""
    while [[ $# -gt 0 ]]; do
      case "$1" in
        -o) output="$2"; shift 2 ;;
        -*) shift ;;
        *) url="$1"; shift ;;
      esac
    done
    [[ "$CURL_MODE" != unreachable ]] || return 56
    if [[ "$url" == *.sha256 ]]; then
      if [[ "$CURL_MODE" == bad-checksum ]]; then echo deadbeef >"$output"; else cp "$PWD/release.tar.gz.sha256" "$output"; fi
    else
      cp "$PWD/release.tar.gz" "$output"
    fi
  }
}

# --- Tests --------------------------------------------------------------------------------------

test_go_version_from_the_go_directive() {
  write_file go.mod $'module example\n\ngo 1.26.5\n'
  expect_equal "$(go_version_from_mod go.mod)" "1.26.5"
}

test_a_toolchain_directive_wins_over_go() {
  write_file go.mod $'module example\n\ngo 1.26.0\n\ntoolchain go1.26.5\n'
  expect_equal "$(go_version_from_mod go.mod)" "1.26.5"
}

test_a_go_mod_without_a_version_fails() {
  write_file go.mod $'module example\n'
  EXPECTED="no go or toolchain directive" expect_failure go_version_from_mod go.mod
}

test_the_golangci_pin_is_the_lint_step_version() {
  write_file ci.yml $'      - uses: actions/setup-go@abc\n        with:\n          version: v9.9.9\n      - uses: golangci/golangci-lint-action@def # v9\n        with:\n          # a comment\n          version: v2.13.2\n'
  expect_equal "$(golangci_version_from_workflow ci.yml)" "v2.13.2"
}

test_a_workflow_without_the_lint_pin_fails() {
  write_file ci.yml $'      - uses: actions/setup-go@abc\n'
  EXPECTED="no golangci-lint version pin" expect_failure golangci_version_from_workflow ci.yml
}

test_the_gremlins_pin_is_the_go_install_version() {
  write_file ci.yml $'        run: go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0\n'
  expect_equal "$(gremlins_version_from_workflow ci.yml)" "v0.6.0"
}

test_a_workflow_without_the_gremlins_pin_fails() {
  write_file ci.yml $'        run: echo nothing\n'
  EXPECTED="no gremlins version pin" expect_failure gremlins_version_from_workflow ci.yml
}

# The real pins must stay readable: if CI's layout changes, this fails before setup does.
test_the_real_go_mod_and_workflow_give_every_pin() {
  read_pins
  [[ "$GO_VERSION" =~ ^[0-9]+\.[0-9]+(\.[0-9]+)?$ ]] || { echo "go: $GO_VERSION"; return 1; }
  [[ "$GOLANGCI_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "golangci-lint: $GOLANGCI_VERSION"; return 1; }
  [[ "$GREMLINS_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "gremlins: $GREMLINS_VERSION"; return 1; }
}

test_cpu_names_map_to_go_arch() {
  expect_equal "$(go_arch x86_64)" amd64
  expect_equal "$(go_arch amd64)" amd64
  expect_equal "$(go_arch aarch64)" arm64
  expect_equal "$(go_arch arm64)" arm64
  EXPECTED="unsupported CPU" expect_failure go_arch riscv64
}

test_a_verified_download_installs_go_and_links_it() {
  make_release 1.26.5
  stub_curl_serving ok
  install_go 1.26.5 >/dev/null
  expect_equal "$(reported_go_version "$LOOM_DEV_HOME/go/bin/go")" "1.26.5"
}

test_a_bad_checksum_stops_without_building_from_source() {
  make_release 1.26.5
  stub_curl_serving bad-checksum
  build_go() { echo "build_go must not run"; exit 3; }
  EXPECTED="failed its checksum" expect_failure install_go 1.26.5
  [[ ! -e "$LOOM_DEV_HOME/go1.26.5" ]] || { echo "an unverified Go was left installed"; return 1; }
}

test_an_unreachable_download_falls_back_to_the_source_build() {
  stub_curl_serving unreachable
  build_go() { make_release "$1" && mkdir -p "$2/bin" && cp release/go/bin/go "$2/bin/go"; }
  install_go 1.26.5 >/dev/null
  expect_equal "$(reported_go_version "$LOOM_DEV_HOME/go/bin/go")" "1.26.5"
}

test_an_installed_go_is_not_fetched_again() {
  make_release 1.26.5
  stub_curl_serving ok
  install_go 1.26.5 >/dev/null
  stub_curl_serving unreachable
  build_go() { echo "build_go must not run"; exit 3; }
  expect_equal "$(install_go 1.26.5)" "  Go 1.26.5: already installed"
}

test_no_bootstrap_go_gives_a_clear_failure() {
  stub_curl_serving unreachable
  LOOM_DEV_BOOTSTRAP_GO=/nonexistent/go
  EXPECTED="no Go is available to build Go 1.26.5" expect_failure install_go 1.26.5
}

# go stub: `go install module@version` writes a binary named after the module into $GOBIN.
stub_go_install() {
  mkdir -p "$LOOM_DEV_HOME/go1.26.5/bin"
  write_file "$LOOM_DEV_HOME/go1.26.5/bin/go" '#!/usr/bin/env bash
[[ "$1" == install ]] || exit 1
module="${2%@*}"; name="${module##*/}"
printf "#!/usr/bin/env bash\necho %s %s\n" "$name" "${2#*@}" >"$GOBIN/$name" 2>/dev/null || { mkdir -p "$GOBIN"; printf "#!/usr/bin/env bash\necho %s %s\n" "$name" "${2#*@}" >"$GOBIN/$name"; }
chmod +x "$GOBIN/$name"'
  chmod +x "$LOOM_DEV_HOME/go1.26.5/bin/go"
}

test_a_tool_installs_into_a_versioned_directory_on_path() {
  stub_go_install
  install_tool gremlins example.com/cmd/gremlins v0.6.0 1.26.5 >/dev/null
  expect_equal "$("$LOOM_DEV_HOME/bin/gremlins")" "gremlins v0.6.0"
  expect_equal "$(readlink "$LOOM_DEV_HOME/bin/gremlins")" "$LOOM_DEV_HOME/tools/gremlins-v0.6.0-go1.26.5/gremlins"
}

test_a_new_pin_repoints_the_tool() {
  stub_go_install
  install_tool gremlins example.com/cmd/gremlins v0.6.0 1.26.5 >/dev/null
  install_tool gremlins example.com/cmd/gremlins v0.7.0 1.26.5 >/dev/null
  expect_equal "$("$LOOM_DEV_HOME/bin/gremlins")" "gremlins v0.7.0"
}

test_an_installed_tool_is_not_installed_again() {
  stub_go_install
  install_tool gremlins example.com/cmd/gremlins v0.6.0 1.26.5 >/dev/null
  expect_equal "$(install_tool gremlins example.com/cmd/gremlins v0.6.0 1.26.5)" "  gremlins v0.6.0: already installed"
}

test_a_failed_tool_install_names_the_module_proxy() {
  mkdir -p "$LOOM_DEV_HOME/go1.26.5/bin"
  write_file "$LOOM_DEV_HOME/go1.26.5/bin/go" $'#!/usr/bin/env bash\nexit 1'
  chmod +x "$LOOM_DEV_HOME/go1.26.5/bin/go"
  EXPECTED="needs proxy.golang.org" expect_failure install_tool gremlins example.com/cmd/gremlins v0.6.0 1.26.5
}

test_check_reports_a_match_and_a_mismatch() {
  expect_equal "$(check_tool go 1.26.5 1.26.5)" "  ok        go 1.26.5"
  expect_equal "$(check_tool go 1.26.5 1.27.3 || true)" "  MISMATCH  go: CI uses 1.26.5, PATH has 1.27.3"
  expect_equal "$(check_tool gremlins v0.6.0 "" || true)" "  MISMATCH  gremlins: CI uses v0.6.0, PATH has nothing"
}

test_env_puts_the_tools_first_on_path() {
  expect_equal "$(print_env)" "export PATH=\"$LOOM_DEV_HOME/go/bin:$LOOM_DEV_HOME/bin:\$PATH\"
export GOTOOLCHAIN=local"
}

# --- Runner -------------------------------------------------------------------------------------

if [[ "${1:-}" == --definitions-only ]]; then
  return 0
fi

echo "=== dev-setup.sh tests ==="
for test_function in $(declare -F | awk '{ print $3 }' | grep '^test_'); do
  run_test "${test_function#test_}" "$test_function"
done
echo ""
echo "Results: $PASS_COUNT passed, $FAIL_COUNT failed"
[[ $FAIL_COUNT -eq 0 ]]
