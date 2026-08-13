package repository

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// transactionManifest is the durable journal for one warehouse mutation.
type transactionManifest struct {
	ID        string           `json:"id"`
	Operation string           `json:"operation"`
	Status    string           `json:"status"`
	CreatedAt string           `json:"createdAt"`
	Snapshots []snapshotRecord `json:"snapshots"`
}

// snapshotRecord describes one exact pre-mutation filesystem object.
type snapshotRecord struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Mode uint32 `json:"mode"`
	Link string `json:"link,omitempty"`
}

// lockRecord identifies one process instance holding a warehouse mutation lock.
type lockRecord struct {
	PID       int    `json:"pid"`
	StartedAt string `json:"startedAt"`
	Token     string `json:"token"`
}

// Transaction is a prepared warehouse mutation with exact filesystem snapshots.
type Transaction struct {
	repository Repository
	id         string
	directory  string
	manifest   transactionManifest
	lock       *mutationLock
	finished   bool
}

// BeginMutation acquires the warehouse-wide lock, recovers old work, and prepares a snapshot.
func (repository Repository) BeginMutation(operation string, paths ...string) (*Transaction, error) {
	lock, err := acquireMutationLock(repository.RootPath)
	if err != nil {
		return nil, err
	}
	if err := repository.runHook(StageRecovery); err != nil {
		_ = lock.Close()
		return nil, err
	}
	if err := repository.recoverLocked(); err != nil {
		_ = lock.Close()
		return nil, err
	}
	transaction, err := repository.beginLocked(operation, paths)
	if err != nil {
		_ = lock.Close()
		return nil, err
	}
	transaction.lock = lock
	return transaction, nil
}

// WithMutation runs a callback inside a recoverable prepared transaction.
func (repository Repository) WithMutation(operation string, paths []string, mutate func() error) error {
	transaction, err := repository.BeginMutation(operation, paths...)
	if err != nil {
		return err
	}
	if err := repository.runHook(StageWrite); err != nil {
		rollbackErr := transaction.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("write stage: %w; rollback: %v", err, rollbackErr)
		}
		return err
	}
	if err := mutate(); err != nil {
		rollbackErr := transaction.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("mutation: %w; rollback: %v", err, rollbackErr)
		}
		return err
	}
	if err := transaction.Commit(); err != nil {
		rollbackErr := transaction.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("commit: %w; rollback: %v", err, rollbackErr)
		}
		return err
	}
	return nil
}

// Commit marks a transaction committed and removes its durable journal.
func (transaction *Transaction) Commit() error {
	if transaction == nil || transaction.finished {
		return nil
	}
	if err := transaction.repository.runHook(StageCommitted); err != nil {
		return err
	}
	transaction.manifest.Status = "committed"
	if err := saveJSON(filepath.Join(transaction.directory, manifestFileName), transaction.manifest); err != nil {
		return err
	}
	if err := transaction.repository.runHook(StageCleanup); err != nil {
		return err
	}
	if err := os.RemoveAll(transaction.directory); err != nil {
		return err
	}
	transaction.finished = true
	return transaction.closeLock()
}

// Rollback restores every snapshotted path and removes the transaction journal.
func (transaction *Transaction) Rollback() error {
	if transaction == nil || transaction.finished {
		return nil
	}
	if err := transaction.restore(); err != nil {
		return err
	}
	if err := os.RemoveAll(transaction.directory); err != nil {
		return err
	}
	transaction.finished = true
	return transaction.closeLock()
}

// LeavePrepared closes the process lock while intentionally retaining a prepared journal.
// It is useful to model a process stop in restart-recovery tests.
func (transaction *Transaction) LeavePrepared() error {
	if transaction == nil || transaction.finished {
		return nil
	}
	transaction.finished = true
	return transaction.closeLock()
}

// closeLock releases the transaction's warehouse lock.
func (transaction *Transaction) closeLock() error {
	if transaction.lock == nil {
		return nil
	}
	err := transaction.lock.Close()
	transaction.lock = nil
	return err
}

// ID returns the durable transaction identifier.
func (transaction *Transaction) ID() string {
	if transaction == nil {
		return ""
	}
	return transaction.id
}

// Directory returns the reserved transaction directory.
func (transaction *Transaction) Directory() string {
	if transaction == nil {
		return ""
	}
	return transaction.directory
}

