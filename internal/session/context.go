package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/xenon/ConfigFacilitator/internal/repository"
)

const sessionDirName = ".cfgfc-session"

// Store keeps PPID-scoped convenience context for a warehouse root.
type Store struct {
	RootPath string
}

// NewStore constructs a session store rooted at the provided warehouse path.
func NewStore(rootPath string) Store {
	return Store{RootPath: rootPath}
}

// Set writes the active project for a PPID-scoped convenience session.
// Set writes the active project for a PPID-scoped convenience session.
func (store Store) Set(ppid int, project string) error {
	if project == "" {
		return fmt.Errorf("project cannot be empty")
	}
	return repository.SaveSession(store.recordPath(ppid), repository.SessionRecord{Project: project})
}

// Clear removes the active project for a PPID-scoped convenience session.
// Clear removes the active project for a PPID-scoped convenience session.
func (store Store) Clear(ppid int) error {
	err := os.Remove(store.recordPath(ppid))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Get returns the project stored for the given PPID, if any.
// Get returns the project stored for the given PPID, if any.
func (store Store) Get(ppid int) (string, bool, error) {
	record, ok, err := repository.LoadSession(store.recordPath(ppid))
	if err != nil {
		return "", false, err
	}
	return record.Project, ok, nil
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
