package jsonlstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/orieken/loom/internal/capture"
)

// TestMain lets the concurrency test re-run this test binary as an appender.
func TestMain(m *testing.M) {
	if os.Getenv(appenderEnv) != "" {
		runAppender()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// newLayout builds a project root that holds .claude/memory, as an
// installed project does.
func newLayout(t *testing.T) capture.Layout {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude", "memory"), 0o700); err != nil {
		t.Fatal(err)
	}
	return capture.NewLayout(root)
}

// event builds a stored-shape event. The path text sets the line size.
func event(id string, at time.Time, kind capture.EventKind, pathLength int) capture.SessionEvent {
	built := capture.NewSessionEvent(id, at, kind).WithTool("Edit")
	if pathLength > 0 {
		display := strings.Repeat("p", pathLength)
		built = built.WithPath("ref", &display)
	}
	return built
}

func hunkEvent(id string, at time.Time, kind capture.EventKind, pathRef string, hash string) capture.SessionEvent {
	display := "a.go"
	return capture.NewSessionEvent(id, at, kind).WithTool("Edit").WithPath(pathRef, &display).
		WithHunk(capture.HunkFacts{RangeStart: 1, RangeEnd: 2, PostHash: hash})
}

// lineOf encodes an event as one day-file line.
func lineOf(t *testing.T, e capture.SessionEvent) string {
	t.Helper()
	line, err := capture.MarshalLine(e)
	if err != nil {
		t.Fatal(err)
	}
	return string(line)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func idsOf(events []capture.SessionEvent) []string {
	ids := make([]string, len(events))
	for i, e := range events {
		ids[i] = e.ID
	}
	return ids
}

func cancelledContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	return ctx, cancel
}

// equal fails the test when two values differ. Moving the comparison here
// keeps each test free of the branches that a failure message needs.
func equal[T comparable](t *testing.T, what string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", what, got, want)
	}
}

func sequence(prefix string, from, to int) []string {
	var ids []string
	for i := from; i < to; i++ {
		ids = append(ids, fmt.Sprintf("%s%04d", prefix, i))
	}
	return ids
}