// Recover restores all prepared transactions and removes committed journals.
func (repository Repository) Recover() error {
	lock, err := acquireMutationLock(repository.RootPath)
	if err != nil {
		return err
	}
	defer lock.Close()
	return repository.recoverLocked()
}

// Diagnostics reports incomplete transactions without changing any filesystem state.
func (repository Repository) Diagnostics() ([]TransactionInfo, error) {
	entries, err := os.ReadDir(filepath.Join(repository.RootPath, transactionDirectoryName))
	if err != nil {
		if os.IsNotExist(err) {
			return []TransactionInfo{}, nil
		}
		return nil, err
	}
	result := []TransactionInfo{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(repository.RootPath, transactionDirectoryName, entry.Name(), manifestFileName)
		data, readErr := os.ReadFile(manifestPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				// Crash before the prepared manifest was written: no mutation ever
				// started, so the staging directory is safe to discard.
				continue
			}
			return nil, readErr
		}
		var manifest transactionManifest
		if unmarshalErr := json.Unmarshal(data, &manifest); unmarshalErr != nil {
			return nil, unmarshalErr
		}
		if manifest.Status == "committed" {
			continue
		}
		result = append(result, TransactionInfo{Directory: filepath.Dir(manifestPath), Operation: manifest.Operation, Status: manifest.Status})
	}
	return result, nil
}

// beginLocked snapshots affected paths and durably records the prepared manifest.
func (repository Repository) beginLocked(operation string, paths []string) (transaction *Transaction, err error) {
	if strings.TrimSpace(operation) == "" {
		return nil, fmt.Errorf("transaction operation cannot be empty")
	}
	id := fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())
	directory := filepath.Join(repository.RootPath, transactionDirectoryName, id)
	if err := os.MkdirAll(filepath.Join(directory, "snapshot"), 0o700); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(directory)
		}
	}()
	transaction = &Transaction{repository: repository, id: id, directory: directory, manifest: transactionManifest{ID: id, Operation: operation, Status: "preparing", CreatedAt: nowUTC()}}
	if err = repository.runHook(StageSnapshot); err != nil {
		return nil, err
	}
	snapshotPaths, err := collectSnapshotPaths(repository.RootPath, paths)
	if err != nil {
		return nil, err
	}
	for _, path := range snapshotPaths {
		record, snapshotErr := snapshotObject(repository.RootPath, filepath.Join(directory, "snapshot"), path)
		if snapshotErr != nil {
			return nil, snapshotErr
		}
		transaction.manifest.Snapshots = append(transaction.manifest.Snapshots, record)
	}
	transaction.manifest.Status = "prepared"
	if err = repository.runHook(StagePrepared); err != nil {
		return nil, err
	}
	if err = saveJSON(filepath.Join(directory, manifestFileName), transaction.manifest); err != nil {
		return nil, err
	}
	return transaction, nil
}

// recoverLocked restores each prepared transaction before allowing new mutation.
func (repository Repository) recoverLocked() error {
	root := filepath.Join(repository.RootPath, transactionDirectoryName)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		directory := filepath.Join(root, entry.Name())
		manifestPath := filepath.Join(directory, manifestFileName)
		data, readErr := os.ReadFile(manifestPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				// Crash before the prepared manifest was written: the mutation never
				// started, so discard the empty staging directory and continue.
				if err := os.RemoveAll(directory); err != nil {
					return err
				}
				continue
			}
			return readErr
		}
		var manifest transactionManifest
		if unmarshalErr := json.Unmarshal(data, &manifest); unmarshalErr != nil {
			return unmarshalErr
		}
		if manifest.Status == "committed" {
			if err := os.RemoveAll(directory); err != nil {
				return err
			}
			continue
		}
		transaction := &Transaction{repository: repository, id: manifest.ID, directory: directory, manifest: manifest, finished: false}
		if err := transaction.restore(); err != nil {
			return err
		}
		if err := os.RemoveAll(directory); err != nil {
			return err
		}
	}
	return nil
}

// restore removes affected current objects and reconstructs their exact snapshots.
func (transaction *Transaction) restore() error {
	for index := len(transaction.manifest.Snapshots) - 1; index >= 0; index-- {
		record := transaction.manifest.Snapshots[index]
		path := filepath.Clean(record.Path)
		if err := removeObject(path); err != nil {
			return err
		}
	}
	for _, record := range transaction.manifest.Snapshots {
		if err := restoreObject(transaction.repository.RootPath, filepath.Join(transaction.directory, "snapshot"), record); err != nil {
			return err
		}
	}
	return nil
}

