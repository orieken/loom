package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/orieken/loom/internal/orchestrator"
	"github.com/orieken/loom/internal/provider/mock"
	"github.com/orieken/loom/internal/state"
)

// A mock solution is what the end-to-end harness gives the mock to deliver
// (roadmap L3.73): its developer writes the directory's source files and its
// QA engineer writes the test files, each claiming exactly what it wrote. The
// harness's hidden acceptance tests then judge a run whose code really moved
// from a stage, through the worktree, onto a pushed branch.

// loadSolution reads the solution directory, splitting Go test files from
// the rest. Nothing set means the mock's placeholders, as before.
func (o *mockOptions) loadSolution() error {
	if o.solution == "" {
		return nil
	}
	o.code, o.tests = map[string][]byte{}, map[string][]byte{}
	err := filepath.WalkDir(o.solution, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		return o.addSolutionFile(path)
	})
	if err != nil {
		return fmt.Errorf("read --mock-solution %s: %w", o.solution, err)
	}
	return nil
}

func (o *mockOptions) addSolutionFile(path string) error {
	relative, err := filepath.Rel(o.solution, path)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	relative = filepath.ToSlash(relative)
	if strings.HasSuffix(relative, "_test.go") {
		o.tests[relative] = content
	} else {
		o.code[relative] = content
	}
	return nil
}

// scriptFor is the stage's script, with the solution in place of the
// placeholder for the stages that write code.
func (o mockOptions) scriptFor(stage orchestrator.Stage) mock.Script {
	switch {
	case stage.StateKind == string(state.KindImplementation) && len(o.code) > 0:
		return mock.ImplementationWriting(o.code)
	case stage.StateKind == string(state.KindQA) && len(o.tests) > 0:
		return mock.QAWriting(o.tests)
	}
	return scriptFor(stage)
}
