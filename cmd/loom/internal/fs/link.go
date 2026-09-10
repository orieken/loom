package frameworkfs

import (
	"fmt"
	"os"
	"path/filepath"
)

func (writer *Writer) cachePath(source string) (string, error) {
	return safeDestination(writer.cache, filepath.FromSlash(source))
}

func (writer *Writer) materialize(source, cachePath string) error {
	if _, err := os.Stat(cachePath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect cache path: %w", err)
	}
	if err := copyEmbedded(writer.content, source, cachePath); err != nil {
		return fmt.Errorf("extract %s to cache: %w", source, err)
	}
	return sealCache(cachePath)
}

// sealCache makes extracted cache content read-only (roadmap L3.23, L3.26).
//
// A linked install points every project on the machine at the same files, so
// a write through one project's symlink lands in every other project's
// framework. This was observed: appending one line to a linked
// .claude/agents/analyst.md modified the shared copy. Read-only content
// makes that a failed write in the project that attempts it rather than
// silent corruption everywhere else.
func sealCache(path string) error {
	return filepath.Walk(path, func(current string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		return os.Chmod(current, sealedMode(info))
	})
}

// sealedMode makes files unwritable while leaving directories writable.
//
// Only the file mode matters for the corruption this prevents: appending
// through a symlink opens the file for writing, which 0444 refuses.
// Directories stay 0755 so the cache can still be evicted, updated, or
// removed — sealing those too turns a safety measure into an uninstall bug.
func sealedMode(info os.FileInfo) os.FileMode {
	if info.IsDir() {
		return 0o755
	}
	if info.Mode()&0o100 != 0 {
		return 0o555
	}
	return 0o444
}

func (writer *Writer) backup(path, displayPath string) error {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect %s: %w", displayPath, err)
	}
	backupPath := fmt.Sprintf("%s.bak.%d", path, writer.now().UnixNano())
	if err := os.Rename(path, backupPath); err != nil {
		return fmt.Errorf("backup %s: %w", displayPath, err)
	}
	writer.report("backed up " + displayPath + " -> " + filepath.Base(backupPath))
	return nil
}

func isSameSymlink(path, expectedTarget string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect symlink %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	actualTarget, err := os.Readlink(path)
	if err != nil {
		return false, fmt.Errorf("read symlink %s: %w", path, err)
	}
	return actualTarget == expectedTarget, nil
}
