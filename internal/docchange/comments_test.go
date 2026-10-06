package docchange

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/orieken/loom/internal/factsheet"
	"github.com/orieken/loom/internal/genblock"
)

func sheetOf(dir string, files ...string) factsheet.FactSheet {
	return factsheet.FactSheet{Path: dir, Files: files}
}

func seed(t *testing.T, root, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		writeFile(t, root, filepath.ToSlash(filepath.Join(dir, name)), content)
	}
}

func TestGoDocIsCreatedAsADocGoWithTheMarkersAndAPackageClause(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "lib", map[string]string{"lib.go": "package lib\n"})
	edit := goDoc(root, sheetOf("lib", "lib.go"), "Does things.")
	if edit == nil || edit.target != "lib/doc.go" || edit.status != genblock.Created {
		t.Fatalf("%+v", edit)
	}
	if !strings.Contains(edit.content, "// loom:generated:start hash=") || !strings.Contains(edit.content, "// Package lib: Does things.") || !strings.HasSuffix(edit.content, "// loom:generated:end\npackage lib\n") {
		t.Errorf("the comment must sit directly above the package clause:\n%s", edit.content)
	}
}

func TestGoDocIsSkippedWhenThePackageAlreadyHasADocComment(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "lib", map[string]string{"lib.go": "// Package lib was documented by a person.\npackage lib\n", "other.go": "package lib\n"})
	edit := goDoc(root, sheetOf("lib", "lib.go", "other.go"), "x")
	if edit == nil || !strings.Contains(edit.skip, "already has a doc comment in lib.go") {
		t.Errorf("%+v", edit)
	}
}

func TestGoDocNeverTouchesAHandWrittenDocGo(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "lib", map[string]string{"lib.go": "package lib\n", "doc.go": "// Package lib: mine.\npackage lib\n"})
	if edit := goDoc(root, sheetOf("lib", "lib.go", "doc.go"), "x"); edit == nil || !strings.Contains(edit.skip, "was not generated") {
		t.Errorf("%+v", edit)
	}
}

func TestGoDocUpdatesADocGoItOwnsAndFlagsAHandEditedOne(t *testing.T) {
	root := t.TempDir()
	created := goDoc(rootWith(t, root, "lib", "package lib\n"), sheetOf("lib", "lib.go"), "one")
	seed(t, root, "lib", map[string]string{"doc.go": created.content})
	updated := goDoc(root, sheetOf("lib", "lib.go", "doc.go"), "two")
	if updated == nil || updated.status != genblock.Replaced || !strings.Contains(updated.content, "Package lib: two") || !strings.HasSuffix(updated.content, "package lib\n") {
		t.Fatalf("%+v", updated)
	}
	seed(t, root, "lib", map[string]string{"doc.go": strings.Replace(created.content, "one", "one, edited", 1)})
	if conflict := goDoc(root, sheetOf("lib", "lib.go", "doc.go"), "two"); conflict == nil || conflict.status != genblock.Conflict {
		t.Errorf("%+v", conflict)
	}
}

func rootWith(t *testing.T, root, dir, goFile string) string {
	t.Helper()
	seed(t, root, dir, map[string]string{"lib.go": goFile})
	return root
}

func TestGoDocIgnoresDirectoriesWithNoGoPackageAndTestOnlyFiles(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "docs", map[string]string{"README.md": "x"})
	if goDoc(root, sheetOf("docs", "README.md"), "x") != nil {
		t.Error("no Go files, nothing to document")
	}
	seed(t, root, "t", map[string]string{"a_test.go": "package t\n"})
	if goDoc(root, sheetOf("t", "a_test.go"), "x") != nil {
		t.Error("a package with only test files has no package to document")
	}
}

func javaSeed(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	seed(t, root, "src/acme", map[string]string{"A.java": "package com.acme.core;\n\npublic class A {}\n"})
	return root
}

