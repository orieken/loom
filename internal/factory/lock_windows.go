//go:build windows

package factory

import (
	"errors"
	"fmt"
	"os"
)

// ErrSupervisorRunning reports a second supervisor on the same ledger.
var ErrSupervisorRunning = errors.New("another factory supervisor holds the lock")

// Lock holds the supervisor lock until Release. On Windows it is an
// exclusive-create file, which a crashed supervisor leaves behind; the error
// names the file to delete.
type Lock struct{ path string }

// AcquireLock takes the supervisor lock without waiting.
func AcquireLock(path string) (*Lock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("%w (%s — delete it if no supervisor is running)", ErrSupervisorRunning, path)
	}
	file.Close()
	return &Lock{path: path}, nil
}

// Release gives the lock up.
func (l *Lock) Release() error { return os.Remove(l.path) }
