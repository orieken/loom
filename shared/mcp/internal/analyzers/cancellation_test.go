package analyzers

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// wideTree writes enough files that an unstoppable walk would visit them all.
func wideTree(t *testing.T, count int) string {
	t.Helper()
	root := t.TempDir()
	for index := 0; index < count; index++ {
		write(t, filepath.Join(root, fmt.Sprintf("d%02d", index%50), fmt.Sprintf("f%04d.go", index)), "package p\n")
	}
	return root
}

// The L2.2 done-when: a context cancelled while the walk is under way stops
// it within 100ms. The walk cancels itself on its first file, so the cancel
// lands mid-walk by construction rather than by timing.
func TestACancelledWalkStopsWithin100ms(t *testing.T) {
	const total = 3000
	root := wideTree(t, total)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var cancelledAt time.Time
	include := func(string) bool {
		if cancelledAt.IsZero() {
			cancelledAt = time.Now()
			cancel()
		}
		return true
	}

	files, err := CollectFiles(ctx, root, include)
	stopped := time.Since(cancelledAt)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if stopped > 100*time.Millisecond {
		t.Errorf("walk ran %v after cancellation, want under 100ms", stopped)
	}
	if len(files) >= total {
		t.Errorf("walk collected all %d files despite cancellation", len(files))
	}
}

func TestAWalkWhoseContextIsAlreadyDoneDoesNotStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	files, err := CollectFiles(ctx, wideTree(t, 10), func(string) bool { return true })
	if !errors.Is(err, context.Canceled) || len(files) != 0 {
		t.Errorf("files=%d err=%v, want none and context.Canceled", len(files), err)
	}
}

func TestForEachFileStopsWhenTheContextIsDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var seen []string
	err := ForEachFile(ctx, []string{"a", "b", "c"}, func(file string) {
		seen = append(seen, file)
		cancel()
	})
	if !errors.Is(err, context.Canceled) || len(seen) != 1 {
		t.Errorf("seen=%v err=%v, want one file then context.Canceled", seen, err)
	}
	if err := ForEachFile(context.Background(), []string{"a", "b"}, func(string) {}); err != nil {
		t.Errorf("an uncancelled loop returned %v", err)
	}
}

// Every analyzer passes its context down, so a cancelled one stops each.
func TestEveryAnalyzerStopsOnACancelledContext(t *testing.T) {
	root := analyzedProject(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	failures := map[string]error{}
	_, failures["complexity"] = NewComplexityAnalyzer().Analyze(ctx, root, 7, 30)
	_, failures["accessibility"] = NewAccessibilityAnalyzer().Analyze(ctx, root)
	_, failures["dependencies"] = NewDependencyBoundaryAnalyzer().Analyze(ctx, root)
	_, failures["ubiquitous language"] = NewUbiquitousLanguageAnalyzer().Analyze(ctx, root, filepath.Join(root, "DICTIONARY.md"))
	for analyzer, err := range failures {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("%s: err = %v, want context.Canceled", analyzer, err)
		}
	}
}

// countdownContext reports itself done after a set number of Err calls, so a
// test can cancel between the walk and the per-file analysis that follows it
// — a window a context cancelled up front never reaches.
type countdownContext struct {
	context.Context
	remaining int
}

func (c *countdownContext) Err() error {
	if c.remaining <= 0 {
		return context.Canceled
	}
	c.remaining--
	return nil
}

// cancelAfterWalk lets a walk over a directory holding one file finish (two
// Err calls: the directory, then the file) and fails the first check after.
func cancelAfterWalk() context.Context {
	return &countdownContext{Context: context.Background(), remaining: 2}
}

func TestEveryAnalyzerStopsWhenCancelledBetweenFiles(t *testing.T) {
	single := func(name, content string) string {
		root := t.TempDir()
		write(t, filepath.Join(root, name), content)
		return root
	}
	dictionaryRoot := single("DICTIONARY.md", "## Customer\n**Synonyms to avoid**: `client`\n")
	failures := map[string]error{}
	_, failures["complexity"] = NewComplexityAnalyzer().Analyze(cancelAfterWalk(), single("a.go", "package a\n"), 7, 30)
	_, failures["accessibility"] = NewAccessibilityAnalyzer().Analyze(cancelAfterWalk(), single("a.html", "<p></p>\n"))
	_, failures["dependencies"] = NewDependencyBoundaryAnalyzer().Analyze(cancelAfterWalk(), single("a.go", "package a\n"))
	_, failures["ubiquitous language"] = NewUbiquitousLanguageAnalyzer().Analyze(cancelAfterWalk(),
		single("a.go", "package a\n"), filepath.Join(dictionaryRoot, "DICTIONARY.md"))
	for analyzer, err := range failures {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("%s: err = %v, want context.Canceled from the per-file loop", analyzer, err)
		}
	}
}
