package orchestrator_test

// The fitness function for architecture-guardrails.md #1: "Inner layers
// NEVER import from outer layers."
//
// internal/telemetry/boundary_test.go asserts one slice of this — that no
// inner layer reaches OpenTelemetry — and nothing asserted the general rule.
// The direction held in fact and by nobody's decision: a single import of
// internal/telemetry from internal/state would have compiled, passed every
// test in this repository, and inverted the dependency the Tracer interface
// exists to keep inverted.
//
// It lives here because internal/orchestrator is the use-case layer the rule
// most constrains, and because invariants_test.go already makes this package
// the home for assertions about the run's shape rather than its behavior.
//
// The check is transitive. Reaching an outer layer through a helper package
// violates the guardrail exactly as much as importing it directly, and is
// much easier to do by accident.

import (
	"os/exec"
	"sort"
	"strings"
	"testing"
)

const modulePrefix = "github.com/orieken/loom/"

// innerLayers pins each inner package to the COMPLETE set of loom packages
// it may reach. Only the inner layers are pinned: adapters legitimately
// import inward and each other, so pinning them would be a brittle
// description of ordinary churn rather than a guardrail.
//
// The assertion is equality, not a ceiling. An extra entry is a violation —
// an inner layer reaching outward. A missing one means the pin has gone
// stale and now describes a structure that no longer exists, which is how a
// guardrail quietly becomes decoration.
var innerLayers = map[string][]string{
	// The domain. These own the types and rules; they may reach nothing.
	"internal/state":    {},
	"internal/policy":   {},
	"internal/worktree": {},
	// The use-case layer. It defines the Provider and Tracer interfaces and
	// must not know what implements them — that seam is the whole reason
	// internal/telemetry and internal/provider can be swapped or absent.
	"internal/orchestrator": {"internal/policy", "internal/state"},
}

func TestInnerLayersImportOnlyInward(t *testing.T) {
	for layer, permitted := range innerLayers {
		t.Run(layer, func(t *testing.T) {
			reached := loomDependenciesOf(t, layer)
			allowed := asSet(permitted)

			for _, dependency := range reached {
				if !allowed[dependency] {
					t.Errorf("%s reaches %s — guardrail #1 forbids an inner layer importing an outer one;\n"+
						"depend on an interface defined in %s and let the outer layer implement it",
						layer, dependency, layer)
				}
			}

			// A pin that over-declares stops describing the code. Catching
			// that here is what keeps this list a live statement of the
			// architecture rather than a ceiling nobody revisits.
			found := asSet(reached)
			for _, expected := range permitted {
				if !found[expected] {
					t.Errorf("%s no longer reaches %s — the pin in innerLayers is stale; remove it",
						layer, expected)
				}
			}
		})
	}
}

// loomDependenciesOf returns the loom packages a package reaches
// transitively, as repo-relative paths, excluding the package itself.
func loomDependenciesOf(t *testing.T, pkg string) []string {
	t.Helper()
	output, err := exec.Command("go", "list", "-deps", modulePrefix+pkg).CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", pkg, err, output)
	}

	var reached []string
	for _, dependency := range strings.Fields(string(output)) {
		if !strings.HasPrefix(dependency, modulePrefix) {
			continue
		}
		relative := strings.TrimPrefix(dependency, modulePrefix)
		if relative != pkg {
			reached = append(reached, relative)
		}
	}
	sort.Strings(reached)
	return reached
}

func asSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}
