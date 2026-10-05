package repograph

import "testing"

func TestGoExtractorMakesExactEdgesAndSkipsTestImports(t *testing.T) {
	graph := indexFixture(t, "go").Graph
	if !hasEdge(graph, "cmd/app", "internal/service", Exact) || !hasEdge(graph, "internal/service", "internal/store", Exact) {
		t.Errorf("expected exact in-module edges, got %+v", graph.Edges)
	}
	if hasEdge(graph, "internal/service", "internal/testsupport", Exact) {
		t.Error("an import made only by a _test.go file became an edge")
	}
	if !hasUnresolved(graph, "internal/service/legacy.go", "example.com/app/internal/missing", "not-found") {
		t.Errorf("in-module import of a missing package was not recorded: %+v", graph.Unresolved)
	}
}

func TestTypeScriptExtractorResolvesRelativeAliasAndIndexImports(t *testing.T) {
	graph := indexFixture(t, "typescript").Graph
	for _, to := range []string{"src/util", "src/lib"} {
		if !hasEdge(graph, "src", to, Exact) {
			t.Errorf("missing exact edge src -> %s in %+v", to, graph.Edges)
		}
	}
	if !hasEdge(graph, "src/lib", "src/shared", Exact) || !hasEdge(graph, "src/util", "src/shared", Exact) {
		t.Error("missing a parent-relative edge into src/shared")
	}
}

func TestTypeScriptExtractorNeverGuessesDynamicOrMissingImports(t *testing.T) {
	graph := indexFixture(t, "typescript").Graph
	for _, want := range []Unresolved{
		{From: "src/app.ts", Spec: "./plugins/${name}", Reason: "dynamic"},
		{From: "src/app.ts", Spec: "./missing", Reason: "not-found"},
		{From: "src/app.ts", Spec: "@ghost/thing", Reason: "alias-not-found"},
	} {
		if !hasUnresolved(graph, want.From, want.Spec, want.Reason) {
			t.Errorf("expected unresolved %+v in %+v", want, graph.Unresolved)
		}
	}
	assertNoneReported(t, graph, "./commented-out", "./block-commented-out", "react")
}

func assertNoneReported(t *testing.T, graph Graph, specs ...string) {
	t.Helper()
	for _, item := range graph.Unresolved {
		for _, spec := range specs {
			if item.Spec == spec {
				t.Errorf("%q must not be reported: %+v", spec, item)
			}
		}
	}
}

func TestTypeScriptRequireOfJsPathResolvesToTsSource(t *testing.T) {
	graph := indexFixture(t, "typescript").Graph
	for _, item := range graph.Unresolved {
		if item.Spec == "./helpers.js" {
			t.Errorf("./helpers.js should resolve to helpers.ts, got unresolved %+v", item)
		}
	}
}

func TestPythonRelativeImportsAreExactAndDocstringsAreIgnored(t *testing.T) {
	graph := indexFixture(t, "python").Graph
	if !hasEdge(graph, "app", "app/models", Exact) || !hasEdge(graph, "app", "app/services", Exact) {
		t.Errorf("missing exact edges from app: %+v", graph.Edges)
	}
	if !hasEdge(graph, "app/models", "app/services", Exact) {
		t.Error("`from ..services.billing import charge` should be an exact edge")
	}
	for _, item := range graph.Unresolved {
		if item.Spec == "fake.thing" || item.Spec == "also_fake" {
			t.Errorf("import inside a docstring or comment was reported: %+v", item)
		}
	}
}

func TestPythonAbsoluteImportFoundOnlyBySearchingAncestorsIsApproximate(t *testing.T) {
	graph := indexFixture(t, "python").Graph
	if !hasEdge(graph, "scripts", "app", Exact) {
		t.Error("`from app.main import run` resolves from the repo root and should be exact")
	}
	if !hasEdge(graph, "scripts/sub", "scripts", Approximate) {
		t.Errorf("`import helper` found only in an ancestor directory must be approximate: %+v", graph.Edges)
	}
	if hasEdge(graph, "scripts", "scripts", Exact) || hasEdge(graph, "scripts", "scripts", Approximate) {
		t.Error("a self edge leaked into the graph")
	}
	if !hasUnresolved(graph, "scripts/run.py", "app.nonexistent", "not-found") {
		t.Errorf("missing submodule of a first-party package not recorded: %+v", graph.Unresolved)
	}
}

func TestJavaExtractorResolvesClassesStaticMembersAndWildcards(t *testing.T) {
	graph := indexFixture(t, "java").Graph
	main := "src/main/java/com/acme/app"
	for _, to := range []string{"core", "util", "core/model"} {
		if !hasEdge(graph, main, "src/main/java/com/acme/"+to, Exact) {
			t.Errorf("expected exact edge to %s: %+v", to, graph.Edges)
		}
	}
	if !hasUnresolved(graph, main+"/Main.java", "com.acme.core.Missing", "not-found") {
		t.Errorf("a missing class in an in-repo package must be recorded: %+v", graph.Unresolved)
	}
	assertNoneReported(t, graph, "com.acme.missing.Ghost", "java.util.List")
}

func TestJavaWildcardSpanningSeveralDirectoriesIsApproximate(t *testing.T) {
	graph := indexFixture(t, "java").Graph
	main := "src/main/java/com/acme/app"
	for _, to := range []string{"src/main/java/com/acme/shared", "src/test/java/com/acme/shared"} {
		if !hasEdge(graph, main, to, Approximate) {
			t.Errorf("expected approximate edge to %s in %+v", to, graph.Edges)
		}
	}
}

func TestMixedRepoSplitsADirectoryHoldingTwoLanguages(t *testing.T) {
	graph := indexFixture(t, "mixed").Graph
	if !hasNode(graph, "tools[go]") || !hasNode(graph, "tools[python]") {
		t.Errorf("a directory with two languages must yield one node per language: %+v", graph.Nodes)
	}
	if !hasEdge(graph, "service", "tools[go]", Exact) {
		t.Error("the Go edge must land on the Go node of the shared directory")
	}
	if !hasEdge(graph, "tools[python]", "web[python]", Exact) {
		t.Errorf("the Python edge must stay within Python nodes: %+v", graph.Edges)
	}
}

func TestTypeScriptMethodNamedRequireIsNotAnImport(t *testing.T) {
	graph := indexFixture(t, "typescript").Graph
	for _, item := range graph.Unresolved {
		if item.From == "src/shim.ts" {
			t.Errorf("registry.require(thing) is a method call, not an import: %+v", item)
		}
	}
}

// Imports inside a code emitter's template literal are generated text. This
// was a real false positive when the index first ran over this repository.
func TestTypeScriptImportsInsideTemplateLiteralsAreNotImports(t *testing.T) {
	graph := indexFixture(t, "typescript").Graph
	for _, item := range graph.Unresolved {
		if item.From == "src/codegen.ts" {
			t.Errorf("generated source inside a template literal was read as an import: %+v", item)
		}
	}
}
