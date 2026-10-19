package capture

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func formatAll(format string, args ...any) string { return fmt.Sprintf(format, args...) }

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func TestRecorderEditRecordsTheEditedLines(t *testing.T) {
	r := newRig(t, edit("a.go"), EditText{}.With("x", "c"))
	r.files.put("a.go", "a\nb\nc\nd\n")

	err := r.call(t, edit("a.go"))

	equal(t, "error", err, nil)
	equal(t, "event count", len(r.sink.events), 1)
	event := r.sink.events[0]
	equal(t, "kind", event.Kind, EventToolResult)
	equal(t, "range start", event.RangeStart, 3)
	equal(t, "range end", event.RangeEnd, 3)
	equal(t, "post hash", event.PostHash, r.lineHash("a\nb\nc\nd\n", 3, 3))
	equal(t, "pre hash", event.PreHash, r.hasher.Hash(HashLines, "x"))
	equal(t, "path ref", event.PathRef, r.pathRef("a.go"))
	equal(t, "path", *event.Path, "a.go")
}

func TestRecorderMultiEditKeepsEveryRangeOfOneCall(t *testing.T) {
	text := EditText{}.With("1", "b").With("2", "d")
	r := newRig(t, edit("a.go"), text)
	r.files.put("a.go", "a\nb\nc\nd\n")

	if err := r.call(t, edit("a.go")); err != nil {
		t.Fatal(err)
	}

	if len(r.sink.events) != 2 || r.sink.events[0].At != r.sink.events[1].At {
		t.Fatalf("events = %+v, want two events at one instant", r.sink.events)
	}
	detector := NewDetector(DefaultConfig())
	for _, event := range r.sink.events {
		detector.Observe(event)
	}
	if detector.OpenHunks() != 2 {
		t.Fatalf("open hunks = %d, want 2: the second range must not drop the first", detector.OpenHunks())
	}
}

func TestRecorderRecordsAnUnknownChangeInsteadOfGuessing(t *testing.T) {
	tests := []struct {
		name    string
		content string
		reader  func(*memoryFiles)
	}{
		{name: "ambiguous text", content: "dup\nmid\ndup\n"},
		{name: "text not in the file", content: "a\nb\n"},
		{name: "file over the cap", content: strings.Repeat("x", MaxTargetFileBytes+1)},
		{name: "file missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRig(t, edit("a.go"), EditText{}.With("x", "dup"))
			if tt.content != "" {
				r.files.put("a.go", tt.content)
			}

			err := r.call(t, edit("a.go"))

			event := r.sink.events[0]
			equal(t, "error", err, nil)
			equal(t, "event count", len(r.sink.events), 1)
			equal(t, "has range", event.HasHunk(), false)
			equal(t, "post hash", event.PostHash, "")
			equal(t, "path ref kept", event.PathRef == "", false)
		})
	}
}

func TestRecorderNeverReadsAFileOverTheCapTwice(t *testing.T) {
	r := newRig(t, edit("big.go"), EditText{}.With("x", "y"))
	r.files.put("big.go", strings.Repeat("x", MaxTargetFileBytes+1))

	_ = r.call(t, edit("big.go"))

	if got := r.files.reads[filepath.Join(testRoot, "big.go")]; got != 1 {
		t.Fatalf("reads = %d, want 1", got)
	}
}

func TestRecorderReadsNoFileForARedactedOrOutsidePath(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "env file", path: filepath.Join(testRoot, ".env.local")},
		{name: "key file in a folder", path: filepath.Join(testRoot, "deploy", "server.pem")},
		{name: "outside the project", path: "/etc/passwd"},
		{name: "parent traversal", path: filepath.Join(testRoot, "..", "other", "x.go")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs := edit("x")
			obs.FilePath = tt.path
			r := newRig(t, obs, EditText{}.With("x", "y"))
			r.files.files[tt.path] = "y\n"

			err := r.call(t, obs)

			event := r.sink.events[0]
			if err != nil || r.files.totalReads() != 0 || event.HasHunk() || event.Path != nil || event.PathRef == "" {
				t.Fatalf("err = %v, reads = %d, event = %+v: want no read, no range, no path, a hash", err, r.files.totalReads(), event)
			}
		})
	}
}

