# Dev container for loom. The image holds only what the OS provides; scripts/dev-setup.sh installs
# Go, golangci-lint and gremlins on create, at the versions CI reads from go.mod and
# .github/workflows/framework-ci.yml, so this file never names a Go version of its own.
#
# The base matches the CI runner (ubuntu-latest) and scripts/ci-check.sh.
FROM ubuntu:24.04

# Not version-pinned, the same as the CI runner and scripts/ci-check.sh: Ubuntu's archive replaces
# exact package versions, so a pin breaks the build within weeks. Everything that decides a test
# result (Go and the linters) is pinned by dev-setup.sh instead.
#   curl, ca-certificates  Go release download         gcc, libc6-dev  go test -race (cgo)
#   git                    source fallback, CI scripts  python3-yaml, python3-jsonschema
#                                                          check-parity.sh, health-check.sh
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       bash ca-certificates curl gcc git libc6-dev python3 python3-jsonschema python3-yaml \
    && rm -rf /var/lib/apt/lists/*

# A non-root user. The two cache directories exist before the volumes in devcontainer.json mount
# over them, so Docker gives the new volumes this user's ownership.
RUN useradd --create-home --shell /bin/bash dev \
    && mkdir -p /home/dev/.cache /home/dev/go \
    && chown -R dev:dev /home/dev
USER dev
WORKDIR /home/dev
