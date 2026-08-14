package linker

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/xenon/ConfigFacilitator/internal/repository"
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
type Mapping = repository.Mapping

// ColumnSelection stores one Current Column selection.
type ColumnSelection = repository.ColumnSelection

// CurrentRelation describes how the Current state relates to a named Mode.
type CurrentRelation = repository.CurrentRelation

// CurrentState stores the currently active project-owned mappings and selection.
type CurrentState = repository.CurrentState

// HistoryEntry stores one single-step restore snapshot event.
type HistoryEntry = repository.HistoryEntry

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
		writeFile: repository.WriteFileAtomic,
	}
}

// LoadCurrentState reads the project's active mapping set.
func (engine Engine) LoadCurrentState(project warehouse.Project) (CurrentState, error) {
	return repository.LoadCurrentState(project.CurrentStatePath)
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
	entries, err := repository.LoadHistory(project.HistoryLogPath)
	if err != nil {
		return CurrentState{}, err
	}
	if len(entries) == 0 {
		return CurrentState{Columns: map[string]ColumnSelection{}, Mappings: []Mapping{}}, nil
	}
	last := entries[len(entries)-1]
	return cloneState(CurrentState{Columns: last.PreviousColumns, Relation: last.PreviousRelation, Mappings: last.PreviousMappings}), nil
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

// ReplaceState installs a new managed state and persists mappings plus the
// Current selection atomically under the warehouse mutation lock.
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
	snapshotPaths := []string{project.CurrentStatePath, project.HistoryLogPath}
	for _, mapping := range append(append([]Mapping{}, previousState.Mappings...), nextState.Mappings...) {
		snapshotPaths = append(snapshotPaths, mapping.Target)
	}
	repositoryRoot := filepath.Dir(project.Path)
	transaction, err := repository.New(repositoryRoot).BeginMutation("link-state", snapshotPaths...)
	if err != nil {
		return err
	}
	restoreManagedLinks := func() error {
		return engine.applyMappingSet(nextState.Mappings, previousState.Mappings, replaceOptions{force: true})
	}
	if err := engine.applyMappingSet(previousState.Mappings, nextState.Mappings, options); err != nil {
		rollbackErr := transaction.Rollback()
		linkRollbackErr := restoreManagedLinks()
		if rollbackErr != nil || linkRollbackErr != nil {
			return fmt.Errorf("apply mappings: %w; snapshot rollback: %v; link rollback: %v", err, rollbackErr, linkRollbackErr)
		}
		return err
	}
	if err := engine.persistState(project, previousState, nextState); err != nil {
		rollbackErr := transaction.Rollback()
		linkRollbackErr := restoreManagedLinks()
		if rollbackErr != nil || linkRollbackErr != nil {
			return fmt.Errorf("persist state: %w; snapshot rollback: %v; link rollback: %v", err, rollbackErr, linkRollbackErr)
		}
		return err
	}
	return transaction.Commit()
}

// ReplaceStateLocked installs a new managed state while the caller already
// holds the warehouse mutation lock. Workflow operations that rewrite the
// ModeIndex and the Current state in one transaction use this entry point;
// their enclosing transaction rolls back all snapshots on failure.
func (engine Engine) ReplaceStateLocked(project warehouse.Project, nextState CurrentState, opts ...ReplaceOption) error {
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
	return engine.persistState(project, previousState, nextState)
}

