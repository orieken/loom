//go:build windows

package factory

import (
	"errors"
	"os"
	"syscall"
)

func ownProcessGroup() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// interruptProcess is unsupported: Windows has no SIGINT to send another
// process, so `loom factory stop` is POSIX-only for now.
func interruptProcess(int) error {
	return errors.New("stopping a run is not supported on Windows")
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	_ = process.Release()
	return true
}
