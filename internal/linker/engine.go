package linker

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// Ownership describes how a target relates to a requested mapping.
type Ownership string

const (
	OwnershipAbsent    Ownership = "absent"
	OwnershipOwned     Ownership = "owned"
	OwnershipUnmanaged Ownership = "unmanaged"
)

// Mapping stores one source-target pair managed by the engine.
type Mapping struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// CurrentState stores the currently active project-owned mappings.
type CurrentState struct {
	Mappings []Mapping `json:"mappings"`
}

// HistoryEntry stores one single-step restore snapshot event.
type HistoryEntry struct {
	Timestamp        string    `json:"timestamp"`
	PreviousMappings []Mapping `json:"previousMappings"`
	NextMappings     []Mapping `json:"nextMappings"`
}

// Engine performs filesystem-safe link lifecycle operations.
type Engine struct {
	now       func() time.Time
	writeFile func(path string, data []byte, perm os.FileMode) error
}

// New returns an engine with default filesystem behavior.
func New() Engine {
	return Engine{
		now:       time.Now,
		writeFile: os.WriteFile,
	}
}

// LoadCurrentState reads the project's active mapping set.
func (engine Engine) LoadCurrentState(project warehouse.Project) (CurrentState, error) {
	data, err := os.ReadFile(project.CurrentStatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return CurrentState{Mappings: []Mapping{}}, nil
		}
		return CurrentState{}, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return CurrentState{Mappings: []Mapping{}}, nil
	}
	var state CurrentState
	if err := json.Unmarshal(data, &state); err != nil {
		return CurrentState{}, err
	}
	if state.Mappings == nil {
		state.Mappings = []Mapping{}
	}
	return state, nil
	}

// LoadPreviousSnapshot reads the most recent previous mapping set from history.
func (engine Engine) LoadPreviousSnapshot(project warehouse.Project) ([]Mapping, error) {
	data, err := os.ReadFile(project.HistoryLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Mapping{}, nil
		}
		return nil, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var last HistoryEntry
	found := false
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		if err := json.Unmarshal(line, &last); err != nil {
			return nil, err
		}
		found = true
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if !found {
		return []Mapping{}, nil
	}
	return cloneMappings(last.PreviousMappings), nil
}

// InspectOwnership reports whether the target is absent, owned by the exact mapping, or unmanaged.
func (engine Engine) InspectOwnership(mapping Mapping) (Ownership, error) {
	info, err := os.Lstat(mapping.Target)
	if err != nil {
		if os.IsNotExist(err) {
			return OwnershipAbsent, nil
		}
		return OwnershipUnmanaged, err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return OwnershipUnmanaged, nil
	}
	resolved, err := os.Readlink(mapping.Target)
	if err != nil {
		return OwnershipUnmanaged, err
	}
	if resolved == mapping.Source {
		return OwnershipOwned, nil
	}
	return OwnershipUnmanaged, nil
}

// ReplaceMappings installs a new managed mapping set and persists current state and history.
func (engine Engine) ReplaceMappings(project warehouse.Project, next []Mapping) error {
	currentState, err := engine.LoadCurrentState(project)
	if err != nil {
		return err
	}
	previous := cloneMappings(currentState.Mappings)
	if err := engine.validateMappings(next); err != nil {
		return err
	}
	if err := engine.ensureReplacementAllowed(previous, next); err != nil {
		return err
	}
	if err := engine.applyMappingSet(previous, next); err != nil {
		return err
	}
	if err := engine.persistState(project, previous, next); err != nil {
		rollbackErr := engine.applyMappingSet(next, previous)
		if rollbackErr != nil {
			return fmt.Errorf("persist state: %w; rollback: %v", err, rollbackErr)
		}
		return err
	}
	return nil
}

// Reset removes only the currently owned mappings and persists an empty current state.
func (engine Engine) Reset(project warehouse.Project) error {
	return engine.ReplaceMappings(project, []Mapping{})
}

func (engine Engine) validateMappings(mappings []Mapping) error {
	for _, mapping := range mappings {
		if mapping.Source == "" || mapping.Target == "" {
			return fmt.Errorf("mapping source and target must both be set")
		}
	}
	return nil
}

