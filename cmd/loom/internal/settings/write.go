package settings

import (
	"errors"
	"os"
	"path/filepath"
)

const (
	// newFileMode is the mode of a file the adapter creates.
	newFileMode os.FileMode = 0o600
	// directoryMode is the mode of a directory the adapter creates.
	directoryMode os.FileMode = 0o755
)

// writeAtomic writes through a temporary file in the same directory and a
// rename, so a reader sees the old file or the new one and never half of
// either. An existing file keeps its mode. A symbolic link is followed, so
// the link stays a link.
func writeAtomic(path string, content []byte) error {
	destination, mode := resolveDestination(path)
	directory := filepath.Dir(destination)
	if err := os.MkdirAll(directory, directoryMode); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".settings-*.tmp")
	if err != nil {
		return err
	}
	if err := fillTemporary(temporary, content, mode); err != nil {
		_ = os.Remove(temporary.Name())
		return err
	}
	if err := os.Rename(temporary.Name(), destination); err != nil {
		_ = os.Remove(temporary.Name())
		return err
	}
	return nil
}

func fillTemporary(file *os.File, content []byte, mode os.FileMode) error {
	_, writeErr := file.Write(content)
	chmodErr := file.Chmod(mode)
	syncErr := file.Sync()
	return errors.Join(writeErr, chmodErr, syncErr, file.Close())
}

// resolveDestination returns the real path and the mode to keep.
func resolveDestination(path string) (string, os.FileMode) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path, newFileMode
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return resolved, newFileMode
	}
	return resolved, info.Mode().Perm()
}