// replaceSnapshotPaths lists the files and targets covered by one replace.
func replaceSnapshotPaths(project warehouse.Project, nextState CurrentState) []string {
	previous, err := repository.LoadCurrentState(project.CurrentStatePath)
	paths := []string{project.CurrentStatePath, project.HistoryLogPath}
	if err == nil {
		for _, mapping := range append(append([]Mapping{}, previous.Mappings...), nextState.Mappings...) {
			paths = append(paths, mapping.Target)
		}
	}
	return paths
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
	history, err := repository.LoadHistory(project.HistoryLogPath)
	if err != nil {
		return err
	}
	history = append(history, repository.HistoryEntry{
		Timestamp:        engine.now().UTC().Format(time.RFC3339Nano),
		PreviousColumns:  previous.Columns,
		NextColumns:      next.Columns,
		PreviousRelation: previous.Relation,
		NextRelation:     next.Relation,
		PreviousMappings: previous.Mappings,
		NextMappings:     next.Mappings,
	})
	var historyData bytes.Buffer
	for _, entry := range history {
		encoded, marshalErr := json.Marshal(entry)
		if marshalErr != nil {
			return marshalErr
		}
		historyData.Write(encoded)
		historyData.WriteByte('\n')
	}
	if err := engine.writeFile(project.CurrentStatePath, mustMarshalJSON(next), 0o644); err != nil {
		return err
	}
	return engine.writeFile(project.HistoryLogPath, historyData.Bytes(), 0o644)
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
	// Idempotent: an already-in-place owned link (target is a symlink pointing
	// at the recorded source) needs no replacement. This also covers states
	// where the recorded Current mappings are empty but legacy owned links
	// still exist on the filesystem, e.g. right after `sync` rebuilt a missing
	// current_state.json.
	if resolved, err := os.Readlink(mapping.Target); err == nil && resolved == mapping.Source {
		return nil
	}
	if err := createSymlink(mapping.Source, mapping.Target); err != nil {
		return err
	}
	return nil
}

// createSymlink creates a real filesystem symlink after confirming that the
// source exists. This keeps source kind implicit so Windows can infer whether
// the target should be a file or directory symlink without persisting that kind.
func createSymlink(source string, target string) error {
	return createSymlinkForOS(source, target, runtime.GOOS, os.Lstat, os.Symlink)
}

// createSymlinkForOS is the testable core for symlink creation and platform
// diagnostics. It never attempts hardlinks, junctions, copies, or shell fallbacks.
func createSymlinkForOS(source string, target string, operatingSystem string, lstat func(string) (os.FileInfo, error), symlink func(string, string) error) error {
	if _, err := lstat(source); err != nil {
		if os.IsNotExist(err) {
			return wrapSymlinkErrorForOS(operatingSystem, fmt.Errorf("symlink source %s does not exist: %w", source, err))
		}
		return wrapSymlinkErrorForOS(operatingSystem, fmt.Errorf("inspect symlink source %s: %w", source, err))
	}
	if err := symlink(source, target); err != nil {
		return wrapSymlinkErrorForOS(operatingSystem, fmt.Errorf("create symlink %s -> %s: %w", target, source, err))
	}
	return nil
}

// wrapSymlinkErrorForOS adds native Windows guidance while preserving the
// original failure. Non-Windows platforms keep the original error unchanged.
func wrapSymlinkErrorForOS(operatingSystem string, err error) error {
	if operatingSystem != "windows" || err == nil {
		return err
	}
	return fmt.Errorf("%w; ConfigFacilitator uses real symlinks only and did not try hardlinks, junctions, copies, or shell fallbacks; on Windows, enable Developer Mode or run as Administrator to allow symlink creation", err)
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

func cloneColumns(columns map[string]ColumnSelection) map[string]ColumnSelection {
	if columns == nil {
		return nil
	}
	cloned := make(map[string]ColumnSelection, len(columns))
	for name, selection := range columns {
		cloned[name] = ColumnSelection{Strategy: selection.Strategy, Settings: append([]string{}, selection.Settings...)}
	}
	return cloned
}

func cloneRelation(relation *CurrentRelation) *CurrentRelation {
	if relation == nil {
		return nil
	}
	cloned := *relation
	return &cloned
}

func cloneState(state CurrentState) CurrentState {
	cloned := CurrentState{
		Columns:  cloneColumns(state.Columns),
		Relation: cloneRelation(state.Relation),
		Mappings: cloneMappings(state.Mappings),
	}
	if cloned.Columns == nil {
		cloned.Columns = map[string]ColumnSelection{}
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

// mustMarshalJSON serializes a runtime state for injected linker writers.
func mustMarshalJSON(value any) []byte {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil
	}
	return append(data, '\n')
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
