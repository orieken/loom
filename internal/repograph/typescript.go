package repograph

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"
)

// typeScriptExtractor reads static import syntax. Relative specifiers and
// tsconfig `paths` / `baseUrl` aliases resolve to tracked files; a dynamic
// specifier is recorded as unresolved, never guessed (ADR-010). `extends` in
// tsconfig is not followed.
type typeScriptExtractor struct{}

var tsExtensions = []string{".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs"}

func (typeScriptExtractor) Language() string { return "typescript" }
func (typeScriptExtractor) NodeKind() string { return "module" }
func (typeScriptExtractor) Handles(file string) bool {
	return hasExtension(file, tsExtensions...) && !strings.HasSuffix(file, ".d.ts")
}

var (
	jsBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	jsLineComment  = regexp.MustCompile(`(?m)(^|[^:'"` + "`" + `\\])//[^\n]*`)
	// A multi-line template literal is almost always generated source (a code
	// emitter), whose import lines are text, not imports of this file.
	jsTemplateLiteral = regexp.MustCompile("(?s)`[^`]*`")
	jsFromImport      = regexp.MustCompile(`(?s)\b(?:import|export)\b[^'"` + "`" + `;]*?\bfrom\s*(['"])(.*?)['"]`)
	jsBareImport      = regexp.MustCompile(`\bimport\s*(['"])(.*?)['"]`)
	jsCallImport      = regexp.MustCompile(`(?:^|[^.\w$])(?:require|import)\s*\(\s*([^)]*?)\s*\)`)
)

type jsImport struct {
	spec    string
	dynamic bool
}

// blankMultilineTemplate empties a multi-line template literal. Literals are
// paired in order of appearance, so a single-line one cannot shift the pairing.
func blankMultilineTemplate(literal string) string {
	if strings.Contains(literal, "\n") {
		return "``"
	}
	return literal
}

func scanJSImports(source string) []jsImport {
	source = jsBlockComment.ReplaceAllString(source, "")
	source = jsLineComment.ReplaceAllString(source, "$1")
	source = jsTemplateLiteral.ReplaceAllStringFunc(source, blankMultilineTemplate)
	var found []jsImport
	for _, match := range jsFromImport.FindAllStringSubmatch(source, -1) {
		found = append(found, jsImport{spec: match[2]})
	}
	for _, match := range jsBareImport.FindAllStringSubmatch(source, -1) {
		found = append(found, jsImport{spec: match[2]})
	}
	for _, match := range jsCallImport.FindAllStringSubmatch(source, -1) {
		if call := classifyCallArgument(match[1]); call.spec != "" {
			found = append(found, call)
		}
	}
	return found
}

// classifyCallArgument separates require('x') from require(expr).
func classifyCallArgument(argument string) jsImport {
	if strings.Contains(argument, "\n") {
		return jsImport{}
	}
	if len(argument) >= 2 && isQuote(argument[0]) && argument[len(argument)-1] == argument[0] {
		inner := argument[1 : len(argument)-1]
		if !strings.Contains(inner, "${") && !strings.ContainsAny(inner, `'"`+"`") {
			return jsImport{spec: inner}
		}
		return jsImport{spec: inner, dynamic: true}
	}
	return jsImport{spec: argument, dynamic: true}
}

func isQuote(b byte) bool { return b == '\'' || b == '"' || b == '`' }

func (typeScriptExtractor) Extract(repo Repo) (Extraction, error) {
	files := handledFiles(repo, typeScriptExtractor{})
	resolver := tsResolver{tracked: fileSet(repo.Files()), configs: loadTSConfigs(repo)}
	var out Extraction
	for _, file := range files {
		source, err := repo.Read(file)
		if err != nil {
			continue
		}
		for _, found := range scanJSImports(string(source)) {
			resolver.resolve(file, found, &out)
		}
	}
	return out, nil
}

type tsResolver struct {
	tracked map[string]bool
	configs map[string]tsConfig
}

func (r tsResolver) resolve(file string, found jsImport, out *Extraction) {
	if found.dynamic {
		out.Unresolved = append(out.Unresolved, Unresolved{From: file, Spec: found.spec, Reason: "dynamic"})
		return
	}
	if strings.HasPrefix(found.spec, ".") {
		r.resolveRelative(file, found.spec, out)
		return
	}
	r.resolveAlias(file, found.spec, out)
}

func (r tsResolver) resolveRelative(file, spec string, out *Extraction) {
	target := path.Join(dirOf(file), spec)
	switch resolved, status := r.lookup(target); status {
	case lookupSource:
		out.Edges = append(out.Edges, FileEdge{FromFile: file, ToDir: dirOf(resolved), Confidence: Exact})
	case lookupMissing:
		out.Unresolved = append(out.Unresolved, Unresolved{From: file, Spec: spec, Reason: "not-found"})
	}
}

type lookupStatus int

