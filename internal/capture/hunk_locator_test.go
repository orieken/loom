package capture

import "testing"

func TestHunkLocatorPlacesEdits(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		oldText     string
		newText     string
		wantStart   int
		wantEnd     int
		wantPreText string // the lines as they were before the edit
		wantPlaced  bool
	}{
		{name: "whole line", content: "a\nb\nc\nd\n", oldText: "x", newText: "c",
			wantStart: 3, wantEnd: 3, wantPreText: "x", wantPlaced: true},
		{name: "fragment inside a line widens to the line", content: "alpha BETA gamma\nsecond\n",
			oldText: "beta", newText: "BETA", wantStart: 1, wantEnd: 1, wantPreText: "alpha beta gamma", wantPlaced: true},
		{name: "several lines", content: "a\nx\ny\nb\n", oldText: "q", newText: "x\ny",
			wantStart: 2, wantEnd: 3, wantPreText: "q", wantPlaced: true},
		{name: "new text ends with a newline", content: "a\nx\ny\nb\n", oldText: "q\n", newText: "x\ny\n",
			wantStart: 2, wantEnd: 3, wantPreText: "q", wantPlaced: true},
		{name: "last line without a final newline", content: "a\nb", oldText: "z", newText: "b",
			wantStart: 2, wantEnd: 2, wantPreText: "z", wantPlaced: true},
		{name: "pure insertion has an empty pre-image", content: "a\nnew\nb\n", oldText: "", newText: "new",
			wantStart: 2, wantEnd: 2, wantPreText: "", wantPlaced: true},
		{name: "line-ending change falls back to a line comparison", content: "a\r\nb\r\nc\r\n",
			oldText: "old", newText: "b\nc", wantStart: 2, wantEnd: 3, wantPreText: "old", wantPlaced: true},
		{name: "ambiguous text is never guessed", content: "dup\nmid\ndup\n", oldText: "x", newText: "dup"},
		{name: "ambiguous lines are never guessed", content: "k\r\nm\r\nk\r\n", oldText: "x", newText: "k\n"},
		{name: "text that is not in the file", content: "a\nb\n", oldText: "x", newText: "zzz"},
		{name: "empty new text", content: "a\nb\n", oldText: "x", newText: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasher := NewHasher(newTestSalt(t))
			content := []byte(tt.content)

			found := NewHunkLocator(hasher).LocateAll(content, SplitLines(content), EditText{}.With(tt.oldText, tt.newText))

			if !tt.wantPlaced {
				if len(found) != 0 {
					t.Fatalf("placed %+v, want no hunk", found)
				}
				return
			}
			if len(found) != 1 {
				t.Fatalf("placed %d hunks, want 1", len(found))
			}
			wantPost, _ := hasher.HashLineRange(content, tt.wantStart, tt.wantEnd)
			want := HunkFacts{RangeStart: tt.wantStart, RangeEnd: tt.wantEnd, PostHash: wantPost,
				PreHash: hasher.Hash(HashLines, tt.wantPreText)}
			if found[0] != want {
				t.Fatalf("facts = %+v, want %+v", found[0], want)
			}
		})
	}
}

func TestHunkLocatorKeepsTheEditsThatPlaceAndDropsTheRest(t *testing.T) {
	hasher := NewHasher(newTestSalt(t))
	content := []byte("a\nb\nc\nd\n")
	text := EditText{}.With("1", "b").With("2", "nowhere").With("3", "d")

	found := NewHunkLocator(hasher).LocateAll(content, SplitLines(content), text)

	if len(found) != 2 || found[0].RangeStart != 2 || found[1].RangeStart != 4 {
		t.Fatalf("found %+v, want ranges starting at 2 and 4", found)
	}
}

func TestHunkLocatorWholeFileHasNoPreImage(t *testing.T) {
	hasher := NewHasher(newTestSalt(t))
	content := []byte("a\nb\nc\n")

	facts, isInside := NewHunkLocator(hasher).WholeFile(SplitLines(content))

	want, _ := hasher.HashLineRange(content, 1, 3)
	if !isInside || facts.RangeStart != 1 || facts.RangeEnd != 3 || facts.PostHash != want || facts.PreHash != "" {
		t.Fatalf("facts = %+v (inside %v), want lines 1-3 and no pre-image", facts, isInside)
	}
}

func TestEditTextNeverPrintsItsContent(t *testing.T) {
	const secret = "CANARY-EDIT-TEXT-1f3a"
	text := EditText{}.With(secret, secret+"-new")

	outputs := []string{
		text.String(), text.GoString(),
		formatAll("%v|%+v|%#v|%s|%q|%x|%d", text, text, text, text, text, text, text),
	}
	marshaled, err := text.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	outputs = append(outputs, string(marshaled))

	for _, output := range outputs {
		if containsAny(output, secret) {
			t.Fatalf("output %q leaks the edit text", output)
		}
	}
	if text.Len() != 1 {
		t.Fatalf("Len = %d, want 1", text.Len())
	}
}
