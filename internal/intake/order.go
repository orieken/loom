// Package intake reads and validates factory work orders — the files in a
// project's intake/ directory (roadmap L3.63, L3.64).
//
// An intake file is a work order and a spec in one: YAML frontmatter that the
// factory acts on, and a markdown body the pipeline delivers. This package
// interprets only the frontmatter. The body is handed to `loom run` untouched,
// because it is the run's input and its digest is what approvals bind to.
package intake

// SchemaVersion is the intake schema this build reads. An order declaring any
// other version is refused rather than read as if it were this one.
const SchemaVersion = 1

// Order is an intake file's frontmatter. Every field is human-owned (or
// import-adapter-owned): the supervisor reads an order and never writes one.
type Order struct {
	Intake      int      `yaml:"intake"`
	ID          string   `yaml:"id"`
	Type        string   `yaml:"type"`
	Plan        string   `yaml:"plan"`
	Status      string   `yaml:"status"`
	Priority    int      `yaml:"priority"`
	Created     string   `yaml:"created"`
	RequestedBy string   `yaml:"requested-by"`
	Source      string   `yaml:"source"`
	SourceRef   string   `yaml:"source-ref"`
	Trust       string   `yaml:"trust"`
	BaseBranch  string   `yaml:"base-branch"`
	Finish      string   `yaml:"finish"`
	Autonomy    string   `yaml:"autonomy"`
	DependsOn   []string `yaml:"depends-on"`
	Budget      Budget   `yaml:"budget"`
	Scope       Scope    `yaml:"scope"`
}

// Budget is the ceiling an order sets on its own delivery. Recording it is
// this package's job; enforcing it is roadmap L3.70's.
type Budget struct {
	MaxAttempts int     `yaml:"max-attempts"`
	MaxUSD      float64 `yaml:"max-usd"`
	MaxHours    float64 `yaml:"max-hours"`
}

// Scope is what an order expects its run to touch. Empty means no expectation
// was stated, not that nothing may be touched.
type Scope struct {
	Paths []string `yaml:"paths"`
}

// Document is a parsed intake file: the order, and the spec body below it.
type Document struct {
	Order Order
	Body  string
	// BodyLine is the file line the body starts on, so a finding in the body
	// can be reported against the file a person will open.
	BodyLine int
}

// IsExternal reports whether any of the order came from outside the team,
// which is what decides whether its body is scanned for injection.
func (o Order) IsExternal() bool {
	return o.Trust == TrustExternal
}
