package repograph

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The checked-in expected graph is the contract for each fixture: nodes,
// edges, confidence labels and unresolved imports. Regenerate deliberately
// with UPDATE_GOLDEN=1 and read the diff.
func TestGraphMatchesGoldenFilePerFixture(t *testing.T) {
	for _, name := range []string{"go", "typescript", "python", "java", "mixed"} {
		t.Run(name, func(t *testing.T) {
			result := indexFixture(t, name)
			actual, err := json.MarshalIndent(result.Graph, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			actual = append(actual, '\n')
			golden := filepath.Join(fixturesDir, name+".golden.json")
			if os.Getenv("UPDATE_GOLDEN") != "" {
				if err := os.WriteFile(golden, actual, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			expected, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if !bytes.Equal(expected, actual) {
				t.Errorf("graph for %s differs from %s; run with UPDATE_GOLDEN=1 and review the diff\n%s", name, golden, actual)
			}
		})
	}
}

// Two runs over the same repo must produce byte-identical output.
func TestIndexIsDeterministic(t *testing.T) {
	first, second := indexFixture(t, "mixed"), indexFixture(t, "mixed")
	a, _ := json.Marshal(first.Graph)
	b, _ := json.Marshal(second.Graph)
	if !bytes.Equal(a, b) {
		t.Error("two runs over the same fixture produced different graphs")
	}
}
