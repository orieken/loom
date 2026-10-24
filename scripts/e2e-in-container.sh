#!/usr/bin/env bash
# Runs the factory's end-to-end harness in containers (roadmap L3.73).
#
#   scripts/e2e-in-container.sh [extra deliver flags...]
#
# Delivery runs in one container with no access to the hidden acceptance
# tests; verification runs in a second, with the delivery's working volume and
# the hidden tests mounted read-only. With the default mock provider the
# mock's developer and QA stages write the fixture's solution.
#
# Environment:
#   E2E_IMAGE     image tag to build and run (default loom-e2e:local)
#   E2E_PROVIDER  stage provider (default mock; claude is roadmap L3.74)
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly REPO_DIR
readonly IMAGE="${E2E_IMAGE:-loom-e2e:local}"
readonly PROVIDER="${E2E_PROVIDER:-mock}"
readonly FIXTURE=/e2e/fixture

DOCKER_BUILDKIT=1 docker build -f "$REPO_DIR/docker/e2e.Dockerfile" -t "$IMAGE" "$REPO_DIR"

work="$(docker volume create)"
trap 'docker volume rm -f "$work" >/dev/null' EXIT

deliver_args=(deliver --fixture "$FIXTURE" --workdir /work --provider "$PROVIDER")
if [[ "$PROVIDER" == mock ]]; then
  deliver_args+=(--run-arg=--mock-stage-cost=0.01 "--run-arg=--mock-solution=$FIXTURE/solution")
fi

# A failed delivery still gets verified: the report says what went wrong.
set +e
docker run --rm -v "$work:/work" "$IMAGE" "${deliver_args[@]}" "$@"
set -e

docker run --rm -v "$work:/work" -v "$REPO_DIR/tests/e2e/testdata/hidden:/e2e/hidden:ro" "$IMAGE" \
  verify --fixture "$FIXTURE" --hidden /e2e/hidden --workdir /work
