package settings

import (
	"encoding/json"
	"fmt"
)

// Merge adds the hooks to the file, beside what is there. A hook whose
// command is already registered under its event is not added again, so a
// second Merge changes nothing and writes nothing. When the file has a shape
// the adapter does not understand, Merge returns a *NotMergeableError and
// writes nothing.
func Merge(path string, hooks []Hook) (MergeResult, error) {
	doc, err := load(path)
	if err != nil {
		return MergeResult{}, withHooks(err, hooks)
	}
	added, err := doc.addAll(hooks)
	if err != nil {
		return MergeResult{}, err
	}
	result := MergeResult{Created: !doc.exists, Added: added}
	if added == 0 {
		return result, nil
	}
	return result, doc.save()
}

// CheckMergeable reports the error that Merge would return, and writes
// nothing. The installer calls it before it changes anything.
func CheckMergeable(path string, hooks []Hook) error {
	doc, err := load(path)
	if err != nil {
		return withHooks(err, hooks)
	}
	_, err = doc.addAll(hooks)
	return err
}

// withHooks attaches the manual instructions to a refusal.
func withHooks(err error, hooks []Hook) error {
	if refusal, ok := err.(*NotMergeableError); ok {
		refusal.Hooks = hooks
	}
	return err
}

func (d *document) addAll(hooks []Hook) (int, error) {
	added := 0
	for _, hook := range hooks {
		isAdded, err := d.add(hook)
		if err != nil {
			return 0, withHooks(err, hooks)
		}
		if isAdded {
			added++
		}
	}
	return added, nil
}

func (d *document) add(hook Hook) (bool, error) {
	elements, isArray := decodeArray(d.hooks[hook.Event])
	if !isArray {
		reason := fmt.Sprintf("hooks.%s is not an array, and loom does not rewrite it", hook.Event)
		return false, &NotMergeableError{Path: d.path, Reason: reason}
	}
	if containsCommand(elements, hook.Command) {
		return false, nil
	}
	d.hooks[hook.Event] = joinArray(append(elements, json.RawMessage(newEntry(hook.Command))))
	return true, nil
}
