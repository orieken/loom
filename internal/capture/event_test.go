package capture

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestSessionEventFieldListIsFrozen fails on any change to the v:1 field
// list, so a new field needs a deliberate edit here and a format decision
// (architecture-notes.md decisions 4 and 13).
func TestSessionEventFieldListIsFrozen(t *testing.T) {
	want := []string{
		"V int `json:\"v\"`",
		"ID string `json:\"id\"`",
		"At time.Time `json:\"at\"`",
		"Kind capture.EventKind `json:\"kind\"`",
		"Tool string `json:\"tool,omitempty\"`",
		"SessionRef string `json:\"sessionRef,omitempty\"`",
		"ArgKeys []string `json:\"argKeys,omitempty\"`",
		"ArgTypes []string `json:\"argTypes,omitempty\"`",
		"PathRef string `json:\"pathRef,omitempty\"`",
		"Path *string `json:\"path,omitempty\"`",
		"RangeStart int `json:\"rangeStart,omitempty\"`",
		"RangeEnd int `json:\"rangeEnd,omitempty\"`",
		"PreHash string `json:\"preHash,omitempty\"`",
		"PostHash string `json:\"postHash,omitempty\"`",
		"PromptClass capture.PromptClass `json:\"promptClass,omitempty\"`",
		"ArgsTruncated bool `json:\"argsTruncated,omitempty\"`",
	}
	eventType := reflect.TypeOf(SessionEvent{})
	got := make([]string, 0, eventType.NumField())
	for i := 0; i < eventType.NumField(); i++ {
		field := eventType.Field(i)
		got = append(got, field.Name+" "+field.Type.String()+" `"+string(field.Tag)+"`")
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("field list changed:\n got  %q\n want %q", got, want)
	}
}

func TestNewSessionEventStoresUTC(t *testing.T) {
	local := time.FixedZone("UTC+5", 5*3600)

	event := NewSessionEvent("id", time.Date(2026, 10, 19, 2, 0, 0, 0, local), EventPrompt)

	if event.At.Location() != time.UTC || event.At.Day() != 18 {
		t.Errorf("At = %v, want 2026-10-18 UTC", event.At)
	}
	if event.V != EventVersion {
		t.Errorf("V = %d, want %d", event.V, EventVersion)
	}
}

func TestSessionEventWithMethodsDoNotMutateTheReceiver(t *testing.T) {
	base := NewSessionEvent("id", baseTime, EventToolUse)

	changed := base.WithTool("Edit").WithSessionRef("s").WithPromptClass(PromptOther).
		WithArgShape([]string{"a"}, []string{"string"}).
		WithPath("ref", strPtr("a.go")).
		WithHunk(HunkFacts{RangeStart: 1, RangeEnd: 2, PreHash: "p", PostHash: "q"})

	if !reflect.DeepEqual(base, NewSessionEvent("id", baseTime, EventToolUse)) {
		t.Errorf("receiver changed: %+v", base)
	}
	want := SessionEvent{
		V: EventVersion, ID: "id", At: baseTime, Kind: EventToolUse,
		Tool: "Edit", SessionRef: "s", PromptClass: PromptOther,
		ArgKeys: []string{"a"}, ArgTypes: []string{"string"},
		PathRef: "ref", Path: strPtr("a.go"),
		RangeStart: 1, RangeEnd: 2, PreHash: "p", PostHash: "q",
	}
	if !reflect.DeepEqual(changed, want) {
		t.Errorf("copy lacks the new values:\n got  %+v\n want %+v", changed, want)
	}
}

func TestSessionEventCopiesSlicesAndPathSoCallersCannotChangeIt(t *testing.T) {
	keys := []string{"a"}
	display := "src/a.go"
	event := NewSessionEvent("id", baseTime, EventToolUse).
		WithArgShape(keys, []string{"string"}).
		WithPath("ref", &display)

	keys[0] = "mutated"
	display = "mutated"

	if event.ArgKeys[0] != "a" || *event.Path != "src/a.go" {
		t.Errorf("event changed through the caller: keys=%v path=%q", event.ArgKeys, *event.Path)
	}
}

func TestSessionEventWithPathNilClearsTheReadablePath(t *testing.T) {
	event := NewSessionEvent("id", baseTime, EventToolUse).WithPath("r", strPtr("a.go"))

	redacted := event.WithPath("r2", nil)

	if redacted.Path != nil {
		t.Errorf("Path = %q, want nil", *redacted.Path)
	}
}

