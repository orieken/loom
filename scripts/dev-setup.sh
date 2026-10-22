#!/usr/bin/env bash
# dev-setup.sh — install the Go toolchain and linters that CI uses, at the versions CI uses.
#
# Every version comes from where CI reads it, so nothing here can drift from CI:
#   - Go: go.mod (`toolchain` when present, else `go`) — what setup-go's go-version-file reads.
#   - golangci-lint and gremlins: the pins in .github/workflows/framework-ci.yml.
#
# The tools go into $LOOM_DEV_HOME (default ~/.cache/loom-dev), never over a system Go. A newer
# local Go builds loom too, but CI runs this version, and only the same version gives the same
# answers. golangci-lint is built here with that Go, because a linter built with an older Go
# refuses this module.
#
# Usage:
#   bash scripts/dev-setup.sh             install what is missing, then print how to use it
#   eval "$(bash scripts/dev-setup.sh --env)"
#                                         put the tools first on PATH in this shell
#   bash scripts/dev-setup.sh --check     compare the tools on PATH with CI; exit 1 on a mismatch
#   bash scripts/dev-setup.sh --versions  print the versions CI uses
#
# Network: Go downloads from dl.google.com with its published SHA-256. When that host is not
# reachable, Go is built from github.com/golang/go at the release tag instead, which needs git and
# an older Go to bootstrap (set LOOM_DEV_BOOTSTRAP_GO, or have `go` on PATH). The linters install
# with `go install` and need the Go module proxy (proxy.golang.org) or the GOPROXY you set.
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKFLOW="$REPO_DIR/.github/workflows/framework-ci.yml"
GO_MOD="$REPO_DIR/go.mod"
DEV_HOME="${LOOM_DEV_HOME:-$HOME/.cache/loom-dev}"
GO_DOWNLOAD_BASE="https://dl.google.com/go"
GO_SOURCE_REPO="https://github.com/golang/go.git"
GOLANGCI_MODULE="github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
GREMLINS_MODULE="github.com/go-gremlins/gremlins/cmd/gremlins"

fail() {
  echo "dev-setup.sh: $*" >&2
  exit 1
}

# --- Versions -----------------------------------------------------------------------------------

# go_version_from_mod FILE — the version setup-go installs: `toolchain goX.Y.Z` when present,
# otherwise the `go X.Y.Z` directive.
go_version_from_mod() {
  local mod_file="$1" toolchain go_line
  toolchain="$(awk '$1 == "toolchain" { sub(/^go/, "", $2); print $2; exit }' "$mod_file")"
  go_line="$(awk '$1 == "go" { print $2; exit }' "$mod_file")"
  local version="${toolchain:-$go_line}"
  [[ -n "$version" ]] || fail "no go or toolchain directive in $mod_file"
  echo "$version"
}

# golangci_version_from_workflow FILE — the `version:` of the golangci-lint-action step.
golangci_version_from_workflow() {
  local workflow="$1" version
  version="$(awk '/golangci-lint-action@/ { found = 1 }
    found && $1 == "version:" { print $2; exit }' "$workflow")"
  [[ -n "$version" ]] || fail "no golangci-lint version pin in $workflow"
  echo "$version"
}

# gremlins_version_from_workflow FILE — the version in the workflow's `go install ...gremlins@vX`.
gremlins_version_from_workflow() {
  local workflow="$1" version
  version="$(grep -oE 'gremlins/cmd/gremlins@v[0-9]+\.[0-9]+\.[0-9]+' "$workflow" | head -1 | sed 's/.*@//')"
  [[ -n "$version" ]] || fail "no gremlins version pin in $workflow"
  echo "$version"
}

# --- Platform -----------------------------------------------------------------------------------

go_os() {
  case "$(uname -s)" in
    Linux) echo linux ;;
    Darwin) echo darwin ;;
    *) fail "unsupported OS $(uname -s); this script covers Linux and macOS" ;;
  esac
}

# go_arch MACHINE — Go's name for a `uname -m` value.
go_arch() {
  case "$1" in
    x86_64 | amd64) echo amd64 ;;
    aarch64 | arm64) echo arm64 ;;
    *) fail "unsupported CPU $1; this script covers amd64 and arm64" ;;
  esac
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{ print $1 }'
  else
    shasum -a 256 "$1" | awk '{ print $1 }'
  fi
}

