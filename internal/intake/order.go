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
	Slice       *Slice   `yaml:"slice"`
}

// Slice is the frontmatter an order sets when it delivers only part of a
// spec package someone split by hand (roadmap L3.63/L3.64). An order with no
// `slice` block has Slice == nil and validates exactly as it did before this
// field existed.
type Slice struct {
	// Of is the spec package this order is a slice of. Every order sharing
	// one Of is checked together by CheckSlices.
	Of string `yaml:"of"`
	// Spec is the project-root-relative path to the spec package's JSON:
	// criteria[] with an id and an observableVia (api, ui or both).
	Spec string `yaml:"spec"`
	// Digest is "sha256:<hex>" of Manifest's bytes. Every slice of one Of
	// must declare the same value.
	Digest string `yaml:"digest"`
	// Manifest is the project-root-relative path to a sha256sum-style
	// listing — one "<hex digest>  <path>" pair per line — of the files the
	// split's spec package depends on.
	Manifest string `yaml:"manifest"`
	// Covers lists the criteria this slice covers: a bare id (e.g. "AC2")
	// covers every level that criterion has; "AC1:ui" covers only its ui
	// level.
	Covers []string `yaml:"covers"`
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
