package state

// RepoDocState is what the repo-documenter returns for one directory (ADR-010,
// roadmap L2.9 typed state): purpose and intent written on top of a fact
// sheet, each claim tied to the fact-sheet fields it relies on. The model is
// never the source of a fact; it is asked to interpret facts it was handed.
//
// "Unknown" is a valid answer. A directory the facts do not explain gets an
// empty purpose and an open question saying why — a plausible guess is worse
// than a gap.

import (
	"fmt"
	"strings"

	"github.com/invopop/jsonschema"
)

// Layer is where a directory sits in the architecture, from a closed set so
// CODEMAP and queries do not fragment across synonyms.
type Layer string

// The layers the documenter may assign.
const (
	LayerDomain      Layer = "domain"
	LayerApplication Layer = "application"
	LayerAdapter     Layer = "adapter"
	LayerFramework   Layer = "framework"
	LayerShared      Layer = "shared"
	LayerTest        Layer = "test"
	LayerDocs        Layer = "docs"
	LayerConfig      Layer = "config"
	LayerTooling     Layer = "tooling"
	LayerUnknown     Layer = "unknown"
)

// Values lists the layers, in schema order.
func (Layer) Values() []string {
	return []string{
		string(LayerDomain), string(LayerApplication), string(LayerAdapter), string(LayerFramework), string(LayerShared),
		string(LayerTest), string(LayerDocs), string(LayerConfig), string(LayerTooling), string(LayerUnknown),
	}
}

// JSONSchema derives the layer enum from the constants Validate checks.
func (l Layer) JSONSchema() *jsonschema.Schema { return enumSchema(l.Values()) }

// FactFieldRoots are the fact-sheet fields a claim may cite. A citation is
// `root` or `root:value` — `exports:Verify`, `imports:internal/telemetry` —
// and `child_summaries` cites a child directory's generated summary.
func FactFieldRoots() []string {
	return []string{
		"files", "exports", "imports", "approximate_imports", "dependents", "approximate_dependents",
		"entrypoints", "tests", "children", "child_summaries",
	}
}

// DocEvidence ties one claim to the fact-sheet fields behind it.
type DocEvidence struct {
	Claim  string   `json:"claim" jsonschema:"required,description=The purpose or responsibility or invariant text exactly as written elsewhere in this document"`
	Fields []string `json:"fields" jsonschema:"required,description=Fact-sheet fields the claim rests on: exports or exports:Name or imports:dir and so on"`
}

// DocInvariant is something the directory's code is relied on to keep true,
// with the evidence that it does.
type DocInvariant struct {
	Statement string   `json:"statement" jsonschema:"required"`
	Evidence  []string `json:"evidence" jsonschema:"required,description=Fact-sheet fields or file line ranges that establish the invariant"`
}

// DocFileRead records a source file the documenter read because a fact-sheet
// field was ambiguous. Reading source is the exception, so it is recorded.
type DocFileRead struct {
	Path      string `json:"path" jsonschema:"required"`
	StartLine int    `json:"startLine" jsonschema:"required,minimum=1"`
	EndLine   int    `json:"endLine" jsonschema:"required,minimum=1"`
	Reason    string `json:"reason" jsonschema:"required,description=Which fact-sheet field was ambiguous and why"`
}

// RepoDocState is the typed output of the repo-documenter for one directory.
type RepoDocState struct {
	SchemaVersion    int            `json:"schemaVersion" jsonschema:"required"`
	Path             string         `json:"path" jsonschema:"required,description=The directory documented: copy it from the fact sheet"`
	InputHash        string         `json:"inputHash" jsonschema:"required,description=The fact sheet's input_hash: copy it exactly"`
	Purpose          string         `json:"purpose" jsonschema:"description=One sentence on what the directory is for. Empty only when the facts do not establish it; then openQuestions must say why"`
	Responsibilities []string       `json:"responsibilities"`
	KeyEntrypoints   []string       `json:"keyEntrypoints" jsonschema:"description=Entrypoints and exported symbols a newcomer should read first. Only names the fact sheet lists"`
	Tags             []string       `json:"tags" jsonschema:"description=Tags from the allowed vocabulary only"`
	Layer            Layer          `json:"layer" jsonschema:"required"`
	Invariants       []DocInvariant `json:"invariants"`
	OpenQuestions    []string       `json:"openQuestions" jsonschema:"description=What the facts could not settle"`
	Evidence         []DocEvidence  `json:"evidence" jsonschema:"required"`
	FilesRead        []DocFileRead  `json:"filesRead"`
}

