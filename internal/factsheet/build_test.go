package factsheet

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFactSheetsMatchGoldenFilePerFixture(t *testing.T) {
	for _, name := range []string{"go", "typescript", "python", "java", "mixed"} {
		t.Run(name, func(t *testing.T) {
			actual, err := json.MarshalIndent(sheetsFor(t, copyFixture(t, name)), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			actual = append(actual, '\n')
			golden := filepath.Join(fixturesDir, name+".facts.golden.json")
			if os.Getenv("UPDATE_GOLDEN") != "" {
				if err := os.WriteFile(golden, actual, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			expected, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if !bytes.Equal(expected, actual) {
				t.Errorf("fact sheets for %s differ from %s; run with UPDATE_GOLDEN=1 and review the diff\n%s", name, golden, actual)
			}
		})
	}
}

func TestBuildIsDeterministic(t *testing.T) {
	a, _ := json.Marshal(sheetsFor(t, copyFixture(t, "mixed")))
	b, _ := json.Marshal(sheetsFor(t, copyFixture(t, "mixed")))
	if !bytes.Equal(a, b) {
		t.Error("two builds over the same repo differ")
	}
}

// Editing a leaf's body stales that directory and its ancestors, and nothing else.
func TestEditingALeafStalesItAndItsAncestorsOnly(t *testing.T) {
	root := copyFixture(t, "go")
	before := sheetsFor(t, root)
	writeText(t, root, "internal/store/store.go", "package store\n\n// Get returns a different value.\nfunc Get() string { return \"y\" }\n")
	after := sheetsFor(t, root)
	want := []string{".", "internal", "internal/store"}
	if got := changedDirectories(before, after); !reflect.DeepEqual(got, want) {
		t.Errorf("changed directories = %v, want %v", got, want)
	}
}

func TestAddingAnExportChangesTheDirectorysFactsAndHash(t *testing.T) {
	root := copyFixture(t, "go")
	before := sheetFor(t, sheetsFor(t, root), "internal/store")
	writeText(t, root, "internal/store/more.go", "package store\n\n// Put stores.\nfunc Put() {}\n")
	after := sheetFor(t, sheetsFor(t, root), "internal/store")
	if !reflect.DeepEqual(after.Exports, []string{"Get", "Put"}) || before.InputHash == after.InputHash {
		t.Errorf("exports %v, hash changed=%v", after.Exports, before.InputHash != after.InputHash)
	}
}

func TestGoSheetFactsDependentsEntrypointsAndTests(t *testing.T) {
	sheets := sheetsFor(t, copyFixture(t, "go"))
	service := sheetFor(t, sheets, "internal/service")
	if !reflect.DeepEqual(service.Imports, []string{"internal/store"}) || !reflect.DeepEqual(service.Dependents, []string{"cmd/app"}) {
		t.Errorf("service imports %v dependents %v", service.Imports, service.Dependents)
	}
	if !reflect.DeepEqual(service.Tests, []string{"service_test.go"}) || !reflect.DeepEqual(service.Exports, []string{"Run"}) {
		t.Errorf("service tests %v exports %v", service.Tests, service.Exports)
	}
	if app := sheetFor(t, sheets, "cmd/app"); !reflect.DeepEqual(app.Entrypoints, []string{"main.go"}) {
		t.Errorf("a package main with func main is an entrypoint: %v", app.Entrypoints)
	}
}

func TestDirectoryHoldingOnlySubdirectoriesStillGetsASheet(t *testing.T) {
	sheets := sheetsFor(t, copyFixture(t, "go"))
	cmd := sheetFor(t, sheets, "cmd")
	if len(cmd.Files) != 0 || !reflect.DeepEqual(cmd.Children, []string{"cmd/app"}) {
		t.Errorf("cmd should have no files and one child: %+v", cmd)
	}
	if root := sheetFor(t, sheets, "."); !reflect.DeepEqual(root.Children, []string{"cmd", "internal"}) {
		t.Errorf("root children = %v", root.Children)
	}
}

func TestTypeScriptExportsEntrypointsAndTests(t *testing.T) {
	sheets := sheetsFor(t, copyFixture(t, "typescript"))
	shared := sheetFor(t, sheets, "src/shared")
	want := []string{"LIMIT", "MAX", "Registry", "default", "describe", "text"}
	if !reflect.DeepEqual(shared.Exports, want) {
		t.Errorf("shared exports = %v, want %v", shared.Exports, want)
	}
	root := sheetFor(t, sheets, ".")
	if !reflect.DeepEqual(root.Entrypoints, []string{"src/app.ts", "src/other.ts"}) {
		t.Errorf("package.json main and bin that are tracked: %v", root.Entrypoints)
	}
	if src := sheetFor(t, sheets, "src"); !reflect.DeepEqual(src.Tests, []string{"app.test.ts"}) {
		t.Errorf("src tests = %v", src.Tests)
	}
	if lib := sheetFor(t, sheets, "src/lib"); !reflect.DeepEqual(lib.Tests, []string{"format.spec.ts"}) || contains(lib.Exports, "unused") {
		t.Errorf("a spec file's exports are not the directory's: %+v", lib)
	}
}

func TestPythonExportsHonourAllAndSkipPrivateNames(t *testing.T) {
	sheets := sheetsFor(t, copyFixture(t, "python"))
	if billing := sheetFor(t, sheets, "app/services"); !reflect.DeepEqual(billing.Exports, []string{"charge"}) {
		t.Errorf("__all__ limits exports: %v", billing.Exports)
	}
	app := sheetFor(t, sheets, "app")
	if !reflect.DeepEqual(app.Exports, []string{"DEBUG", "MAX_RETRIES", "Settings", "run"}) {
		t.Errorf("exports = %v", app.Exports)
	}
	if !reflect.DeepEqual(app.Entrypoints, []string{"__main__.py"}) || !reflect.DeepEqual(app.Tests, []string{"test_config.py"}) {
		t.Errorf("entrypoints %v tests %v", app.Entrypoints, app.Tests)
	}
	if scripts := sheetFor(t, sheets, "scripts"); !reflect.DeepEqual(scripts.Entrypoints, []string{"run.py"}) {
		t.Errorf("a __main__ guard marks an entrypoint: %v", scripts.Entrypoints)
	}
}

func TestJavaExportsOnlyThePublicTypeNamedAfterItsFile(t *testing.T) {
	sheets := sheetsFor(t, copyFixture(t, "java"))
	app := sheetFor(t, sheets, "src/main/java/com/acme/app")
	if !reflect.DeepEqual(app.Exports, []string{"Main"}) || !reflect.DeepEqual(app.Entrypoints, []string{"Main.java"}) {
		t.Errorf("exports %v entrypoints %v (Nested must not be exported)", app.Exports, app.Entrypoints)
	}
	core := sheetFor(t, sheets, "src/main/java/com/acme/core")
	if !reflect.DeepEqual(core.Tests, []string{"EngineTest.java"}) || !reflect.DeepEqual(core.Exports, []string{"Engine"}) {
		t.Errorf("core tests %v exports %v", core.Tests, core.Exports)
	}
	if shared := sheetFor(t, sheets, "src/test/java/com/acme/shared"); !reflect.DeepEqual(shared.Tests, []string{"B.java"}) {
		t.Errorf("anything under src/test is a test: %v", shared.Tests)
	}
}

// An approximate edge must never be listed with the exact ones.
func TestApproximateEdgesAreListedApartFromExactOnes(t *testing.T) {
	sheets := sheetsFor(t, copyFixture(t, "java"))
	app := sheetFor(t, sheets, "src/main/java/com/acme/app")
	wantApproximate := []string{"src/main/java/com/acme/shared", "src/test/java/com/acme/shared"}
	if !reflect.DeepEqual(app.ApproximateImports, wantApproximate) {
		t.Errorf("approximate imports = %v", app.ApproximateImports)
	}
	for _, exact := range app.Imports {
		if contains(wantApproximate, exact) {
			t.Errorf("%s is approximate and must not be in imports", exact)
		}
	}
	if shared := sheetFor(t, sheets, "src/main/java/com/acme/shared"); !reflect.DeepEqual(shared.ApproximateDependents, []string{"src/main/java/com/acme/app"}) {
		t.Errorf("approximate dependents = %v", shared.ApproximateDependents)
	}
}

func TestMixedLanguageDirectoryMergesIntoOneSheet(t *testing.T) {
	tools := sheetFor(t, sheetsFor(t, copyFixture(t, "mixed")), "tools")
	if !reflect.DeepEqual(tools.Files, []string{"gen.go", "gen.py"}) || !reflect.DeepEqual(tools.Dependents, []string{"service"}) {
		t.Errorf("tools = %+v", tools)
	}
}

func TestDotfileKeepsItsFullNameInTheRootSheet(t *testing.T) {
	root := sheetFor(t, sheetsFor(t, copyFixture(t, "mixed")), ".")
	if !contains(root.Files, ".loomignore") {
		t.Errorf("root files = %v", root.Files)
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