func TestJavaDocCreatesPackageInfoWithTheRightPackage(t *testing.T) {
	edit := javaDoc(javaSeed(t), sheetOf("src/acme", "A.java"), "Core.")
	if edit == nil || edit.target != "src/acme/package-info.java" || edit.status != genblock.Created {
		t.Fatalf("%+v", edit)
	}
	if !strings.HasPrefix(edit.content, "/**\n * loom:generated:start hash=") || !strings.HasSuffix(edit.content, " */\npackage com.acme.core;\n") {
		t.Errorf("%s", edit.content)
	}
}

func TestJavaDocLeavesAHumanPackageInfoAlone(t *testing.T) {
	root := javaSeed(t)
	seed(t, root, "src/acme", map[string]string{"package-info.java": "/** Mine. */\npackage com.acme.core;\n"})
	if skip := javaDoc(root, sheetOf("src/acme", "A.java", "package-info.java"), "x"); skip == nil || skip.skip == "" {
		t.Errorf("%+v", skip)
	}
}

func TestJavaDocUpdatesAPackageInfoItOwns(t *testing.T) {
	root := javaSeed(t)
	created := javaDoc(root, sheetOf("src/acme", "A.java"), "Core.")
	seed(t, root, "src/acme", map[string]string{"package-info.java": created.content})
	if update := javaDoc(root, sheetOf("src/acme", "A.java", "package-info.java"), "Core, revised."); update == nil || update.status != genblock.Replaced {
		t.Errorf("%+v", update)
	}
}

func TestJavaDocNeedsAPackageDeclaration(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "d", map[string]string{"A.java": "class A {}\n"})
	if javaDoc(root, sheetOf("d", "A.java"), "x") != nil || javaDoc(root, sheetOf("d", "gone.java"), "x") != nil {
		t.Error("without a package there is nothing to document")
	}
}

func pythonSeed(t *testing.T) (string, *sourceEdit) {
	t.Helper()
	root := t.TempDir()
	seed(t, root, "pkg", map[string]string{"__init__.py": "import os\n\nVALUE = 1\n"})
	return root, pythonDoc(root, sheetOf("pkg", "__init__.py"), "A package.")
}

func TestPythonDocPutsADocstringAtTheTopOfAnExistingInit(t *testing.T) {
	_, edit := pythonSeed(t)
	if edit == nil || edit.target != "pkg/__init__.py" || !strings.HasPrefix(edit.content, "\"\"\"\nloom:generated:start hash=") || !strings.HasSuffix(edit.content, "\"\"\"\nimport os\n\nVALUE = 1\n") {
		t.Fatalf("%+v", edit)
	}
}

func TestPythonDocUpdatesItsOwnDocstringAndKeepsTheCode(t *testing.T) {
	root, edit := pythonSeed(t)
	seed(t, root, "pkg", map[string]string{"__init__.py": edit.content})
	update := pythonDoc(root, sheetOf("pkg", "__init__.py"), "A package, revised.")
	if update == nil || update.status != genblock.Replaced || !strings.HasSuffix(update.content, "import os\n\nVALUE = 1\n") {
		t.Errorf("%+v", update)
	}
}

func TestPythonDocNeverTouchesAHumanDocstringOrCreatesAnInit(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "pkg", map[string]string{"__init__.py": "\"\"\"Mine.\"\"\"\n"})
	if skip := pythonDoc(root, sheetOf("pkg", "__init__.py"), "x"); skip == nil || skip.skip == "" {
		t.Errorf("%+v", skip)
	}
	seed(t, root, "ns", map[string]string{"mod.py": "x = 1\n"})
	if pythonDoc(root, sheetOf("ns", "mod.py"), "x") != nil {
		t.Error("creating __init__.py would change a namespace package into a regular one")
	}
	if pythonDoc(root, sheetOf("pkg", "__init__.py", "missing"), "x") == nil {
		t.Error("an existing __init__ is considered")
	}
}

func scriptSeed(t *testing.T) (string, *sourceEdit) {
	t.Helper()
	root := t.TempDir()
	seed(t, root, "web", map[string]string{"index.ts": "export const x = 1;\n"})
	return root, scriptDoc(root, sheetOf("web", "index.ts"), "The web module.")
}

