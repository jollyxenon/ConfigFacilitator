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

// ApplyIntent stores the semantic apply operation that produced the current mappings.
type ApplyIntent struct {
	Kind     string   `json:"kind"`
	Mode     string   `json:"mode,omitempty"`
	Column   string   `json:"column,omitempty"`
	Settings []string `json:"settings,omitempty"`
}

// CurrentState stores the currently active project-owned mappings and optional apply intent.
type CurrentState struct {
	Mappings []Mapping    `json:"mappings"`
	Intent   *ApplyIntent `json:"intent,omitempty"`
}

// HistoryEntry stores one single-step restore snapshot event.
type HistoryEntry struct {
	Timestamp        string       `json:"timestamp"`
	PreviousMappings []Mapping    `json:"previousMappings"`
	NextMappings     []Mapping    `json:"nextMappings"`
	PreviousIntent   *ApplyIntent `json:"previousIntent,omitempty"`
	NextIntent       *ApplyIntent `json:"nextIntent,omitempty"`
}

// Engine performs filesystem-safe link lifecycle operations.
type Engine struct {
	now       func() time.Time
	writeFile func(path string, data []byte, perm os.FileMode) error
}

// replaceOptions controls destructive replace/reset behavior.
type replaceOptions struct {
	force bool
}

// ReplaceOption customizes linker mutation behavior.
type ReplaceOption func(*replaceOptions)

// WithForce enables destructive target reclamation for one engine operation.
func WithForce(force bool) ReplaceOption {
	return func(options *replaceOptions) {
		options.force = force
	}
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
	state, err := engine.LoadPreviousState(project)
	if err != nil {
		return nil, err
	}
	return cloneMappings(state.Mappings), nil
}

// LoadPreviousState reads the most recent previous state from history.
func (engine Engine) LoadPreviousState(project warehouse.Project) (CurrentState, error) {
	data, err := os.ReadFile(project.HistoryLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return CurrentState{Mappings: []Mapping{}}, nil
		}
		return CurrentState{}, err
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
			return CurrentState{}, err
		}
		found = true
	}
	if err := scanner.Err(); err != nil {
		return CurrentState{}, err
	}
	if !found {
		return CurrentState{Mappings: []Mapping{}}, nil
	}
	return cloneState(CurrentState{Mappings: last.PreviousMappings, Intent: last.PreviousIntent}), nil
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
func (engine Engine) ReplaceMappings(project warehouse.Project, next []Mapping, opts ...ReplaceOption) error {
	return engine.ReplaceState(project, CurrentState{Mappings: next}, opts...)
}

// ReplaceState installs a new managed state and persists mappings plus optional intent atomically.
func (engine Engine) ReplaceState(project warehouse.Project, nextState CurrentState, opts ...ReplaceOption) error {
	options := buildReplaceOptions(opts)
	currentState, err := engine.LoadCurrentState(project)
	if err != nil {
		return err
	}
	previousState := cloneState(currentState)
	nextState = cloneState(nextState)
	if nextState.Mappings == nil {
		nextState.Mappings = []Mapping{}
	}
	if err := engine.validateMappings(nextState.Mappings); err != nil {
		return err
	}
	if err := engine.ensureReplacementAllowed(previousState.Mappings, nextState.Mappings, options); err != nil {
		return err
	}
	if err := engine.applyMappingSet(previousState.Mappings, nextState.Mappings, options); err != nil {
		return err
	}
	if err := engine.persistState(project, previousState, nextState); err != nil {
		rollbackErr := engine.applyMappingSet(nextState.Mappings, previousState.Mappings, options)
		if rollbackErr != nil {
			return fmt.Errorf("persist state: %w; rollback: %v", err, rollbackErr)
		}
		return err
	}
	return nil
}

// Reset removes the current mappings and persists an empty current state.
func (engine Engine) Reset(project warehouse.Project, opts ...ReplaceOption) error {
	return engine.ReplaceState(project, CurrentState{Mappings: []Mapping{}}, opts...)
}

