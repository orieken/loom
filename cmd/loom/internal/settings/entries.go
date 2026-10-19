package settings

import (
	"bytes"
	"encoding/json"
)

const commandType = "command"

// This file holds the whole entry shape (UNVERIFIED, see the package
// comment). A corrected shape is a change here and nowhere else.

type commandEntry struct {
	Hooks []commandRef `json:"hooks"`
}

type commandRef struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// newEntry renders the entry for one command.
func newEntry(command string) json.RawMessage {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	// Encoding a struct of two strings cannot fail.
	_ = encoder.Encode(commandEntry{Hooks: []commandRef{{Type: commandType, Command: command}}})
	return bytes.TrimSpace(buffer.Bytes())
}

// commandsOf lists the commands in one array element. An element of any
// other shape has none, so it is never taken for ours.
func commandsOf(element json.RawMessage) []string {
	var entry commandEntry
	if err := json.Unmarshal(element, &entry); err != nil {
		return nil
	}
	commands := make([]string, len(entry.Hooks))
	for index, ref := range entry.Hooks {
		commands[index] = ref.Command
	}
	return commands
}

// decodeArray splits a JSON array into its raw elements. An absent value is
// an empty array. Any other kind of value reports false.
func decodeArray(raw json.RawMessage) ([]json.RawMessage, bool) {
	if raw == nil {
		return nil, true
	}
	var elements []json.RawMessage
	if err := json.Unmarshal(raw, &elements); err != nil || elements == nil {
		return nil, false
	}
	return elements, true
}

// joinArray renders raw elements as one array, element bytes unchanged.
func joinArray(elements []json.RawMessage) json.RawMessage {
	parts := make([][]byte, len(elements))
	for index, element := range elements {
		parts[index] = element
	}
	return append(append([]byte{'['}, bytes.Join(parts, []byte{','})...), ']')
}

func containsCommand(elements []json.RawMessage, command string) bool {
	for _, element := range elements {
		for _, existing := range commandsOf(element) {
			if existing == command {
				return true
			}
		}
	}
	return false
}
