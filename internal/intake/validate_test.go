package intake

import (
	"strings"
	"testing"
)

// validOrder is an order that breaks no rule; each test changes one field.
func validOrder() Order {
	return Order{
		Intake: SchemaVersion, ID: "user-auth-lockout", Type: "feature", Plan: "deliver-feature",
		Status: StatusReady, Priority: 2, Created: "2026-10-23", RequestedBy: "oscar",
		Source: SourceManual, Trust: TrustInternal, BaseBranch: "main", Finish: "pr",
		Autonomy: "inherit", Budget: Budget{MaxAttempts: 2, MaxUSD: 10, MaxHours: 4},
	}
}

func validContext() Context {
	return Context{FileStem: "user-auth-lockout", Plans: []string{"deliver-feature", PlanBugfix}}
}

func TestValidateAcceptsAValidOrder(t *testing.T) {
	if violations := Validate(validOrder(), validContext()); len(violations) != 0 {
		t.Errorf("violations = %v, want none", violations)
	}
}

// Each case breaks exactly one rule and must be reported against its field.
func TestValidateReportsEachBrokenRuleAgainstItsField(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*Order, *Context)
		field   string
		message string
	}{
		{"unknown schema version", func(o *Order, _ *Context) { o.Intake = 2 }, "intake", "this build reads 1"},
		{"missing schema version", func(o *Order, _ *Context) { o.Intake = 0 }, "intake", "schema version is 0"},
		{"id not kebab-case", func(o *Order, _ *Context) { o.ID = "User_Auth" }, "id", "not kebab-case"},
		{"id differs from file", func(o *Order, c *Context) { c.FileStem = "other" }, "id", `file name "other"`},
		{"unknown type", func(o *Order, _ *Context) { o.Type = "epic" }, "type", `"epic" is not one of`},
		{"unknown status", func(o *Order, _ *Context) { o.Status = "queued" }, "status", "draft, hold, ready"},
		{"unknown source", func(o *Order, _ *Context) { o.Source = "email" }, "source", `"email"`},
		{"unknown trust", func(o *Order, _ *Context) { o.Trust = "trusted" }, "trust", `"trusted"`},
		{"unknown finish", func(o *Order, _ *Context) { o.Finish = "merge" }, "finish", `"merge"`},
		{"autonomy cannot loosen", func(o *Order, _ *Context) { o.Autonomy = "unattended" }, "autonomy", `"unattended"`},
		{"plan not runnable here", func(o *Order, _ *Context) { o.Plan = "deliver-everything" }, "plan", "not a plan this project can run"},
		{"bugfix plan for a feature", func(o *Order, _ *Context) { o.Plan = PlanBugfix }, "plan", `for type "bug" only`},
		{"priority too urgent", func(o *Order, _ *Context) { o.Priority = 0 }, "priority", "outside 1"},
		{"priority too low", func(o *Order, _ *Context) { o.Priority = 5 }, "priority", "outside 1"},
		{"created placeholder", func(o *Order, _ *Context) { o.Created = "YYYY-MM-DD" }, "created", "YYYY-MM-DD form"},
		{"import vouched internal", func(o *Order, _ *Context) { o.Source = "github-issue" }, "trust", `must be "external"`},
		{"ready without owner", func(o *Order, _ *Context) { o.RequestedBy = " " }, "requested-by", "accountable"},
		{"branch with no base", func(o *Order, _ *Context) { o.BaseBranch = "" }, "base-branch", "needs a base branch"},
		{"dependency not an id", func(o *Order, _ *Context) { o.DependsOn = []string{"PROJ-1"} }, "depends-on", "not an order id"},
		{"self dependency", func(o *Order, _ *Context) { o.DependsOn = []string{o.ID} }, "depends-on", "itself"},
		{"duplicate dependency", func(o *Order, _ *Context) { o.DependsOn = []string{"a", "a"} }, "depends-on", "listed twice"},
		{"no attempts", func(o *Order, _ *Context) { o.Budget.MaxAttempts = 0 }, "budget.max-attempts", "at least 1"},
		{"no spend", func(o *Order, _ *Context) { o.Budget.MaxUSD = 0 }, "budget.max-usd", "greater than 0"},
		{"no time", func(o *Order, _ *Context) { o.Budget.MaxHours = -1 }, "budget.max-hours", "greater than 0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			order, context := validOrder(), validContext()
			tc.mutate(&order, &context)
			violations := Validate(order, context)
			if len(violations) != 1 {
				t.Fatalf("violations = %v, want exactly one", violations)
			}
			if violations[0].Field != tc.field || !strings.Contains(violations[0].Message, tc.message) {
				t.Errorf("got %q, want field %q with message containing %q", violations[0], tc.field, tc.message)
			}
		})
	}
}

// The relaxations are part of the contract too: each is allowed on purpose.
func TestValidateAllowsWhatTheContractAllows(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Order)
	}{
		{"a bug through the bugfix plan", func(o *Order) { o.Type, o.Plan = TypeBug, PlanBugfix }},
		{"a bug through the full plan", func(o *Order) { o.Type = TypeBug }},
		{"an import marked external", func(o *Order) { o.Source, o.Trust = "linear", TrustExternal }},
		{"a draft with no owner yet", func(o *Order) { o.Status, o.RequestedBy = "draft", "" }},
		{"no base branch when nothing ships", func(o *Order) { o.Finish, o.BaseBranch = "none", "" }},
		{"distinct dependencies", func(o *Order) { o.DependsOn = []string{"a", "b-2"} }},
		{"the highest priority", func(o *Order) { o.Priority = HighestPriority }},
		{"the lowest priority", func(o *Order) { o.Priority = LowestPriority }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			order := validOrder()
			tc.mutate(&order)
			if violations := Validate(order, validContext()); len(violations) != 0 {
				t.Errorf("violations = %v, want none", violations)
			}
		})
	}
}

// One pass must report everything, so a file can be fixed in one edit.
func TestValidateReportsEveryViolationAtOnce(t *testing.T) {
	order := validOrder()
	order.Type, order.Priority, order.Created = "epic", 9, "soon"
	if violations := Validate(order, validContext()); len(violations) != 3 {
		t.Errorf("violations = %v, want three", violations)
	}
}

func TestViolationStringNamesItsField(t *testing.T) {
	if got := (Violation{Field: "id", Message: "bad"}).String(); got != "id: bad" {
		t.Errorf("String() = %q", got)
	}
}
