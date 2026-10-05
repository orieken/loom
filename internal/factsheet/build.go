package factsheet

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"sort"
	"strings"

	"github.com/orieken/loom/internal/repograph"
)

// RootPath names the repo root directory, as path.Dir does.
const RootPath = "."

// Build computes one sheet per directory that holds files, directly or in a
// descendant. Output is sorted by path and has no timestamps, so identical
// input gives byte-identical sheets.
func Build(repo repograph.Repo, graph repograph.Graph) []FactSheet {
	directories := collectDirectories(repo.Files())
	links := linkDirectories(graph)
	tracked := fileSet(repo.Files())
	sheets := map[string]*FactSheet{}
	digests := map[string]map[string]string{}
	for dir, files := range directories.files {
		sheet, fileDigests := describeDirectory(repo, tracked, dir, files)
		links.apply(sheet)
		sheets[dir], digests[dir] = sheet, fileDigests
	}
	for dir, children := range directories.children {
		sheet := ensureSheet(sheets, dir)
		sheet.Children = sortedKeys(children)
	}
	return finish(sheets, digests)
}

// ensureSheet returns the sheet for a directory that holds only subdirectories.
func ensureSheet(sheets map[string]*FactSheet, dir string) *FactSheet {
	if sheets[dir] == nil {
		sheets[dir] = emptySheet(dir)
	}
	return sheets[dir]
}

func emptySheet(dir string) *FactSheet {
	return &FactSheet{
		Path: dir, Files: []string{}, Exports: []string{}, Imports: []string{}, ApproximateImports: []string{},
		Dependents: []string{}, ApproximateDependents: []string{}, Entrypoints: []string{}, Tests: []string{}, Children: []string{},
	}
}

type directoryTree struct {
	files    map[string][]string
	children map[string]map[string]bool
}

// collectDirectories groups files by directory and records each directory's
// immediate subdirectories, walking up so a directory holding only
// subdirectories still appears.
func collectDirectories(files []string) directoryTree {
	tree := directoryTree{files: map[string][]string{}, children: map[string]map[string]bool{}}
	for _, file := range files {
		dir := path.Dir(file)
		tree.files[dir] = append(tree.files[dir], file)
		for ; dir != RootPath; dir = path.Dir(dir) {
			parent := path.Dir(dir)
			if tree.children[parent] == nil {
				tree.children[parent] = map[string]bool{}
			}
			tree.children[parent][dir] = true
		}
	}
	return tree
}

func describeDirectory(repo repograph.Repo, tracked map[string]bool, dir string, files []string) (*FactSheet, map[string]string) {
	sheet := emptySheet(dir)
	digests := map[string]string{}
	exports := map[string]bool{}
	sort.Strings(files)
	for _, file := range files {
		name := path.Base(file)
		sheet.Files = append(sheet.Files, name)
		source, err := repo.Read(file)
		if err != nil {
			continue
		}
		digests[name] = digest(source)
		recordFile(sheet, file, name, source, exports)
	}
	sheet.Exports = sortedKeys(exports)
	sheet.Entrypoints = append(sheet.Entrypoints, packageEntrypoints(repo, tracked, dir)...)
	sort.Strings(sheet.Entrypoints)
	return sheet, digests
}

func recordFile(sheet *FactSheet, file, name string, source []byte, exports map[string]bool) {
	if repograph.IsTestFile(file) {
		sheet.Tests = append(sheet.Tests, name)
		return
	}
	facts := describeFile(file, source)
	for _, export := range facts.exports {
		exports[export] = true
	}
	if facts.entrypoint {
		sheet.Entrypoints = append(sheet.Entrypoints, name)
	}
}

func fileSet(files []string) map[string]bool {
	set := make(map[string]bool, len(files))
	for _, file := range files {
		set[file] = true
	}
	return set
}

func digest(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// finish computes input hashes children-first, so a parent's hash includes
// its children's and a change propagates upward.
func finish(sheets map[string]*FactSheet, digests map[string]map[string]string) []FactSheet {
	order := make([]string, 0, len(sheets))
	for dir := range sheets {
		order = append(order, dir)
	}
	sort.Slice(order, func(i, j int) bool {
		if depth(order[i]) != depth(order[j]) {
			return depth(order[i]) > depth(order[j])
		}
		return order[i] < order[j]
	})
	for _, dir := range order {
		sheets[dir].InputHash = inputHash(sheets, dir, digests[dir])
	}
	sort.Strings(order)
	out := make([]FactSheet, 0, len(order))
	for _, dir := range order {
		out = append(out, *sheets[dir])
	}
	return out
}

func depth(dir string) int {
	if dir == RootPath {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

// inputHash covers the file digests, every fact on the sheet and the
// children's hashes — not itself.
func inputHash(sheets map[string]*FactSheet, dir string, fileDigests map[string]string) string {
	sheet := *sheets[dir]
	sheet.InputHash = ""
	childHashes := map[string]string{}
	for _, child := range sheet.Children {
		childHashes[child] = sheets[child].InputHash
	}
	raw, _ := json.Marshal(struct {
		Sheet    FactSheet         `json:"sheet"`
		Digests  map[string]string `json:"digests"`
		Children map[string]string `json:"children"`
	}{sheet, fileDigests, childHashes})
	return digest(raw)
}
