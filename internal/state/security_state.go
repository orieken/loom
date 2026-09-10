package state

// SecurityState is the security-reviewer's output, modelling
// shared/contracts/security-contract.md — including the rule that contract
// already asserts and validate-artifact checks by grep: a Critical or High
// finding must carry a fix, not a recommendation.

import "fmt"

// Severity is the finding ladder the security-reviewer already uses. The
// pipeline's "block on Critical findings" guardrail reads this, so it is an
// enum rather than a string compared against prose.
type Severity string

// The severities, most serious first.
const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
	SeverityInfo     Severity = "INFO"
)

// blocking reports whether a severity must be fixed rather than merely
// recommended, per the agent's own rule.
func (s Severity) blocking() bool {
	return s == SeverityCritical || s == SeverityHigh
}

func (s Severity) valid() bool {
	switch s {
	case SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo:
		return true
	default:
		return false
	}
}

// StrideCategory is the STRIDE dimension a finding belongs to. The contract
// requires all six sections to be present; the analysis records which one
// each finding came from.
type StrideCategory string

// The six STRIDE categories.
const (
	StrideSpoofing              StrideCategory = "SPOOFING"
	StrideTampering             StrideCategory = "TAMPERING"
	StrideRepudiation           StrideCategory = "REPUDIATION"
	StrideInformationDisclosure StrideCategory = "INFORMATION_DISCLOSURE"
	StrideDenialOfService       StrideCategory = "DENIAL_OF_SERVICE"
	StrideElevationOfPrivilege  StrideCategory = "ELEVATION_OF_PRIVILEGE"
)

// StrideAnalysis is what the reviewer found in each category. Every
// category is present in the rendered document whether or not it has
// findings — the contract requires the heading either way.
type StrideAnalysis struct {
	Category StrideCategory `json:"category" jsonschema:"required"`
	Assessed string         `json:"assessed" jsonschema:"required,description=What was considered, or an explicit statement that the category does not apply"`
}

// SecurityFinding is one vulnerability. FixApplied is the field the
// contract's rule turns on: "A Critical/High finding with `Fix applied:
// Recommendation only` is a FAIL".
type SecurityFinding struct {
	Severity     Severity       `json:"severity" jsonschema:"required"`
	Title        string         `json:"title" jsonschema:"required"`
	Location     string         `json:"location" jsonschema:"required,description=Repo-relative path and line"`
	Category     StrideCategory `json:"category,omitempty"`
	Description  string         `json:"description" jsonschema:"required"`
	FixApplied   string         `json:"fixApplied,omitempty" jsonschema:"description=What was changed. Required for CRITICAL and HIGH — a recommendation is not a fix"`
	Verification string         `json:"verification,omitempty" jsonschema:"description=How QA can verify the fix"`
}

// SecurityState is the typed form of security-report.md.
type SecurityState struct {
	SchemaVersion int    `json:"schemaVersion" jsonschema:"required"`
	Feature       string `json:"feature" jsonschema:"required"`

	ThreatModelSummary string            `json:"threatModelSummary" jsonschema:"required"`
	DependencyAudit    []string          `json:"dependencyAudit,omitempty"`
	Stride             []StrideAnalysis  `json:"stride" jsonschema:"required"`
	Findings           []SecurityFinding `json:"findings,omitempty"`
	FilesModified      []string          `json:"filesModified,omitempty"`
	SecurityChecklist  []ChecklistItem   `json:"securityChecklist,omitempty"`
	NotesForQA         []string          `json:"notesForQa,omitempty"`
	NotesForTechWriter []string          `json:"notesForTechWriter,omitempty"`

	Retrieval Retrieval `json:"retrieval,omitempty"`
}

// HasBlockingFindings reports whether this review found something the
// pipeline must not proceed past. It is the field the "block on Critical
// findings" guardrail should read, rather than searching the prose.
func (s SecurityState) HasBlockingFindings() bool {
	for _, finding := range s.Findings {
		if finding.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

// Validate enforces the contract's stated rules, in code rather than by
// grep: all six STRIDE categories assessed, and every Critical or High
// finding carrying an actual fix.
func (s SecurityState) Validate() error {
	return firstError(
		requireSchemaVersion(s.SchemaVersion),
		requireText("feature", s.Feature),
		requireText("threatModelSummary", s.ThreatModelSummary),
		requireEveryStrideCategory(s.Stride),
		validateSecurityFindings(s.Findings),
	)
}

// requireEveryStrideCategory mirrors "all six STRIDE categories included".
// A category that does not apply is stated as not applying — silence and
// "considered, nothing found" are different facts.
func requireEveryStrideCategory(analyses []StrideAnalysis) error {
	assessed := make(map[StrideCategory]bool, len(analyses))
	for _, analysis := range analyses {
		assessed[analysis.Category] = analysis.Assessed != ""
	}
	for _, category := range strideCategories() {
		if !assessed[category] {
			return &ValidationError{Field: "stride",
				Reason: fmt.Sprintf("does not assess %s — the contract requires all six categories", category)}
		}
	}
	return nil
}

func strideCategories() []StrideCategory {
	return []StrideCategory{
		StrideSpoofing, StrideTampering, StrideRepudiation,
		StrideInformationDisclosure, StrideDenialOfService, StrideElevationOfPrivilege,
	}
}

func validateSecurityFindings(findings []SecurityFinding) error {
	for index, finding := range findings {
		if err := validateSecurityFinding(index, finding); err != nil {
			return err
		}
	}
	return nil
}

func validateSecurityFinding(index int, finding SecurityFinding) error {
	if !finding.Severity.valid() {
		return &ValidationError{Field: fmt.Sprintf("findings[%d].severity", index),
			Reason: fmt.Sprintf("is %q, want one of CRITICAL, HIGH, MEDIUM, LOW, INFO", finding.Severity)}
	}
	if finding.Severity.blocking() && finding.FixApplied == "" {
		return &ValidationError{Field: fmt.Sprintf("findings[%d].fixApplied", index),
			Reason: fmt.Sprintf("is required for a %s finding (%q) — a recommendation is not a fix, and the pipeline blocks on this", finding.Severity, finding.Title)}
	}
	return nil
}
