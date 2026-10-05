// Package factsheet computes, per directory, the verifiable facts a model may
// build prose on (ADR-010): files, exported symbols, what the directory
// imports and what imports it, entrypoints and tests. Every field is derived
// in Go from the source and the repo graph; none comes from a model.
package factsheet

// SchemaDir and SchemaFile locate the generated JSON Schema, beside the
// graph's.
const (
	SchemaDir  = "shared/schemas/repograph"
	SchemaFile = "fact-sheet.schema.json"
)

// OutputDir is where sheets are written, relative to the repo root.
const OutputDir = ".loom/facts"

// FactSheet is the document written to .loom/facts/<dir>.json.
//
// Imports and Dependents hold only exact edges. An approximate edge is listed
// separately so no reader mistakes a heuristic for a fact (ADR-010).
type FactSheet struct {
	Path                  string   `json:"path" jsonschema:"description=Directory path or . for the repo root"`
	Files                 []string `json:"files" jsonschema:"description=Files directly in the directory with names relative to it"`
	Exports               []string `json:"exports" jsonschema:"description=Exported top-level symbols of the non-test source files"`
	Imports               []string `json:"imports" jsonschema:"description=Directories this one imports through exact edges"`
	ApproximateImports    []string `json:"approximate_imports"`
	Dependents            []string `json:"dependents" jsonschema:"description=Directories that import this one through exact edges"`
	ApproximateDependents []string `json:"approximate_dependents"`
	Entrypoints           []string `json:"entrypoints" jsonschema:"description=Files that start a program with names relative to the directory"`
	Tests                 []string `json:"tests"`
	Children              []string `json:"children" jsonschema:"description=Immediate subdirectories"`
	InputHash             string   `json:"input_hash" jsonschema:"description=sha256 over the file digests and these facts and the children's hashes"`
}

// fileFacts is what one source file contributes to its directory's sheet.
type fileFacts struct {
	exports    []string
	entrypoint bool
}
