# The factory's end-to-end image (roadmap L3.73).
#
# It holds the loom binary, the harness, and the fixture — but never the
# hidden acceptance tests. Those are mounted read-only into a second
# container that only verifies, after the delivering one has exited, so an
# agent in the run cannot read or write its own judge. That separation is
# what makes the real-provider run (L3.74) worth trusting; with the mock it
# costs nothing.
#
#   scripts/e2e-in-container.sh        build, deliver, then verify
#   make e2e-in-container              the same
#
# PRECONDITION: the base image tag names an exact Go release, never `latest`.
# ENFORCED-BY: judgment-only — as in docker/test.Dockerfile, pinning by digest
# is roadmap L3.72's open follow-up.

ARG GO_VERSION=1.26.5
ARG DEBIAN_RELEASE=bookworm

FROM golang:${GO_VERSION}-${DEBIAN_RELEASE} AS build
ENV GOTOOLCHAIN=local CGO_ENABLED=0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/loom ./cmd/loom \
 && go build -o /out/factory-e2e ./tests/e2e/cmd/factory-e2e

FROM golang:${GO_VERSION}-${DEBIAN_RELEASE} AS e2e
# The fixture is its own module with no dependencies, so the acceptance tests
# need no network. Every writable path is under /tmp or /work.
ENV GOTOOLCHAIN=local \
    GOPROXY=off \
    GOCACHE=/tmp/go-build \
    HOME=/tmp
COPY --from=build /out/loom /out/factory-e2e /usr/local/bin/
COPY tests/e2e/testdata/repo /e2e/fixture/repo
COPY tests/e2e/testdata/order.md /e2e/fixture/order.md
COPY tests/e2e/testdata/solution /e2e/fixture/solution
# /work is owned by the runtime user so a fresh named volume mounted there
# inherits that ownership.
RUN git config --system --add safe.directory '*' \
 && useradd --uid 10001 --no-create-home --shell /usr/sbin/nologin loom \
 && mkdir /work && chown loom /work
USER loom
WORKDIR /work
ENTRYPOINT ["factory-e2e"]
