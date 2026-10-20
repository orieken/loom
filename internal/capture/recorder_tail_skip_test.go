package capture

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// countingTail counts the tail reads of a hook call.
type countingTail struct {
	inner EventTail
	reads int
}

func (c *countingTail) Recent(ctx context.Context, now time.Time, window time.Duration) ([]SessionEvent, error) {
	c.reads++
	return c.inner.Recent(ctx, now, window)
}

func TestRecorderReadsTheTailOnlyForACallThatCanUseIt(t *testing.T) {
	path := filepath.Join(testRoot, "app/main.go")
	tests := []struct {
		name      string
		obs       Observation
		wantReads int
	}{
		{name: "Bash call has no path", obs: Observation{Kind: EventToolResult, Tool: "Bash", SessionID: "s1"}, wantReads: 0},
		{name: "Bash use has no path", obs: Observation{Kind: EventToolUse, Tool: "Bash", SessionID: "s1"}, wantReads: 0},
		{name: "Stop ends the turn", obs: Observation{Kind: EventStop, SessionID: "s1"}, wantReads: 0},
		{name: "prompt sees every path", obs: prompt(), wantReads: 1},
		{name: "Edit result has a path", obs: edit("app/main.go"), wantReads: 1},
		{name: "Read use has a path", obs: Observation{Kind: EventToolUse, Tool: "Read", SessionID: "s1", FilePath: path}, wantReads: 1},
		{name: "Write result looks for the pre-image", obs: Observation{Kind: EventToolResult, Tool: ToolWrite, SessionID: "s1", FilePath: path}, wantReads: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newRig(t, tt.obs, EditText{})
			tail := &countingTail{inner: rig.sink}
			rig.deps.Tail = tail

			rig.mustCall(t, tt.obs)

			equal(t, "tail reads", tail.reads, tt.wantReads)
		})
	}
}

func TestRecorderStillWritesTheCallWhenTheTailIsSkipped(t *testing.T) {
	obs := Observation{Kind: EventToolResult, Tool: "Bash", SessionID: "s1"}
	rig := newRig(t, obs, EditText{})

	rig.mustCall(t, obs)

	equal(t, "events written", len(rig.sink.events), 1)
	equal(t, "files read", rig.files.totalReads(), 0)
}
