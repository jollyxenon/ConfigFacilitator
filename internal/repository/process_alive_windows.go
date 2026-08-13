//go:build windows

package repository

import (
	"os"
	"syscall"
)

// processAlive reports whether a Windows process still exists without terminating it.
func processAlive(pid int) (bool, error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false, nil
	}
	return true, nil
}
