//go:build windows

package content

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestWindowsSymlinkPathGuidance verifies native Windows symlink behavior when privileges are available.
func TestWindowsSymlinkPathGuidance(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "linked")
	if err := os.Symlink(outside, link); err != nil {
		if errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrInvalid) || errors.Is(err, syscall.Errno(1314)) {
			t.Skip("native Windows symlink creation requires Developer Mode or Administrator privileges")
		}
		t.Fatalf("create native Windows symlink: %v", err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("native Windows link was not a real symlink")
	}
	if _, err := ValidateRelativePath(root, `linked\escaped.txt`, false); err == nil {
		t.Fatal("symlink component was accepted on native Windows")
	}
}
