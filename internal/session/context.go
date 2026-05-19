package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

const sessionDirName = ".cfgfc-session"

// Store keeps PPID-scoped convenience context for a warehouse root.
type Store struct {
	RootPath string
}

type contextRecord struct {
	Project string `json:"project"`
}

// NewStore constructs a session store rooted at the provided warehouse path.
func NewStore(rootPath string) Store {
	return Store{RootPath: rootPath}
}

// Set writes the active project for a PPID-scoped convenience session.
func (store Store) Set(ppid int, project string) error {
	if project == "" {
		return fmt.Errorf("project cannot be empty")
	}
	if err := os.MkdirAll(store.directoryPath(), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(contextRecord{Project: project})
	if err != nil {
		return err
	}
	return os.WriteFile(store.recordPath(ppid), data, 0o644)
}

// Get returns the project stored for the given PPID, if any.
func (store Store) Get(ppid int) (string, bool, error) {
	data, err := os.ReadFile(store.recordPath(ppid))
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	var record contextRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return "", false, err
	}
	if record.Project == "" {
		return "", false, nil
	}
	return record.Project, true, nil
}

// ResolveProject returns the effective project and whether it came from convenience context.
func ResolveProject(explicitProject string, ppid int, store Store) (string, bool, error) {
	if explicitProject != "" {
		return explicitProject, false, nil
	}
	project, ok, err := store.Get(ppid)
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, nil
	}
	return project, true, nil
}

// directoryPath returns the directory containing PPID-scoped context files.
func (store Store) directoryPath() string {
	return filepath.Join(store.RootPath, sessionDirName)
}

// recordPath returns the file path for a PPID-scoped context record.
func (store Store) recordPath(ppid int) string {
	return filepath.Join(store.directoryPath(), strconv.Itoa(ppid)+".json")
}
