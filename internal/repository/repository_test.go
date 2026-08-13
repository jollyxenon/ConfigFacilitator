package repository

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// TestWriteFileAtomicPreservesModeAndNeverLeavesTemporaryFiles verifies atomic replacement details.
func TestWriteFileAtomicPreservesModeAndNeverLeavesTemporaryFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "index.jsonc")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("new"), 0o644); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("content = %q, want new", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %o, want 640", info.Mode().Perm())
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".cfgfc-tmp-") {
			t.Fatalf("temporary file remained: %s", entry.Name())
		}
	}
}

// TestRepositoryIndexRoundTripPreservesCanonicalMetadataAndExtra verifies all index APIs.
func TestRepositoryIndexRoundTripPreservesCanonicalMetadataAndExtra(t *testing.T) {
	root := t.TempDir()
	repository := New(root)
	want := index.ProjectIndex{Projects: map[string]index.ProjectEntry{
		"Canonical": {WarehouseName: "Canonical", DisplayName: "Shown", Aliases: []string{"alias"}, Description: "description", Extra: map[string]json.RawMessage{"extension": json.RawMessage(`true`)}},
	}}
	if err := repository.SaveProjectIndex(want); err != nil {
		t.Fatalf("SaveProjectIndex: %v", err)
	}
	got, err := repository.LoadProjectIndex()
	if err != nil {
		t.Fatalf("LoadProjectIndex: %v", err)
	}
	entry := got.Projects["Canonical"]
	if entry.DisplayName != "Shown" || !reflect.DeepEqual(entry.Aliases, []string{"alias"}) || entry.Description != "description" {
		t.Fatalf("metadata was not preserved: %#v", entry)
	}
	if string(entry.Extra["extension"]) != "true" {
		t.Fatalf("extra field was not preserved: %s", entry.Extra["extension"])
	}
}

// TestMutationRollbackRestoresExactFiles verifies ordinary callback failures restore bytes and mode.
func TestMutationRollbackRestoresExactFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ProjectIndex.jsonc")
	original := []byte("{\n  // authored\n  \"OpenCode\": {\"aliases\": [\"oc\"]}\n}\n")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	repository := New(root)
	err := repository.WithMutation("test", []string{path}, func() error {
		return WriteFileAtomic(path, []byte("partial replacement"), 0o644)
	})
	if err != nil {
		t.Fatalf("mutation callback should commit: %v", err)
	}
	if err := repository.WithMutation("test-failure", []string{path}, func() error {
		if err := WriteFileAtomic(path, []byte("bad"), 0o644); err != nil {
			return err
		}
		return errors.New("injected mutation failure")
	}); err == nil {
		t.Fatal("expected callback failure")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "partial replacement" {
		t.Fatalf("rollback content = %q", data)
	}
}

// TestRestartRecoveryRestoresPreparedTransaction verifies recovery after a process-like stop.
func TestRestartRecoveryRestoresPreparedTransaction(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "state.json")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	repository := New(root)
	transaction, err := repository.BeginMutation("restart", path)
	if err != nil {
		t.Fatalf("BeginMutation: %v", err)
	}
	if err := WriteFileAtomic(path, []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := transaction.LeavePrepared(); err != nil {
		t.Fatal(err)
	}
	if err := repository.Recover(); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "before" {
		t.Fatalf("recovered content = %q", data)
	}
	if diagnostics, err := repository.Diagnostics(); err != nil || len(diagnostics) != 0 {
		t.Fatalf("transaction diagnostics after recovery = %#v, err=%v", diagnostics, err)
	}
}

// TestMutationLockIsWarehouseWide verifies a second mutation cannot enter concurrently.
func TestMutationLockIsWarehouseWide(t *testing.T) {
	root := t.TempDir()
	first, err := New(root).BeginMutation("first")
	if err != nil {
		t.Fatalf("first BeginMutation: %v", err)
	}
	defer first.Rollback()
	if _, err := New(root).BeginMutation("second"); err == nil {
		t.Fatal("expected second mutation to be rejected by lock")
	}
}

