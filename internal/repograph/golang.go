package repograph

import (
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strconv"
	"strings"
)

// goExtractor reads imports with the standard library's parser, so it needs
// no toolchain, no compiling tree and no network (ADR-010). Build tags are
// not evaluated, and _test.go files contribute no edges.
type goExtractor struct{}

func (goExtractor) Language() string { return "go" }
func (goExtractor) NodeKind() string { return "package" }
func (goExtractor) Handles(file string) bool {
	return strings.HasSuffix(file, ".go")
}

type goModule struct {
	dir  string
	path string
}

func (goExtractor) Extract(repo Repo) (Extraction, error) {
	files := handledFiles(repo, goExtractor{})
	index := goIndex{modules: findGoModules(repo), dirs: goPackageDirs(files)}
	var out Extraction
	for _, file := range files {
		if !strings.HasSuffix(file, "_test.go") {
			index.extractFile(repo, file, &out)
		}
	}
	return out, nil
}

type goIndex struct {
	modules []goModule
	dirs    map[string]bool
}

func goPackageDirs(files []string) map[string]bool {
	dirs := map[string]bool{}
	for _, file := range files {
		dirs[dirOf(file)] = true
	}
	return dirs
}

func (g goIndex) extractFile(repo Repo, file string, out *Extraction) {
	source, err := repo.Read(file)
	if err != nil {
		return
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), file, source, parser.ImportsOnly)
	if err != nil {
		out.Unresolved = append(out.Unresolved, Unresolved{From: file, Reason: "parse-error"})
		return
	}
	for _, spec := range parsed.Imports {
		importPath, _ := strconv.Unquote(spec.Path.Value)
		g.resolve(file, importPath, out)
	}
}

// resolve turns an import path into an edge when it lies inside a module of
// this repo. Anything else is the standard library or a third party.
func (g goIndex) resolve(file, importPath string, out *Extraction) {
	module, found := g.moduleFor(importPath)
	if !found {
		return
	}
	target := path.Join(module.dir, strings.TrimPrefix(strings.TrimPrefix(importPath, module.path), "/"))
	if g.dirs[target] {
		out.Edges = append(out.Edges, FileEdge{FromFile: file, ToDir: target, Confidence: Exact})
		return
	}
	out.Unresolved = append(out.Unresolved, Unresolved{From: file, Spec: importPath, Reason: "not-found"})
}

// moduleFor picks the module with the longest path that prefixes the import.
func (g goIndex) moduleFor(importPath string) (goModule, bool) {
	var best goModule
	found := false
	for _, module := range g.modules {
		if importPath != module.path && !strings.HasPrefix(importPath, module.path+"/") {
			continue
		}
		if !found || len(module.path) > len(best.path) {
			best, found = module, true
		}
	}
	return best, found
}

func findGoModules(repo Repo) []goModule {
	var modules []goModule
	for _, file := range repo.Files() {
		if path.Base(file) != "go.mod" {
			continue
		}
		if source, err := repo.Read(file); err == nil {
			if modulePath := parseModulePath(string(source)); modulePath != "" {
				modules = append(modules, goModule{dir: dirOf(file), path: modulePath})
			}
		}
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].dir < modules[j].dir })
	return modules
}

func parseModulePath(goMod string) string {
	for _, line := range strings.Split(goMod, "\n") {
		if before, _, found := strings.Cut(line, "//"); found {
			line = before
		}
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return strings.Trim(fields[1], `"`)
		}
	}
	return ""
}
