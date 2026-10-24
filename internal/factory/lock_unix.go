//go:build !windows

package factory

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// ErrSupervisorRunning reports a second supervisor on the same ledger.
var ErrSupervisorRunning = errors.New("another factory supervisor holds the lock")

// Lock holds the supervisor lock until Release. The kernel drops an flock
// when its process dies, so a crashed supervisor never leaves a stale lock.
type Lock struct{ file *os.File }

// AcquireLock takes the supervisor lock without waiting.
func AcquireLock(path string) (*Lock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open supervisor lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("%w (%s)", ErrSupervisorRunning, path)
	}
	return &Lock{file: file}, nil
}

// Release gives the lock up.
func (l *Lock) Release() error { return l.file.Close() }