// Validate enforces structure only. Whether a cited symbol exists in the fact
// sheet, or a tag is in the vocabulary, is checked against the sheet by the
// verifier (roadmap Phase 5) — this cannot see the sheet.
func (d RepoDocState) Validate() error {
	return firstError(
		requireSchemaVersion(d.SchemaVersion),
		requireText("path", d.Path),
		requireText("inputHash", d.InputHash),
		d.validatePurpose(),
		validateLayer(d.Layer),
		d.validateEvidence(),
		validateInvariants(d.Invariants),
		validateFilesRead(d.FilesRead),
	)
}

// validatePurpose lets purpose be empty only with an explanation.
func (d RepoDocState) validatePurpose() error {
	if d.Purpose == "" && len(d.OpenQuestions) == 0 {
		return &ValidationError{Field: "purpose", Reason: "is empty and openQuestions is too — an unknown purpose must say what the facts could not settle"}
	}
	return nil
}

func validateLayer(layer Layer) error {
	for _, value := range layer.Values() {
		if string(layer) == value {
			return nil
		}
	}
	return &ValidationError{Field: "layer", Reason: fmt.Sprintf("%q is not one of %s", layer, strings.Join(layer.Values(), ", "))}
}

// validateEvidence requires evidence whenever there is a claim to back, and
// that every citation names a real fact-sheet field.
func (d RepoDocState) validateEvidence() error {
	if d.hasClaims() && len(d.Evidence) == 0 {
		return &ValidationError{Field: "evidence", Reason: "is empty although the document makes claims — every claim must cite fact-sheet fields"}
	}
	for index, item := range d.Evidence {
		if err := validateEvidenceItem(index, item); err != nil {
			return err
		}
	}
	return nil
}

func (d RepoDocState) hasClaims() bool {
	return d.Purpose != "" || len(d.Responsibilities) > 0 || len(d.Invariants) > 0
}

func validateEvidenceItem(index int, item DocEvidence) error {
	if item.Claim == "" {
		return &ValidationError{Field: fmt.Sprintf("evidence[%d].claim", index), Reason: "is required and empty"}
	}
	if len(item.Fields) == 0 {
		return &ValidationError{Field: fmt.Sprintf("evidence[%d].fields", index), Reason: "must cite at least one fact-sheet field"}
	}
	for _, field := range item.Fields {
		if !isFactField(field) {
			return &ValidationError{Field: fmt.Sprintf("evidence[%d].fields", index),
				Reason: fmt.Sprintf("%q does not start with a fact-sheet field (%s)", field, strings.Join(FactFieldRoots(), ", "))}
		}
	}
	return nil
}

// isFactField accepts `root` or `root:value` for a known root.
func isFactField(field string) bool {
	root, _, _ := strings.Cut(field, ":")
	for _, known := range FactFieldRoots() {
		if root == known {
			return true
		}
	}
	return false
}

func validateInvariants(invariants []DocInvariant) error {
	for index, invariant := range invariants {
		if invariant.Statement == "" {
			return &ValidationError{Field: fmt.Sprintf("invariants[%d].statement", index), Reason: "is required and empty"}
		}
		if len(invariant.Evidence) == 0 {
			return &ValidationError{Field: fmt.Sprintf("invariants[%d].evidence", index), Reason: "is required — an invariant with no evidence is a guess"}
		}
	}
	return nil
}

func validateFilesRead(reads []DocFileRead) error {
	for index, read := range reads {
		field := fmt.Sprintf("filesRead[%d]", index)
		if read.Path == "" || read.Reason == "" {
			return &ValidationError{Field: field, Reason: "needs a path and the reason the fact sheet was not enough"}
		}
		if read.StartLine < 1 || read.EndLine < read.StartLine {
			return &ValidationError{Field: field, Reason: "needs a line range with 1 <= startLine <= endLine"}
		}
	}
	return nil
}
