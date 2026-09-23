package diffcover

import (
	"go/ast"
	"go/parser"
	"go/token"
)

// FileHasStatements reports whether the Go file at path contains a function
// body or function literal — the only places Go's coverage instruments.
// It answers true whenever it cannot tell, so a file it fails to read is
// treated as unmeasured, never as having nothing to measure.
func FileHasStatements(path string) bool {
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		return true
	}
	found := false
	ast.Inspect(parsed, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.FuncDecl:
			found = found || typed.Body != nil
		case *ast.FuncLit:
			found = true
		}
		return !found
	})
	return found
}
