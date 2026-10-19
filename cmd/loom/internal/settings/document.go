package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// maxSettingsBytes caps the file the adapter reads. A settings file is a
// few kilobytes; a larger file is not one.
const maxSettingsBytes = 1 << 20

const hooksKey = "hooks"

// document is the parsed file. Every value stays raw JSON.
type document struct {
	path   string
	exists bool
	top    map[string]json.RawMessage
	hooks  map[string]json.RawMessage
}

func emptyDocument(path string, exists bool) *document {
	return &document{path: path, exists: exists, top: map[string]json.RawMessage{}, hooks: map[string]json.RawMessage{}}
}

// load reads and parses the file. A missing file is an empty document. A
// file that is not a JSON object, or whose hooks value is not an object, is
// refused.
func load(path string) (*document, error) {
	content, err := readCapped(path)
	if errors.Is(err, os.ErrNotExist) {
		return emptyDocument(path, false), nil
	}
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(content)) == 0 {
		return emptyDocument(path, true), nil
	}
	doc := &document{path: path, exists: true}
	if err := doc.parse(content); err != nil {
		return nil, err
	}
	return doc, nil
}

func (d *document) parse(content []byte) error {
	if err := json.Unmarshal(content, &d.top); err != nil || d.top == nil {
		return &NotMergeableError{Path: d.path, Reason: "the file is not a JSON object"}
	}
	d.hooks = map[string]json.RawMessage{}
	raw, hasHooks := d.top[hooksKey]
	if !hasHooks {
		return nil
	}
	if err := json.Unmarshal(raw, &d.hooks); err != nil || d.hooks == nil {
		return &NotMergeableError{Path: d.path, Reason: `the "hooks" value is not an object`}
	}
	return nil
}

func readCapped(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	content, err := io.ReadAll(io.LimitReader(file, maxSettingsBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(content) > maxSettingsBytes {
		return nil, &NotMergeableError{Path: path, Reason: "the file is larger than 1 MB"}
	}
	return content, nil
}

// encode renders the document. Escaping of < > & is off, so a command such
// as `npm test && lint` keeps its bytes.
func (d *document) encode() ([]byte, error) {
	if len(d.hooks) == 0 {
		delete(d.top, hooksKey)
	} else {
		raw, err := marshalNoEscape(d.hooks, "")
		if err != nil {
			return nil, err
		}
		d.top[hooksKey] = raw
	}
	return marshalNoEscape(d.top, "  ")
}

// marshalNoEscape encodes a raw-JSON map with HTML escaping off. An empty
// indent gives compact output without the trailing newline.
func marshalNoEscape(value map[string]json.RawMessage, indent string) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", indent)
	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("encode settings: %w", err)
	}
	if indent == "" {
		return bytes.TrimSpace(buffer.Bytes()), nil
	}
	return buffer.Bytes(), nil
}
