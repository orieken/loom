package state

// A projection is the narrowly-scoped slice of upstream state a stage is
// allowed to read — the point of L2.9. The architect does not receive the
// whole analysis; it receives the fields architecture-contract.md needs.
//
// Projections are keyed by **(consuming stage, upstream kind)**, because
// what a stage reads and what it writes vary independently: `tech-writer`
// still produces markdown but reads typed state, and the architect and the
// tech-writer need different slices of the same analysis.
//
// Every projection is a field selection. No model sits on this path —
// retiring the summarization that used to is L2.10, of which the two
// analysis projections below are the first part.

import (
	"encoding/json"
	"fmt"
)

// projections maps a consuming stage to the upstream kinds it reads and how.
func projections() map[string]map[Kind]func([]byte) ([]byte, error) {
	return map[string]map[Kind]func([]byte) ([]byte, error){
		"architect":         {KindAnalysis: projectAnalysisForArchitect},
		"developer":         {KindReview: projectReviewForDeveloper},
		"security-reviewer": {KindImplementation: projectImplementationForSecurity},
		// Written before the developer runs, from the analysis alone
		// (roadmap L3.62). The only upstream it may ever read is analysis:
		// see TestAcceptanceScenariosAreWrittenBlind in internal/orchestrator.
		AcceptanceScenariosStageID: {KindAnalysis: projectAnalysisForQA},
		"qa-engineer": {
			KindScenarios:      projectScenariosForQA,
			KindImplementation: projectImplementationForQA,
			KindSecurity:       projectSecurityForQA,
			KindAnalysis:       projectAnalysisForQA,
		},
		"tech-writer": {
			KindQA:       projectQAForTechWriter,
			KindAnalysis: projectAnalysisForTechWriter,
		},
	}
}

// ProjectionFor computes what one consuming stage reads from one upstream
// document.
func ProjectionFor(consumerStage string, upstream Kind, payload []byte) ([]byte, error) {
	fromKind, reads := projections()[consumerStage]
	if !reads {
		return nil, fmt.Errorf("stage %q declares no projections", consumerStage)
	}
	project, declared := fromKind[upstream]
	if !declared {
		return nil, fmt.Errorf("stage %q has no projection declared from %q", consumerStage, upstream)
	}
	return project(payload)
}

// decodeUpstream parses an upstream document of a known kind.
func decodeUpstream[T Validatable](kind Kind, payload []byte) (T, error) {
	var zero T
	decoded, err := Decode(kind, payload)
	if err != nil {
		return zero, fmt.Errorf("upstream %s state: %w", kind, err)
	}
	typed, ok := decoded.(T)
	if !ok {
		return zero, fmt.Errorf("upstream state is not %s", kind)
	}
	return typed, nil
}

// ArchitectInput is what the architect reads from the analysis: the
// constraints and structural facts it decides against. Notably absent are
// the QA, tech-writer, and devops task lists, the definition of done, and
// the edge cases — the architect has no use for them.
type ArchitectInput struct {
	Feature                   string                     `json:"feature"`
	Summary                   string                     `json:"summary"`
	AcceptanceCriteria        []AcceptanceCriterion      `json:"acceptanceCriteria"`
	NonFunctionalRequirements []NonFunctionalRequirement `json:"nonFunctionalRequirements,omitempty"`
	ProposedFitnessFunctions  []FitnessFunction          `json:"proposedFitnessFunctions,omitempty"`
	BoundedContext            BoundedContext             `json:"boundedContext"`
	DomainEvents              DomainEvents               `json:"domainEvents,omitempty"`
	AffectedComponents        []AffectedComponent        `json:"affectedComponents"`
	DataModelChanges          []DataModelChange          `json:"dataModelChanges,omitempty"`
	APIChanges                []APIChange                `json:"apiChanges,omitempty"`
	NewDependencies           []string                   `json:"newDependencies,omitempty"`
	ArchitecturalFlags        []string                   `json:"architecturalFlags,omitempty"`
	DeveloperTasks            []string                   `json:"developerTasks,omitempty"`
	OpenQuestions             []string                   `json:"openQuestions,omitempty"`
}

