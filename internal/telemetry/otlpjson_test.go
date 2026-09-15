package telemetry_test

// The enforcer for encodeValue's stated precondition.
//
// encodeValue handles STRINGSLICE properly and falls back to a value's string
// form for every other slice type. That fallback is only acceptable while
// nothing in the package emits those types — a bool or int slice rendered as
// "[true false]" is not something a consumer reading OTLP can parse.
//
// The comment used to assert that condition in prose, and the prose was stale
// in the commit that wrote it (roadmap L3.43). This is the same claim as a
// test, which is the whole point of the PRECONDITION / ENFORCED-BY convention
// in docs/patterns/framework-meta-patterns.md.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unencodableSliceConstructors are the attribute helpers whose values
// encodeValue cannot render as a real ArrayValue.
var unencodableSliceConstructors = []string{
	"attribute.BoolSlice",
	"attribute.IntSlice",
	"attribute.Int64Slice",
	"attribute.Float64Slice",
}

func TestNoUnencodableSliceAttributeIsEmitted(t *testing.T) {
	sources := packageSourceFiles(t)
	// A scan that silently matched nothing would pass forever. This is the
	// same guard the boundary test uses: prove the check had something to
	// look at.
	if len(sources) == 0 {
		t.Fatal("no non-test Go files scanned — the check would pass vacuously")
	}
	for _, name := range sources {
		assertNoUnencodableSlice(t, name)
	}
}

// packageSourceFiles lists this package's non-test Go files.
func packageSourceFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}
	var sources []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		sources = append(sources, name)
	}
	return sources
}

func assertNoUnencodableSlice(t *testing.T, name string) {
	t.Helper()
	source, err := os.ReadFile(filepath.Clean(name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	for _, constructor := range unencodableSliceConstructors {
		if strings.Contains(string(source), constructor) {
			t.Errorf("%s uses %s, which encodeValue renders as a Go string rather than an "+
				"OTLP ArrayValue. Either add a case to encodeValue or do not emit it.", name, constructor)
		}
	}
}
