// Package testlint finds tests that cannot fail (roadmap L3.60, ADR-008).
//
// A test "can fail" when some call in its body — or in a subtest closure, or in
// a helper it hands its *testing.T to, followed across packages in the same
// module — reaches t.Error, t.Errorf, t.Fatal, t.Fatalf, t.Fail or t.FailNow.
// A test with no such path passes whatever the code under test does.
//
// What this deliberately does NOT check: ADR-008's "only asserts no error"
// clause. In plain Go the error check is often the assertion itself —
// `if _, err := os.Stat(archived); err != nil { t.Errorf("not archived") }`
// asserts the file exists — and no AST rule can tell that apart from checking
// that the code under test merely did not fail. A first cut flagged 22 tests
// this way and every one inspected was a real assertion. Tests that assert
// something weak are mutation testing's job (L3.59), not this lint's.
//
// The analysis is lenient where it cannot see: a helper outside the module, or
// a method, that is handed a testing value is assumed able to fail. That makes
// the check miss some vacuous tests rather than block honest ones, and the
// fixtures under testdata/ pin exactly where the line sits.
package testlint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Finding is a test with no path to a failure.
type Finding struct {
	File string // relative to the module root, slash-separated
	Line int
	Test string
}

// Key identifies a finding independently of its line, for pinning.
func (finding Finding) Key() string { return finding.File + ":" + finding.Test }

// function is a top-level function declaration and the file that holds it,
// which carries the imports needed to resolve calls it makes.
type function struct {
	declaration *ast.FuncDecl
	file        *ast.File
	directory   string
}

type module struct {
	root      string
	path      string
	fileSet   *token.FileSet
	functions map[string]map[string]function // directory -> name -> function
	tests     []function
}

// loadModule parses every Go file under root that belongs to the module at
// root — skipping testdata, hidden directories and nested modules.
func loadModule(root, modulePath string) (*module, error) {
	loaded := &module{root: root, path: modulePath, fileSet: token.NewFileSet(),
		functions: map[string]map[string]function{}}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return directoryVerdict(root, path, entry.Name())
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		return loaded.addFile(path)
	})
	return loaded, err
}

func directoryVerdict(root, path, name string) error {
	if path == root {
		return nil
	}
	if name == "testdata" || name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".") {
		return filepath.SkipDir
	}
	if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
		return filepath.SkipDir // a nested module resolves its own imports
	}
	return nil
}

func (loaded *module) addFile(path string) error {
	parsed, err := parser.ParseFile(loaded.fileSet, path, nil, 0)
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if loaded.functions[directory] == nil {
		loaded.functions[directory] = map[string]function{}
	}
	for _, declaration := range parsed.Decls {
		loaded.addDeclaration(declaration, parsed, directory, strings.HasSuffix(path, "_test.go"))
	}
	return nil
}

func (loaded *module) addDeclaration(declaration ast.Decl, file *ast.File, directory string, isTestFile bool) {
	funcDeclaration, ok := declaration.(*ast.FuncDecl)
	if !ok || funcDeclaration.Recv != nil || funcDeclaration.Body == nil {
		return
	}
	entry := function{declaration: funcDeclaration, file: file, directory: directory}
	loaded.functions[directory][funcDeclaration.Name.Name] = entry
	if isTestFile && isTestFunction(funcDeclaration) {
		loaded.tests = append(loaded.tests, entry)
	}
}

// isTestFunction mirrors `go test`: TestXxx where Xxx does not start with a
// lower-case letter, taking exactly one *testing.T. TestMain takes *testing.M
// and is excluded by the parameter check.
func isTestFunction(declaration *ast.FuncDecl) bool {
	name := declaration.Name.Name
	if !strings.HasPrefix(name, "Test") {
		return false
	}
	if suffix := name[len("Test"):]; suffix != "" && suffix[0] >= 'a' && suffix[0] <= 'z' {
		return false
	}
	params := declaration.Type.Params.List
	return len(params) == 1 && testingTypeName(params[0].Type) == "T"
}

// directoryFor maps an import path inside this module to its directory.
func (loaded *module) directoryFor(importPath string) (string, bool) {
	if importPath == loaded.path {
		return loaded.root, true
	}
	relative, found := strings.CutPrefix(importPath, loaded.path+"/")
	if !found {
		return "", false
	}
	return filepath.Join(loaded.root, filepath.FromSlash(relative)), true
}

func (loaded *module) finding(test function) Finding {
	position := loaded.fileSet.Position(test.declaration.Pos())
	relative, err := filepath.Rel(loaded.root, position.Filename)
	if err != nil {
		relative = position.Filename
	}
	return Finding{File: filepath.ToSlash(relative), Line: position.Line, Test: test.declaration.Name.Name}
}