func projectAnalysisForArchitect(payload []byte) ([]byte, error) {
	analysis, err := decodeUpstream[*AnalysisState](KindAnalysis, payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(ArchitectInput{
		Feature: analysis.Feature, Summary: analysis.Summary,
		AcceptanceCriteria:        analysis.AcceptanceCriteria,
		NonFunctionalRequirements: analysis.NonFunctionalRequirements,
		ProposedFitnessFunctions:  analysis.ProposedFitnessFunctions,
		BoundedContext:            analysis.BoundedContext,
		DomainEvents:              analysis.DomainEvents,
		AffectedComponents:        analysis.AffectedComponents,
		DataModelChanges:          analysis.DataModelChanges,
		APIChanges:                analysis.APIChanges,
		NewDependencies:           analysis.NewDependencies,
		ArchitecturalFlags:        analysis.ArchitecturalFlags,
		DeveloperTasks:            analysis.Tasks.Developer,
		OpenQuestions:             analysis.OpenQuestions,
	})
}

// DeveloperInput is what the developer reads when the review loop sends it
// round again: the verdict and the findings to address. Absent is
// everything the reviewer said about surfaces and scores — the next
// iteration acts on instructions, not on an assessment of its last attempt.
type DeveloperInput struct {
	Feature  string    `json:"feature"`
	Verdict  Verdict   `json:"verdict"`
	Findings []Finding `json:"findings"`
}

func projectReviewForDeveloper(payload []byte) ([]byte, error) {
	review, err := decodeUpstream[*ReviewState](KindReview, payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(DeveloperInput{
		Feature: review.Feature, Verdict: review.Verdict, Findings: review.Findings,
	})
}

// SecurityReviewInput is what the security-reviewer reads from the
// implementation: where to look and what came in with it. The self-review
// checklist and refactoring log are the code-reviewer's business.
type SecurityReviewInput struct {
	Feature           string            `json:"feature"`
	FilesCreated      []string          `json:"filesCreated,omitempty"`
	FilesModified     []string          `json:"filesModified,omitempty"`
	InterfaceDesign   []string          `json:"interfaceDesign,omitempty"`
	DependenciesAdded []DependencyAdded `json:"dependenciesAdded,omitempty"`
}

func projectImplementationForSecurity(payload []byte) ([]byte, error) {
	implementation, err := decodeUpstream[*ImplementationState](KindImplementation, payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(SecurityReviewInput{
		Feature: implementation.Feature, FilesCreated: implementation.FilesCreated,
		FilesModified: implementation.FilesModified, InterfaceDesign: implementation.InterfaceDesign,
		DependenciesAdded: implementation.DependenciesAdded,
	})
}

// QAImplementationInput is what QA reads from the implementation: what
// changed, what the developer flagged for testing, and where the build
// departed from the analysis.
type QAImplementationInput struct {
	Feature       string      `json:"feature"`
	FilesCreated  []string    `json:"filesCreated,omitempty"`
	FilesModified []string    `json:"filesModified,omitempty"`
	NotesForQA    []string    `json:"notesForQa,omitempty"`
	Deviations    []Deviation `json:"deviations,omitempty"`
}

func projectImplementationForQA(payload []byte) ([]byte, error) {
	implementation, err := decodeUpstream[*ImplementationState](KindImplementation, payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(QAImplementationInput{
		Feature: implementation.Feature, FilesCreated: implementation.FilesCreated,
		FilesModified: implementation.FilesModified, NotesForQA: implementation.NotesForQA,
		Deviations: implementation.Deviations,
	})
}

// QASecurityInput is what QA reads from the security review: what to verify
// and what the reviewer wants tested.
type QASecurityInput struct {
	Feature    string            `json:"feature"`
	Findings   []SecurityFinding `json:"findings,omitempty"`
	NotesForQA []string          `json:"notesForQa,omitempty"`
}

func projectSecurityForQA(payload []byte) ([]byte, error) {
	security, err := decodeUpstream[*SecurityState](KindSecurity, payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(QASecurityInput{
		Feature: security.Feature, Findings: security.Findings, NotesForQA: security.NotesForQA,
	})
}

// QAAcceptanceInput is what QA reads from the analysis — the acceptance
// criteria and edge cases it tests against. This projection replaces a
// `summarize-artifact` call: an LLM was summarising a document that has
// been typed since epic 79 (part of L2.10).
type QAAcceptanceInput struct {
	Feature            string                `json:"feature"`
	AcceptanceCriteria []AcceptanceCriterion `json:"acceptanceCriteria"`
	EdgeCases          []EdgeCase            `json:"edgeCases,omitempty"`
	QATasks            []string              `json:"qaTasks,omitempty"`
	DefinitionOfDone   []string              `json:"definitionOfDone,omitempty"`
}

func projectAnalysisForQA(payload []byte) ([]byte, error) {
	analysis, err := decodeUpstream[*AnalysisState](KindAnalysis, payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(QAAcceptanceInput{
		Feature: analysis.Feature, AcceptanceCriteria: analysis.AcceptanceCriteria,
		EdgeCases: analysis.EdgeCases, QATasks: analysis.Tasks.QA,
		DefinitionOfDone: analysis.DefinitionOfDone,
	})
}

// TechWriterScopeInput is what the tech-writer reads from the analysis —
// feature intent and scope. This projection replaces the second
// `summarize-artifact` call site (part of L2.10).
type TechWriterScopeInput struct {
	Feature        string   `json:"feature"`
	Summary        string   `json:"summary"`
	OutOfScope     []string `json:"outOfScope,omitempty"`
	TechWriterTask []string `json:"techWriterTasks,omitempty"`
}

func projectAnalysisForTechWriter(payload []byte) ([]byte, error) {
	analysis, err := decodeUpstream[*AnalysisState](KindAnalysis, payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(TechWriterScopeInput{
		Feature: analysis.Feature, Summary: analysis.Summary,
		OutOfScope: analysis.OutOfScope, TechWriterTask: analysis.Tasks.TechWriter,
	})
}

// TechWriterQAInput is what the tech-writer reads from the QA report: the
// behaviour QA found surprising, and what could not be tested.
type TechWriterQAInput struct {
	Feature            string     `json:"feature"`
	NotesForTechWriter []string   `json:"notesForTechWriter,omitempty"`
	KnownGaps          []KnownGap `json:"knownGaps,omitempty"`
}

func projectQAForTechWriter(payload []byte) ([]byte, error) {
	qa, err := decodeUpstream[*QAState](KindQA, payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(TechWriterQAInput{
		Feature: qa.Feature, NotesForTechWriter: qa.NotesForTechWriter, KnownGaps: qa.KnownGaps,
	})
}

// AcceptanceScenariosStageID is the stage that writes acceptance scenarios
// before implementation. It is qa-engineer under its own stage ID, so its
// projection and its place in the plan can differ from qa-engineer's.
const AcceptanceScenariosStageID = "acceptance-scenarios"

// QAScenariosInput is what qa-engineer reads from the scenarios written
// before the build: every one of them, to automate unchanged. It automates
// the scenarios it was given rather than writing new ones from the code.
type QAScenariosInput struct {
	Feature   string               `json:"feature"`
	Scenarios []AcceptanceScenario `json:"scenarios"`
}

func projectScenariosForQA(payload []byte) ([]byte, error) {
	scenarios, err := decodeUpstream[*ScenariosState](KindScenarios, payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(QAScenariosInput{Feature: scenarios.Feature, Scenarios: scenarios.Scenarios})
}
