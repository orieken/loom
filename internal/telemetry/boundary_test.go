package telemetry_test

// The fitness function for architecture-guardrails.md #8: "No OpenTelemetry
// instrumentation logic is allowed inside domain entities... Traces and
// spans must only be emitted from the adapter layer."
//
// The framework asserts this rule about the code its agents write. This is
// it asserting the rule about itself, as a test rather than as a review
// note — the standard guardrail #7 sets for every structural decision.
//
// The check is transitive, not just direct imports: reaching OTel through a
// helper package would violate the guardrail exactly as much as importing
// it, and would be much easier to do by accident.

import (
	"os/exec"
	"strings"
	"testing"
)

// otelModulePrefix matches every OpenTelemetry package.
const otelModulePrefix = "go.opentelemetry.io/"

// innerLayers must reach no OpenTelemetry package by any path.
//
// internal/state is the domain layer the guardrail names. internal/
// orchestrator is held to the same bar deliberately, though the guardrail
// only requires it of entities: it defines the Tracer interface but must
// not know what implements it, which is what keeps the seam a seam.
var innerLayers = []string{
	"github.com/orieken/loom/internal/state",
	"github.com/orieken/loom/internal/orchestrator",
	// The MCP server's domain layer is held to it too. Tool-call spans are
	// emitted from mcp_adapter.go, which is explicitly the only file allowed
	// to translate between domain types and the wire — so it is also the
	// only place telemetry belongs (M0.3 and guardrail #8 agreeing).
	"github.com/orieken/loom/shared/mcp/internal/domain",
	"github.com/orieken/loom/tools",
}

// L3.8 / AC: no inner layer reaches an OpenTelemetry package by any path.
// exemplar: a guardrail asserted as a test rather than a review note, checking
// the transitive graph instead of the import list a reader would eyeball.
func TestInnerLayersDoNotImportOpenTelemetry(t *testing.T) {
	for _, layer := range innerLayers {
		t.Run(layer, func(t *testing.T) {
			for _, dependency := range dependenciesOf(t, layer) {
				if strings.HasPrefix(dependency, otelModulePrefix) {
					t.Errorf("%s reaches %s — guardrail #8 confines spans to the adapter layer;\n"+
						"emit them from internal/telemetry and pass an orchestrator.Tracer instead", layer, dependency)
				}
			}
		})
	}
}

// dependenciesOf returns a package's full transitive import set.
func dependenciesOf(t *testing.T, pkg string) []string {
	t.Helper()
	output, err := exec.Command("go", "list", "-deps", pkg).CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", pkg, err, output)
	}
	return strings.Fields(string(output))
}

// The adapter layer is where OTel is supposed to be. If this package ever
// stops importing it, the guardrail test above has become vacuous — it
// would pass just as well with the instrumentation deleted.
func TestTelemetryPackageDoesImportOpenTelemetry(t *testing.T) {
	for _, dependency := range dependenciesOf(t, "github.com/orieken/loom/internal/telemetry") {
		if strings.HasPrefix(dependency, otelModulePrefix) {
			return
		}
	}
	t.Fatal("internal/telemetry imports no OpenTelemetry package — the guardrail test above is now vacuous")
}