// TestDiagnosticsIsReadOnly verifies status inspection reports prepared work without recovery.
func TestDiagnosticsIsReadOnly(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "state.json")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	repository := New(root)
	transaction, err := repository.BeginMutation("diagnostic", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := transaction.LeavePrepared(); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := repository.Diagnostics()
	if err != nil {
		t.Fatalf("Diagnostics: %v", err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Status != "prepared" {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "after" {
		t.Fatalf("read-only diagnostics changed content to %q", data)
	}
	if err := repository.Recover(); err != nil {
		t.Fatal(err)
	}
}

// TestWarehouseDiscoveryIgnoresTransactionArtifacts verifies reserved staging is not a resource.
func TestWarehouseDiscoveryIgnoresTransactionArtifacts(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".cfgfc-transactions", "staging"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "OpenCode", "Column", "Skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ProjectIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "OpenCode", "Column", "ColumnIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "OpenCode", "Column", "Skills", "SettingIndex.jsonc"), []byte(`{"targetNumber":0}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "OpenCode", "Column", "Skills", ".cfgfc-tmp-staged"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := warehouse.LoadWarehouse(root)
	if err != nil {
		t.Fatalf("LoadWarehouse: %v", err)
	}
	if _, ok := loaded.Projects[".cfgfc-transactions"]; ok {
		t.Fatal("transaction directory discovered as Project")
	}
	if _, ok := loaded.Projects["OpenCode"].Columns["Skills"].Settings[".cfgfc-tmp-staged"]; ok {
		t.Fatal("temporary staging file discovered as Setting")
	}
}

// TestSnapshotIncludesAbsentPathRemovesNewObject verifies requested absence is restored by rollback and recovery.
func TestSnapshotIncludesAbsentPathRemovesNewObject(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "new.json")
	repository := New(root)
	if err := repository.WithMutation("absent", []string{path}, func() error {
		if err := WriteFileAtomic(path, []byte("new"), 0o644); err != nil {
			return err
		}
		return errors.New("rollback")
	}); err == nil {
		t.Fatal("expected rollback failure")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("rolled-back new path error = %v", err)
	}
	transaction, err := repository.BeginMutation("absent-recovery", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := transaction.LeavePrepared(); err != nil {
		t.Fatal(err)
	}
	if err := repository.Recover(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("recovered new path error = %v", err)
	}
}

// TestStaleLockRecoveryDoesNotRemoveLiveOwner verifies dead locks are reclaimed but live locks remain.
func TestStaleLockRecoveryDoesNotRemoveLiveOwner(t *testing.T) {
	root := t.TempDir()
	lockPath := filepath.Join(root, mutationLockDirectoryName)
	if err := os.MkdirAll(lockPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := saveLockRecord(lockPath, lockRecord{PID: 99999999, StartedAt: nowUTC(), Token: "dead"}); err != nil {
		t.Fatal(err)
	}
	lock, err := acquireMutationLock(root)
	if err != nil {
		t.Fatalf("dead lock was not reclaimed: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	livePath := filepath.Join(root, mutationLockDirectoryName)
	if err := os.MkdirAll(livePath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := saveLockRecord(livePath, lockRecord{PID: os.Getpid(), StartedAt: nowUTC(), Token: "live"}); err != nil {
		t.Fatal(err)
	}
	if _, err := acquireMutationLock(root); err == nil {
		t.Fatal("live lock was incorrectly reclaimed")
	}
}

// TestFaultInjectionAtDurableStagesLeavesNoCommittedMutation verifies each injected stage.
func TestFaultInjectionAtDurableStagesLeavesNoCommittedMutation(t *testing.T) {
	for _, stage := range []Stage{StageRecovery, StageSnapshot, StagePrepared, StageWrite, StageCommitted, StageCleanup} {
		t.Run(string(stage), func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "state.json")
			if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
				t.Fatal(err)
			}
			repository := New(root, WithHooks(Hooks{BeforeStage: func(current Stage) error {
				if current == stage {
					return errors.New("injected")
				}
				return nil
			}}))
			err := repository.WithMutation("fault", []string{path}, func() error {
				return WriteFileAtomic(path, []byte("after"), 0o644)
			})
			if err == nil {
				t.Fatal("expected injected failure")
			}
			if recoveryErr := New(root).Recover(); recoveryErr != nil {
				t.Fatalf("recover after injected failure: %v", recoveryErr)
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(data) != "before" {
				t.Fatalf("content after %s failure = %q", stage, data)
			}
		})
	}
}

// TestConcurrentMutationRejectsLiveOwner verifies concurrent writers cannot overlap or reclaim a live lock.
func TestConcurrentMutationRejectsLiveOwner(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "state.json")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	repository := New(root, WithHooks(Hooks{BeforeStage: func(stage Stage) error {
		if stage == StageWrite {
			close(entered)
			<-release
		}
		return nil
	}}))
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- repository.WithMutation("first", []string{path}, func() error {
			return WriteFileAtomic(path, []byte("first"), 0o644)
		})
	}()
	<-entered
	if err := New(root).WithMutation("second", []string{path}, func() error {
		return WriteFileAtomic(path, []byte("second"), 0o644)
	}); err == nil {
		t.Fatal("concurrent mutation unexpectedly entered live lock")
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first mutation: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "first" {
		t.Fatalf("concurrent mutation content = %q", data)
	}
	if err := New(root).WithMutation("after", []string{path}, func() error {
		return WriteFileAtomic(path, []byte("after"), 0o644)
	}); err != nil {
		t.Fatalf("mutation after live owner released: %v", err)
	}
}
