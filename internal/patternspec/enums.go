package patternspec

// Executor is the Executor Tag: who runs a step. It is mandatory on every
// step, and F5 renders each step by it.
type Executor string

// The executors a step may name.
const (
	ExecutorTool  Executor = "tool"
	ExecutorSkill Executor = "skill"
	ExecutorHuman Executor = "human"
)

// Executors lists every executor, in documentation order.
func Executors() []Executor { return []Executor{ExecutorTool, ExecutorSkill, ExecutorHuman} }

// HumanReason says why a step stays with a person (Human Step Reason).
type HumanReason string

// The reasons a human step may give.
const (
	ReasonApproval           HumanReason = "approval"
	ReasonIrreversible       HumanReason = "irreversible"
	ReasonJudgmentOutOfScope HumanReason = "judgment-out-of-scope"
)

// HumanReasons lists every human step reason, in documentation order.
func HumanReasons() []HumanReason {
	return []HumanReason{ReasonApproval, ReasonIrreversible, ReasonJudgmentOutOfScope}
}

// InputType is the closed set of types an input may have.
type InputType string

// The input types.
const (
	InputString  InputType = "string"
	InputNumber  InputType = "number"
	InputBoolean InputType = "boolean"
	InputPath    InputType = "path"
	InputEnum    InputType = "enum"
	InputList    InputType = "list"
)

// InputTypes lists every input type, in documentation order.
func InputTypes() []InputType {
	return []InputType{InputString, InputNumber, InputBoolean, InputPath, InputEnum, InputList}
}

// Source says where a pattern spec came from.
type Source string

// The sources of a pattern spec.
const (
	SourceHandWritten Source = "hand-written"
	SourceMined       Source = "mined"
)

// Sources lists every source, in documentation order.
func Sources() []Source { return []Source{SourceHandWritten, SourceMined} }

// Status is the Pattern Status: where a spec is in its lifecycle.
type Status string

// The pattern statuses.
const (
	StatusCandidate Status = "candidate"
	StatusApproved  Status = "approved"
	StatusForged    Status = "forged"
	StatusRetired   Status = "retired"
)

// Statuses lists every status, in lifecycle order.
func Statuses() []Status {
	return []Status{StatusCandidate, StatusApproved, StatusForged, StatusRetired}
}

// IsPromotable reports whether a spec with this status may live in
// shared/patterns/. Only a human-approved spec may.
func (status Status) IsPromotable() bool { return status != StatusCandidate }

// VerifyKind is what a verify reference names: a skill or an MCP tool.
type VerifyKind string

// The kinds of verify reference.
const (
	VerifySkill VerifyKind = "skill"
	VerifyTool  VerifyKind = "tool"
)

// VerifyKinds lists every verify kind, in documentation order.
func VerifyKinds() []VerifyKind { return []VerifyKind{VerifySkill, VerifyTool} }
