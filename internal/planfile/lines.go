package planfile

// Line reporting for plan-file errors (roadmap L3.27's done-when: "an
// invalid plan is rejected with the line that is wrong").
//
// A validation message that names a stage but not a line makes the reader
// search a file they have just been told is broken. The YAML node tree
// carries positions, so the loader keeps one and reports from it.

import (
	"fmt"
	"sort"

	"github.com/orieken/loom/internal/orchestrator"
	yaml "go.yaml.in/yaml/v4"
)

// lineIndex maps a top-level key, and each item under it, to a source line.
type lineIndex struct {
	keys  map[string]int
	items map[string][]int
}

func newLineIndex(document *yaml.Node) lineIndex {
	index := lineIndex{keys: map[string]int{}, items: map[string][]int{}}
	mapping := mappingOf(document)
	if mapping == nil {
		return index
	}
	for position := 0; position+1 < len(mapping.Content); position += 2 {
		key, value := mapping.Content[position], mapping.Content[position+1]
		index.keys[key.Value] = key.Line
		index.items[key.Value] = itemLines(value)
	}
	return index
}

func mappingOf(document *yaml.Node) *yaml.Node {
	if document == nil {
		return nil
	}
	if document.Kind == yaml.DocumentNode && len(document.Content) > 0 {
		document = document.Content[0]
	}
	if document.Kind != yaml.MappingNode {
		return nil
	}
	return document
}

func itemLines(value *yaml.Node) []int {
	if value == nil || value.Kind != yaml.SequenceNode {
		return nil
	}
	lines := make([]int, 0, len(value.Content))
	for _, item := range value.Content {
		lines = append(lines, item.Line)
	}
	return lines
}

// errorAt reports a problem with a top-level key.
func (index lineIndex) errorAt(key, path, message string) error {
	return locatedError(path, index.keys[key], message)
}

// errorAtItem reports a problem with one entry of a sequence, falling back
// to the key's own line when the entry cannot be located.
func (index lineIndex) errorAtItem(key string, position int, path, message string) error {
	lines := index.items[key]
	if position >= 0 && position < len(lines) {
		return locatedError(path, lines[position], message)
	}
	return locatedError(path, index.keys[key], message)
}

func locatedError(path string, line int, message string) error {
	if line <= 0 {
		return fmt.Errorf("%s: %s", path, message)
	}
	return fmt.Errorf("%s:%d: %s", path, line, message)
}

func stageIDs(catalogue map[string]orchestrator.Stage) []string {
	ids := make([]string, 0, len(catalogue))
	for id := range catalogue {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func loopIDs(loops map[string]orchestrator.Loop) []string {
	ids := make([]string, 0, len(loops))
	for id := range loops {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func quotedKeys(ids []string) string {
	quoted := make([]string, 0, len(ids))
	for _, id := range ids {
		quoted = append(quoted, fmt.Sprintf("%q", id))
	}
	return joinWithCommas(quoted)
}

func joinWithCommas(values []string) string {
	result := ""
	for index, value := range values {
		if index > 0 {
			result += ", "
		}
		result += value
	}
	return result
}
