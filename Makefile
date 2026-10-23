.PHONY: install uninstall generate check test-agents health test-image test-in-container test-in-container-quick

install:
	./install.sh --global

uninstall:
	./uninstall.sh --global

generate:
	bash scripts/generate-configs.sh

check:
	bash scripts/check-parity.sh

test-agents:
	bash scripts/test-agents.sh

health:
	bash scripts/health-check.sh --verbose

# --- Pinned test image (roadmap L3.72) ---------------------------------------
# The same toolchain, modules and linters CI uses, on any machine with Docker.
# The checkout is mounted read-only and the container runs as you, so nothing
# it writes lands in the tree or belongs to root. Pass STEPS to run a subset:
#   make test-in-container STEPS="build test"
# Changed-line checks diff against the merge-base with origin/main unless
# LOOM_DIFF_BASE names another commit.
TEST_IMAGE ?= loom-test:local
STEPS ?=

test-image:
	docker build -f docker/test.Dockerfile -t $(TEST_IMAGE) .

test-in-container: test-image
	docker run --rm --user "$$(id -u):$$(id -g)" -v "$(CURDIR):/workspace:ro" \
		-e LOOM_DIFF_BASE $(TEST_IMAGE) $(STEPS)

# Everything except mutation, which re-runs each changed package's tests per
# mutant and takes minutes rather than seconds.
test-in-container-quick: STEPS = toolchain build test coverage lint smoke
test-in-container-quick: test-in-container
