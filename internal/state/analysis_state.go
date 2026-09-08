package state

import (
	"fmt"
	"strings"
)

// AnalysisState is the analyst's output, modelling
// shared/contracts/analysis-contract.md as typed fields rather than twenty
// markdown headings. Only what a downstream stage actually reads is
// modelled — inventing fields nobody consumes is how a schema rots.

// AcceptanceCriterion is one testable statement of done. Examples carry
// Specification by Example data the analyst was asked to make concrete.
type AcceptanceCriterion struct {
	Statement string   `json:"statement" jsonschema:"required,description=What must be true, in business language, never implementation"`
	Examples  []string `json:"examples,omitempty" jsonschema:"description=Concrete examples or data rows for complex rules"`
}

// Threshold is a measurable limit: a number, a unit, and what is being
// measured. It is structured rather than prose because two routing
// decisions depend on a threshold being real (roadmap L3.24).
//
// It used to be a free-text string, and the third real end-to-end run
// routed both the architect and the performance-engineer into a three-line
// synchronous array filter on the strength of "O(n) over the captured logs
// array... no I/O" — a sentence, in the threshold field, which a non-empty
// test read as a measurable target. Together those two stages cost $1.45 to
// report having nothing to do. A number and a unit cannot be satisfied by
// writing a sentence.
type Threshold struct {
	Metric string  `json:"metric" jsonschema:"required,description=What is measured, e.g. p99 request latency"`
	Value  float64 `json:"value" jsonschema:"required,description=The numeric limit, e.g. 200"`
	Unit   string  `json:"unit" jsonschema:"required,description=The unit of the limit, e.g. ms, rps, MB"`
}

// IsMeasurable reports whether a threshold states something a fitness
// function could check. A threshold missing its number or its unit is prose
// wearing a struct.
func (t *Threshold) IsMeasurable() bool {
	return t != nil && t.Value > 0 && t.Unit != "" && t.Metric != ""
}

// String renders a threshold the way a report shows it.
func (t *Threshold) String() string {
	if !t.IsMeasurable() {
		return ""
	}
	return fmt.Sprintf("%s %g%s", t.Metric, t.Value, t.Unit)
}

// NonFunctionalRequirement is a performance, security, or scaling
// constraint. Threshold is separate from the prose so a fitness function
// can be checked against it — and typed, so that separation means
// something.
type NonFunctionalRequirement struct {
	Category    string     `json:"category" jsonschema:"required,enum=performance,enum=security,enum=scaling,enum=accessibility,enum=other"`
	Requirement string     `json:"requirement" jsonschema:"required"`
	Threshold   *Threshold `json:"threshold,omitempty" jsonschema:"description=The measurable limit. Omit entirely when the requirement carries no number — do not describe it in prose here"`
}

// Surfaces are the kinds of surface a feature exposes. Each one decides
// whether a stage that can only review that surface is worth invoking
// (roadmap L3.24).
//
// These are declared, not inferred from prose. The third real run skipped
// accessibility-engineer correctly for having no UI, and in the same run ran
// visual-qa-engineer — which reported "no visual QA surface exists" — because
// it was hard-coded as always-runs. Two stages answering the same question
// disagreed about it, at $0.55.
type Surfaces struct {
	// UI is true when the feature renders something a person looks at.
	UI bool `json:"ui,omitempty" jsonschema:"description=True when this feature adds or changes a user interface a person sees"`
	// Runtime is true when the feature runs in a served process with
	// availability someone is on the hook for — not an in-process library
	// or test utility.
	Runtime bool `json:"runtime,omitempty" jsonschema:"description=True when this feature runs in a deployed service with availability or latency someone operates"`
}

// AffectedComponent is one file or module the feature touches.
type AffectedComponent struct {
	Path   string `json:"path" jsonschema:"required,description=Repo-relative path"`
	Reason string `json:"reason" jsonschema:"required"`
}

// MigrationPhase is the Expand/Contract phase of a data model change.
// Destructive work cannot ship in the same release as the code it supports
// (architecture-guardrails.md #2), so the phase is a field, not prose.
type MigrationPhase string

// The phases a data model change can be in.
const (
	MigrationPhaseNone     MigrationPhase = "none"
	MigrationPhaseExpand   MigrationPhase = "expand"
	MigrationPhaseContract MigrationPhase = "contract"
)

// DataModelChange is one schema change and when it may run.
type DataModelChange struct {
	Description string         `json:"description" jsonschema:"required"`
	Phase       MigrationPhase `json:"phase" jsonschema:"required"`
}

