package localfs

import (
	"errors"
	"io"
	"os"

	"github.com/orieken/loom/internal/capture"
)

// Errors of the salt file. None of them carries salt bytes.
var (
	// ErrSaltMissing marks a salt file that does not exist or cannot be opened.
	ErrSaltMissing = errors.New("localfs: capture salt is missing")
	// ErrSaltMode marks a salt file that other users can read or write.
	ErrSaltMode = errors.New("localfs: capture salt is readable by other users")
	// ErrSaltCorrupt marks a salt file with the wrong size or type.
	ErrSaltCorrupt = errors.New("localfs: capture salt is not 32 bytes")
)

// groupAndOtherBits are the permission bits that must be clear.
const groupAndOtherBits os.FileMode = 0o077

var _ capture.SaltSource = SaltFile{}

// SaltFile reads the Capture salt from a file. It never creates or changes
// the file: a new salt would break every stored hash. Installing the salt is
// the job of the installer.
type SaltFile struct {
	path string
}

// NewSaltFile builds a source for the given path.
func NewSaltFile(path string) SaltFile {
	return SaltFile{path: path}
}

// Salt reads the 32 bytes. A missing file, a mode that lets other users in,
// or a wrong size is a typed error. The caller counts a failure and writes
// nothing.
func (s SaltFile) Salt() (capture.Salt, error) {
	info, err := os.Stat(s.path)
	if err != nil {
		return capture.Salt{}, ErrSaltMissing
	}
	if !info.Mode().IsRegular() || info.Size() != capture.SaltBytes {
		return capture.Salt{}, ErrSaltCorrupt
	}
	if info.Mode().Perm()&groupAndOtherBits != 0 {
		return capture.Salt{}, ErrSaltMode
	}
	file, err := os.Open(s.path)
	if err != nil {
		return capture.Salt{}, ErrSaltMissing
	}
	defer file.Close()
	return readSalt(file)
}

func readSalt(file io.Reader) (capture.Salt, error) {
	raw := make([]byte, capture.SaltBytes+1)
	count, err := io.ReadFull(file, raw)
	if count != capture.SaltBytes || (err != nil && !errors.Is(err, io.ErrUnexpectedEOF)) {
		return capture.Salt{}, ErrSaltCorrupt
	}
	salt, err := capture.NewSalt(raw[:count])
	if err != nil {
		return capture.Salt{}, ErrSaltCorrupt
	}
	return salt, nil
}
