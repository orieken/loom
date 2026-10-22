// Package patternspec is the Pattern Spec: one typed, versioned file that
// describes one repeatable workflow (pattern mining F2).
//
// Mining (F3), proposals (F4) and rendering (F5) share this one shape rather
// than each parsing prose. The Go types here are the source of truth:
// cmd/gen-schemas renders shared/schemas/pattern-spec.schema.json from them,
// and the loader validates every spec against that same generated schema
// before it applies the contract rules a JSON Schema cannot express
// (shared/contracts/pattern-spec-contract.md).
//
// Version 1 is deliberately small: steps are a linear list, inputs use a
// closed set of types, and no spec references another.
package patternspec

// SchemaVersion is the pattern spec schema version this build reads.
const SchemaVersion = 1

// Spec is one pattern spec. Field names are lowerCamelCase, the same as
// skill frontmatter.
type Spec struct {
	SchemaVersion int               `json:"schemaVersion" jsonschema_description:"Pattern spec schema version. This build reads 1."`
	Name          string            `json:"name" jsonschema:"pattern=^[a-z][a-z0-9]*(-[a-z0-9]+)*$" jsonschema_description:"kebab-case name. It MUST match the file name without .pattern.yaml."`
	Version       string            `json:"version" jsonschema:"pattern=^[0-9]+\\.[0-9]+\\.[0-9]+$" jsonschema_description:"Semantic version of this pattern."`
	Intent        string            `json:"intent" jsonschema:"minLength=1" jsonschema_description:"One sentence: what the workflow achieves."`
	Owner         string            `json:"owner" jsonschema:"minLength=1" jsonschema_description:"The team or person who answers for this pattern."`
	Triggers      Triggers          `json:"triggers" jsonschema_description:"When an assistant should offer this pattern. Same keys as skill triggers."`
	Inputs        []Input           `json:"inputs,omitempty" jsonschema_description:"Values that vary between runs of the workflow."`
	Preconditions []string          `json:"preconditions,omitempty" jsonschema_description:"What MUST be true before the first step."`
	Steps         []Step            `json:"steps" jsonschema:"minItems=1" jsonschema_description:"The steps, in order. Version 1 is a linear list."`
	Verify        []VerifyReference `json:"verify,omitempty" jsonschema_description:"Existing skills or MCP tools that check the result."`
	Provenance    Provenance        `json:"provenance" jsonschema_description:"Where this spec came from, and its lifecycle status."`
}

// Triggers says when an assistant should offer the pattern.
type Triggers struct {
	Keywords       []string `json:"keywords" jsonschema_description:"Literal phrases that suggest this pattern."`
	IntentPatterns []string `json:"intentPatterns" jsonschema_description:"Descriptions of the user intent this pattern serves."`
}

// Input is one value that varies between runs of the workflow.
type Input struct {
	Name        string    `json:"name" jsonschema:"pattern=^[a-z][a-zA-Z0-9]*$" jsonschema_description:"lowerCamelCase name. Steps consume it by this name."`
	Type        InputType `json:"type" jsonschema_description:"One of the closed set of input types."`
	Description string    `json:"description" jsonschema:"minLength=1" jsonschema_description:"What the value is, for the person or model that supplies it."`
	Required    bool      `json:"required,omitempty" jsonschema_description:"true when a run cannot start without the value. A required input has no default."`
	Values      []string  `json:"values,omitempty" jsonschema_description:"The allowed values of an enum input. Only for type enum."`
	// Default holds a value whose Go type follows Type: a string, a number,
	// a boolean or a list of strings. It is the one heterogeneous field, a
	// sum type Go does not have; the inputDefaults rule narrows it.
	Default any `json:"default,omitempty" jsonschema_description:"The value a run uses when none is supplied. It MUST match the type."`
}

// IsEnum reports whether the input takes one of a fixed list of values.
func (input Input) IsEnum() bool { return input.Type == InputEnum }

// HasDefault reports whether the input declares a default value.
func (input Input) HasDefault() bool { return input.Default != nil }

// Step is one step of the workflow.
type Step struct {
	ID          string      `json:"id" jsonschema:"pattern=^[a-z][a-z0-9]*(-[a-z0-9]+)*$" jsonschema_description:"kebab-case id, unique in the spec."`
	Description string      `json:"description" jsonschema:"minLength=1" jsonschema_description:"What the step does."`
	Executor    Executor    `json:"executor" jsonschema_description:"Who runs the step: tool, skill or human."`
	Consumes    []string    `json:"consumes,omitempty" jsonschema:"pattern=^[a-z][a-zA-Z0-9]*$" jsonschema_description:"Inputs or earlier step outputs this step reads, by name."`
	Produces    []string    `json:"produces,omitempty" jsonschema:"pattern=^[a-z][a-zA-Z0-9]*$" jsonschema_description:"Names of the values this step makes for later steps."`
	Reason      HumanReason `json:"reason,omitempty" jsonschema_description:"Why a person runs the step. Required for a human step, and only for one."`
}

// IsHuman reports whether a person runs the step.
func (step Step) IsHuman() bool { return step.Executor == ExecutorHuman }

// HasReason reports whether the step gives a human step reason.
func (step Step) HasReason() bool { return step.Reason != "" }

// VerifyReference names an existing skill or MCP tool that checks the result.
type VerifyReference struct {
	Kind VerifyKind `json:"kind" jsonschema_description:"skill or tool."`
	Name string     `json:"name" jsonschema:"pattern=^[a-z][a-z0-9_-]*$" jsonschema_description:"The skill directory name or the registered MCP tool name."`
}

// Provenance is the Pattern Provenance of a spec.
type Provenance struct {
	Source   Source    `json:"source" jsonschema_description:"hand-written or mined."`
	Status   Status    `json:"status" jsonschema_description:"candidate, approved, forged or retired."`
	Evidence *Evidence `json:"evidence,omitempty" jsonschema_description:"Required when source is mined."`
}

// IsMined reports whether the spec came from the miner (F3).
func (provenance Provenance) IsMined() bool { return provenance.Source == SourceMined }

// Evidence is what the miner saw. It holds counts, dates and identifiers
// only, never prompt text, code or file paths (architecture guardrail #9).
type Evidence struct {
	ClusterID     string   `json:"clusterId" jsonschema:"pattern=^[A-Za-z0-9._-]+$" jsonschema_description:"The miner's cluster id."`
	InstanceCount int      `json:"instanceCount" jsonschema:"minimum=1" jsonschema_description:"How many times the workflow was seen."`
	Repos         []string `json:"repos" jsonschema:"minItems=1" jsonschema_description:"Identifiers of the repositories it was seen in."`
	FirstSeen     string   `json:"firstSeen" jsonschema:"format=date" jsonschema_description:"Date of the first instance, YYYY-MM-DD."`
	LastSeen      string   `json:"lastSeen" jsonschema:"format=date" jsonschema_description:"Date of the last instance, YYYY-MM-DD. Not before firstSeen."`
}
