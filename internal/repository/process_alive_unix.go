//go:build !windows

package repository

import "syscall"

// processAlive reports whether a Unix process still exists without terminating it.
func processAlive(pid int) (bool, error) {
	err := syscall.Kill(pid, syscall.Signal(0))
	if err == nil || err == syscall.EPERM {
		return true, nil
	}
	if err == syscall.ESRCH {
		return false, nil
	}
	return false, err
}
