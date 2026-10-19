package capture

import (
	"errors"
	"reflect"
	"testing"
)

func TestAssemblerStoresPropertiesForANonRedactedFile(t *testing.T) {
	assembler := newTestAssembler(t, "/proj")
	hasher := NewHasher(newTestSalt(t))

	event, err := assembler.Assemble(Observation{
		Kind: EventToolResult, Tool: ToolEdit, SessionID: "session-1", FilePath: "/proj/src/a.go",
		ArgKeys: []string{"file_path"}, ArgTypes: []string{"string"},
		Hunk: HunkFacts{RangeStart: 10, RangeEnd: 14, PreHash: "A", PostHash: "B"},
	})

	want := NewSessionEvent("id-1", baseTime, EventToolResult).
		WithTool(ToolEdit).
		WithSessionRef(hasher.Hash(HashSession, "session-1")).
		WithArgShape([]string{"file_path"}, []string{"string"}).
		WithPath(hasher.Hash(HashPath, "src/a.go"), strPtr("src/a.go")).
		WithHunk(HunkFacts{RangeStart: 10, RangeEnd: 14, PreHash: "A", PostHash: "B"})
	if err != nil || !reflect.DeepEqual(event, want) {
		t.Errorf("err=%v\n got  %+v\n want %+v", err, event, want)
	}
}

func TestAssemblerRedactedPathKeepsOnlyTheHash(t *testing.T) {
	assembler := newTestAssembler(t, "/proj")

	event, err := assembler.Assemble(Observation{
		Kind: EventToolResult, Tool: ToolWrite, FilePath: "/proj/.env.local",
		Hunk: HunkFacts{RangeStart: 1, RangeEnd: 3, PreHash: "A", PostHash: "B"},
	})

	hasher := NewHasher(newTestSalt(t))
	want := NewSessionEvent("id-1", baseTime, EventToolResult).
		WithTool(ToolWrite).
		WithPath(hasher.Hash(HashPath, ".env.local"), nil)
	if err != nil || !reflect.DeepEqual(event, want) {
		t.Errorf("err=%v\n got  %+v\n want a hash only: %+v", err, event, want)
	}
}

func TestAssemblerReplacesPromptTextWithAClassLabel(t *testing.T) {
	assembler := newTestAssembler(t, "/proj")

	corrective, _ := assembler.Assemble(Observation{Kind: EventPrompt, Prompt: "no, use the other file"})
	other, _ := assembler.Assemble(Observation{Kind: EventPrompt, Prompt: "note that it moved"})
	tool, _ := assembler.Assemble(Observation{Kind: EventToolUse, Prompt: "no, ignored for a tool event"})

	if corrective.PromptClass != PromptCorrective || other.PromptClass != PromptOther {
		t.Errorf("classes = %q and %q", corrective.PromptClass, other.PromptClass)
	}
	if tool.PromptClass != "" {
		t.Errorf("a tool event must carry no prompt class, got %q", tool.PromptClass)
	}
}

func TestAssemblerReplacesUnsafeNamesAndKeys(t *testing.T) {
	assembler := newTestAssembler(t, "/proj")

	event, _ := assembler.Assemble(Observation{
		Kind: EventToolUse, Tool: "Bad Tool; rm -rf", ArgKeys: []string{"ok_key", "bad key=SECRET"},
		ArgTypes: []string{"string", "string"},
	})

	if event.Tool != UnsafeNamePlaceholder || event.ArgKeys[0] != "ok_key" || event.ArgKeys[1] != UnsafeNamePlaceholder {
		t.Errorf("tool=%q keys=%v", event.Tool, event.ArgKeys)
	}
}

func TestAssemblerLeavesOutSessionAndPathWhenAbsent(t *testing.T) {
	assembler := newTestAssembler(t, "/proj")

	event, _ := assembler.Assemble(Observation{Kind: EventStop})

	if event.SessionRef != "" || event.PathRef != "" || event.Path != nil {
		t.Errorf("event = %+v, want no session or path", event)
	}
}

func TestAssemblerReturnsTheIDSourceError(t *testing.T) {
	failing := errors.New("no entropy")
	assembler := NewAssembler(AssemblerDeps{
		Hasher:     NewHasher(newTestSalt(t)),
		Paths:      NewPathPolicy("/proj", newTestRedactor(t)),
		Clock:      fixedClock{now: baseTime},
		NewEventID: func() (string, error) { return "", failing },
	})

	_, err := assembler.Assemble(Observation{Kind: EventStop})

	if !errors.Is(err, failing) {
		t.Errorf("error = %v, want the id source error", err)
	}
}
