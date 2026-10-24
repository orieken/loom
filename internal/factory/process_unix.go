//go:build !windows

package factory

import (
	"errors"
	"syscall"
)

func ownProcessGroup() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// interruptProcess sends SIGINT, which `loom run` turns into a clean
// INTERRUPTED checkpoint.
func interruptProcess(pid int) error {
	return syscall.Kill(pid, syscall.SIGINT)
}

// processAlive reports whether a process exists. EPERM means it exists and
// belongs to someone else, which still counts.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