func (engine Engine) validateMappings(mappings []Mapping) error {
	seenTargets := map[string]struct{}{}
	for _, mapping := range mappings {
		if mapping.Source == "" || mapping.Target == "" {
			return fmt.Errorf("mapping source and target must both be set")
		}
		if _, exists := seenTargets[mapping.Target]; exists {
			return fmt.Errorf("duplicate target %s", mapping.Target)
		}
		seenTargets[mapping.Target] = struct{}{}
	}
	return nil
}

// ensureReplacementAllowed rejects unmanaged or drifted targets unless force is enabled.
func (engine Engine) ensureReplacementAllowed(previous []Mapping, next []Mapping, options replaceOptions) error {
	if options.force {
		return nil
	}
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

// applyMappingSet removes stale targets and creates the next managed mappings.
func (engine Engine) applyMappingSet(previous []Mapping, next []Mapping, options replaceOptions) error {
	previousByTarget := mappingIndex(previous)
	nextByTarget := mappingIndex(next)
	for _, mapping := range previous {
		if _, keep := nextByTarget[mapping.Target]; keep {
			continue
		}
		if err := removeManagedTarget(mapping, options.force); err != nil {
			return err
		}
	}
	for _, mapping := range next {
		if current, ok := previousByTarget[mapping.Target]; ok {
			if current.Source == mapping.Source {
				if !options.force {
					continue
				}
				ownership, err := engine.InspectOwnership(mapping)
				if err != nil {
					return err
				}
				if ownership == OwnershipOwned {
					continue
				}
				if err := removeTargetPath(mapping.Target); err != nil {
					return err
				}
				if err := createOwnedSymlink(mapping); err != nil {
					return err
				}
				continue
			}
			if err := removeManagedTarget(current, options.force); err != nil {
				return err
			}
		}
		if options.force {
			if err := removeTargetPath(mapping.Target); err != nil {
				return err
			}
		}
		if err := createOwnedSymlink(mapping); err != nil {
			return err
		}
	}
	return nil
}

// buildReplaceOptions materializes operation options from variadic setters.
func buildReplaceOptions(opts []ReplaceOption) replaceOptions {
	options := replaceOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	return options
}

func (engine Engine) persistState(project warehouse.Project, previous CurrentState, next CurrentState) error {
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
	stateData, err := json.MarshalIndent(cloneState(next), "", "  ")
	if err != nil {
		return err
	}
	historyEntry, err := json.Marshal(HistoryEntry{
		Timestamp:        engine.now().UTC().Format(time.RFC3339Nano),
		PreviousMappings: cloneMappings(previous.Mappings),
		NextMappings:     cloneMappings(next.Mappings),
		PreviousIntent:   cloneIntent(previous.Intent),
		NextIntent:       cloneIntent(next.Intent),
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

// removeManagedTarget removes one recorded target with optional force semantics.
func removeManagedTarget(mapping Mapping, force bool) error {
	if force {
		return removeTargetPath(mapping.Target)
	}
	return removeOwnedSymlink(mapping)
}

// removeTargetPath deletes the exact target path, recursively when it is a directory.
func removeTargetPath(target string) error {
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(target)
	}
	if info.IsDir() {
		return os.RemoveAll(target)
	}
	return os.Remove(target)
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

func cloneIntent(intent *ApplyIntent) *ApplyIntent {
	if intent == nil {
		return nil
	}
	cloned := *intent
	cloned.Settings = append([]string{}, intent.Settings...)
	return &cloned
}

func cloneState(state CurrentState) CurrentState {
	cloned := CurrentState{
		Mappings: cloneMappings(state.Mappings),
		Intent:   cloneIntent(state.Intent),
	}
	if cloned.Mappings == nil {
		cloned.Mappings = []Mapping{}
	}
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
