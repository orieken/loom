package capture

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// bannedImports are the packages the domain code must not import
// (architecture-guardrails.md #1; architecture-notes.md decision 1). The
// os package is banned because the domain does no file system access.
var bannedImports = []string{
	"os",
	"os/exec",
	"os/signal",
	"syscall",
	"net",
	"net/http",
	"database/sql",
	"github.com/spf13/cobra",
	"modernc.org/sqlite",
	"github.com/orieken/loom/internal/memory",
	"github.com/orieken/loom/internal/capture/claudehook",
	"github.com/orieken/loom/internal/capture/jsonlstore",
	"github.com/orieken/loom/internal/capture/localfs",
}

// TestDomainCodeImportsNoAdapter is the dependency-direction fitness
// function for the package. It reads the import lists of the non-test files.
func TestDomainCodeImportsNoAdapter(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		checked++
		assertNoBannedImport(t, name)
	}
	if checked == 0 {
		t.Fatal("no domain file was checked")
	}
}

func assertNoBannedImport(t *testing.T, file string) {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range parsed.Imports {
		importPath, _ := strconv.Unquote(spec.Path.Value)
		for _, banned := range bannedImports {
			if importPath == banned {
				t.Errorf("%s imports %q, which the domain must not import", file, importPath)
			}
		}
	}
}
