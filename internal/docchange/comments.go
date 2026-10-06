package docchange

import (
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/orieken/loom/internal/factsheet"
	"github.com/orieken/loom/internal/genblock"
	"github.com/orieken/loom/internal/state"
)

// sourceEdit is a planned change to a source file's documentation. skip, when
// set, says why nothing is staged. Source documentation a human wrote is never
// touched: only a file this tool created, or a block it already owns, is
// updated.
type sourceEdit struct {
	target  string
	content string
	status  genblock.Status
	reason  string
	skip    string
}

// commentBody is the plain text placed in a doc comment. Unlike the README it
// has no markdown, since it sits in a language's own comment syntax.
func commentBody(doc state.RepoDocState) string {
	var out strings.Builder
	if doc.Purpose == "" {
		out.WriteString("Purpose not established by the facts.")
	} else {
		out.WriteString(doc.Purpose)
	}
	if len(doc.Responsibilities) > 0 {
		out.WriteString("\n\nResponsibilities:")
		for _, responsibility := range doc.Responsibilities {
			out.WriteString("\n  - " + responsibility)
		}
	}
	return escapeComment(out.String())
}

// escapeComment keeps a body from closing the comment it sits in.
func escapeComment(text string) string {
	return strings.NewReplacer("*/", "* /", `"""`, "'''").Replace(text)
}

// commentEdits plans documentation edits for every language the directory
// holds. Languages with no applicable file contribute nothing.
func commentEdits(root string, doc state.RepoDocState, sheet factsheet.FactSheet) []sourceEdit {
	body := commentBody(doc)
	var edits []sourceEdit
	for _, adapter := range []func(string, factsheet.FactSheet, string) *sourceEdit{goDoc, javaDoc, pythonDoc, scriptDoc} {
		if edit := adapter(root, sheet, body); edit != nil {
			edits = append(edits, *edit)
		}
	}
	return edits
}

// hasFile reports whether the directory's own file list holds this exact name.
func hasFile(sheet factsheet.FactSheet, name string) bool {
	for _, file := range sheet.Files {
		if file == name {
			return true
		}
	}
	return false
}

func existing(root, dir, name string) (target, content string, found bool) {
	target = path.Join(dir, name)
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(target)))
	return target, string(raw), err == nil
}

func skipped(target, reason string) *sourceEdit { return &sourceEdit{target: target, skip: reason} }

// upsertWhole runs genblock over a file this tool owns the block in.
func upsertWhole(target, text string, syntax genblock.Syntax, body string) *sourceEdit {
	updated, status, reason := syntax.Upsert(text, true, body)
	return &sourceEdit{target: target, content: updated, status: status, reason: reason}
}

// goDoc writes a package comment into doc.go, but only when the package has no
// documentation yet or doc.go is already ours.
func goDoc(root string, sheet factsheet.FactSheet, body string) *sourceEdit {
	name, otherDoc := goPackageFacts(root, sheet)
	if name == "" {
		return nil
	}
	target, text, found := existing(root, sheet.Path, "doc.go")
	syntax := genblock.LineComment("//")
	if found {
		if _, _, owned := genblock.Extract(text); !owned {
			return skipped(target, "doc.go exists and was not generated")
		}
		return upsertWhole(target, text, syntax, "Package "+name+": "+body)
	}
	if otherDoc != "" {
		return skipped(target, "the package already has a doc comment in "+otherDoc)
	}
	created, _, _ := syntax.Upsert("", false, "Package "+name+": "+body)
	return &sourceEdit{target: target, content: created + "package " + name + "\n", status: genblock.Created}
}

// goPackageFacts returns the package name and the first non-doc.go file that
// already carries a package comment.
func goPackageFacts(root string, sheet factsheet.FactSheet) (name, documentedIn string) {
	for _, file := range sheet.Files {
		if !isGoSource(file) {
			continue
		}
		packageName, hasDoc := parseGoPackage(filepath.Join(root, filepath.FromSlash(sheet.Path), file))
		if packageName == "" {
			continue
		}
		name = packageName
		if documentedIn == "" && hasDoc && file != "doc.go" {
			documentedIn = file
		}
	}
	return name, documentedIn
}