func TestRecorderWriteHashesTheWholeFileAndKeepsThePreImage(t *testing.T) {
	r := newRig(t, edit("w.go"), EditText{})
	r.files.put("w.go", "old1\nold2\n")
	pre := Observation{Kind: EventToolUse, Tool: ToolWrite, SessionID: "s1", FilePath: filepath.Join(testRoot, "w.go")}
	post := pre
	post.Kind = EventToolResult

	if err := r.call(t, pre); err != nil {
		t.Fatal(err)
	}
	r.files.put("w.go", "new1\nnew2\nnew3\n")
	r.clock.advance(time.Second)
	if err := r.call(t, post); err != nil {
		t.Fatal(err)
	}

	use, result := r.sink.events[0], r.sink.events[1]
	equal(t, "pre event kind", use.Kind, EventToolUse)
	equal(t, "pre event end", use.RangeEnd, 2)
	equal(t, "pre event snapshot", use.PostHash, r.lineHash("old1\nold2\n", 1, 2))
	equal(t, "post event end", result.RangeEnd, 3)
	equal(t, "post event hash", result.PostHash, r.lineHash("new1\nnew2\nnew3\n", 1, 3))
	equal(t, "pre-image", result.PreHash, use.PostHash)
}

func TestRecorderWriteOfANewFileHasNoPreImage(t *testing.T) {
	r := newRig(t, edit("n.go"), EditText{})
	post := Observation{Kind: EventToolResult, Tool: ToolWrite, FilePath: filepath.Join(testRoot, "n.go")}
	r.files.put("n.go", "x\n")

	_ = r.call(t, post)

	if event := r.sink.events[0]; event.PreHash != "" || event.RangeEnd != 1 {
		t.Fatalf("event = %+v, want a range with no pre-image", event)
	}
}

func TestRecorderOnlyAFileToolReadsAFile(t *testing.T) {
	r := newRig(t, edit("a.go"), EditText{}.With("x", "b"))
	r.files.put("a.go", "a\nb\n")
	bash := Observation{Kind: EventToolResult, Tool: ToolBash, FilePath: filepath.Join(testRoot, "a.go")}

	_ = r.call(t, bash)

	if r.files.totalReads() != 0 || r.sink.events[0].HasHunk() {
		t.Fatalf("reads = %d, event = %+v, want no read and no range", r.files.totalReads(), r.sink.events[0])
	}
}

func TestRecorderFailureCauses(t *testing.T) {
	tests := []struct {
		name   string
		damage func(*rig)
		want   FailureCause
	}{
		{name: "decode", damage: func(r *rig) { r.deps.Decoder = scriptedDecoder{err: errors.New("bad")} }, want: CauseDecode},
		{name: "config", damage: func(r *rig) { r.deps.Config = staticConfig{err: errors.New("bad")} }, want: CauseConfig},
		{name: "salt", damage: func(r *rig) { r.deps.Salts = staticSalt{err: errors.New("none")} }, want: CauseSalt},
		{name: "append", damage: func(r *rig) { r.sink.failWith = errors.New("full") }, want: CauseAppend},
		{name: "bad redaction glob", damage: func(r *rig) {
			r.deps.Config = staticConfig{cfg: Config{Window: time.Minute, RedactionGlobs: []string{"[bad"}}}
		}, want: CauseConfig},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRig(t, edit("a.go"), EditText{})
			tt.damage(r)
			r.recorder = NewRecorder(r.deps)

			err := r.recorder.Record(t.Context(), "event", []byte("{}"))

			if err == nil || CauseOf(err) != tt.want {
				t.Fatalf("err = %v, cause = %q, want cause %q", err, CauseOf(err), tt.want)
			}
			if len(r.sink.events) != 0 {
				t.Fatalf("a failed call wrote %d events, want none", len(r.sink.events))
			}
		})
	}
}