const (
	lookupMissing lookupStatus = iota
	lookupSource
	lookupAsset
)

// lookup finds the tracked file a path denotes: the path itself, with an
// extension, as a directory index, or — for ESM-style `./x.js` — its .ts
// source.
func (r tsResolver) lookup(target string) (string, lookupStatus) {
	for _, candidate := range tsCandidates(target) {
		if !r.tracked[candidate] {
			continue
		}
		if (typeScriptExtractor{}).Handles(candidate) {
			return candidate, lookupSource
		}
		return candidate, lookupAsset
	}
	return "", lookupMissing
}

func tsCandidates(target string) []string {
	candidates := []string{target}
	for _, extension := range tsExtensions {
		candidates = append(candidates, target+extension, path.Join(target, "index"+extension))
	}
	if extension := path.Ext(target); extension == ".js" || extension == ".jsx" || extension == ".mjs" || extension == ".cjs" {
		stem := strings.TrimSuffix(target, extension)
		candidates = append(candidates, stem+".ts", stem+".tsx", stem+".mts", stem+".cts")
	}
	return candidates
}

func (r tsResolver) resolveAlias(file, spec string, out *Extraction) {
	config, dir, found := r.nearestConfig(file)
	if !found {
		return
	}
	targets, matched := config.expand(spec)
	for _, target := range targets {
		if resolved, status := r.lookup(path.Join(dir, target)); status == lookupSource {
			out.Edges = append(out.Edges, FileEdge{FromFile: file, ToDir: dirOf(resolved), Confidence: Exact})
			return
		} else if status == lookupAsset {
			return
		}
	}
	if matched {
		out.Unresolved = append(out.Unresolved, Unresolved{From: file, Spec: spec, Reason: "alias-not-found"})
	}
}

// nearestConfig finds the tsconfig.json governing a file.
func (r tsResolver) nearestConfig(file string) (tsConfig, string, bool) {
	for dir := dirOf(file); ; dir = path.Dir(dir) {
		if config, found := r.configs[dir]; found {
			return config, dir, true
		}
		if dir == "." || dir == "/" {
			return tsConfig{}, "", false
		}
	}
}

type tsConfig struct {
	baseURL string
	paths   map[string][]string
}

type tsConfigFile struct {
	CompilerOptions struct {
		BaseURL string              `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
}

func loadTSConfigs(repo Repo) map[string]tsConfig {
	configs := map[string]tsConfig{}
	for _, file := range repo.Files() {
		if path.Base(file) != "tsconfig.json" {
			continue
		}
		source, err := repo.Read(file)
		if err != nil {
			continue
		}
		if config, ok := parseTSConfig(source); ok {
			configs[dirOf(file)] = config
		}
	}
	return configs
}

var (
	jsoncTrailingComma = regexp.MustCompile(`,(\s*[}\]])`)
)

func parseTSConfig(source []byte) (tsConfig, bool) {
	text := jsBlockComment.ReplaceAllString(string(source), "")
	text = jsLineComment.ReplaceAllString(text, "$1")
	text = jsoncTrailingComma.ReplaceAllString(text, "$1")
	var parsed tsConfigFile
	if json.Unmarshal([]byte(text), &parsed) != nil {
		return tsConfig{}, false
	}
	return tsConfig{baseURL: parsed.CompilerOptions.BaseURL, paths: parsed.CompilerOptions.Paths}, true
}

// expand maps a bare specifier to candidate paths relative to the tsconfig
// directory. matched says a `paths` pattern applied, so a miss is worth
// reporting; a specifier no pattern claims is a third-party package.
func (c tsConfig) expand(spec string) ([]string, bool) {
	patterns := make([]string, 0, len(c.paths))
	for pattern := range c.paths {
		patterns = append(patterns, pattern)
	}
	sort.Slice(patterns, func(i, j int) bool {
		return len(patterns[i]) > len(patterns[j]) || (len(patterns[i]) == len(patterns[j]) && patterns[i] < patterns[j])
	})
	for _, pattern := range patterns {
		if wildcard, ok := matchPathPattern(pattern, spec); ok {
			return c.substitute(c.paths[pattern], wildcard), true
		}
	}
	if c.baseURL != "" {
		return []string{path.Join(c.baseURL, spec)}, false
	}
	return nil, false
}

func (c tsConfig) substitute(targets []string, wildcard string) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		out = append(out, path.Join(c.baseURL, strings.Replace(target, "*", wildcard, 1)))
	}
	return out
}

func matchPathPattern(pattern, spec string) (string, bool) {
	prefix, suffix, hasWildcard := strings.Cut(pattern, "*")
	if !hasWildcard {
		return "", pattern == spec
	}
	if len(spec) < len(prefix)+len(suffix) || !strings.HasPrefix(spec, prefix) || !strings.HasSuffix(spec, suffix) {
		return "", false
	}
	return spec[len(prefix) : len(spec)-len(suffix)], true
}