func TestSessionEventHasHunk(t *testing.T) {
	tests := []struct {
		name       string
		start, end int
		want       bool
	}{
		{"range of several lines", 10, 14, true},
		{"one line", 3, 3, true},
		{"no range", 0, 0, false},
		{"end before start", 5, 4, false},
		{"start below one", 0, 3, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := SessionEvent{RangeStart: tt.start, RangeEnd: tt.end}
			if got := event.HasHunk(); got != tt.want {
				t.Errorf("HasHunk() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMarshalLineRoundTrips(t *testing.T) {
	event := NewSessionEvent("abc", baseTime, EventHunkObserved).
		WithTool("Edit").WithSessionRef("sref").
		WithArgShape([]string{"file_path"}, []string{"string"}).
		WithPath("pref", strPtr("src/a.go")).
		WithHunk(HunkFacts{RangeStart: 1, RangeEnd: 2, PreHash: "p", PostHash: "q"})

	line, err := MarshalLine(event)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeLine(line)

	if err != nil || !reflect.DeepEqual(decoded, event) {
		t.Errorf("round trip: err=%v\n got  %+v\n want %+v", err, decoded, event)
	}
	if !strings.HasSuffix(string(line), "\n") || !strings.Contains(string(line), `"v":1`) {
		t.Errorf("line = %q, want a v:1 line ending in a newline", line)
	}
}

func TestMarshalLineOmitsPathForARedactedPath(t *testing.T) {
	event := NewSessionEvent("abc", baseTime, EventToolUse).WithPath("pref", nil)

	line, _ := MarshalLine(event)

	if strings.Contains(string(line), `"path"`) {
		t.Errorf("line = %s, must hold no path field", line)
	}
}

func TestMarshalLineTrimsInTheDocumentedOrder(t *testing.T) {
	manyKeys := namedKeys(300, 8)
	longNames := namedKeys(MaxArgKeys, 400)
	tests := []struct {
		name         string
		event        SessionEvent
		wantKeyCount int
		wantMaxName  int
	}{
		{"keys beyond the cap are cut first", base().WithArgShape(manyKeys, typesFor(manyKeys)), MaxArgKeys, 8},
		{"then long key names are cut", base().WithArgShape(longNames, typesFor(longNames)), MaxArgKeys, MaxNameBytes},
		{"then the whole shape is dropped", base().WithArgShape(longNames, typesFor(longNames)).
			WithPath("r", strPtr(strings.Repeat("p", 3900))), 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded := marshalAndDecode(t, tt.event)
			if !decoded.ArgsTruncated || len(decoded.ArgKeys) != tt.wantKeyCount {
				t.Errorf("truncated=%v keys=%d, want true and %d", decoded.ArgsTruncated, len(decoded.ArgKeys), tt.wantKeyCount)
			}
			for _, key := range decoded.ArgKeys {
				if len(key) > tt.wantMaxName {
					t.Errorf("key length %d, want at most %d", len(key), tt.wantMaxName)
				}
			}
		})
	}
}

// marshalAndDecode checks the line is within the cap and parses.
func marshalAndDecode(t *testing.T, event SessionEvent) SessionEvent {
	t.Helper()
	line, err := MarshalLine(event)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeLine(line)
	if err != nil || len(line) > MaxLineBytes {
		t.Fatalf("err=%v len=%d, want a parsable line within the cap", err, len(line))
	}
	return decoded
}

func TestMarshalLineKeepsASmallEventUntouched(t *testing.T) {
	keys := namedKeys(MaxArgKeys+4, 8)

	line, _ := MarshalLine(base().WithArgShape(keys, typesFor(keys)))
	decoded, _ := DecodeLine(line)

	if decoded.ArgsTruncated || len(decoded.ArgKeys) != MaxArgKeys+4 {
		t.Errorf("a line within the cap must not be trimmed: %d keys", len(decoded.ArgKeys))
	}
}

func TestMarshalLineFailsWhenNothingFits(t *testing.T) {
	event := base().WithPath("r", strPtr(strings.Repeat("p", 2*MaxLineBytes)))

	_, err := MarshalLine(event)

	if err != ErrLineTooLong {
		t.Errorf("error = %v, want ErrLineTooLong", err)
	}
}

func TestDecodeLineRejectsAnUnknownVersionAndBadLines(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantErr error
	}{
		{"version two", `{"v":2,"id":"x","at":"2026-10-18T12:00:00Z","kind":"prompt"}`, ErrUnknownVersion},
		{"no version", `{"id":"x"}`, ErrUnknownVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeLine([]byte(tt.line)); err != tt.wantErr {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
	for _, torn := range []string{`{"v":1,"id":"x","at":"2026-10`, ``, `not json`} {
		if _, err := DecodeLine([]byte(torn)); err == nil || err == ErrUnknownVersion {
			t.Errorf("torn line %q: error = %v, want a decode error", torn, err)
		}
	}
}

func TestDecodeLineIgnoresUnknownFieldsAndReturnsUTC(t *testing.T) {
	line := `{"v":1,"id":"x","at":"2026-10-18T14:00:00+02:00","kind":"prompt","future":true}`

	event, err := DecodeLine([]byte(line))

	if err != nil || event.At.Location() != time.UTC || event.At.Hour() != 12 {
		t.Errorf("err=%v at=%v, want 12:00 UTC", err, event.At)
	}
}

func base() SessionEvent { return NewSessionEvent("id", baseTime, EventToolUse) }

func namedKeys(count, length int) []string {
	keys := make([]string, count)
	for i := range keys {
		keys[i] = strings.Repeat("k", length-2) + string(rune('a'+i/26)) + string(rune('a'+i%26))
	}
	return keys
}

func typesFor(keys []string) []string {
	types := make([]string, len(keys))
	for i := range types {
		types[i] = "string"
	}
	return types
}