// APIChange is one endpoint or signature change.
type APIChange struct {
	Endpoint string `json:"endpoint" jsonschema:"required"`
	Change   string `json:"change" jsonschema:"required"`
}

// TaskList is the per-role work the analyst broke the feature into. The
// roles are separate fields because each downstream agent reads only its
// own list — the projection L2.9 exists to make possible.
type TaskList struct {
	Developer  []string `json:"developer,omitempty"`
	QA         []string `json:"qa,omitempty"`
	TechWriter []string `json:"techWriter,omitempty"`
	DevOps     []string `json:"devops,omitempty"`
}

// EdgeCase is a boundary condition and how the feature handles it.
type EdgeCase struct {
	Case     string `json:"case" jsonschema:"required"`
	Handling string `json:"handling" jsonschema:"required"`
}

// AnalysisState is the typed form of analysis.md.
type AnalysisState struct {
	SchemaVersion int    `json:"schemaVersion" jsonschema:"required"`
	Feature       string `json:"feature" jsonschema:"required,description=kebab-case feature slug"`
	Summary       string `json:"summary" jsonschema:"required,description=One paragraph of what this does and why"`

	AcceptanceCriteria        []AcceptanceCriterion      `json:"acceptanceCriteria" jsonschema:"required"`
	NonFunctionalRequirements []NonFunctionalRequirement `json:"nonFunctionalRequirements,omitempty"`
	ProposedFitnessFunctions  []FitnessFunction          `json:"proposedFitnessFunctions,omitempty"`
	OutOfScope                []string                   `json:"outOfScope,omitempty"`

	// Surfaces decides which surface-specific review stages run. Absent
	// means no surface, which is the honest default: a feature that renders
	// nothing and serves nothing should not summon reviewers for either.
	Surfaces Surfaces `json:"surfaces,omitempty"`

	BoundedContext     BoundedContext      `json:"boundedContext" jsonschema:"required"`
	DomainEvents       DomainEvents        `json:"domainEvents,omitempty"`
	AffectedComponents []AffectedComponent `json:"affectedComponents" jsonschema:"required"`
	DataModelChanges   []DataModelChange   `json:"dataModelChanges,omitempty"`
	APIChanges         []APIChange         `json:"apiChanges,omitempty"`
	NewDependencies    []string            `json:"newDependencies,omitempty"`

	// ArchitecturalFlags is the explicit escape hatch for structural work
	// the derived signals in RequiresArchitect cannot see — a new base
	// class, a reversal of an existing ADR — and the way a human forces an
	// architect regardless. Acting on it is L3.1.
	ArchitecturalFlags []string `json:"architecturalFlags,omitempty"`

	// Retrieval carries the frontmatter fields the retrieval corpus
	// indexes on and validate-artifact checks for.
	Retrieval Retrieval `json:"retrieval,omitempty"`

	Tasks            TaskList   `json:"tasks" jsonschema:"required"`
	EdgeCases        []EdgeCase `json:"edgeCases,omitempty"`
	DefinitionOfDone []string   `json:"definitionOfDone" jsonschema:"required"`
	OpenQuestions    []string   `json:"openQuestions,omitempty"`
}

// Validate enforces the fields a downstream stage cannot work without.
// Optional lists stay optional: an analyst legitimately has nothing to say
// about API changes on a docs-only feature.
func (a AnalysisState) Validate() error {
	return firstError(
		requireSchemaVersion(a.SchemaVersion),
		requireText("feature", a.Feature),
		requireText("summary", a.Summary),
		requireItems("acceptanceCriteria", len(a.AcceptanceCriteria)),
		requireText("boundedContext.owning", a.BoundedContext.Owning),
		requireItems("affectedComponents", len(a.AffectedComponents)),
		requireItems("definitionOfDone", len(a.DefinitionOfDone)),
	)
}

// RequiresArchitect reports whether this analysis describes structural work
// that needs an architect before code is written.
//
// deliver-feature/SKILL.md step 12 states the same condition in prose, and
// this is its tested form. Deriving the answer from contract-validated
// facts makes it a function anyone can check, rather than a self-assessment
// the analyst writes about its own work. (Step 12 used to route on an
// "Architectural Flags" heading that existed in neither the template nor
// the contract; epic 79 corrected it to name fields that exist.)
//
// The four derived signals below cover the common cases. They cannot see
// "this introduces a new base class" or "this reverses ADR-004", which
// architect.md also exists for — that is what ArchitecturalFlags is for,
// and why the two are OR'd rather than the derivation standing alone.
//
// Deciding what to DO with this answer (skipping the stage, routing around
// it) is L3.1; this is a fact about the analysis, not a routing rule.
func (a AnalysisState) RequiresArchitect() bool {
	if len(a.ArchitecturalFlags) > 0 {
		return true
	}
	return a.crossesContexts() || a.changesDataModel() ||
		len(a.NewDependencies) > 0 || a.hasPerformanceThreshold()
}