func (engine Engine) ensureReplacementAllowed(previous []Mapping, next []Mapping) error {
	previousByTarget := mappingIndex(previous)
	for _, mapping := range next {
		ownership, err := engine.InspectOwnership(mapping)
		if err != nil {
			return err
		}
		switch ownership {
		case OwnershipAbsent, OwnershipOwned:
			continue
		case OwnershipUnmanaged:
			if previousMapping, ok := previousByTarget[mapping.Target]; ok {
				previousOwnership, inspectErr := engine.InspectOwnership(previousMapping)
				if inspectErr != nil {
					return inspectErr
				}
				if previousOwnership == OwnershipOwned {
					continue
				}
				return fmt.Errorf("managed target %s no longer matches the recorded source", mapping.Target)
			}
			return fmt.Errorf("target %s is unmanaged", mapping.Target)
		}
	}
	for _, mapping := range previous {
		ownership, err := engine.InspectOwnership(mapping)
		if err != nil {
			return err
		}
		if ownership == OwnershipUnmanaged {
			return fmt.Errorf("recorded target %s is no longer owned by source %s", mapping.Target, mapping.Source)
		}
	}
	return nil
}

func (engine Engine) applyMappingSet(previous []Mapping, next []Mapping) error {
	previousByTarget := mappingIndex(previous)
	nextByTarget := mappingIndex(next)
	for _, mapping := range previous {
		if _, keep := nextByTarget[mapping.Target]; keep {
			continue
		}
		if err := removeOwnedSymlink(mapping); err != nil {
			return err
		}
	}
	for _, mapping := range next {
		if current, ok := previousByTarget[mapping.Target]; ok {
			if current.Source == mapping.Source {
				continue
			}
			if err := removeOwnedSymlink(current); err != nil {
				return err
			}
		}
		if err := createOwnedSymlink(mapping); err != nil {
			return err
		}
	}
	return nil
}

func (engine Engine) persistState(project warehouse.Project, previous []Mapping, next []Mapping) error {
	if err := os.MkdirAll(filepath.Dir(project.CurrentStatePath), 0o755); err != nil {
		return err
	}
	oldCurrent, err := readOptional(project.CurrentStatePath)
	if err != nil {
		return err
	}
	oldHistory, err := readOptional(project.HistoryLogPath)
	if err != nil {
		return err
	}
	stateData, err := json.MarshalIndent(CurrentState{Mappings: cloneMappings(next)}, "", "  ")
	if err != nil {
		return err
	}
	historyEntry, err := json.Marshal(HistoryEntry{
		Timestamp:        engine.now().UTC().Format(time.RFC3339Nano),
		PreviousMappings: cloneMappings(previous),
		NextMappings:     cloneMappings(next),
	})
	if err != nil {
		return err
	}
	historyData := append([]byte{}, oldHistory...)
	if len(bytes.TrimSpace(historyData)) > 0 {
		historyData = append(historyData, '\n')
	}
	historyData = append(historyData, historyEntry...)
	historyData = append(historyData, '\n')
	if err := engine.writeFile(project.CurrentStatePath, stateData, 0o644); err != nil {
		return err
	}
	if err := engine.writeFile(project.HistoryLogPath, historyData, 0o644); err != nil {
		if writeBackErr := engine.writeFile(project.CurrentStatePath, oldCurrent, 0o644); writeBackErr != nil {
			return fmt.Errorf("write history: %w; restore current state: %v", err, writeBackErr)
		}
		return err
	}
	return nil
}

func removeOwnedSymlink(mapping Mapping) error {
	info, err := os.Lstat(mapping.Target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("target %s is not a symlink", mapping.Target)
	}
	resolved, err := os.Readlink(mapping.Target)
	if err != nil {
		return err
	}
	if resolved != mapping.Source {
		return fmt.Errorf("target %s does not point to source %s", mapping.Target, mapping.Source)
	}
	return os.Remove(mapping.Target)
}

func createOwnedSymlink(mapping Mapping) error {
	if err := os.MkdirAll(filepath.Dir(mapping.Target), 0o755); err != nil {
		return err
	}
	if err := os.Symlink(mapping.Source, mapping.Target); err != nil {
		return err
	}
	return nil
}

func mappingIndex(mappings []Mapping) map[string]Mapping {
	indexed := make(map[string]Mapping, len(mappings))
	for _, mapping := range mappings {
		indexed[mapping.Target] = mapping
	}
	return indexed
}

func ownedByPrevious(previous []Mapping, target string) bool {
	for _, mapping := range previous {
		if mapping.Target == target {
			return true
		}
	}
	return false
}

func cloneMappings(mappings []Mapping) []Mapping {
	cloned := make([]Mapping, len(mappings))
	copy(cloned, mappings)
	return cloned
}

func readOptional(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []byte{}, nil
		}
		return nil, err
	}
	return data, nil
}

// ReadHistoryEntries returns the parsed history log for inspection and tests.
func ReadHistoryEntries(reader io.Reader) ([]HistoryEntry, error) {
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
