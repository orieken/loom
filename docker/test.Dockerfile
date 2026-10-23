# loom's pinned build-and-test image (roadmap L3.72).
#
# Everything a full verification needs is baked in, so a run needs no network:
# the Go toolchain go.mod requires, the module cache for both modules, and the
# linters CI pins. Source is not copied in. It is mounted read-only at run time,
# so this image changes only when the toolchain, a tool, or go.sum changes.
#
#   make test-image                build it
#   make test-in-container         run every step against the working tree
#   make test-in-container-quick   every step except mutation
#
# The versions below must match .github/workflows/framework-ci.yml. The image
# does not trust that: scripts/test-in-container.sh's toolchain step fails when
# the Go here differs from go.mod's, and GOTOOLCHAIN=local stops Go from
# quietly downloading a different one.
#
# PRECONDITION: the base image tag names an exact Go release, never `latest`.
# ENFORCED-BY: judgment-only — a digest pin is roadmap L3.72's open follow-up,
# because the session that wrote this could not reach Docker Hub to read one.

ARG GO_VERSION=1.26.5
ARG DEBIAN_RELEASE=bookworm

# --- Linters, built once, copied as binaries ----------------------------------
FROM golang:${GO_VERSION}-${DEBIAN_RELEASE} AS tools
# Same versions CI pins: golangci-lint-action's `version:` and the mutation
# job's `go install` line.
ARG GOLANGCI_LINT_VERSION=v2.13.2
ARG GREMLINS_VERSION=v0.6.0
ENV GOTOOLCHAIN=local
RUN go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}" \
 && go install "github.com/go-gremlins/gremlins/cmd/gremlins@${GREMLINS_VERSION}"

# --- Module cache for the loom module and the embedding example ---------------
FROM golang:${GO_VERSION}-${DEBIAN_RELEASE} AS modules
ENV GOTOOLCHAIN=local \
    GOMODCACHE=/opt/go/mod
WORKDIR /src
COPY go.mod go.sum ./
COPY examples/embedding/go.mod examples/embedding/go.sum ./examples/embedding/
# The embedding example replaces loom with ../.., which is why the root go.mod
# is copied first and the example is downloaded from inside the same tree.
RUN go mod download \
 && cd examples/embedding \
 && go mod download

# --- The image that runs ------------------------------------------------------
FROM golang:${GO_VERSION}-${DEBIAN_RELEASE} AS test
# GOPROXY=off and -mod=readonly: a run that needs a module the image lacks
# fails naming it, instead of fetching it and passing somewhere CI would not.
# Every writable path is under /tmp, so the container works as any uid — the
# Makefile runs it as the host user so nothing it writes is owned by root.
ENV GOTOOLCHAIN=local \
    GOPROXY=off \
    GOFLAGS=-mod=readonly \
    GOMODCACHE=/opt/go/mod \
    GOCACHE=/tmp/go-build \
    GOLANGCI_LINT_CACHE=/tmp/golangci-lint \
    HOME=/tmp
COPY --from=tools /go/bin/golangci-lint /go/bin/gremlins /usr/local/bin/
COPY --from=modules /opt/go/mod /opt/go/mod
# The mounted checkout belongs to whoever runs the container, and git refuses
# to read a repository owned by another uid unless told it is safe.
RUN git config --system --add safe.directory '*' \
 && useradd --uid 10001 --no-create-home --shell /usr/sbin/nologin loom
USER loom
WORKDIR /workspace
ENTRYPOINT ["bash", "scripts/test-in-container.sh"]
