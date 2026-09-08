package state

// ArchitectureState is the architect's output, modelling
// shared/contracts/architecture-contract.md as typed fields.

// StructuralDecision is one decision and the fitness function that keeps it
// true. architect.md's own rule: a decision without one must be explicitly
// justified and flagged judgment-only — so that is a field, not a comment.
type StructuralDecision struct {
	Decision  string           `json:"decision" jsonschema:"required"`
	Rationale string           `json:"rationale" jsonschema:"required"`
	TradeOffs []string         `json:"tradeOffs,omitempty" jsonschema:"description=What this decision costs; every decision has a cost"`
	Fitness   *FitnessFunction `json:"fitness,omitempty" jsonschema:"description=Omitted only when judgmentOnly is true"`
	// JudgmentOnly marks a decision that cannot be machine-verified.
	// Rationale then has to carry the justification.
	JudgmentOnly bool `json:"judgmentOnly,omitempty"`
}

// ComponentPlacement says where a component lives and why, which is the
// Clean Architecture layer decision made explicit.
type ComponentPlacement struct {
	Component string `json:"component" jsonschema:"required"`
	Layer     string `json:"layer" jsonschema:"required,description=Entities, UseCases, Adapters, or Frameworks"`
	Package   string `json:"package" jsonschema:"required,description=Where it goes in this repo"`
	Reason    string `json:"reason,omitempty"`
}

// StabilityPattern is a Nygard stability pattern applied to a component.
type StabilityPattern struct {
	Pattern   string `json:"pattern" jsonschema:"required,description=Circuit Breaker, Bulkhead, Timeout, Retry, Idempotency key, Fail Fast"`
	AppliesTo string `json:"appliesTo" jsonschema:"required"`
	Rationale string `json:"rationale,omitempty"`
}

// ObservabilitySignal is a trace, metric, or log and where it is emitted
// from. The layer matters: guardrail #8 forbids instrumentation inside
// domain entities.
type ObservabilitySignal struct {
	Signal      string `json:"signal" jsonschema:"required"`
	EmittedFrom string `json:"emittedFrom" jsonschema:"required,description=Adapter or interceptor layer, never domain"`
	Cardinality string `json:"cardinality,omitempty" jsonschema:"description=Expected label cardinality, for anything that becomes a metric"`
}

// CheckVerdict is the outcome of a boundary or anti-pattern check.
type CheckVerdict string

// The verdicts a check can return.
const (
	VerdictPass          CheckVerdict = "PASS"
	VerdictFail          CheckVerdict = "FAIL"
	VerdictNotApplicable CheckVerdict = "NOT_APPLICABLE"
)

// BoundaryCheck records one layer-boundary rule and whether this design
// honours it.
type BoundaryCheck struct {
	Rule    string       `json:"rule" jsonschema:"required"`
	Verdict CheckVerdict `json:"verdict" jsonschema:"required"`
	Notes   string       `json:"notes,omitempty"`
}

// AntiPatternCheck records one anti-pattern the architect looked for.
// Found is a boolean rather than prose so a later stage can act on it.
type AntiPatternCheck struct {
	Pattern string `json:"pattern" jsonschema:"required,description=Distributed monolith, anemic domain model, god object, shotgun surgery, leaky abstraction, premature generalization"`
	Found   bool   `json:"found"`
	Notes   string `json:"notes,omitempty"`
}

// RefactoringOpportunity is a named Fowler operation on adjacent code.
type RefactoringOpportunity struct {
	Operation string `json:"operation" jsonschema:"required,description=A named Fowler operation, e.g. Extract Function"`
	Target    string `json:"target" jsonschema:"required"`
	Reason    string `json:"reason,omitempty"`
}

// ArchitectureState is the typed form of architecture-notes.md.
type ArchitectureState struct {
	SchemaVersion int    `json:"schemaVersion" jsonschema:"required"`
	Feature       string `json:"feature" jsonschema:"required"`

	StructuralDecisions []StructuralDecision `json:"structuralDecisions" jsonschema:"required"`
	ComponentPlacement  []ComponentPlacement `json:"componentPlacement,omitempty"`
	BoundedContext      BoundedContext       `json:"boundedContext" jsonschema:"required"`

	StabilityDesign     []StabilityPattern    `json:"stabilityDesign,omitempty"`
	ObservabilityDesign []ObservabilitySignal `json:"observabilityDesign,omitempty"`

	LayerBoundaryChecks []BoundaryCheck    `json:"layerBoundaryChecks,omitempty"`
	AntiPatternChecks   []AntiPatternCheck `json:"antiPatternChecks,omitempty"`

	FitnessFunctions         []FitnessFunction        `json:"fitnessFunctions" jsonschema:"required"`
	RefactoringOpportunities []RefactoringOpportunity `json:"refactoringOpportunities,omitempty"`

	// Retrieval carries the frontmatter fields the retrieval corpus
	// indexes on and validate-artifact checks for.
	Retrieval Retrieval `json:"retrieval,omitempty"`

	DeveloperHandoffNotes []string `json:"developerHandoffNotes,omitempty"`
	OpenQuestions         []string `json:"openQuestions,omitempty"`
	// RFCPath is set when the architect wrote an RFC, which the pipeline
	// pauses on for human acknowledgement.
	RFCPath string `json:"rfcPath,omitempty"`
}

// Validate enforces what the developer and code-reviewer cannot work
// without, plus the architect's own rule that every structural decision
// either carries a fitness function or is explicitly judgment-only.
func (a ArchitectureState) Validate() error {
	return firstError(
		requireSchemaVersion(a.SchemaVersion),
		requireText("feature", a.Feature),
		requireItems("structuralDecisions", len(a.StructuralDecisions)),
		requireText("boundedContext.owning", a.BoundedContext.Owning),
		requireItems("fitnessFunctions", len(a.FitnessFunctions)),
		validateDecisions(a.StructuralDecisions),
	)
}

func validateDecisions(decisions []StructuralDecision) error {
	for i, decision := range decisions {
		if err := validateDecision(i, decision); err != nil {
			return err
		}
	}
	return nil
}

func validateDecision(index int, decision StructuralDecision) error {
	if decision.Decision == "" {
		return &ValidationError{Field: fieldPath(index, "decision"), Reason: "is required and empty"}
	}
	if decision.Fitness == nil && !decision.JudgmentOnly {
		return &ValidationError{Field: fieldPath(index, "fitness"),
			Reason: "is required unless the decision is flagged judgmentOnly (architecture-guardrails.md #7)"}
	}
	return nil
}