// RequiresPerformanceEngineer reports whether the analysis carries a
// measurable performance target. A threshold is what a performance review
// has to work against; prose about feeling fast is not reviewable.
func (a AnalysisState) RequiresPerformanceEngineer() bool {
	return a.hasPerformanceThreshold()
}

// RequiresDataEngineer reports whether this feature changes the data model
// in a way that has to be sequenced across a deploy.
func (a AnalysisState) RequiresDataEngineer() bool {
	return a.changesDataModel()
}

// RequiresAccessibilityEngineer reports whether the feature has a UI
// surface. The declared surface is OR'd with the accessibility requirement
// the contract already makes mandatory for any UI feature, the same way
// RequiresArchitect ORs its explicit flag with its derived signals.
func (a AnalysisState) RequiresAccessibilityEngineer() bool {
	return a.hasUISurface()
}

// RequiresVisualQAEngineer reports whether there is anything to look at.
// It answers the same question as RequiresAccessibilityEngineer and so must
// answer it the same way: the two disagreeing is the defect L3.24 records.
func (a AnalysisState) RequiresVisualQAEngineer() bool {
	return a.hasUISurface()
}

// RequiresSREEngineer reports whether the feature runs somewhere with
// availability or latency an operator is accountable for. An in-process
// utility has no SLI, and the third real run spent $0.63 establishing that
// about an array filter.
func (a AnalysisState) RequiresSREEngineer() bool {
	return a.Surfaces.Runtime || len(a.APIChanges) > 0
}

func (a AnalysisState) hasUISurface() bool {
	return a.Surfaces.UI || a.hasRequirementCategory("accessibility")
}

// RequiresDevOpsEngineer reports whether this feature asks for CI,
// environment, or deployment work.
//
// Entries that only say "nothing to do" are not counted (roadmap L3.18).
// The contract now tells the analyst to omit the list entirely in that case,
// and this does not depend on the analyst having complied: the second real
// run emitted exactly one DevOps task reading "None required by this spec —
// no CI or deployment config changes requested", the router counted one item
// and spent $0.64 discovering the sentence meant zero.
func (a AnalysisState) RequiresDevOpsEngineer() bool {
	for _, task := range a.Tasks.DevOps {
		if !isNoOpTask(task) {
			return true
		}
	}
	return false
}

// noOpTaskOpeners are the ways a model writes "nothing here". A real task is
// written as an imperative — "Add a workflow", "Update the pipeline" — so
// matching on the opening word is specific enough to be safe and blunt
// enough to be obvious. The contract, not this list, is the primary
// mechanism; this is the net under it.
var noOpTaskOpeners = []string{"none", "n/a", "na", "nothing", "not applicable", "not required"}

func isNoOpTask(task string) bool {
	normalized := strings.ToLower(strings.TrimSpace(task))
	for _, opener := range noOpTaskOpeners {
		if normalized == opener || strings.HasPrefix(normalized, opener+" ") {
			return true
		}
	}
	return false
}

func (a AnalysisState) hasRequirementCategory(category string) bool {
	for _, requirement := range a.NonFunctionalRequirements {
		if requirement.Category == category {
			return true
		}
	}
	return false
}

func (a AnalysisState) crossesContexts() bool {
	return len(a.BoundedContext.Crossings) > 0
}

// changesDataModel ignores entries explicitly recorded as "none" so that an
// analyst documenting the absence of a migration does not summon an
// architect.
func (a AnalysisState) changesDataModel() bool {
	for _, change := range a.DataModelChanges {
		if change.Phase == MigrationPhaseExpand || change.Phase == MigrationPhaseContract {
			return true
		}
	}
	return false
}

// hasPerformanceThreshold treats a measurable latency or throughput target
// as structural: thresholds are what force timeouts, circuit breakers, and
// idempotency decisions (Nygard stability patterns).
//
// "Measurable" is the load-bearing word, and it is now enforced by the type
// rather than by a non-empty check (roadmap L3.24).
func (a AnalysisState) hasPerformanceThreshold() bool {
	for _, requirement := range a.NonFunctionalRequirements {
		if requirement.Category == "performance" && requirement.Threshold.IsMeasurable() {
			return true
		}
	}
	return false
}
