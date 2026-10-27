package policy

// Pre-ship checks: project-declared commands that must exit 0 before a
// human is asked to approve shipping (closing the gap PR #18 exposed —
// every agent's tests passed and the PR was approved, but CI's own
// changed-line coverage gate still rejected it, because nothing in the
// pipeline had run the check CI runs).
//
// Decoded as a node walk, like decode.go's Condition, rather than a plain
// struct unmarshal: an unknown key inside one entry must be named, not
// silently dropped, which only a walk over the raw document achieves.

import (
	"fmt"
	"time"

	"go.yaml.in/yaml/v4"
)

// defaultPreShipCheckTimeout bounds a check whose entry sets none.
const defaultPreShipCheckTimeout = 10 * time.Minute

// PreShipCheck is one command a run must pass before confirm-ship.
type PreShipCheck struct {
	Name    string
	Command string
	Timeout time.Duration
}

// keptPreShipCheckKeys are the only keys one preShipChecks entry may set.
var keptPreShipCheckKeys = map[string]bool{"name": true, "command": true, "timeout": true}

// decodePreShipChecksKey finds and decodes the top-level preShipChecks key
// from the raw document. An absent key decodes to no checks.
func decodePreShipChecksKey(raw []byte) ([]PreShipCheck, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return nil, err
	}
	node := mappingValue(document, "preShipChecks")
	if node == nil {
		return nil, nil
	}
	return decodePreShipChecks(node)
}

// mappingValue returns the value node for a key at the document's top
// level, or nil when the document is empty or the key is absent.
func mappingValue(document yaml.Node, key string) *yaml.Node {
	if len(document.Content) == 0 {
		return nil
	}
	mapping := document.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1]
		}
	}
	return nil
}

func decodePreShipChecks(node *yaml.Node) ([]PreShipCheck, error) {
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("preShipChecks must be a list, got %s", nodeKind(node))
	}
	checks := make([]PreShipCheck, 0, len(node.Content))
	for position, item := range node.Content {
		check, err := decodePreShipCheck(item, position)
		if err != nil {
			return nil, err
		}
		checks = append(checks, check)
	}
	return checks, nil
}

func decodePreShipCheck(node *yaml.Node, position int) (PreShipCheck, error) {
	if node.Kind != yaml.MappingNode {
		return PreShipCheck{}, fmt.Errorf("preShipChecks entry at position %d must be a mapping, got %s", position, nodeKind(node))
	}
	raw := rawEntryFields(node)
	label := preShipCheckLabel(raw, position)
	if err := rejectUnknownPreShipCheckKeys(node, label); err != nil {
		return PreShipCheck{}, err
	}
	if raw["name"] == "" {
		return PreShipCheck{}, fmt.Errorf("preShipChecks entry %s has no name", label)
	}
	if raw["command"] == "" {
		return PreShipCheck{}, fmt.Errorf("preShipChecks entry %s has no command", label)
	}
	timeout, err := preShipCheckTimeout(raw["timeout"])
	if err != nil {
		return PreShipCheck{}, fmt.Errorf("preShipChecks entry %s: %w", label, err)
	}
	return PreShipCheck{Name: raw["name"], Command: raw["command"], Timeout: timeout}, nil
}

// rawEntryFields reads one entry's keys as plain text, before any
// validation, so a rejection can still name the entry by whatever it does
// have.
func rawEntryFields(node *yaml.Node) map[string]string {
	fields := make(map[string]string, len(node.Content)/2)
	for index := 0; index+1 < len(node.Content); index += 2 {
		fields[node.Content[index].Value] = node.Content[index+1].Value
	}
	return fields
}

func rejectUnknownPreShipCheckKeys(node *yaml.Node, label string) error {
	for index := 0; index < len(node.Content); index += 2 {
		key := node.Content[index].Value
		if !keptPreShipCheckKeys[key] {
			return fmt.Errorf("preShipChecks entry %s: unknown key %q", label, key)
		}
	}
	return nil
}

// preShipCheckLabel names an entry for an error: by its name, else its
// command, else its position — whichever the entry actually has, since an
// entry rejected for having no name cannot be named by one.
func preShipCheckLabel(raw map[string]string, position int) string {
	if raw["name"] != "" {
		return fmt.Sprintf("%q", raw["name"])
	}
	if raw["command"] != "" {
		return fmt.Sprintf("with command %q", raw["command"])
	}
	return fmt.Sprintf("at position %d", position)
}

func preShipCheckTimeout(text string) (time.Duration, error) {
	if text == "" {
		return defaultPreShipCheckTimeout, nil
	}
	timeout, err := time.ParseDuration(text)
	if err != nil {
		return 0, fmt.Errorf("timeout %q is not a valid duration", text)
	}
	return timeout, nil
}
