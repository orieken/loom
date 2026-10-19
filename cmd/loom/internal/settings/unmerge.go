package settings

import (
	"encoding/json"
	"fmt"
	"os"
)

// Unmerge removes the commands the selector matches and nothing else. A
// user hook, and any hooks value that is not an array, stays as it was. An
// event array that the removal empties is dropped, and so is the hooks key
// when it ends empty. When the file then holds no key, Unmerge deletes it if
// loom created it, and keeps an empty object if the user had the file. When
// nothing matches, Unmerge writes nothing.
func Unmerge(path string, selector Selector, origin FileOrigin) (UnmergeResult, error) {
	doc, err := load(path)
	if err != nil || !doc.exists {
		return UnmergeResult{}, err
	}
	removed := doc.removeAll(selector)
	if removed == 0 {
		return UnmergeResult{}, nil
	}
	if len(doc.hooks) == 0 {
		delete(doc.top, hooksKey)
	}
	if len(doc.top) == 0 && origin == CreatedByLoom {
		return UnmergeResult{Removed: removed, DeletedFile: true}, os.Remove(path)
	}
	return UnmergeResult{Removed: removed}, doc.save()
}

// CountEntries counts the commands the selector matches. It reads only.
func CountEntries(path string, selector Selector) (int, error) {
	doc, err := load(path)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, raw := range doc.hooks {
		elements, _ := decodeArray(raw)
		count += countMatches(elements, selector)
	}
	return count, nil
}

func countMatches(elements []json.RawMessage, selector Selector) int {
	count := 0
	for _, element := range elements {
		_, matched := splitCommands(element, selector)
		count += matched
	}
	return count
}

func (d *document) removeAll(selector Selector) int {
	removed := 0
	for event, raw := range d.hooks {
		elements, isArray := decodeArray(raw)
		if !isArray {
			continue
		}
		kept, count := filterElements(elements, selector)
		removed += count
		d.apply(event, kept, count)
	}
	return removed
}

// apply stores the kept elements, or drops the event when none are left.
func (d *document) apply(event string, kept []json.RawMessage, removed int) {
	if removed == 0 {
		return
	}
	if len(kept) == 0 {
		delete(d.hooks, event)
		return
	}
	d.hooks[event] = joinArray(kept)
}

// filterElements drops the matching commands. An element that holds only
// matching commands goes away whole. An element that mixes ours and the
// user's keeps the user's.
func filterElements(elements []json.RawMessage, selector Selector) ([]json.RawMessage, int) {
	kept := make([]json.RawMessage, 0, len(elements))
	removed := 0
	for _, element := range elements {
		total, matched := splitCommands(element, selector)
		switch {
		case matched == 0:
			kept = append(kept, element)
		case matched < total:
			kept = append(kept, withoutMatches(element, selector))
			removed += matched
		default:
			removed += matched
		}
	}
	return kept, removed
}

// splitCommands returns how many commands an element holds and how many of
// them the selector matches.
func splitCommands(element json.RawMessage, selector Selector) (total, matched int) {
	for _, command := range commandsOf(element) {
		total++
		if selector.Matches(command) {
			matched++
		}
	}
	return total, matched
}

// withoutMatches removes the matching commands from a mixed element and
// keeps every other key and command as raw JSON.
func withoutMatches(element json.RawMessage, selector Selector) json.RawMessage {
	var fields map[string]json.RawMessage
	var refs []json.RawMessage
	if json.Unmarshal(element, &fields) != nil || json.Unmarshal(fields[hooksKey], &refs) != nil {
		return element
	}
	remaining := make([]json.RawMessage, 0, len(refs))
	for _, ref := range refs {
		if !isMatchingRef(ref, selector) {
			remaining = append(remaining, ref)
		}
	}
	fields[hooksKey] = joinArray(remaining)
	rebuilt, err := marshalNoEscape(fields, "")
	if err != nil {
		return element
	}
	return rebuilt
}

func isMatchingRef(ref json.RawMessage, selector Selector) bool {
	var parsed commandRef
	return json.Unmarshal(ref, &parsed) == nil && selector.Matches(parsed.Command)
}

// save writes the document atomically.
func (d *document) save() error {
	content, err := d.encode()
	if err != nil {
		return err
	}
	if err := writeAtomic(d.path, content); err != nil {
		return fmt.Errorf("write %s: %w", d.path, err)
	}
	return nil
}
