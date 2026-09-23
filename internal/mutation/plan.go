// Package mutation scopes mutation testing to the lines a change adds or
// modifies, and scores the result (roadmap L3.59, ADR-008 clause 2).
//
// The mutation engine is gremlins. Its own --diff flag does not scope in
// v0.6.0 — the spike measured identical mutant counts with and without it,
// and upstream has eight open issues on the subject — so scoping is done
// here: gremlins runs once per changed package with that package's unchanged
// files excluded, and only mutants on changed lines are kept. When a released
// gremlins scopes correctly, Plan can shrink to a single --diff run.
package mutation

import (
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/orieken/loom/internal/diffcover"
)

// Target is one gremlins run: a package directory, relative to the
// repository root, and the exclusion patterns that confine mutation to the
// package's changed files.
//
// Integration is set when the package's name differs from its directory's.
// gremlins v0.6.0 finds the package to test by walking up for a directory
// named after the package, so for `package main` — or any mismatch — it tests
// the module root instead, the mutated code's tests never run, and every
// mutant reads LIVED. Measured: 17 false survivors in cmd/diff-mutation, each
// of which `go test` kills by hand. Integration mode runs the whole suite, in
// which the mutated package's tests are included.
type Target struct {
	Dir         string
	Exclude     []string
	Integration bool
}

// excludeSubdirectories is an --exclude-files pattern matching every file
// below the target directory. gremlins matches exclusions against the path
// relative to the target, so any slash means a subpackage — which, if it
// changed, is its own Target.
const excludeSubdirectories = "/"

// Plan returns a Target for each package directory holding a changed
// production Go file that is not excluded. root is the repository root.
func Plan(root string, changes diffcover.Changes, excluded map[string]bool) ([]Target, error) {
	changedByDir := changedFilesByDirectory(changes, excluded)
	targets := make([]Target, 0, len(changedByDir))
	for _, dir := range sortedKeys(changedByDir) {
		target, err := planTarget(root, dir, changedByDir[dir])
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func planTarget(root, dir string, changed map[string]bool) (Target, error) {
	directory := filepath.Join(root, dir)
	exclude, err := unchangedFilePatterns(directory, changed)
	if err != nil {
		return Target{}, err
	}
	name, err := packageName(filepath.Join(directory, sortedKeys(changed)[0]))
	if err != nil {
		return Target{}, err
	}
	return Target{
		Dir:         dir,
		Exclude:     append(exclude, excludeSubdirectories),
		Integration: name != filepath.Base(directory),
	}, nil
}

// packageName reads a file's package clause and nothing else.
func packageName(file string) (string, error) {
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.PackageClauseOnly)
	if err != nil {
		return "", err
	}
	return parsed.Name.Name, nil
}

func changedFilesByDirectory(changes diffcover.Changes, excluded map[string]bool) map[string]map[string]bool {
	byDir := map[string]map[string]bool{}
	for file, lines := range changes {
		if len(lines) == 0 || !diffcover.IsProductionGo(file) || diffcover.IsExcluded(excluded, file) {
			continue
		}
		dir := path.Dir(file)
		if byDir[dir] == nil {
			byDir[dir] = map[string]bool{}
		}
		byDir[dir][path.Base(file)] = true
	}
	return byDir
}

// unchangedFilePatterns anchors each unchanged production file in dir as an
// exact-name exclusion, so a file named a.go never excludes data.go.
func unchangedFilePatterns(dir string, changed map[string]bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var patterns []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || changed[name] || !diffcover.IsProductionGo(name) {
			continue
		}
		patterns = append(patterns, "^"+regexp.QuoteMeta(name)+"$")
	}
	return patterns, nil
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// repositoryPath joins a target directory and a file name gremlins reported
// relative to it.
func repositoryPath(dir, file string) string {
	if dir == "." || dir == "" {
		return file
	}
	return strings.TrimSuffix(dir, "/") + "/" + file
}
