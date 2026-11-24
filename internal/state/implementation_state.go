package state

// ImplementationState is the developer's output, modelling
// shared/contracts/implementation-contract.md. It is the other half of the
// review loop (L2.17): the reviewer's verdict is already typed, and this is
// what the verdict is about.

import "fmt"

// RefactoringEntry is one named Fowler operation from the developer's
// Named Refactoring Log. The operation is a field because the code-reviewer
// is asked to verify the refactor pass was taken seriously, and "was
// anything named" is a question a field can answer.
type RefactoringEntry struct {
	Operation string `json:"operation" jsonschema:"required,description=A named Fowler operation, e.g. Extract Function"`
	Target    string `json:"target" jsonschema:"required"`
	Reason    string `json:"reason,omitempty"`
}

// ChecklistItem is one line of the Self-Review Checklist or the Simple
// Design Verification. `Checked` is a boolean rather than a rendered `[x]`
// because the contract's own rule turns on whether the developer actually
// filled it in.
type ChecklistItem struct {
	Item    string `json:"item" jsonschema:"required"`
	Checked bool   `json:"checked"`
	Note    string `json:"note,omitempty"`
}

// Decision is one choice the developer made and why.
type Decision struct {
	Decision  string `json:"decision" jsonschema:"required"`
	Reasoning string `json:"reasoning" jsonschema:"required"`
}

// Deviation is a task from the analysis that was skipped or changed. The
// analysis is typed, so this is the one place the two documents disagree on
// purpose — and it is worth reading as a field.
type Deviation struct {
	Task   string `json:"task" jsonschema:"required"`
	Reason string `json:"reason" jsonschema:"required"`
}

// DependencyAdded is one package the implementation introduced.
type DependencyAdded struct {
	Package string `json:"package" jsonschema:"required"`
	Version string `json:"version,omitempty"`
	Reason  string `json:"reason" jsonschema:"required"`
}

// ImplementationState is the typed form of implementation-notes.md.
type ImplementationState struct {
	SchemaVersion int    `json:"schemaVersion" jsonschema:"required"`
	Feature       string `json:"feature" jsonschema:"required"`

	FilesCreated  []string `json:"filesCreated,omitempty"`
	FilesModified []string `json:"filesModified,omitempty"`
	// FilesDeleted names a path this change removed. A rename is the new
	// path in FilesCreated and the old path here, never "old -> new" in one
	// entry.
	FilesDeleted []string `json:"filesDeleted,omitempty" jsonschema:"description=A path this change removed. A rename is the new path in filesCreated and the old path here. Never write 'old -> new' as a single entry in either field."`

	InterfaceDesign []string           `json:"interfaceDesign,omitempty" jsonschema:"description=Public surface introduced or changed"`
	RefactoringLog  []RefactoringEntry `json:"refactoringLog,omitempty"`

	SelfReview        []ChecklistItem   `json:"selfReview" jsonschema:"required"`
	SimpleDesign      []ChecklistItem   `json:"simpleDesign" jsonschema:"required"`
	KeyDecisions      []Decision        `json:"keyDecisions,omitempty"`
	Deviations        []Deviation       `json:"deviations,omitempty"`
	DependenciesAdded []DependencyAdded `json:"dependenciesAdded,omitempty"`

	NotesForQA     []string `json:"notesForQa,omitempty"`
	NotesForDevOps []string `json:"notesForDevops,omitempty"`

	Retrieval Retrieval `json:"retrieval,omitempty"`
}

// Validate enforces the contract's own rule: the Self-Review Checklist and
// the Simple Design Verification must each be filled in. An unfilled
// checklist is a FAIL there "since it means the developer skipped
// self-review rather than completed it" — a rule that was a grep for `[x]`
// and is now a question about a boolean.
func (i ImplementationState) Validate() error {
	return firstError(
		requireSchemaVersion(i.SchemaVersion),
		requireText("feature", i.Feature),
		requireAnsweredChecklist("selfReview", i.SelfReview),
		requireAnsweredChecklist("simpleDesign", i.SimpleDesign),
		requireTouchedFiles(i),
		requireNoRenameArrows(i.PathClaims()),
	)
}

// requireAnsweredChecklist mirrors the contract: at least one item, and
// every item stating what it is. A checklist nobody filled in is the
// failure this catches.
func requireAnsweredChecklist(field string, items []ChecklistItem) error {
	if len(items) == 0 {
		return &ValidationError{Field: field,
			Reason: "must have at least one item — an unfilled checklist means self-review was skipped, not passed"}
	}
	for index, item := range items {
		if item.Item == "" {
			return &ValidationError{Field: fmt.Sprintf("%s[%d].item", field, index), Reason: "is required and empty"}
		}
	}
	return nil
}

// requireTouchedFiles catches an implementation that claims to have changed
// nothing. Every downstream reviewer needs somewhere to look. A deletion
// counts: removing a file is work too.
func requireTouchedFiles(implementation ImplementationState) error {
	touched := len(implementation.FilesCreated) + len(implementation.FilesModified) + len(implementation.FilesDeleted)
	if touched > 0 {
		return nil
	}
	return &ValidationError{Field: "filesCreated/filesModified/filesDeleted",
		Reason: "must name at least one file — an implementation that touched nothing gives its reviewers nowhere to look"}
}
