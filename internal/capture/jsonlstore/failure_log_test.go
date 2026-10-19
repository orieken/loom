package jsonlstore

import (
	"os"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
)

func TestFailureLogAppendsOneByteForEachFailure(t *testing.T) {
	layout := newLayout(t)
	log := NewFailureLog(layout)

	log.Record(capture.CauseSalt)
	log.Record(capture.CauseDecode)
	log.Record(capture.CauseTimeout)

	if got := readFile(t, layout.FailuresPath()); got != "sdt" {
		t.Fatalf("failure file = %q, want %q", got, "sdt")
	}
}

func TestFailureLogStaysSilentWhenItCannotWrite(t *testing.T) {
	log := NewFailureLog(capture.NewLayout(t.TempDir()))

	log.Record(capture.CauseSalt)

	if log.IsStalled(time.Now()) {
		t.Fatal("a log that never wrote cannot be stalled")
	}
}

func TestFailureLogStallMarker(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		content string
		age     time.Duration
		want    bool
	}{
		{name: "stall marker, 30 s old", content: "t", age: 30 * time.Second, want: true},
		{name: "stall marker last of several", content: "sdt", age: time.Second, want: true},
		{name: "stall marker, 59 s old", content: "t", age: 59 * time.Second, want: true},
		{name: "stall marker, exactly 60 s old", content: "t", age: capture.StallWindow, want: false},
		{name: "stall marker, 61 s old", content: "t", age: 61 * time.Second, want: false},
		{name: "another cause is last", content: "td", age: time.Second, want: false},
		{name: "stall marker in the future", content: "t", age: -time.Hour, want: true},
		{name: "empty file", content: "", age: time.Second, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := newLayout(t)
			writeFile(t, layout.FailuresPath(), tt.content)
			setAge(t, layout.FailuresPath(), now, tt.age)

			got := NewFailureLog(layout).IsStalled(now)

			if got != tt.want {
				t.Fatalf("IsStalled = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFailureLogMissingFileIsNotStalled(t *testing.T) {
	if NewFailureLog(newLayout(t)).IsStalled(time.Now()) {
		t.Fatal("a missing file must not trip the breaker")
	}
}

func TestFailureLogCheckWritesNothing(t *testing.T) {
	layout := newLayout(t)
	now := time.Now()
	writeFile(t, layout.FailuresPath(), "t")
	setAge(t, layout.FailuresPath(), now, 30*time.Second)
	before, _ := os.Stat(layout.FailuresPath())

	_ = NewFailureLog(layout).IsStalled(now)

	after, _ := os.Stat(layout.FailuresPath())
	if after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("the breaker check must leave the file as it was, or the breaker never closes")
	}
}

func setAge(t *testing.T, path string, now time.Time, age time.Duration) {
	t.Helper()
	stamp := now.Add(-age)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
}
