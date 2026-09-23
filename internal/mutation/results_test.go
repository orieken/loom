package mutation_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/mutation"
)

// The shape gremlins v0.6.0 writes with -o, trimmed to what is read.
const report = `{
  "go_module": "github.com/orieken/loom",
  "test_efficacy": 50,
  "files": [
    {"file_name": "a.go", "mutations": [
      {"type": "CONDITIONALS_NEGATION", "status": "KILLED", "line": 3, "column": 9},
      {"type": "ARITHMETIC_BASE", "status": "LIVED", "line": 4, "column": 12}
    ]}
  ]
}`

func TestParseResultsLocatesMutantsFromTheRepositoryRoot(t *testing.T) {
	for dir, wantFile := range map[string]string{"internal/pkg": "internal/pkg/a.go", ".": "a.go"} {
		t.Run(dir, func(t *testing.T) {
			mutants, err := mutation.ParseResults(strings.NewReader(report), dir)
			if err != nil {
				t.Fatalf("ParseResults: %v", err)
			}
			want := []mutation.Mutant{
				{File: wantFile, Line: 3, Column: 9, Type: "CONDITIONALS_NEGATION", Status: mutation.StatusKilled},
				{File: wantFile, Line: 4, Column: 12, Type: "ARITHMETIC_BASE", Status: mutation.StatusLived},
			}
			if !reflect.DeepEqual(mutants, want) {
				t.Errorf("mutants = %+v\nwant      %+v", mutants, want)
			}
		})
	}
}

func TestParseResultsRejectsAStatusItDoesNotKnow(t *testing.T) {
	unknown := strings.Replace(report, `"LIVED"`, `"SURVIVED"`, 1)
	if _, err := mutation.ParseResults(strings.NewReader(unknown), "pkg"); err == nil {
		t.Error("an unknown status was accepted — it could only be guessed, and a wrong guess scores a survivor as killed")
	}
}

func TestParseResultsRejectsMalformedJSON(t *testing.T) {
	if _, err := mutation.ParseResults(strings.NewReader("{"), "pkg"); err == nil {
		t.Error("malformed JSON was accepted")
	}
}
