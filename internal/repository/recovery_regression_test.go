package repository

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRecoverDiscardsManifestlessTransaction verifies that a crash before the
// prepared manifest is written (during snapshot staging) never wedges future
// mutations: the staging directory is discarded and recovery continues.
func TestRecoverDiscardsManifestlessTransaction(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".cfgfc-transactions", "stale-crash"), 0o700); err != nil {
		t.Fatal(err)
	}

	repo := New(root)
	indexPath := filepath.Join(root, "index.jsonc")
	if err := repo.WithMutation("probe", []string{indexPath}, func() error {
		return WriteFileAtomic(indexPath, []byte("x"), 0o644)
	}); err != nil {
		t.Fatalf("manifest-less transaction dir wedged mutation: %v", err)
	}

	diagnostics, err := repo.Diagnostics()
	if err != nil {
		t.Fatalf("Diagnostics failed on manifest-less transaction: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}

	entries, err := os.ReadDir(filepath.Join(root, ".cfgfc-transactions"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("stale transaction directory was not discarded: %#v", entries)
	}
}
