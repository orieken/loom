package policy

// The policy document, and how a YAML file becomes one.

import (
	"fmt"

	"go.yaml.in/yaml/v4"
)

// FileSuffix is the extension every policy file must carry.
const FileSuffix = ".policy.yaml"

// DefaultDir is where a project keeps its policies, relative to the project
// root.
const DefaultDir = ".claude/policies"

// ActionType is what a policy asks for when its condition holds.
type ActionType string

// The action types policy-schema.md declares.
const (
	ActionAutoApprove  ActionType = "auto-approve"
	ActionAutoReject   ActionType = "auto-reject"
	ActionRequireHuman ActionType = "require-human"
	ActionEscalate     ActionType = "escalate"
)

// Action is what happens when a policy matches.
type Action struct {
	Type       ActionType `yaml:"type"`
	Reason     string     `yaml:"reason"`
	EscalateTo string     `yaml:"escalateTo,omitempty"`
}

// Policy is one loaded, validated policy file.
type Policy struct {
	Name        string    `yaml:"name"`
	Version     string    `yaml:"version"`
	Description string    `yaml:"description,omitempty"`
	Enabled     *bool     `yaml:"enabled,omitempty"`
	Matcher     Matcher   `yaml:"matcher"`
	Condition   Condition `yaml:"condition"`
	Action      Action    `yaml:"action"`
	// Source is the file it came from, so an error at evaluation time can
	// name something a human can open.
	Source string `yaml:"-"`
}

// IsEnabled defaults to true, matching the schema: `enabled` absent means
// the policy is live.
func (p Policy) IsEnabled() bool { return p.Enabled == nil || *p.Enabled }

// Matcher names the gates a policy watches.
type Matcher struct {
	Gate  GateID   `yaml:"gate,omitempty"`
	Gates []GateID `yaml:"gates,omitempty"`
}

// Watched returns every gate this matcher covers.
func (m Matcher) Watched() []GateID {
	if m.Gate != "" {
		return append([]GateID{m.Gate}, m.Gates...)
	}
	return m.Gates
}

// Parse decodes and validates one policy document.
//
// Decoding is strict: unknown fields are rejected, and duplicate mapping
// keys are an error rather than a last-one-wins silent overwrite. Both
// matter more here than in most parsers — a policy is an authorization
// document, and the shipped auto-approve-refactor example had a duplicate
// `filePaths` key that no parser had ever seen, so half of what it claimed
// to check was never going to be checked.
func Parse(source string, raw []byte) (Policy, error) {
	var document policyDocument
	decoder := yaml.NewDecoder(newReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&document); err != nil {
		return Policy{}, fmt.Errorf("%s: %w", source, err)
	}
	policy, err := document.toPolicy(source)
	if err != nil {
		return Policy{}, fmt.Errorf("%s: %w", source, err)
	}
	return policy, nil
}

// policyDocument is the wire shape: the condition arrives as a raw node so
// it can be walked into typed checks rather than into a map of any.
type policyDocument struct {
	Name        string    `yaml:"name"`
	Version     string    `yaml:"version"`
	Description string    `yaml:"description,omitempty"`
	Enabled     *bool     `yaml:"enabled,omitempty"`
	Matcher     Matcher   `yaml:"matcher"`
	Condition   yaml.Node `yaml:"condition"`
	Action      Action    `yaml:"action"`
}

func (d policyDocument) toPolicy(source string) (Policy, error) {
	condition, err := decodeCondition(&d.Condition)
	if err != nil {
		return Policy{}, err
	}
	policy := Policy{
		Name: d.Name, Version: d.Version, Description: d.Description, Enabled: d.Enabled,
		Matcher: d.Matcher, Condition: condition, Action: d.Action, Source: source,
	}
	return policy, policy.validate()
}

func (p Policy) validate() error {
	if p.Name == "" {
		return fmt.Errorf("policy has no name — every policy needs a unique kebab-case name")
	}
	if err := p.validateGates(); err != nil {
		return err
	}
	if p.Condition.IsEmpty() {
		return fmt.Errorf("policy %q has an empty condition — it would fire on every gate it watches", p.Name)
	}
	if err := p.validateSelfReportedFields(); err != nil {
		return err
	}
	return p.validateAction()
}

// selfReported names every fact the reviewed party supplies about itself.
// Each entry is a field a policy may READ but must not use to OPEN a gate.
var selfReported = map[Field]string{
	FieldReviewBehaviorChange: "the code-reviewer's own assertion that nothing behavioural changed",
}

// validateSelfReportedFields refuses a self-reported fact in a policy that
// can open a gate.
//
// codeReviewer.verdict is also self-reported and is deliberately NOT here:
// the verdict IS the review's output, the thing a gate exists to act on,
// and forbidding it would leave nothing to write a policy about.
// behaviorChange is different in kind — it is the reviewer grading the
// significance of its own work, and an auto-approve policy reading it lets
// a mistaken or compromised reviewer approve itself by asserting the change
// was trivial. Restricting it costs one direction of expressiveness and
// removes that path.
//
// Checked at LOAD, so someone who writes it finds out before a run rather
// than from a decision that silently never fires.
func (p Policy) validateSelfReportedFields() error {
	if p.Action.Type != ActionAutoApprove {
		return nil
	}
	for _, field := range p.Condition.Fields() {
		if why, restricted := selfReported[field]; restricted {
			return fmt.Errorf("policy %q is auto-approve and tests %q, which is %s — "+
				"a self-reported fact may not open a gate; use it in a require-human, "+
				"auto-reject or escalate policy instead (shared/policies/policy-schema.md)",
				p.Name, field, why)
		}
	}
	return nil
}

func (p Policy) validateGates() error {
	watched := p.Matcher.Watched()
	if len(watched) == 0 {
		return fmt.Errorf("policy %q watches no gate — set matcher.gate or matcher.gates", p.Name)
	}
	for _, gate := range watched {
		if err := CheckGate(gate); err != nil {
			return fmt.Errorf("policy %q: %w", p.Name, err)
		}
	}
	return nil
}

func (p Policy) validateAction() error {
	switch p.Action.Type {
	case ActionAutoApprove, ActionAutoReject, ActionRequireHuman, ActionEscalate:
	default:
		return fmt.Errorf("policy %q has unknown action type %q", p.Name, p.Action.Type)
	}
	if p.Action.Type == ActionEscalate && p.Action.EscalateTo == "" {
		return fmt.Errorf("policy %q escalates but names no escalateTo target", p.Name)
	}
	if p.Action.Reason == "" {
		return fmt.Errorf("policy %q has no reason — a decision nobody can explain is not auditable", p.Name)
	}
	return nil
}
