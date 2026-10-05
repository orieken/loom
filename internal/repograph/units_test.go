package repograph

import (
	"reflect"
	"strings"
	"testing"
)

func TestAnExactEdgeIsNeverDowngradedByALaterApproximateOne(t *testing.T) {
	ids := map[nodeKey]string{{"a", "go"}: "a", {"b", "go"}: "b"}
	for _, order := range [][]Confidence{{Exact, Approximate}, {Approximate, Exact}} {
		edges := map[[2]string]Edge{}
		for _, confidence := range order {
			addEdges(edges, ids, "go", []FileEdge{{FromFile: "a/x.go", ToDir: "b", Confidence: confidence}})
		}
		if got := edges[[2]string{"a", "b"}].Confidence; got != Exact {
			t.Errorf("order %v ended %q, want exact", order, got)
		}
	}
}

func TestAnEdgeToANodeThatDoesNotExistIsDropped(t *testing.T) {
	edges := map[[2]string]Edge{}
	ids := map[nodeKey]string{{"a", "go"}: "a"}
	addEdges(edges, ids, "go", []FileEdge{{FromFile: "a/x.go", ToDir: "nowhere", Confidence: Exact}, {FromFile: "a/x.go", ToDir: "a", Confidence: Exact}})
	if len(edges) != 0 {
		t.Errorf("edges to missing nodes and self edges must be dropped: %+v", edges)
	}
}

func TestGoImportResolvesToTheModuleWithTheLongestMatchingPath(t *testing.T) {
	index := goIndex{
		modules: []goModule{{dir: ".", path: "example.com/app"}, {dir: "tools", path: "example.com/app/tools"}},
		dirs:    map[string]bool{"tools/gen": true, "internal": true},
	}
	var out Extraction
	index.resolve("main.go", "example.com/app/tools/gen", &out)
	if len(out.Edges) != 1 || out.Edges[0].ToDir != "tools/gen" {
		t.Errorf("want an edge to tools/gen via the nested module, got %+v %+v", out.Edges, out.Unresolved)
	}
	module, found := index.moduleFor("example.com/application")
	if found {
		t.Errorf("a path sharing only a string prefix must not match module %+v", module)
	}
}

func TestParseModulePath(t *testing.T) {
	cases := map[string]string{
		"module example.com/a\n":                     "example.com/a",
		"// header\nmodule \"example.com/q\" // c\n": "example.com/q",
		"go 1.22\n": "",
	}
	for input, want := range cases {
		if got := parseModulePath(input); got != want {
			t.Errorf("parseModulePath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestHeadSHAIsEmptyForARepositoryWithNoCommits(t *testing.T) {
	root := t.TempDir()
	runIn(t, root, "init", "-q")
	if sha := (GitLister{}).HeadSHA(root); sha != "" {
		t.Errorf("a repo with no commits has no HEAD, got %q", sha)
	}
	populated := gitRepo(t, map[string]string{"a.txt": "x"})
	if sha := (GitLister{}).HeadSHA(populated); len(sha) != 40 {
		t.Errorf("HeadSHA = %q, want a 40-character commit", sha)
	}
}

func TestMatchPathPattern(t *testing.T) {
	cases := []struct {
		pattern, spec, wildcard string
		matched                 bool
	}{
		{"@lib/*", "@lib/format", "format", true},
		{"@lib/*", "@lib/", "", true},
		{"@lib/*", "@lib", "", false},
		{"~/*.ts", "~/a.ts", "a", true},
		{"~/*.ts", "~/a.js", "", false},
		{"exact", "exact", "", true},
		{"exact", "other", "", false},
	}
	for _, c := range cases {
		wildcard, matched := matchPathPattern(c.pattern, c.spec)
		if matched != c.matched || wildcard != c.wildcard {
			t.Errorf("matchPathPattern(%q, %q) = (%q, %v), want (%q, %v)", c.pattern, c.spec, wildcard, matched, c.wildcard, c.matched)
		}
	}
}

func TestTSConfigPrefersTheMoreSpecificPatternThenBaseURL(t *testing.T) {
	config, ok := parseTSConfig([]byte(`{ /* c */ "compilerOptions": { "baseUrl": "src", "paths": {
		"@/*": ["a/*"], "@/ui/*": ["ui/*",], }, }, }`))
	if !ok {
		t.Fatal("jsonc with comments and trailing commas must parse")
	}
	targets, matched := config.expand("@/ui/button")
	if !matched || !reflect.DeepEqual(targets, []string{"src/ui/button"}) {
		t.Errorf("most specific pattern must win: %v %v", targets, matched)
	}
	targets, matched = config.expand("lodash")
	if matched || !reflect.DeepEqual(targets, []string{"src/lodash"}) {
		t.Errorf("an unmatched bare specifier is tried under baseUrl without being reportable: %v %v", targets, matched)
	}
	if _, ok := parseTSConfig([]byte(`{not json`)); ok {
		t.Error("an unparseable tsconfig must be ignored, not trusted")
	}
}

func TestNearestTSConfigWalksUpToTheGoverningDirectory(t *testing.T) {
	resolver := tsResolver{configs: map[string]tsConfig{"web": {baseURL: "."}}}
	if _, dir, found := resolver.nearestConfig("web/src/deep/x.ts"); !found || dir != "web" {
		t.Errorf("want web, got %q found=%v", dir, found)
	}
	if _, _, found := resolver.nearestConfig("other/x.ts"); found {
		t.Error("a file outside every tsconfig has none")
	}
}

func TestSplitImportNames(t *testing.T) {
	got := splitImportNames("(a, b as c,\n d)")
	if want := []string{"a", "b", "d"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := splitImportNames("*"); len(got) != 0 {
		t.Errorf("a star import names nothing: %v", got)
	}
}

func TestJavaPackageDirsAreDeduplicatedAndSorted(t *testing.T) {
	index := javaIndex{packageDirs: map[string][]string{}}
	for _, dir := range []string{"z", "a", "z"} {
		index.addPackageDir("p", dir)
	}
	if want := []string{"a", "z"}; !reflect.DeepEqual(index.packageDirs["p"], want) {
		t.Errorf("got %v, want %v", index.packageDirs["p"], want)
	}
}

func TestJavaImportOfAMissingPackageInsideARepoPackageIsUnresolved(t *testing.T) {
	index := javaIndex{classDirs: map[string]string{}, packageDirs: map[string][]string{"com.acme.core": {"core"}}}
	var out Extraction
	index.resolve("A.java", javaImportStatement{name: "com.acme.core.sub", wildcard: true}, &out)
	if len(out.Unresolved) != 1 || out.Unresolved[0].Spec != "com.acme.core.sub.*" {
		t.Errorf("wildcard of an absent sub-package: %+v", out)
	}
	if parentName("single") != "" {
		t.Error("a name with no dot has no parent")
	}
}

func TestGraphHashCoversContentButNotMetadata(t *testing.T) {
	base := Graph{Nodes: []Node{{ID: "a"}}}
	same := hashContent(base.Nodes, nil, nil)
	if same != hashContent([]Node{{ID: "a"}}, nil, nil) {
		t.Error("hash must be stable for equal content")
	}
	if same == hashContent([]Node{{ID: "b"}}, nil, nil) {
		t.Error("hash must change when content changes")
	}
}

func TestGeneratedSchemaPinsTheSchemaVersion(t *testing.T) {
	raw, err := GenerateSchema()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"const": 1`) {
		t.Errorf("schema_version must be pinned to the constant:\n%s", raw)
	}
}
