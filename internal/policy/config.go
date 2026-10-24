package policy

// The schema of .claude/delivery-policy.yaml after ADR-010 (decision 4B).
//
// Automation lives in one place: .claude/policies/*.policy.yaml, evaluated
// by `loom run`, which records every decision. This file keeps only what is
// not automation — the kill switch, the markdown pipeline's bound on
// structural retries, and the two /orchestrate knobs until that path is
// retired (review item D-7).
//
// The keys that used to switch on auto-proceed are rejected by name rather
// than ignored. Ignoring them was the defect: a project that set
// `mode: policy-driven` believed it was configuring something.

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"go.yaml.in/yaml/v4"
)

// DefaultMaxContractRetries bounds the markdown pipeline's structural retry
// loop when the file does not set one.
const DefaultMaxContractRetries = 3

// orchestrationRuntime is the one value orchestrationMode has ever had.
const orchestrationRuntime = "runtime"

// ErrInvalidConfig marks a delivery-policy.yaml that does not match the schema.
var ErrInvalidConfig = errors.New("invalid delivery policy")

// Config is delivery-policy.yaml as decoded. A pointer is "not set", which
// is different from false: only an explicit false turns anything off.
type Config struct {
	PoliciesEnabled       *bool  `yaml:"policiesEnabled"`
	MaxContractRetries    int    `yaml:"maxContractRetries"`
	WorkflowAuditsEnabled *bool  `yaml:"workflowAuditsEnabled"`
	OrchestrationMode     string `yaml:"orchestrationMode"`
}

// keptKeys are the only top-level keys the schema allows.
var keptKeys = map[string]bool{
	"policiesEnabled": true, "maxContractRetries": true,
	"workflowAuditsEnabled": true, "orchestrationMode": true,
}

// removedKeys are the pre-ADR-010 keys, rejected with a pointer to where
// automation went. `version` versioned that schema; `maxDiffLines`,
// `requireGreenTests` and `autoProceed*` only ever fed auto-proceed.
var removedKeys = map[string]bool{
	"version": true, "mode": true, "defaults": true, "gates": true, "stages": true,
	"escalations": true, "maxDiffLines": true, "requireGreenTests": true,
}

// PoliciesEvaluated reports whether `loom run` should evaluate policies.
func (c Config) PoliciesEvaluated() bool {
	return c.PoliciesEnabled == nil || *c.PoliciesEnabled
}

// ContractRetries is the markdown pipeline's structural retry bound.
func (c Config) ContractRetries() int {
	if c.MaxContractRetries == 0 {
		return DefaultMaxContractRetries
	}
	return c.MaxContractRetries
}

// LoadConfig reads and strictly validates delivery-policy.yaml. A missing
// or empty file decodes to the defaults — the normal case for a project
// that never configured anything.
func LoadConfig(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("%w: %s: %v", ErrInvalidConfig, path, err)
	}
	if err := checkKeys(raw, path); err != nil {
		return Config{}, err
	}
	config, err := decodeConfig(raw)
	if err != nil {
		return Config{}, fmt.Errorf("%w: %s: %v", ErrInvalidConfig, path, err)
	}
	return config, checkValues(config, path)
}

// decodeConfig is the one decoder both readers share. It ignores keys it
// does not know; LoadConfig rejects them before calling it.
func decodeConfig(raw []byte) (Config, error) {
	var config Config
	err := yaml.Unmarshal(raw, &config)
	return config, err
}

// checkKeys names every removed and unknown key in one error, so a file can
// be fixed in one edit.
func checkKeys(raw []byte, path string) error {
	keys, err := topLevelKeys(raw)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrInvalidConfig, path, err)
	}
	removed, unknown := classifyKeys(keys)
	if len(removed) == 0 && len(unknown) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrInvalidConfig, describeKeys(path, removed, unknown))
}

// topLevelKeys reads the key names from the document's node tree, so nothing
// untyped is decoded. An empty file has no keys.
func topLevelKeys(raw []byte) ([]string, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return nil, err
	}
	if len(document.Content) == 0 {
		return nil, nil
	}
	mapping := document.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return nil, errors.New("the file must be a YAML mapping")
	}
	keys := make([]string, 0, len(mapping.Content)/2)
	for index := 0; index < len(mapping.Content); index += 2 {
		keys = append(keys, mapping.Content[index].Value)
	}
	return keys, nil
}

func classifyKeys(keys []string) ([]string, []string) {
	var removed, unknown []string
	for _, key := range keys {
		switch {
		case keptKeys[key]:
		case removedKeys[key] || strings.HasPrefix(key, "autoProceed"):
			removed = append(removed, key)
		default:
			unknown = append(unknown, key)
		}
	}
	sort.Strings(removed)
	sort.Strings(unknown)
	return removed, unknown
}

func describeKeys(path string, removed, unknown []string) string {
	var problems []string
	if len(removed) > 0 {
		problems = append(problems, fmt.Sprintf("%s sets %s, removed by ADR-010: automation now lives only in "+
			".claude/policies/*.policy.yaml, which loom run evaluates and records — delete them (see docs/MIGRATION.md)",
			path, quoteKeys(removed)))
	}
	for _, key := range unknown {
		problems = append(problems, fmt.Sprintf("%s: unknown key `%s`", path, key))
	}
	return strings.Join(problems, "; ")
}

func quoteKeys(keys []string) string {
	quoted := make([]string, len(keys))
	for index, key := range keys {
		quoted[index] = "`" + key + "`"
	}
	return strings.Join(quoted, ", ")
}

func checkValues(config Config, path string) error {
	if config.MaxContractRetries < 0 {
		return fmt.Errorf("%w: %s: maxContractRetries is %d, want 1 or more", ErrInvalidConfig, path, config.MaxContractRetries)
	}
	if config.OrchestrationMode != "" && config.OrchestrationMode != orchestrationRuntime {
		return fmt.Errorf("%w: %s: orchestrationMode is %q, the only value is %q",
			ErrInvalidConfig, path, config.OrchestrationMode, orchestrationRuntime)
	}
	return nil
}
