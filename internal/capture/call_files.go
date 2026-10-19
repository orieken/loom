package capture

import (
	"context"
	"errors"
	"path/filepath"
)

// errReadBudgetSpent marks a file that was skipped because the read budget
// of the call ended. It loses recall only.
var errReadBudgetSpent = errors.New("capture: read budget of the call is spent")

// fileRead is the result of one file read in one call.
type fileRead struct {
	content []byte
	err     error
	lines   [][]byte
}

// splitLines splits the content once and keeps the result, so every hunk of
// the file hashes from the same buffer.
func (f *fileRead) splitLines() [][]byte {
	if f.lines == nil {
		f.lines = SplitLines(f.content)
	}
	return f.lines
}

// callFiles reads project files for one hook call. It reads each path once
// and counts every byte against the read budget (performance-report.md
// mandate D and finding 1).
type callFiles struct {
	reader    FileReader
	root      string
	remaining int64
	reads     map[string]*fileRead
}

func newCallFiles(reader FileReader, root string) *callFiles {
	return &callFiles{
		reader:    reader,
		root:      root,
		remaining: MaxReadBudgetBytes,
		reads:     make(map[string]*fileRead),
	}
}

// isSpent reports whether the budget ended, so a caller can stop early.
func (f *callFiles) isSpent() bool { return f.remaining <= 0 }

// read returns the content of a project-relative path. A second call for
// the same path gives the first result and reads nothing.
func (f *callFiles) read(ctx context.Context, relative string) *fileRead {
	if known, isKnown := f.reads[relative]; isKnown {
		return known
	}
	result := &fileRead{err: errReadBudgetSpent}
	if !f.isSpent() {
		limit := min(int64(MaxTargetFileBytes), f.remaining)
		result.content, result.err = f.reader.ReadCapped(ctx, f.absolute(relative), limit)
		f.remaining -= int64(len(result.content))
	}
	f.reads[relative] = result
	return result
}

func (f *callFiles) absolute(relative string) string {
	return filepath.Join(f.root, filepath.FromSlash(relative))
}
