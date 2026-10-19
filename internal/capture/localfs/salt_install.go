package localfs

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/orieken/loom/internal/capture"
)

const (
	// saltFileMode keeps the salt private to the user.
	saltFileMode os.FileMode = 0o600
	// memoryDirMode keeps the capture directory private to the user.
	memoryDirMode os.FileMode = 0o700
)

// SaltInstallOutcome says what InstallSalt did.
type SaltInstallOutcome int

const (
	// SaltCreated means a new salt was written.
	SaltCreated SaltInstallOutcome = iota
	// SaltKept means a salt file already existed and was left as it was.
	SaltKept
)

// InstallSalt creates the Capture salt: 32 bytes from the random source, in
// a file made with O_CREATE|O_EXCL and mode 0600. A file that already exists
// is kept (EEXIST is success). The installer NEVER rotates or overwrites a
// salt, because a new salt would break every stored hash. The salt value is
// never returned, printed or logged.
//
// A nil random source means crypto/rand.
func InstallSalt(path string, random io.Reader) (SaltInstallOutcome, error) {
	if random == nil {
		random = rand.Reader
	}
	if err := os.MkdirAll(filepath.Dir(path), memoryDirMode); err != nil {
		return SaltCreated, fmt.Errorf("create the capture directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, saltFileMode)
	if errors.Is(err, os.ErrExist) {
		return SaltKept, nil
	}
	if err != nil {
		return SaltCreated, fmt.Errorf("create the capture salt: %w", err)
	}
	return SaltCreated, fillSalt(file, path, random)
}

// fillSalt writes the random bytes. A failure removes the partial file, so a
// later install does not keep a salt that is too short.
func fillSalt(file *os.File, path string, random io.Reader) error {
	raw := make([]byte, capture.SaltBytes)
	_, readErr := io.ReadFull(random, raw)
	var writeErr error
	if readErr == nil {
		_, writeErr = file.Write(raw)
	}
	failure := errors.Join(readErr, writeErr, file.Chmod(saltFileMode), file.Close())
	if failure != nil {
		_ = os.Remove(path)
		return fmt.Errorf("write the capture salt: %w", failure)
	}
	return nil
}
