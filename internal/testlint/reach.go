package testlint

import (
	"go/ast"
	"strconv"
)

// failureMethods are the testing.TB methods that mark a test failed.
// Skip is not one: a test that can only skip cannot fail.
var failureMethods = map[string]bool{
	"Error": true, "Errorf": true, "Fatal": true, "Fatalf": true, "Fail": true, "FailNow": true,
}

// Scan returns every test in the module rooted at root that has no path to a
// failure. modulePath is the module's import path, used to follow helpers
// into other packages of the same module.
func Scan(root, modulePath string) ([]Finding, error) {
	loaded, err := loadModule(root, modulePath)
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for _, test := range loaded.tests {
		if !loaded.canFail(test, map[*ast.FuncDecl]bool{}) {
			findings = append(findings, loaded.finding(test))
		}
	}
	return findings, nil
}

// canFail reports whether any call reachable from fn fails the test. visiting
// breaks recursion between helpers; a cycle contributes nothing.
func (loaded *module) canFail(fn function, visiting map[*ast.FuncDecl]bool) bool {
	if visiting[fn.declaration] {
		return false
	}
	visiting[fn.declaration] = true
	testingValues := testingParameterNames(fn.declaration)
	found := false
	ast.Inspect(fn.declaration.Body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok && !found {
			found = loaded.callCanFail(fn, call, testingValues, visiting)
		}
		return !found
	})
	return found
}

func (loaded *module) callCanFail(caller function, call *ast.CallExpr, testingValues map[string]bool, visiting map[*ast.FuncDecl]bool) bool {
	if isFailureCall(call, testingValues) {
		return true
	}
	if !handsOverTestingValue(call, testingValues) {
		return false
	}
	callee, resolved := loaded.resolve(caller, call.Fun)
	if !resolved {
		return true // a helper we cannot see was handed t: assumed able to fail
	}
	return loaded.canFail(callee, visiting)
}

// isFailureCall matches t.Errorf(...) and friends, where t is a declared
// testing value — not any receiver that happens to have an Error method,
// such as a logger.
func isFailureCall(call *ast.CallExpr, testingValues map[string]bool) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !failureMethods[selector.Sel.Name] {
		return false
	}
	receiver, ok := selector.X.(*ast.Ident)
	return ok && testingValues[receiver.Name]
}

// handsOverTestingValue reports whether the call passes a testing value as an
// argument — the shape of every helper that can fail on the test's behalf.
// A method called on the testing value itself (t.Run, t.Cleanup) is not a
// hand-over: its closures are already inside the body being inspected.
func handsOverTestingValue(call *ast.CallExpr, testingValues map[string]bool) bool {
	if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
		if receiver, ok := selector.X.(*ast.Ident); ok && testingValues[receiver.Name] {
			return false
		}
	}
	for _, argument := range call.Args {
		if identifier, ok := argument.(*ast.Ident); ok && testingValues[identifier.Name] {
			return true
		}
	}
	return false
}

// resolve finds the declaration a call targets: a function in the caller's
// package, or an exported function of another package in this module.
func (loaded *module) resolve(caller function, target ast.Expr) (function, bool) {
	switch typed := target.(type) {
	case *ast.Ident:
		callee, ok := loaded.functions[caller.directory][typed.Name]
		return callee, ok
	case *ast.SelectorExpr:
		return loaded.resolveQualified(caller, typed)
	}
	return function{}, false
}

func (loaded *module) resolveQualified(caller function, selector *ast.SelectorExpr) (function, bool) {
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return function{}, false
	}
	importPath, ok := importPathFor(caller.file, qualifier.Name)
	if !ok {
		return function{}, false // a method on a value, not a package-qualified call
	}
	directory, ok := loaded.directoryFor(importPath)
	if !ok {
		return function{}, false
	}
	callee, ok := loaded.functions[directory][selector.Sel.Name]
	return callee, ok
}

// importPathFor maps a qualifier used in file to the path it imports. An
// unaliased import is matched on its last path element, which is the package
// name for every package in this module.
func importPathFor(file *ast.File, qualifier string) (string, bool) {
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if importName(spec, path) == qualifier {
			return path, true
		}
	}
	return "", false
}

func importName(spec *ast.ImportSpec, path string) string {
	if spec.Name != nil {
		return spec.Name.Name
	}
	for index := len(path) - 1; index >= 0; index-- {
		if path[index] == '/' {
			return path[index+1:]
		}
	}
	return path
}

// testingParameterNames collects every name declared as *testing.T,
// testing.TB, *testing.B or *testing.F in fn — its own parameters and those
// of every closure inside it, which is how t.Run subtests declare theirs.
func testingParameterNames(fn *ast.FuncDecl) map[string]bool {
	names := map[string]bool{}
	addTestingParameters(fn.Type.Params, names)
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if literal, ok := node.(*ast.FuncLit); ok {
			addTestingParameters(literal.Type.Params, names)
		}
		return true
	})
	return names
}

func addTestingParameters(params *ast.FieldList, names map[string]bool) {
	if params == nil {
		return
	}
	for _, field := range params.List {
		if testingTypeName(field.Type) == "" {
			continue
		}
		for _, name := range field.Names {
			names[name.Name] = true
		}
	}
}

// testingTypeName returns "T", "TB", "B" or "F" for a testing type
// expression, and "" for anything else.
func testingTypeName(expression ast.Expr) string {
	if star, ok := expression.(*ast.StarExpr); ok {
		expression = star.X
	}
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "testing" {
		return ""
	}
	switch selector.Sel.Name {
	case "T", "TB", "B", "F":
		return selector.Sel.Name
	}
	return ""
}