func TestRecorderStopPurgesWithTheConfiguredRetention(t *testing.T) {
	stop := Observation{Kind: EventStop, SessionID: "s1"}
	r := newRig(t, stop, EditText{})
	cfg := DefaultConfig()
	cfg.Retention = Retention{MaxAgeDays: 7, MaxBytes: 1 << 20}
	r.deps.Config = staticConfig{cfg: cfg}

	err := r.call(t, stop)

	if err != nil || r.purger.calls != 1 || r.purger.retention != cfg.Retention {
		t.Fatalf("err = %v, purges = %d, retention = %+v, want one purge with the configured retention", err, r.purger.calls, r.purger.retention)
	}
	if len(r.sink.events) != 1 || r.sink.events[0].Kind != EventStop {
		t.Fatalf("events = %+v, want one stop event", r.sink.events)
	}
}

func TestRecorderOnlyStopPurges(t *testing.T) {
	r := newRig(t, prompt(), EditText{})

	_ = r.call(t, prompt())
	_ = r.call(t, edit("a.go"))

	if r.purger.calls != 0 {
		t.Fatalf("purges = %d, want 0 for calls that are not Stop", r.purger.calls)
	}
}

func TestRecorderStopStillPurgesWhenTheSaltIsMissing(t *testing.T) {
	stop := Observation{Kind: EventStop}
	r := newRig(t, stop, EditText{})
	r.deps.Salts = staticSalt{err: errors.New("none")}

	err := r.call(t, stop)

	if CauseOf(err) != CauseSalt || r.purger.calls != 1 {
		t.Fatalf("cause = %q, purges = %d, want a salt failure and one purge", CauseOf(err), r.purger.calls)
	}
}

func TestRecorderStopWithInvalidConfigPurgesNothing(t *testing.T) {
	stop := Observation{Kind: EventStop}
	r := newRig(t, stop, EditText{})
	r.deps.Config = staticConfig{err: errors.New("bad")}

	err := r.call(t, stop)

	if CauseOf(err) != CauseConfig || r.purger.calls != 0 {
		t.Fatalf("cause = %q, purges = %d, want a config failure and no purge", CauseOf(err), r.purger.calls)
	}
}

func TestRecorderPurgeFailureIsCounted(t *testing.T) {
	stop := Observation{Kind: EventStop}
	r := newRig(t, stop, EditText{})
	r.purger.err = errors.New("disk")

	err := r.call(t, stop)

	if err == nil || CauseOf(err) != CauseAppend {
		t.Fatalf("err = %v, want an append-cause failure", err)
	}
	if len(r.sink.events) != 1 {
		t.Fatalf("events = %d, want the stop event kept", len(r.sink.events))
	}
}

func TestRecorderAssembleErrorIsCountedAsAppend(t *testing.T) {
	r := newRig(t, edit("a.go"), EditText{})
	r.deps.NewEventID = func() (string, error) { return "", errors.New("no entropy") }
	r.recorder = NewRecorder(r.deps)

	err := r.recorder.Record(t.Context(), "event", nil)

	if CauseOf(err) != CauseAppend {
		t.Fatalf("cause = %q, want append", CauseOf(err))
	}
}

func TestRecorderUsesOneInstantForAllEvents(t *testing.T) {
	r := newRig(t, edit("a.go"), EditText{}.With("1", "b").With("2", "c"))
	r.files.put("a.go", "a\nb\nc\n")

	_ = r.call(t, edit("a.go"))

	want := r.clock.now.UTC()
	for _, event := range r.sink.events {
		if !event.At.Equal(want) {
			t.Fatalf("event at %v, want %v", event.At, want)
		}
	}
}

func TestCauseOfDefaultsToAppend(t *testing.T) {
	if CauseOf(errors.New("plain")) != CauseAppend {
		t.Fatal("an error with no cause must count as an append failure")
	}
	wrapped := fmt.Errorf("outer: %w", failed(CauseSalt, errors.New("inner")))
	if CauseOf(wrapped) != CauseSalt {
		t.Fatal("a wrapped cause must be found")
	}
	if !strings.Contains(failed(CauseSalt, errors.New("inner")).Error(), "inner") {
		t.Fatal("the error text must name the reason")
	}
}
