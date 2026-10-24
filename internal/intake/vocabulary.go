package intake

// The closed vocabularies an order's fields draw from. A value outside its
// set is a violation, not a default: a typo in `status` must never read as
// a status the supervisor acts on.

// Trust values. External orders are scanned for injection before pickup.
const (
	TrustInternal = "internal"
	TrustExternal = "external"
)

// SourceManual is the one source a human writes by hand. Every other source
// is an import adapter, and an imported order is external by definition.
const SourceManual = "manual"

// StatusReady is the only status the supervisor picks up.
const StatusReady = "ready"

// TypeBug is the one type the lighter bugfix plan exists for.
const TypeBug = "bug"

// PlanBugfix skips the architect, data, performance, accessibility and devops
// stages. That is safe for a reproducible bug and nothing else.
const PlanBugfix = "deliver-bugfix"

// Priority bounds: 1 is the most urgent.
const (
	HighestPriority = 1
	LowestPriority  = 4
)

var (
	knownTypes      = set("feature", TypeBug, "spike", "chore")
	knownStatuses   = set("draft", StatusReady, "hold")
	knownSources    = set(SourceManual, "github-issue", "linear", "jira")
	knownTrust      = set(TrustInternal, TrustExternal)
	knownFinishes   = set("pr", "branch", "none")
	knownAutonomies = set("inherit", "strict-human")
)

func set(values ...string) map[string]bool {
	members := make(map[string]bool, len(values))
	for _, value := range values {
		members[value] = true
	}
	return members
}
