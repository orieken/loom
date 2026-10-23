package intake

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Violation is one broken rule, named by the frontmatter field it concerns.
type Violation struct {
	Field   string
	Message string
}

func (v Violation) String() string {
	return v.Field + ": " + v.Message
}

// Context is what validation needs beyond the order itself: the file's name,
// and the plans this project can actually run.
type Context struct {
	// FileStem is the file name without its extension. It becomes the run's
	// workspace name, which is why `id` must equal it.
	FileStem string
	// Plans are the plan names this project can resolve (planfile.Discover).
	// A plan named in an order but not here would fail at `loom run`, after
	// the order had already been claimed.
	Plans []string
}

const dateLayout = "2006-01-02"

var kebabCase = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type rule func(Order, Context) []Violation

// rules run in this order, so a report reads top to bottom like the file.
var rules = []rule{
	checkSchemaVersion,
	checkIdentity,
	checkVocabulary,
	checkPlan,
	checkPriority,
	checkCreated,
	checkProvenance,
	checkAccountability,
	checkFinishLine,
	checkDependencies,
	checkBudget,
}

// Validate applies every rule checkable from the order alone. It reports all
// violations rather than the first, so one edit can fix a file.
func Validate(order Order, context Context) []Violation {
	var violations []Violation
	for _, check := range rules {
		violations = append(violations, check(order, context)...)
	}
	return violations
}

func violation(field, format string, args ...any) []Violation {
	return []Violation{{Field: field, Message: fmt.Sprintf(format, args...)}}
}

func checkSchemaVersion(order Order, _ Context) []Violation {
	if order.Intake == SchemaVersion {
		return nil
	}
	return violation("intake", "schema version is %d, this build reads %d", order.Intake, SchemaVersion)
}

func checkIdentity(order Order, context Context) []Violation {
	if !kebabCase.MatchString(order.ID) {
		return violation("id", "%q is not kebab-case", order.ID)
	}
	if order.ID != context.FileStem {
		return violation("id", "%q does not match the file name %q; the file name becomes the run's workspace",
			order.ID, context.FileStem)
	}
	return nil
}

func checkVocabulary(order Order, _ Context) []Violation {
	var violations []Violation
	for _, field := range []struct {
		name, value string
		allowed     map[string]bool
	}{
		{"type", order.Type, knownTypes},
		{"status", order.Status, knownStatuses},
		{"source", order.Source, knownSources},
		{"trust", order.Trust, knownTrust},
		{"finish", order.Finish, knownFinishes},
		{"autonomy", order.Autonomy, knownAutonomies},
	} {
		if !field.allowed[field.value] {
			violations = append(violations, violation(field.name, "%q is not one of %s",
				field.value, listOf(field.allowed))...)
		}
	}
	return violations
}

func checkPlan(order Order, context Context) []Violation {
	if !contains(context.Plans, order.Plan) {
		return violation("plan", "%q is not a plan this project can run (available: %s)",
			order.Plan, strings.Join(context.Plans, ", "))
	}
	if order.Plan == PlanBugfix && order.Type != TypeBug {
		return violation("plan", "%s skips design and verification stages; it is for type %q only, not %q",
			PlanBugfix, TypeBug, order.Type)
	}
	return nil
}

func checkPriority(order Order, _ Context) []Violation {
	if order.Priority >= HighestPriority && order.Priority <= LowestPriority {
		return nil
	}
	return violation("priority", "%d is outside %d (highest) to %d", order.Priority, HighestPriority, LowestPriority)
}

func checkCreated(order Order, _ Context) []Violation {
	if _, err := time.Parse(dateLayout, order.Created); err != nil {
		return violation("created", "%q is not a date in YYYY-MM-DD form", order.Created)
	}
	return nil
}

// checkProvenance: anything an adapter imported is external. Only a human may
// vouch for text, and a human writes `source: manual`. An unknown source is
// checkVocabulary's to report; judging its provenance would report it twice.
func checkProvenance(order Order, _ Context) []Violation {
	isImport := knownSources[order.Source] && order.Source != SourceManual
	if isImport && order.Trust != TrustExternal {
		return violation("trust", "source %q is an import, so trust must be %q", order.Source, TrustExternal)
	}
	return nil
}

// checkAccountability: an unattended run guesses where the spec is silent, so
// a ready order names who answers for its spec.
func checkAccountability(order Order, _ Context) []Violation {
	if order.Status == StatusReady && strings.TrimSpace(order.RequestedBy) == "" {
		return violation("requested-by", "a ready order must name who is accountable for its spec")
	}
	return nil
}

func checkFinishLine(order Order, _ Context) []Violation {
	if order.Finish != "none" && strings.TrimSpace(order.BaseBranch) == "" {
		return violation("base-branch", "finish %q needs a base branch to branch from", order.Finish)
	}
	return nil
}

func checkDependencies(order Order, _ Context) []Violation {
	var violations []Violation
	seen := map[string]bool{}
	for _, dependency := range order.DependsOn {
		violations = append(violations, checkDependency(order.ID, dependency, seen)...)
		seen[dependency] = true
	}
	return violations
}

func checkDependency(self, dependency string, seen map[string]bool) []Violation {
	switch {
	case !kebabCase.MatchString(dependency):
		return violation("depends-on", "%q is not an order id", dependency)
	case dependency == self:
		return violation("depends-on", "an order cannot depend on itself")
	case seen[dependency]:
		return violation("depends-on", "%q is listed twice", dependency)
	}
	return nil
}

func checkBudget(order Order, _ Context) []Violation {
	var violations []Violation
	if order.Budget.MaxAttempts < 1 {
		violations = append(violations, violation("budget.max-attempts", "must be at least 1")...)
	}
	if order.Budget.MaxUSD <= 0 {
		violations = append(violations, violation("budget.max-usd", "must be greater than 0")...)
	}
	if order.Budget.MaxHours <= 0 {
		violations = append(violations, violation("budget.max-hours", "must be greater than 0")...)
	}
	return violations
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func listOf(members map[string]bool) string {
	names := make([]string, 0, len(members))
	for name := range members {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
