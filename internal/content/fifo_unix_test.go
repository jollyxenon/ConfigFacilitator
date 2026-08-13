//go:build !windows

package content

import "syscall"

// makeFIFO creates one named pipe for unsupported-import coverage on Unix-like systems.
func makeFIFO(path string) error {
	return syscall.Mkfifo(path, 0o600)
}
