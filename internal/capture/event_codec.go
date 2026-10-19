package capture

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Errors of the day-file line codec.
var (
	// ErrUnknownVersion marks a line whose "v" this reader does not know.
	// A reader skips and counts such a line.
	ErrUnknownVersion = errors.New("capture: unknown day-file version")
	// ErrLineTooLong marks an event that does not fit MaxLineBytes even
	// after every trimming step.
	ErrLineTooLong = errors.New("capture: event line exceeds the line cap")
)

const lineTerminator = '\n'

// trimSteps shrink an event in the order of architecture-notes.md
// decision 2: keys beyond the key cap, then long key names, then the whole
// argument section. The step that runs sets ArgsTruncated.
var trimSteps = []func(SessionEvent) SessionEvent{
	limitArgKeys,
	truncateKeyNames,
	dropArgShape,
}

// MarshalLine encodes an event as one day-file line, newline included. The
// line never exceeds MaxLineBytes. The event is dropped only as an error,
// never silently.
func MarshalLine(event SessionEvent) ([]byte, error) {
	line, err := encodeLine(event)
	for _, step := range trimSteps {
		if err != nil || len(line) <= MaxLineBytes {
			break
		}
		event = step(event)
		event.ArgsTruncated = true
		line, err = encodeLine(event)
	}
	return checkLineSize(line, err)
}

func checkLineSize(line []byte, err error) ([]byte, error) {
	if err != nil {
		return nil, fmt.Errorf("capture: encode event: %w", err)
	}
	if len(line) > MaxLineBytes {
		return nil, ErrLineTooLong
	}
	return line, nil
}

func encodeLine(event SessionEvent) ([]byte, error) {
	body, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	return append(body, lineTerminator), nil
}

func limitArgKeys(event SessionEvent) SessionEvent {
	return event.WithArgShape(headOf(event.ArgKeys, MaxArgKeys), headOf(event.ArgTypes, MaxArgKeys))
}

func truncateKeyNames(event SessionEvent) SessionEvent {
	keys := make([]string, len(event.ArgKeys))
	for i, key := range event.ArgKeys {
		keys[i] = key[:min(len(key), MaxNameBytes)]
	}
	return event.WithArgShape(keys, event.ArgTypes)
}

func dropArgShape(event SessionEvent) SessionEvent {
	return event.WithArgShape(nil, nil)
}

func headOf(items []string, limit int) []string {
	return items[:min(len(items), limit)]
}

// DecodeLine decodes one day-file line. A line with another version returns
// ErrUnknownVersion. Unknown fields are ignored.
func DecodeLine(line []byte) (SessionEvent, error) {
	var event SessionEvent
	if err := json.Unmarshal(line, &event); err != nil {
		return SessionEvent{}, fmt.Errorf("capture: decode event: %w", err)
	}
	if event.V != EventVersion {
		return SessionEvent{}, ErrUnknownVersion
	}
	event.At = event.At.UTC()
	return event, nil
}