func isGoSource(file string) bool {
	return strings.HasSuffix(file, ".go") && !strings.HasSuffix(file, "_test.go")
}

// parseGoPackage reads just the package clause and its doc comment.
func parseGoPackage(file string) (name string, hasDoc bool) {
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ParseComments|parser.PackageClauseOnly)
	if err != nil {
		return "", false
	}
	return parsed.Name.Name, parsed.Doc != nil
}

var javaPackageDecl = regexp.MustCompile(`(?m)^\s*package\s+([\w.]+)\s*;`)

// javaDoc writes package-info.java when absent or already generated.
func javaDoc(root string, sheet factsheet.FactSheet, body string) *sourceEdit {
	pkg := javaPackageName(root, sheet)
	if pkg == "" {
		return nil
	}
	target, text, found := existing(root, sheet.Path, "package-info.java")
	syntax := genblock.BlockComment()
	if found {
		if _, _, owned := genblock.Extract(text); !owned {
			return skipped(target, "package-info.java exists and was not generated")
		}
		return upsertWhole(target, text, syntax, body)
	}
	created, _, _ := syntax.Upsert("", false, body)
	return &sourceEdit{target: target, content: "/**\n" + created + " */\npackage " + pkg + ";\n", status: genblock.Created}
}

func javaPackageName(root string, sheet factsheet.FactSheet) string {
	for _, file := range sheet.Files {
		if !strings.HasSuffix(file, ".java") || file == "package-info.java" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(sheet.Path), file))
		if err != nil {
			continue
		}
		if match := javaPackageDecl.FindStringSubmatch(string(raw)); match != nil {
			return match[1]
		}
	}
	return ""
}

// pythonDoc puts a module docstring at the top of an existing __init__.py. It
// never creates the file (that would turn a namespace package into a regular
// one) and never touches a docstring a human wrote.
func pythonDoc(root string, sheet factsheet.FactSheet, body string) *sourceEdit {
	target, text, found := existing(root, sheet.Path, "__init__.py")
	if !found || !hasFile(sheet, "__init__.py") {
		return nil
	}
	syntax := genblock.Docstring()
	if _, _, owned := genblock.Extract(text); owned {
		return upsertWhole(target, text, syntax, body)
	}
	if startsWithString(text) {
		return skipped(target, "__init__.py already has a docstring")
	}
	block := syntax.Render(body)
	return &sourceEdit{target: target, content: "\"\"\"\n" + block + "\n\"\"\"\n" + text, status: genblock.Appended}
}

func startsWithString(text string) bool {
	trimmed := strings.TrimLeft(text, " \t\r\n")
	return strings.HasPrefix(trimmed, `"`) || strings.HasPrefix(trimmed, "'") || strings.HasPrefix(trimmed, `r"`) || strings.HasPrefix(trimmed, `u"`)
}

// scriptIndexFiles are the entry files a TypeScript or JavaScript module may
// be documented in, in order of preference.
var scriptIndexFiles = []string{"index.ts", "index.tsx", "index.js", "index.jsx", "index.mjs"}

// scriptDoc prepends a JSDoc `@module` block to an existing index file. It
// never creates one and never touches a doc comment a human wrote.
func scriptDoc(root string, sheet factsheet.FactSheet, body string) *sourceEdit {
	for _, name := range scriptIndexFiles {
		if !hasFile(sheet, name) {
			continue
		}
		target, text, found := existing(root, sheet.Path, name)
		if found {
			return scriptEdit(target, text, body)
		}
	}
	return nil
}

func scriptEdit(target, text, body string) *sourceEdit {
	syntax := genblock.BlockComment()
	if _, _, owned := genblock.Extract(text); owned {
		return upsertWhole(target, text, syntax, body)
	}
	if strings.HasPrefix(strings.TrimLeft(text, " \t\r\n"), "/**") {
		return skipped(target, "the file already starts with a doc comment")
	}
	return &sourceEdit{target: target, content: "/**\n" + syntax.Render(body) + "\n * @module\n */\n" + text, status: genblock.Appended}
}
