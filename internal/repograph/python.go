package repograph

import (
	"path"
	"regexp"
	"strings"
)

// pythonExtractor reads import statements. A relative import is exact. An
// absolute import is exact only when exactly one of the repo root or `src/`
// satisfies it; a match found only by searching the importing file's own
// directories, or an ambiguous one, is approximate — sys.path is runtime
// state a scanner cannot see (ADR-010).
type pythonExtractor struct{}

func (pythonExtractor) Language() string         { return "python" }
func (pythonExtractor) NodeKind() string         { return "module" }
func (pythonExtractor) Handles(file string) bool { return strings.HasSuffix(file, ".py") }

var (
	pyTripleDouble = regexp.MustCompile(`(?s)""".*?"""`)
	pyTripleSingle = regexp.MustCompile(`(?s)'''.*?'''`)
	pyComment      = regexp.MustCompile(`(?m)#[^\n]*`)
	pyContinuation = regexp.MustCompile(`\\\n`)
	pyImport       = regexp.MustCompile(`(?m)^[ \t]*import[ \t]+([\w., \t]+)`)
	pyFromImport   = regexp.MustCompile(`(?m)^[ \t]*from[ \t]+(\.*)([\w.]*)[ \t]+import[ \t]+(\([^)]*\)|[^\n]*)`)
)

type pyImportStatement struct {
	level  int
	module string
	names  []string
}

func scanPythonImports(source string) []pyImportStatement {
	source = pyTripleDouble.ReplaceAllString(source, "")
	source = pyTripleSingle.ReplaceAllString(source, "")
	source = pyComment.ReplaceAllString(source, "")
	source = pyContinuation.ReplaceAllString(source, " ")
	var found []pyImportStatement
	for _, match := range pyImport.FindAllStringSubmatch(source, -1) {
		for _, name := range splitImportNames(match[1]) {
			found = append(found, pyImportStatement{module: name})
		}
	}
	for _, match := range pyFromImport.FindAllStringSubmatch(source, -1) {
		found = append(found, pyImportStatement{level: len(match[1]), module: match[2], names: splitImportNames(match[3])})
	}
	return found
}

// splitImportNames reads `a, b as c, (d, e)` into [a b d e].
func splitImportNames(list string) []string {
	list = strings.NewReplacer("(", " ", ")", " ").Replace(list)
	var names []string
	for _, part := range strings.Split(list, ",") {
		if fields := strings.Fields(part); len(fields) > 0 && fields[0] != "*" {
			names = append(names, fields[0])
		}
	}
	return names
}

func (pythonExtractor) Extract(repo Repo) (Extraction, error) {
	files := handledFiles(repo, pythonExtractor{})
	resolver := newPyResolver(repo.Files())
	var out Extraction
	for _, file := range files {
		source, err := repo.Read(file)
		if err != nil {
			continue
		}
		for _, statement := range scanPythonImports(string(source)) {
			resolver.resolve(file, statement, &out)
		}
	}
	return out, nil
}

type pyResolver struct {
	tracked map[string]bool
	// dirsWithSource holds every directory that directly contains a .py file,
	// which is what makes it a (namespace) package.
	dirsWithSource map[string]bool
}

func newPyResolver(files []string) pyResolver {
	resolver := pyResolver{tracked: fileSet(files), dirsWithSource: map[string]bool{}}
	for _, file := range files {
		if strings.HasSuffix(file, ".py") {
			resolver.dirsWithSource[dirOf(file)] = true
		}
	}
	return resolver
}

func (r pyResolver) resolve(file string, statement pyImportStatement, out *Extraction) {
	if statement.module == "__future__" {
		return
	}
	if statement.level > 0 {
		r.resolveRelative(file, statement, out)
		return
	}
	r.resolveAbsolute(file, statement, out)
}

func (r pyResolver) resolveRelative(file string, statement pyImportStatement, out *Extraction) {
	base := dirOf(file)
	for i := 1; i < statement.level; i++ {
		base = path.Dir(base)
	}
	specifier := strings.Repeat(".", statement.level) + statement.module
	target := base
	if statement.module != "" {
		resolved, found := r.moduleDir(base, statement.module)
		if !found {
			out.Unresolved = append(out.Unresolved, Unresolved{From: file, Spec: specifier, Reason: "not-found"})
			return
		}
		target = resolved
	}
	out.Edges = append(out.Edges, FileEdge{FromFile: file, ToDir: target, Confidence: Exact})
	r.addSubmodules(file, base, statement, Exact, out)
}

// addSubmodules adds an edge for each imported name that is itself a module:
// `from pkg import mod`.
func (r pyResolver) addSubmodules(file, base string, statement pyImportStatement, confidence Confidence, out *Extraction) {
	dotted := statement.module
	for _, name := range statement.names {
		qualified := strings.TrimPrefix(dotted+"."+name, ".")
		if dir, found := r.moduleDir(base, qualified); found {
			out.Edges = append(out.Edges, FileEdge{FromFile: file, ToDir: dir, Confidence: confidence})
		}
	}
}

func (r pyResolver) resolveAbsolute(file string, statement pyImportStatement, out *Extraction) {
	roots, confidence := r.sourceRoots(file, statement.module)
	if len(roots) == 0 {
		if r.topLevelExists(file, statement.module) {
			out.Unresolved = append(out.Unresolved, Unresolved{From: file, Spec: statement.module, Reason: "not-found"})
		}
		return
	}
	for _, root := range roots {
		dir, _ := r.moduleDir(root, statement.module)
		out.Edges = append(out.Edges, FileEdge{FromFile: file, ToDir: dir, Confidence: confidence})
		r.addSubmodules(file, root, statement, confidence, out)
	}
}

// sourceRoots returns the roots that satisfy a dotted module and how sure we
// are. The repo root and src/ come first; the importing file's own directory
// and its ancestors are a fallback.
func (r pyResolver) sourceRoots(file, module string) ([]string, Confidence) {
	var roots []string
	for _, root := range []string{".", "src"} {
		if _, found := r.moduleDir(root, module); found {
			roots = append(roots, root)
		}
	}
	if len(roots) == 1 {
		return roots, Exact
	}
	if len(roots) > 1 {
		return roots, Approximate
	}
	return r.ancestorRoots(file, module), Approximate
}

func (r pyResolver) ancestorRoots(file, module string) []string {
	var roots []string
	for dir := dirOf(file); ; dir = path.Dir(dir) {
		if _, found := r.moduleDir(dir, module); found {
			roots = append(roots, dir)
		}
		if dir == "." || dir == "/" {
			return roots
		}
	}
}

// topLevelExists tells a first-party package with a missing submodule apart
// from a third-party or standard-library import.
func (r pyResolver) topLevelExists(file, module string) bool {
	top, _, _ := strings.Cut(module, ".")
	if top == "" {
		return false
	}
	roots, _ := r.sourceRoots(file, top)
	return len(roots) > 0
}

// moduleDir resolves a dotted module under root to the directory that holds
// it: its file's directory, its package directory, or a namespace package.
func (r pyResolver) moduleDir(root, dotted string) (string, bool) {
	base := path.Join(root, strings.ReplaceAll(dotted, ".", "/"))
	switch {
	case r.tracked[base+".py"]:
		return dirOf(base + ".py"), true
	case r.tracked[base+"/__init__.py"]:
		return base, true
	case r.hasPythonFiles(base):
		return base, true
	}
	return "", false
}

func (r pyResolver) hasPythonFiles(dir string) bool {
	return r.dirsWithSource[dir]
}
