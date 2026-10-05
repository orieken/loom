package factsheet

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"regexp"
	"strings"

	"github.com/orieken/loom/internal/repograph"
)

// describeFile reads one source file's exported symbols and whether it starts
// a program. A file in no supported language contributes nothing.
func describeFile(file string, source []byte) fileFacts {
	text := string(source)
	switch path.Ext(file) {
	case ".go":
		return describeGo(file, source)
	case ".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs":
		return describeJS(text)
	case ".py":
		return describePython(file, text)
	case ".java":
		return describeJava(file, text)
	}
	return fileFacts{}
}

func describeGo(file string, source []byte) fileFacts {
	parsed, err := parser.ParseFile(token.NewFileSet(), file, source, parser.SkipObjectResolution)
	if err != nil {
		return fileFacts{}
	}
	facts := fileFacts{}
	for _, decl := range parsed.Decls {
		facts.exports = append(facts.exports, goDeclNames(decl)...)
		if function, ok := decl.(*ast.FuncDecl); ok && function.Recv == nil && function.Name.Name == "main" && parsed.Name.Name == "main" {
			facts.entrypoint = true
		}
	}
	return facts
}

// goDeclNames lists the exported names a top-level declaration introduces.
// Methods are not listed: they belong to their type.
func goDeclNames(decl ast.Decl) []string {
	var names []string
	switch typed := decl.(type) {
	case *ast.FuncDecl:
		if typed.Recv == nil && typed.Name.IsExported() {
			names = append(names, typed.Name.Name)
		}
	case *ast.GenDecl:
		for _, spec := range typed.Specs {
			names = append(names, exportedSpecNames(spec)...)
		}
	}
	return names
}

func exportedSpecNames(spec ast.Spec) []string {
	var names []string
	switch typed := spec.(type) {
	case *ast.TypeSpec:
		if typed.Name.IsExported() {
			names = append(names, typed.Name.Name)
		}
	case *ast.ValueSpec:
		for _, name := range typed.Names {
			if name.IsExported() {
				names = append(names, name.Name)
			}
		}
	}
	return names
}

var (
	jsDeclaration = regexp.MustCompile(`(?m)^\s*export\s+(?:declare\s+)?(?:default\s+)?(?:abstract\s+)?(?:async\s+)?(?:function\*?|class|const|let|var|interface|type|enum|namespace)\s+([A-Za-z_$][\w$]*)`)
	jsDefault     = regexp.MustCompile(`(?m)^\s*export\s+default\b`)
	jsNamedList   = regexp.MustCompile(`(?s)\bexport\s*(?:type\s*)?\{([^}]*)\}`)
)

func describeJS(source string) fileFacts {
	source = repograph.StripJSComments(source)
	var exports []string
	for _, match := range jsDeclaration.FindAllStringSubmatch(source, -1) {
		exports = append(exports, match[1])
	}
	if jsDefault.MatchString(source) {
		exports = append(exports, "default")
	}
	for _, match := range jsNamedList.FindAllStringSubmatch(source, -1) {
		exports = append(exports, namedExports(match[1])...)
	}
	return fileFacts{exports: exports}
}

// namedExports reads `a, b as c` and returns the outward names [a c].
func namedExports(list string) []string {
	var names []string
	for _, part := range strings.Split(list, ",") {
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		names = append(names, fields[len(fields)-1])
	}
	return names
}

var (
	pyDefinition = regexp.MustCompile(`(?m)^(?:async[ \t]+def|def|class)[ \t]+([A-Za-z]\w*)`)
	pyConstant   = regexp.MustCompile(`(?m)^([A-Za-z]\w*)[ \t]*(?::[^=\n]*)?=`)
	pyAllLine    = regexp.MustCompile(`(?m)^__all__\s*=\s*[\[(]([^\])]*)[\])]`)
	pyMainGuard  = regexp.MustCompile(`(?m)^if\s+__name__\s*==\s*['"]__main__['"]`)
	pyQuoted     = regexp.MustCompile(`['"]([^'"]+)['"]`)
)

func describePython(file, source string) fileFacts {
	source = repograph.StripPythonNoise(source)
	facts := fileFacts{entrypoint: path.Base(file) == "__main__.py" || pyMainGuard.MatchString(source)}
	if match := pyAllLine.FindStringSubmatch(source); match != nil {
		for _, quoted := range pyQuoted.FindAllStringSubmatch(match[1], -1) {
			facts.exports = append(facts.exports, quoted[1])
		}
		return facts
	}
	for _, match := range pyDefinition.FindAllStringSubmatch(source, -1) {
		facts.exports = append(facts.exports, match[1])
	}
	for _, match := range pyConstant.FindAllStringSubmatch(source, -1) {
		facts.exports = append(facts.exports, match[1])
	}
	return facts
}

var (
	javaPublicType = regexp.MustCompile(`(?m)^\s*public\s+(?:(?:final|abstract|sealed|non-sealed|strictfp|static)\s+)*(?:class|interface|enum|record|@interface)\s+(\w+)`)
	javaMain       = regexp.MustCompile(`public\s+static\s+void\s+main\s*\(`)
)

// describeJava exports the public type named after the file — the only
// public top-level type Java allows.
func describeJava(file, source string) fileFacts {
	source = repograph.StripJavaComments(source)
	stem := strings.TrimSuffix(path.Base(file), ".java")
	facts := fileFacts{entrypoint: javaMain.MatchString(source)}
	for _, match := range javaPublicType.FindAllStringSubmatch(source, -1) {
		if match[1] == stem {
			facts.exports = append(facts.exports, stem)
		}
	}
	return facts
}