# --- Go -----------------------------------------------------------------------------------------

go_root_for() { echo "$DEV_HOME/go$1"; }

# reported_go_version GO_BINARY — "1.26.5" from "go version go1.26.5 linux/amd64".
reported_go_version() {
  GOTOOLCHAIN=local "$1" version 2>/dev/null | awk '{ sub(/^go/, "", $3); print $3 }'
}

is_go_installed() {
  local go_binary
  go_binary="$(go_root_for "$1")/bin/go"
  [[ -x "$go_binary" && "$(reported_go_version "$go_binary")" == "$1" ]]
}

# download_go VERSION TARGET — fetch the release archive and refuse it unless its SHA-256
# matches the one Google publishes beside it. Runs in a subshell so its cleanup trap stays
# local. Exits 1 when the host cannot be reached, 2 when the archive fails verification.
download_go() (
  version="$1" target="$2"
  archive="go${version}.$(go_os)-$(go_arch "$(uname -m)").tar.gz"
  scratch="$(mktemp -d)"
  trap 'rm -rf "$scratch"' EXIT
  curl -fsSL --max-time 300 -o "$scratch/$archive" "$GO_DOWNLOAD_BASE/$archive" 2>/dev/null || exit 1
  curl -fsSL --max-time 30 -o "$scratch/$archive.sha256" "$GO_DOWNLOAD_BASE/$archive.sha256" 2>/dev/null || exit 1
  [[ "$(sha256_of "$scratch/$archive")" == "$(tr -d '[:space:]' <"$scratch/$archive.sha256")" ]] || exit 2
  mkdir -p "$scratch/unpacked"
  tar -xzf "$scratch/$archive" -C "$scratch/unpacked" || exit 2
  mv "$scratch/unpacked/go" "$target"
)

bootstrap_go() {
  local candidate="${LOOM_DEV_BOOTSTRAP_GO:-$(command -v go || true)}"
  [[ -n "$candidate" && -x "$candidate" ]] || return 1
  GOTOOLCHAIN=local "$candidate" env GOROOT
}

# build_go VERSION TARGET — build Go from the release tag, for networks that reach GitHub but
# not dl.google.com.
build_go() {
  local version="$1" target="$2" bootstrap_root
  bootstrap_root="$(bootstrap_go)" ||
    fail "dl.google.com is not reachable and no Go is available to build Go $version from source (set LOOM_DEV_BOOTSTRAP_GO)"
  echo "  dl.google.com not reachable; building Go $version from source (a few minutes)…"
  git -c advice.detachedHead=false clone -q --depth 1 --branch "go$version" "$GO_SOURCE_REPO" "$target"
  (cd "$target/src" && GOROOT_BOOTSTRAP="$bootstrap_root" GOTOOLCHAIN=local ./make.bash >"$target/make.log" 2>&1) ||
    fail "building Go $version failed; see $target/make.log"
}

install_go() {
  local version="$1" target status=0
  target="$(go_root_for "$version")"
  if is_go_installed "$version"; then
    echo "  Go $version: already installed"
  else
    rm -rf "$target"
    mkdir -p "$DEV_HOME"
    download_go "$version" "$target" || status=$?
    case "$status" in
      0) ;;
      1) build_go "$version" "$target" ;;
      *) fail "the Go $version archive failed its checksum or did not unpack; refusing to install it" ;;
    esac
    echo "  Go $version: installed in $target"
  fi
  ln -sfn "$target" "$DEV_HOME/go"
}

# --- Linters ------------------------------------------------------------------------------------

# install_tool NAME MODULE VERSION GO_VERSION — `go install` into a versioned directory, then
# point $DEV_HOME/bin/NAME at it, so changing a pin never leaves a stale binary on PATH.
install_tool() {
  local name="$1" module="$2" version="$3" go_version="$4" tool_dir
  tool_dir="$DEV_HOME/tools/$name-$version-go$go_version"
  if [[ -x "$tool_dir/$name" ]]; then
    echo "  $name $version: already installed"
  else
    GOBIN="$tool_dir" GOTOOLCHAIN=local "$(go_root_for "$go_version")/bin/go" install "$module@$version" ||
      fail "go install $module@$version failed; it needs proxy.golang.org (or your GOPROXY)"
    echo "  $name $version: installed"
  fi
  mkdir -p "$DEV_HOME/bin"
  ln -sfn "$tool_dir/$name" "$DEV_HOME/bin/$name"
}

