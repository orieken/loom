package repograph

import (
	"path"
	"regexp"
	"sort"
	"strings"
)

// javaExtractor resolves imports against a package → directory index built
// from the repo's own `package` declarations. An import with no match in the
// index is treated as the JDK or a third party. Same-package references need
// no import and add no edge.
type javaExtractor struct{}

func (javaExtractor) Language() string         { return "java" }
func (javaExtractor) NodeKind() string         { return "package" }
func (javaExtractor) Handles(file string) bool { return strings.HasSuffix(file, ".java") }

var (
	javaBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	javaLineComment  = regexp.MustCompile(`(?m)//[^\n]*`)
	javaPackage      = regexp.MustCompile(`(?m)^\s*package\s+([\w.]+)\s*;`)
	javaImport       = regexp.MustCompile(`(?m)^\s*import\s+(?:static\s+)?([\w.]+?)(\.\*)?\s*;`)
)

type javaImportStatement struct {
	name     string
	wildcard bool
}

func stripJavaComments(source string) string {
	return javaLineComment.ReplaceAllString(javaBlockComment.ReplaceAllString(source, ""), "")
}

func scanJavaImports(source string) []javaImportStatement {
	var found []javaImportStatement
	for _, match := range javaImport.FindAllStringSubmatch(stripJavaComments(source), -1) {
		found = append(found, javaImportStatement{name: match[1], wildcard: match[2] != ""})
	}
	return found
}

type javaIndex struct {
	classDirs   map[string]string
	packageDirs map[string][]string
}

func buildJavaIndex(repo Repo, files []string) javaIndex {
	index := javaIndex{classDirs: map[string]string{}, packageDirs: map[string][]string{}}
	for _, file := range files {
		source, err := repo.Read(file)
		if err != nil {
			continue
		}
		match := javaPackage.FindStringSubmatch(stripJavaComments(string(source)))
		if match == nil {
			continue
		}
		class := strings.TrimSuffix(path.Base(file), ".java")
		index.classDirs[match[1]+"."+class] = dirOf(file)
		index.addPackageDir(match[1], dirOf(file))
	}
	return index
}

func (i *javaIndex) addPackageDir(pkg, dir string) {
	for _, existing := range i.packageDirs[pkg] {
		if existing == dir {
			return
		}
	}
	i.packageDirs[pkg] = append(i.packageDirs[pkg], dir)
	sort.Strings(i.packageDirs[pkg])
}

func (javaExtractor) Extract(repo Repo) (Extraction, error) {
	files := handledFiles(repo, javaExtractor{})
	index := buildJavaIndex(repo, files)
	var out Extraction
	for _, file := range files {
		source, err := repo.Read(file)
		if err != nil {
			continue
		}
		for _, statement := range scanJavaImports(string(source)) {
			index.resolve(file, statement, &out)
		}
	}
	return out, nil
}

func (i javaIndex) resolve(file string, statement javaImportStatement, out *Extraction) {
	if dir, found := i.classDir(statement.name); found {
		out.Edges = append(out.Edges, FileEdge{FromFile: file, ToDir: dir, Confidence: Exact})
		return
	}
	if statement.wildcard {
		i.resolvePackage(file, statement.name, out)
		return
	}
	if i.hasPackagePrefix(statement.name) {
		out.Unresolved = append(out.Unresolved, Unresolved{From: file, Spec: statement.name, Reason: "not-found"})
	}
}

// classDir finds the class an import names, trying shorter prefixes so a
// nested class or a static member resolves to its top-level class.
func (i javaIndex) classDir(name string) (string, bool) {
	for candidate := name; candidate != ""; candidate = parentName(candidate) {
		if dir, found := i.classDirs[candidate]; found {
			return dir, true
		}
	}
	return "", false
}

func (i javaIndex) resolvePackage(file, pkg string, out *Extraction) {
	dirs := i.packageDirs[pkg]
	confidence := Exact
	if len(dirs) > 1 {
		confidence = Approximate
	}
	for _, dir := range dirs {
		out.Edges = append(out.Edges, FileEdge{FromFile: file, ToDir: dir, Confidence: confidence})
	}
	if len(dirs) == 0 && i.hasPackagePrefix(pkg) {
		out.Unresolved = append(out.Unresolved, Unresolved{From: file, Spec: pkg + ".*", Reason: "not-found"})
	}
}

// hasPackagePrefix says the import points into a package this repo owns.
func (i javaIndex) hasPackagePrefix(name string) bool {
	for candidate := parentName(name); candidate != ""; candidate = parentName(candidate) {
		if len(i.packageDirs[candidate]) > 0 {
			return true
		}
	}
	return false
}

func parentName(name string) string {
	cut := strings.LastIndex(name, ".")
	if cut < 0 {
		return ""
	}
	return name[:cut]
}
