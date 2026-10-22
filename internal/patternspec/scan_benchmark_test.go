package patternspec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// hundredSpecs is a project with 100 copies of the full fixture.
func hundredSpecs(b *testing.B) fstest.MapFS {
	b.Helper()
	source, err := os.ReadFile(filepath.Join("testdata", "valid", "publish-release-notes"+FileSuffix))
	if err != nil {
		b.Fatal(err)
	}
	project := fstest.MapFS{}
	for index := range 100 {
		name := fmt.Sprintf("release-notes-%d", index)
		text := strings.Replace(string(source), "name: publish-release-notes", "name: "+name, 1)
		project[LocalDir+"/"+name+FileSuffix] = &fstest.MapFile{Data: []byte(text)}
	}
	return project
}

// The budget is 200 ms for 100 specs on a warm cache (spec NFR).
func BenchmarkScanOfAHundredSpecs(b *testing.B) {
	project := hundredSpecs(b)
	Scan(project, knownReferences) // warm the compiled schema
	b.ResetTimer()
	for b.Loop() {
		if report := Scan(project, knownReferences); len(report.Documents) != 100 {
			b.Fatalf("want 100 documents, got %d: %v", len(report.Documents), report.Issues)
		}
	}
}
