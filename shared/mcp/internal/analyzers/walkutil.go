package analyzers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SkippedDirNames lists directory basenames the walk should never descend into.
var SkippedDirNames = map[string]struct{}{
	"node_modules": {},
	".git":         {},
	"dist":         {},
	"build":        {},
	"vendor":       {},
	".venv":        {},
}

// SkipUninterestingDir returns filepath.SkipDir for directories the walk should skip:
// any name in SkippedDirNames, plus hidden dot-directories below the root.
func SkipUninterestingDir(root, current, name string) error {
	if _, skip := SkippedDirNames[name]; skip {
		return filepath.SkipDir
	}
	if strings.HasPrefix(name, ".") && current != root {
		return filepath.SkipDir
	}
	return nil
}

// Walk ceilings (roadmap L2.3). A model-supplied path that reaches a huge
// tree must abort rather than walk it: before these, `projectPath: "/"`
// walked the whole disk.
const (
	maxWalkFiles = 50_000
	maxWalkBytes = 512 << 20
)

// ErrWalkTooLarge reports a walk that crossed a ceiling.
var ErrWalkTooLarge = errors.New("the path holds more files than an analysis will walk")

// CollectFiles returns root itself when it is a regular file, or every
// regular file under it that include accepts. It skips the directories
// SkipUninterestingDir names, never follows a symbolic link — a link inside
// the workspace root must not lead the walk outside it — and aborts with
// ErrWalkTooLarge past maxWalkFiles files or maxWalkBytes bytes.
func CollectFiles(root string, include func(path string) bool) ([]string, error) {
	return collectWithin(root, include, walkLimits{files: maxWalkFiles, bytes: maxWalkBytes})
}

// walkLimits are the ceilings one walk enforces; tests pass small ones.
type walkLimits struct {
	files int
	bytes int64
}

func collectWithin(root string, include func(path string) bool, limits walkLimits) ([]string, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{root}, nil
	}
	collector := &fileCollector{root: root, include: include, limits: limits}
	err = filepath.Walk(root, collector.visit)
	return collector.files, err
}

type fileCollector struct {
	root    string
	include func(path string) bool
	limits  walkLimits
	files   []string
	bytes   int64
}

func (c *fileCollector) visit(path string, info os.FileInfo, walkErr error) error {
	if walkErr != nil || info == nil {
		return nil
	}
	if info.IsDir() {
		return SkipUninterestingDir(c.root, path, info.Name())
	}
	if !info.Mode().IsRegular() || !c.include(path) {
		return nil
	}
	return c.admit(path, info.Size())
}

func (c *fileCollector) admit(path string, size int64) error {
	c.bytes += size
	if len(c.files) >= c.limits.files || c.bytes > c.limits.bytes {
		return fmt.Errorf("%w (limits: %d files, %d bytes)", ErrWalkTooLarge, c.limits.files, c.limits.bytes)
	}
	c.files = append(c.files, path)
	return nil
}
