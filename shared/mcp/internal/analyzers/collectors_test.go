package analyzers

import (
	"context"
	"path/filepath"
	"testing"
)

// analyzedProject holds one file for each analyzer plus a vendored Go file the
// shared walk must skip. The synonym in domain/order.go lets the ubiquitous-
// language analyzer prove, by finding it, that the walk reached the file.
func analyzedProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "domain/order.go"), "package domain\n\n// every client pays\nfunc Total() int { return 1 }\n")
	write(t, filepath.Join(root, "web/index.html"), "<html><body><img src=\"a.png\"></body></html>\n")
	write(t, filepath.Join(root, "web/app.js"), "function total() { return 1 }\n")
	write(t, filepath.Join(root, "DICTIONARY.md"), "## Customer\n**Synonyms to avoid**: `client`\n")
	write(t, filepath.Join(root, "vendor/skipped.go"), "package vendored\n")
	return root
}

func TestComplexityCollectsThroughTheSharedWalk(t *testing.T) {
	result, err := NewComplexityAnalyzer().Analyze(context.Background(), analyzedProject(t), 7, 30)
	// order.go and app.js — the Go and the generic analysis paths — and
	// not the vendored file.
	if err != nil || result.TotalFiles != 2 {
		t.Errorf("complexity: %+v err=%v, want the non-vendored Go and JS files", result, err)
	}
}

func TestAccessibilityCollectsThroughTheSharedWalk(t *testing.T) {
	result, err := NewAccessibilityAnalyzer().Analyze(context.Background(), filepath.Join(analyzedProject(t), "web"))
	if err != nil || result.TotalFiles != 1 {
		t.Errorf("accessibility: %+v err=%v, want one HTML file", result, err)
	}
}

func TestDependenciesCollectThroughTheSharedWalk(t *testing.T) {
	files, err := NewDependencyBoundaryAnalyzer().collectSourceFiles(context.Background(), analyzedProject(t))
	if err != nil || len(files) != 1 {
		t.Errorf("dependencies collected %v err=%v, want the one non-vendored Go file", files, err)
	}
}

func TestUbiquitousLanguageCollectsThroughTheSharedWalk(t *testing.T) {
	root := analyzedProject(t)
	result, err := NewUbiquitousLanguageAnalyzer().Analyze(context.Background(), root, filepath.Join(root, "DICTIONARY.md"))
	if err != nil || result.ViolationsCount == 0 {
		t.Errorf("ubiquitous language: %+v err=%v, want the planted synonym found", result, err)
	}
}

func TestEveryAnalyzerReportsAPathThatDoesNotExist(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	dictionary := filepath.Join(analyzedProject(t), "DICTIONARY.md")
	failures := map[string]error{}
	_, failures["complexity"] = NewComplexityAnalyzer().Analyze(context.Background(), missing, 7, 30)
	_, failures["accessibility"] = NewAccessibilityAnalyzer().Analyze(context.Background(), missing)
	_, failures["dependencies"] = NewDependencyBoundaryAnalyzer().Analyze(context.Background(), missing)
	_, failures["ubiquitous language"] = NewUbiquitousLanguageAnalyzer().Analyze(context.Background(), missing, dictionary)
	for analyzer, err := range failures {
		if err == nil {
			t.Errorf("%s analyzed a path that does not exist", analyzer)
		}
	}
}