// collectSnapshotPaths expands requested paths into deterministic top-level objects.
func collectSnapshotPaths(root string, requested []string) ([]string, error) {
	if len(requested) == 0 {
		requested = []string{root}
	}
	seen := map[string]struct{}{}
	paths := []string{}
	for _, requestedPath := range requested {
		path := requestedPath
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		path = filepath.Clean(path)
		if path == root {
			entries, err := os.ReadDir(root)
			if err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			for _, entry := range entries {
				if entry.Name() == transactionDirectoryName || entry.Name() == mutationLockDirectoryName {
					continue
				}
				if err := addSnapshotPath(root, filepath.Join(root, entry.Name()), seen, &paths); err != nil {
					return nil, err
				}
			}
			continue
		}
		if err := addSnapshotPath(root, path, seen, &paths); err != nil {
			return nil, err
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// addSnapshotPath records both existing and absent requested paths.
func addSnapshotPath(root, path string, seen map[string]struct{}, paths *[]string) error {
	if _, ok := seen[path]; ok {
		return nil
	}
	if _, err := filepath.Rel(root, path); err != nil {
		return fmt.Errorf("snapshot path %q cannot be normalized: %w", path, err)
	}
	if _, err := os.Lstat(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	seen[path] = struct{}{}
	*paths = append(*paths, path)
	return nil
}

// snapshotObject copies one exact object into transaction staging or records absence.
func snapshotObject(root, snapshotRoot, path string) (snapshotRecord, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return snapshotRecord{Path: filepath.Clean(path), Kind: "absent"}, nil
		}
		return snapshotRecord{}, err
	}
	record := snapshotRecord{Path: filepath.Clean(path), Mode: uint32(info.Mode().Perm())}
	destination := snapshotObjectPath(snapshotRoot, record.Path)
	if info.Mode()&os.ModeSymlink != 0 {
		record.Kind = "symlink"
		record.Link, err = os.Readlink(path)
		if err == nil {
			err = os.MkdirAll(filepath.Dir(destination), 0o700)
		}
		if err == nil {
			err = os.Symlink(record.Link, destination)
		}
		return record, err
	}
	if info.IsDir() {
		record.Kind = "directory"
		return record, copyDirectory(path, destination)
	}
	record.Kind = "file"
	return record, copyRegularFile(path, destination, info.Mode().Perm())
}

// restoreObject reconstructs one staged snapshot object at its original path.
func restoreObject(root, snapshotRoot string, record snapshotRecord) error {
	source := snapshotObjectPath(snapshotRoot, record.Path)
	destination := filepath.Clean(record.Path)
	switch record.Kind {
	case "absent":
		return nil
	case "directory":
		return copyDirectory(source, destination)
	case "file":
		return copyRegularFile(source, destination, os.FileMode(record.Mode))
	case "symlink":
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		return os.Symlink(record.Link, destination)
	default:
		return fmt.Errorf("unknown snapshot kind %q", record.Kind)
	}
}

// snapshotObjectPath maps an absolute path to a safe staged object name.
func snapshotObjectPath(snapshotRoot string, path string) string {
	hash := sha256.Sum256([]byte(filepath.Clean(path)))
	return filepath.Join(snapshotRoot, hex.EncodeToString(hash[:]))
}

// copyRegularFile copies one regular file into a destination while preserving mode.
func copyRegularFile(source, destination string, mode os.FileMode) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("snapshot source %q is not a regular file", source)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	return WriteFileAtomic(destination, data, mode)
}

// copyDirectory recursively copies directories, regular files, and symlinks.
func copyDirectory(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("snapshot source %q is not a directory", source)
	}
	if err := os.MkdirAll(destination, info.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(source, entry.Name())
		destinationPath := filepath.Join(destination, entry.Name())
		entryInfo, err := os.Lstat(sourcePath)
		if err != nil {
			return err
		}
		if entryInfo.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(sourcePath)
			if err != nil {
				return err
			}
			if err := os.Symlink(link, destinationPath); err != nil {
				return err
			}
		} else if entryInfo.IsDir() {
			if err := copyDirectory(sourcePath, destinationPath); err != nil {
				return err
			}
		} else if entryInfo.Mode().IsRegular() {
			if err := copyRegularFile(sourcePath, destinationPath, entryInfo.Mode().Perm()); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("snapshot object %q is unsupported", sourcePath)
		}
	}
	return nil
}

