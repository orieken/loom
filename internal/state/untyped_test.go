package state_test

// The fitness function for go-conventions.md's `any` / `interface{}` clause.
//
// The rule permits an untyped value at four boundaries — JSON and wire
// decoding, reflection subjects, variadic stdlib pass-through, and
// heterogeneous dispatch where Go has no sum type. Whether a given use sits
// at one of those is a judgement, and no check can make it.
//
// What IS checkable is the clause the judgement cannot soften: an untyped
// value must not travel inward. The domain and the use-case layer hold
// worked-out types or they hold nothing, so `any` appearing there is a
// violation whatever boundary it claims to sit near. That narrow property is
// this test, and it is deliberately not presented as enforcement of the
// whole rule.
//
// Both spellings are checked. The first version of this test matched only
// `*ast.Ident` and so saw `any` but not `interface{}` — which is the older
// spelling, the one actually present in this package, and the one that made
// the pinned exceptions below dead code. It passed, and it was vacuous. Both
// mutants are in the commit message.
//
// It pairs with internal/orchestrator/layers_test.go, which asserts the other
// half of the same idea: outer layers do not leak inward as imports, and
// untyped values do not leak inward as fields and signatures.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// innerPackages are the layers that must hold typed values.
var innerPackages = []string{"../state", "../orchestrator", "../policy"}

// permitted pins every untyped value allowed in an inner layer, keyed by
// file and the declaration holding it. The list is short on purpose: each
// entry is a reflection boundary that could not be typed, and adding to it
// should be a visible, argued edit rather than a quiet one.
//
// StageSchema holds the Go value a schema is generated FROM, and Subject
// returns it. It is handed to a reflector, and the set of types is
// heterogeneous by construction — one per stage — so typing it needs a sum
// type Go does not have. It exists to serve a fitness function (L2.25),
// which is exactly the rule's "reflection subjects" boundary.
var permitted = map[string]bool{
	"schema.go:StageSchema": true,
	"schema.go:Subject":     true,
}

func TestInnerLayersHoldTypedValues(t *testing.T) {
	for _, pkg := range innerPackages {
		t.Run(filepath.Base(pkg), func(t *testing.T) {
			for _, file := range goFilesIn(t, pkg) {
				checkDeclarationsAreTyped(t, file)
			}
		})
	}
}

func checkDeclarationsAreTyped(t *testing.T, path string) {
	t.Helper()
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	name := filepath.Base(path)
	for _, declaration := range parsed.Decls {
		for _, position := range untypedPositions(declaration) {
			if permitted[name+":"+declarationName(declaration)] {
				continue
			}
			t.Errorf("%s:%d — %s holds an untyped value. go-conventions.md permits `any` at a "+
				"boundary, and an inner layer is not one;\nnarrow it where it is received, or pin "+
				"it in `permitted` with the reason it cannot be typed",
				name, fileSet.Position(position).Line, declarationName(declaration))
		}
	}
}

// untypedPositions returns every `any` or empty `interface{}` in a
// declaration's subtree.
func untypedPositions(declaration ast.Decl) []token.Pos {
	var found []token.Pos
	ast.Inspect(declaration, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.InterfaceType:
			// A non-empty interface is a real type. Only `interface{}` is
			// the untyped one.
			if typed.Methods == nil || len(typed.Methods.List) == 0 {
				found = append(found, typed.Pos())
			}
		case *ast.Ident:
			// Obj == nil distinguishes the predeclared `any` from a
			// variable or field someone called "any" — internal/policy has
			// a YAML key by that name, and flagging it would be the false
			// positive that gets a check ignored.
			if typed.Name == "any" && typed.Obj == nil {
				found = append(found, typed.Pos())
			}
		}
		return true
	})
	return found
}

// declarationName names the declaration an untyped value sits in, so the
// pin reads as file:thing rather than a line number that shifts.
func declarationName(declaration ast.Decl) string {
	switch typed := declaration.(type) {
	case *ast.FuncDecl:
		return typed.Name.Name
	case *ast.GenDecl:
		for _, spec := range typed.Specs {
			switch specific := spec.(type) {
			case *ast.TypeSpec:
				return specific.Name.Name
			case *ast.ValueSpec:
				if len(specific.Names) > 0 {
					return specific.Names[0].Name
				}
			}
		}
	}
	return "<unnamed>"
}

func goFilesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	return files
}
