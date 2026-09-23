package diffcover_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/orieken/loom/internal/diffcover"
)

func TestFileHasStatementsOnlyWhereCoverageInstruments(t *testing.T) {
	dir := t.TempDir()
	for name, test := range map[string]struct {
		source string
		want   bool
	}{
		"declarations only":      {"package p\n\nvar Exclusions = map[string]bool{\"a/\": true}\n\ntype T struct{}\n", false},
		"a function body":        {"package p\n\nfunc F() int { return 1 }\n", true},
		"a function literal":     {"package p\n\nvar F = func() int { return 1 }\n", true},
		"a bodiless declaration": {"package p\n\nfunc Assembly() int\n", false},
		"unparseable":            {"not go", true},
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, filepath.Base(t.Name())+".go")
			if err := os.WriteFile(path, []byte(test.source), 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}
			if got := diffcover.FileHasStatements(path); got != test.want {
				t.Errorf("FileHasStatements = %v, want %v", got, test.want)
			}
		})
	}
}

func TestAMissingFileIsAssumedToHaveStatements(t *testing.T) {
	if !diffcover.FileHasStatements(filepath.Join(t.TempDir(), "absent.go")) {
		t.Error("a file that could not be read was declared statement-free — that is a silent pass")
	}
}

// A declarations-only file is absent from Go's coverage profile by
// construction. Found when L3.59's exclusions.go — one var — failed L3.58's
// gate as UNMEASURED.
func TestAnUnprofiledFileWithoutStatementsIsListedNotFailed(t *testing.T) {
	statementFree := map[string]bool{"pkg/decls.go": true}
	report := diffcover.Measure(diffcover.Measurement{
		Profile: profile, ModulePath: module,
		Changes:       diffcover.Changes{"pkg/decls.go": {1, 2}, "pkg/code.go": {1}},
		HasStatements: func(file string) bool { return !statementFree[file] },
	})

	if !reflect.DeepEqual(report.Statementless, []string{"pkg/decls.go"}) {
		t.Errorf("statementless = %v, want [pkg/decls.go]", report.Statementless)
	}
	if !reflect.DeepEqual(report.Unmeasured, []string{"pkg/code.go"}) {
		t.Errorf("unmeasured = %v, want [pkg/code.go] — a file with statements still fails", report.Unmeasured)
	}
}

func TestWithoutAStatementCheckEveryUnprofiledFileIsUnmeasured(t *testing.T) {
	report := diffcover.Measure(diffcover.Measurement{
		Profile: profile, ModulePath: module, Changes: diffcover.Changes{"pkg/decls.go": {1}},
	})
	if !reflect.DeepEqual(report.Unmeasured, []string{"pkg/decls.go"}) || len(report.Statementless) != 0 {
		t.Errorf("report = %+v, want pkg/decls.go unmeasured", report)
	}
}