// removeObject removes one path without following symlinks.
// removeObject removes one path without following symlinks.
func removeObject(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

// runHook invokes an optional fault-injection hook.
// runHook invokes an optional fault-injection hook.
func (repository Repository) runHook(stage Stage) error {
	if repository.hooks.BeforeStage == nil {
		return nil
	}
	return repository.hooks.BeforeStage(stage)
}

// mutationLock represents the warehouse-wide exclusive lock and its unique owner token.
type mutationLock struct {
	path  string
	token string
}

// acquireMutationLock creates a process lock directory atomically and reclaims only dead owners.
func acquireMutationLock(root string) (*mutationLock, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(root, mutationLockDirectoryName)
	for attempts := 0; attempts < 3; attempts++ {
		token := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
		if err := os.Mkdir(path, 0o700); err == nil {
			record := lockRecord{PID: os.Getpid(), StartedAt: nowUTC(), Token: token}
			if err := saveLockRecord(path, record); err != nil {
				_ = os.RemoveAll(path)
				return nil, err
			}
			return &mutationLock{path: path, token: token}, nil
		} else if !os.IsExist(err) {
			return nil, fmt.Errorf("create warehouse mutation lock: %w", err)
		}

		reclaimed, err := reclaimStaleLock(path)
		if err != nil {
			return nil, err
		}
		if !reclaimed {
			return nil, fmt.Errorf("warehouse mutation lock is busy")
		}
	}
	return nil, fmt.Errorf("warehouse mutation lock changed repeatedly")
}

// saveLockRecord durably records the lock owner after the atomic directory claim.
func saveLockRecord(lockPath string, record lockRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	ownerPath := filepath.Join(lockPath, "owner.json")
	file, err := os.OpenFile(ownerPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return syncDirectory(lockPath)
}

// reclaimStaleLock renames and removes a lock only when its recorded process is dead.
func reclaimStaleLock(lockPath string) (bool, error) {
	record, err := loadLockRecord(lockPath)
	if err != nil {
		return false, fmt.Errorf("read warehouse mutation lock: %w", err)
	}
	alive, err := processAlive(record.PID)
	if err != nil {
		return false, fmt.Errorf("inspect warehouse mutation lock owner %d: %w", record.PID, err)
	}
	if alive {
		return false, nil
	}
	stalePath := lockPath + ".stale-" + record.Token
	if err := os.Rename(lockPath, stalePath); err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	if err := os.RemoveAll(stalePath); err != nil {
		return false, err
	}
	return true, nil
}

// loadLockRecord reads a complete owner record and rejects unsafe legacy or partial locks.
func loadLockRecord(lockPath string) (lockRecord, error) {
	data, err := os.ReadFile(filepath.Join(lockPath, "owner.json"))
	if err != nil {
		return lockRecord{}, err
	}
	var record lockRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return lockRecord{}, err
	}
	if record.PID <= 0 || strings.TrimSpace(record.Token) == "" {
		return lockRecord{}, fmt.Errorf("invalid lock owner record")
	}
	return record, nil
}

// Close releases the warehouse-wide mutation lock only when this instance still owns it.
// Close releases the warehouse-wide mutation lock only when this instance still owns it.
func (lock *mutationLock) Close() error {
	if lock == nil || lock.path == "" {
		return nil
	}
	record, err := loadLockRecord(lock.path)
	if err != nil {
		if os.IsNotExist(err) {
			lock.path = ""
			return nil
		}
		return err
	}
	if record.Token != lock.token {
		return fmt.Errorf("warehouse mutation lock ownership changed")
	}
	releasePath := lock.path + ".release-" + lock.token
	if err := os.Rename(lock.path, releasePath); err != nil {
		return err
	}
	lock.path = ""
	return os.RemoveAll(releasePath)
}

// ReadHistory parses line-delimited history from any reader.
// ReadHistoryFrom parses line-delimited history from any reader.
func ReadHistoryFrom(reader io.Reader) ([]HistoryEntry, error) {
	scanner := bufio.NewScanner(reader)
	entries := []HistoryEntry{}
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var entry HistoryEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}