# --- Commands -----------------------------------------------------------------------------------

# read_pins — set GO_VERSION, GOLANGCI_VERSION and GREMLINS_VERSION, or stop. Each is assigned
# on its own line so a missing pin stops the script instead of reaching a command empty.
read_pins() {
  GO_VERSION="$(go_version_from_mod "$GO_MOD")"
  GOLANGCI_VERSION="$(golangci_version_from_workflow "$WORKFLOW")"
  GREMLINS_VERSION="$(gremlins_version_from_workflow "$WORKFLOW")"
}

print_versions() {
  read_pins
  echo "go            $GO_VERSION"
  echo "golangci-lint $GOLANGCI_VERSION"
  echo "gremlins      $GREMLINS_VERSION"
}

print_env() {
  echo "export PATH=\"$DEV_HOME/go/bin:$DEV_HOME/bin:\$PATH\""
  echo "export GOTOOLCHAIN=local"
}

install_all() {
  read_pins
  echo "Installing the CI toolchain into $DEV_HOME"
  install_go "$GO_VERSION"
  install_tool golangci-lint "$GOLANGCI_MODULE" "$GOLANGCI_VERSION" "$GO_VERSION"
  install_tool gremlins "$GREMLINS_MODULE" "$GREMLINS_VERSION" "$GO_VERSION"
  echo ""
  echo "Done. Use it in this shell with:"
  echo "  eval \"\$(bash scripts/dev-setup.sh --env)\""
}

# check_tool LABEL WANT HAVE — one line of --check; returns 1 on a mismatch.
check_tool() {
  local label="$1" want="$2" have="$3"
  if [[ "$have" == "$want" ]]; then
    echo "  ok        $label $have"
    return 0
  fi
  echo "  MISMATCH  $label: CI uses $want, PATH has ${have:-nothing}"
  return 1
}

installed_golangci_version() {
  command -v golangci-lint >/dev/null 2>&1 || return 0
  golangci-lint version 2>/dev/null | grep -oE 'version v?[0-9]+\.[0-9]+\.[0-9]+' | head -1 | awk '{ sub(/^v/, "", $2); print "v" $2 }'
}

installed_gremlins_version() {
  local binary
  binary="$(command -v gremlins || true)"
  [[ -n "$binary" ]] || return 0
  # gremlins prints no version of its own; the versioned directory install_tool used says it.
  readlink "$binary" 2>/dev/null | grep -oE 'gremlins-v[0-9]+\.[0-9]+\.[0-9]+' | sed 's/^gremlins-//'
}

check_all() {
  local mismatches=0 go_on_path have_go
  read_pins
  go_on_path="$(command -v go || true)"
  have_go=""
  [[ -z "$go_on_path" ]] || have_go="$(reported_go_version "$go_on_path")"
  check_tool "go" "$GO_VERSION" "$have_go" || mismatches=$((mismatches + 1))
  check_tool "golangci-lint" "$GOLANGCI_VERSION" "$(installed_golangci_version)" || mismatches=$((mismatches + 1))
  check_tool "gremlins" "$GREMLINS_VERSION" "$(installed_gremlins_version)" || mismatches=$((mismatches + 1))
  if [[ $mismatches -gt 0 ]]; then
    echo ""
    echo "Run: bash scripts/dev-setup.sh && eval \"\$(bash scripts/dev-setup.sh --env)\""
    return 1
  fi
}

main() {
  case "${1:-}" in
    "") install_all ;;
    --env) print_env ;;
    --check) check_all ;;
    --versions) print_versions ;;
    -h | --help) sed -n '2,25p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//' ;;
    *) fail "unknown option $1 (try --help)" ;;
  esac
}

# Sourced by scripts/test-dev-setup.sh to test the functions; run directly to use them.
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