func TestScriptDocPrependsAModuleBlockToAnExistingIndex(t *testing.T) {
	_, edit := scriptSeed(t)
	if edit == nil || edit.target != "web/index.ts" || !strings.HasPrefix(edit.content, "/**\n * loom:generated:start hash=") || !strings.HasSuffix(edit.content, " * @module\n */\nexport const x = 1;\n") {
		t.Fatalf("%+v", edit)
	}
}

func TestScriptDocUpdatesItsOwnBlock(t *testing.T) {
	root, edit := scriptSeed(t)
	seed(t, root, "web", map[string]string{"index.ts": edit.content})
	if update := scriptDoc(root, sheetOf("web", "index.ts"), "Revised."); update == nil || update.status != genblock.Replaced {
		t.Errorf("%+v", update)
	}
}

func TestScriptDocKeepsAHumanDocCommentAndNeverCreatesAnIndex(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "web", map[string]string{"index.ts": "/** My docs. */\nexport {};\n"})
	if skip := scriptDoc(root, sheetOf("web", "index.ts"), "x"); skip == nil || skip.skip == "" {
		t.Errorf("%+v", skip)
	}
	if scriptDoc(root, sheetOf("none", "util.ts"), "x") != nil {
		t.Error("only an existing index file is documented; none is created")
	}
}

func TestScriptDocPrefersIndexTsOverOtherIndexFiles(t *testing.T) {
	root := t.TempDir()
	seed(t, root, "w", map[string]string{"index.js": "a\n", "index.ts": "b\n"})
	if edit := scriptDoc(root, sheetOf("w", "index.js", "index.ts"), "x"); edit == nil || edit.target != "w/index.ts" {
		t.Errorf("%+v", edit)
	}
}

func TestACommentBodyCannotCloseTheCommentItSitsIn(t *testing.T) {
	doc := stateDoc{Purpose: "Ends */ early and \"\"\" too", Responsibilities: []string{"a */ b"}}
	body := commentBody(doc)
	if strings.Contains(body, "*/") || strings.Contains(body, `"""`) {
		t.Errorf("%q", body)
	}
	if got := commentBody(stateDoc{}); got != "Purpose not established by the facts." {
		t.Errorf("an unknown purpose says so: %q", got)
	}
}

func docCommentRepo(t *testing.T) string {
	t.Helper()
	root := newRepo(t, map[string]string{
		"go.mod": goRepo["go.mod"], "lib/lib.go": "// Package lib is already documented.\npackage lib\n", "app/app.go": "package app\n",
	})
	documentedRun(t, root, nil)
	return root
}

func TestDocCommentsAreOptIn(t *testing.T) {
	root := docCommentRepo(t)
	for _, artifact := range plan(t, root, Options{}).Artifacts {
		if artifact.Kind == KindDocComment {
			t.Errorf("doc comments appeared without --doc-comments: %+v", artifact)
		}
	}
}

func TestDocCommentSkipsAreRecordedAtTheGateWithTheirReason(t *testing.T) {
	change := plan(t, docCommentRepo(t), Options{DocComments: true})
	if created := artifactFor(t, change, "app/doc.go"); created.Action != ActionCreate {
		t.Errorf("%+v", created)
	}
	skipped := artifactFor(t, change, "lib/doc.go")
	if skipped.Action != ActionSkip || !strings.Contains(skipped.Reason, "already has a doc comment") {
		t.Errorf("%+v", skipped)
	}
}

func TestApplyingDocCommentsWritesOnlyNewFilesAndLeavesSourceUntouched(t *testing.T) {
	root := docCommentRepo(t)
	result := approveAndApply(t, root, plan(t, root, Options{DocComments: true}).ID)
	if !exists(root, "app/doc.go") || exists(root, "lib/doc.go") || len(result.Withheld) == 0 {
		t.Errorf("only the new doc.go is written: %+v", result)
	}
	if got := readFile(t, root, "lib/lib.go"); got != "// Package lib is already documented.\npackage lib\n" {
		t.Errorf("existing source must be untouched: %q", got)
	}
}
