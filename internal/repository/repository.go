package repository

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/xenon/ConfigFacilitator/internal/index"
)

const (
	transactionDirectoryName  = ".cfgfc-transactions"
	mutationLockDirectoryName = ".cfgfc-lock"
	manifestFileName          = "manifest.json"
)

// ErrUnsupportedCurrentSchema reports a legacy or unknown current-state schema.
var ErrUnsupportedCurrentSchema = fmt.Errorf("current_state schema unsupported")

// CurrentState stores the currently active project-owned mappings and selection.
// The Current state is itself the temporary Mode: columns holds the authoritative
// selection, relation describes whether it follows a named Mode or has detached,
// and mappings keeps the planned links (including increment baselines).
type CurrentState struct {
	Columns  map[string]ColumnSelection `json:"columns"`
	Relation *CurrentRelation           `json:"relation,omitempty"`
	Mappings []Mapping                  `json:"mappings"`
	Extra    map[string]json.RawMessage `json:"-"`
}

// ColumnSelection stores one Current Column selection.
type ColumnSelection struct {
	Strategy string   `json:"strategy"`
	Settings []string `json:"settings"`
}

// CurrentRelation describes how the Current state relates to a named Mode.
type CurrentRelation struct {
	Kind       string `json:"kind"`
	OriginMode string `json:"originMode"`
}

// Mapping stores one source-target pair managed by the link engine.
type Mapping struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// HistoryEntry stores one single-step restore snapshot event.
type HistoryEntry struct {
	Timestamp        string                     `json:"timestamp"`
	PreviousColumns  map[string]ColumnSelection `json:"previousColumns"`
	NextColumns      map[string]ColumnSelection `json:"nextColumns"`
	PreviousRelation *CurrentRelation           `json:"previousRelation,omitempty"`
	NextRelation     *CurrentRelation           `json:"nextRelation,omitempty"`
	PreviousMappings []Mapping                  `json:"previousMappings"`
	NextMappings     []Mapping                  `json:"nextMappings"`
	Extra            map[string]json.RawMessage `json:"-"`
}

// SessionRecord stores a PPID-scoped project selection and extension fields.
type SessionRecord struct {
	Project string                     `json:"project"`
	Extra   map[string]json.RawMessage `json:"-"`
}

// MarshalJSON preserves unknown current-state fields while known fields win collisions.
func (state CurrentState) MarshalJSON() ([]byte, error) {
	data := map[string]any{
		"columns":  columnsOrEmpty(state.Columns),
		"mappings": state.MappingsOrEmpty(),
	}
	if state.Relation != nil {
		data["relation"] = state.Relation
	}
	mergeRaw(data, state.Extra)
	return json.Marshal(data)
}

// UnmarshalJSON parses current state and retains unknown fields.
func (state *CurrentState) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	state.Mappings = []Mapping{}
	state.Extra = map[string]json.RawMessage{}
	if _, legacy := raw["intent"]; legacy {
		return ErrUnsupportedCurrentSchema
	}
	for key, value := range raw {
		switch key {
		case "columns":
			if err := json.Unmarshal(value, &state.Columns); err != nil {
				return err
			}
		case "relation":
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				state.Relation = nil
				continue
			}
			var relation CurrentRelation
			if err := json.Unmarshal(value, &relation); err != nil {
				return err
			}
			state.Relation = &relation
		case "mappings":
			if err := json.Unmarshal(value, &state.Mappings); err != nil {
				return err
			}
		default:
			state.Extra[key] = value
		}
	}
	return nil
}

// MarshalJSON preserves unknown history fields while known fields win collisions.
func (entry HistoryEntry) MarshalJSON() ([]byte, error) {
	data := map[string]any{
		"timestamp":        entry.Timestamp,
		"previousColumns":  columnsOrEmpty(entry.PreviousColumns),
		"nextColumns":      columnsOrEmpty(entry.NextColumns),
		"previousMappings": mappingsOrEmpty(entry.PreviousMappings),
		"nextMappings":     mappingsOrEmpty(entry.NextMappings),
	}
	if entry.PreviousRelation != nil {
		data["previousRelation"] = entry.PreviousRelation
	}
	if entry.NextRelation != nil {
		data["nextRelation"] = entry.NextRelation
	}
	mergeRaw(data, entry.Extra)
	return json.Marshal(data)
}

// UnmarshalJSON parses a history record and retains unknown fields.
func (entry *HistoryEntry) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	entry.Extra = map[string]json.RawMessage{}
	for key, value := range raw {
		switch key {
		case "timestamp":
			if err := json.Unmarshal(value, &entry.Timestamp); err != nil {
				return err
			}
		case "previousColumns":
			if err := json.Unmarshal(value, &entry.PreviousColumns); err != nil {
				return err
			}
		case "nextColumns":
			if err := json.Unmarshal(value, &entry.NextColumns); err != nil {
				return err
			}
		case "previousRelation":
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				entry.PreviousRelation = nil
				continue
			}
			var relation CurrentRelation
			if err := json.Unmarshal(value, &relation); err != nil {
				return err
			}
			entry.PreviousRelation = &relation
		case "nextRelation":
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				entry.NextRelation = nil
				continue
			}
			var relation CurrentRelation
			if err := json.Unmarshal(value, &relation); err != nil {
				return err
			}
			entry.NextRelation = &relation
		case "previousMappings":
			if err := json.Unmarshal(value, &entry.PreviousMappings); err != nil {
				return err
			}
		case "nextMappings":
			if err := json.Unmarshal(value, &entry.NextMappings); err != nil {
				return err
			}
		default:
			entry.Extra[key] = value
		}
	}
	if entry.PreviousMappings == nil {
		entry.PreviousMappings = []Mapping{}
	}
	if entry.NextMappings == nil {
		entry.NextMappings = []Mapping{}
	}
	return nil
}

// MarshalJSON preserves unknown session fields while known fields win collisions.
func (record SessionRecord) MarshalJSON() ([]byte, error) {
	data := map[string]any{"project": record.Project}
	mergeRaw(data, record.Extra)
	return json.Marshal(data)
}

// UnmarshalJSON parses a session record and retains unknown fields.
func (record *SessionRecord) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	record.Extra = map[string]json.RawMessage{}
	for key, value := range raw {
		if key == "project" {
			if err := json.Unmarshal(value, &record.Project); err != nil {
				return err
			}
		} else {
			record.Extra[key] = value
		}
	}
	return nil
}

// Repository resolves durable paths and owns warehouse mutation transactions.
type Repository struct {
	RootPath string
	hooks    Hooks
}

// Hooks injects deterministic failures into transaction stages for tests.
type Hooks struct {
	BeforeStage func(Stage) error
}

// Stage identifies a durable transaction stage exposed for fault-injection tests.
type Stage string

const (
	StageRecovery  Stage = "recovery"
	StageSnapshot  Stage = "snapshot"
	StagePrepared  Stage = "prepared"
	StageWrite     Stage = "write"
	StageCommitted Stage = "committed"
	StageCleanup   Stage = "cleanup"
)

// Option configures a Repository.
type Option func(*Repository)

// WithHooks configures transaction fault-injection hooks.
func WithHooks(hooks Hooks) Option { return func(repository *Repository) { repository.hooks = hooks } }

// New constructs a repository rooted at one effective warehouse path.
func New(rootPath string, options ...Option) Repository {
	repository := Repository{RootPath: rootPath}
	for _, option := range options {
		if option != nil {
			option(&repository)
		}
	}
	return repository
}

// TransactionInfo describes an incomplete transaction without changing it.
type TransactionInfo struct {
	Directory string `json:"directory"`
	Operation string `json:"operation"`
	Status    string `json:"status"`
}

// ProjectIndexPath returns the warehouse-wide Project index path.
func (repository Repository) ProjectIndexPath() string {
	return filepath.Join(repository.RootPath, "ProjectIndex.jsonc")
}

// ColumnIndexPath returns a Project's Column index path.
func (repository Repository) ColumnIndexPath(project string) string {
	return filepath.Join(repository.RootPath, project, "Column", "ColumnIndex.jsonc")
}

// SettingIndexPath returns a Column's Setting index path.
func (repository Repository) SettingIndexPath(project string, column string) string {
	return filepath.Join(repository.RootPath, project, "Column", column, "SettingIndex.jsonc")
}

// ModeIndexPath returns a Project's Mode index path.
func (repository Repository) ModeIndexPath(project string) string {
	return filepath.Join(repository.RootPath, project, "Mode", "ModeIndex.jsonc")
}

// CurrentStatePath returns a Project's current-state path.
func (repository Repository) CurrentStatePath(project string) string {
	return filepath.Join(repository.RootPath, project, "Backup", "current_state.json")
}

// HistoryPath returns a Project's newline-delimited history path.
func (repository Repository) HistoryPath(project string) string {
	return filepath.Join(repository.RootPath, project, "Backup", "history.log")
}

// SessionPath returns a PPID-scoped context record path.
func (repository Repository) SessionPath(ppid int) string {
	return filepath.Join(repository.RootPath, ".cfgfc-session", fmt.Sprintf("%d.json", ppid))
}

// LoadProjectIndex loads the warehouse Project index, accepting an absent file as empty.
func (repository Repository) LoadProjectIndex() (index.ProjectIndex, error) {
	return LoadProjectIndex(repository.ProjectIndexPath())
}

// LoadColumnIndex loads a Project Column index, accepting an absent file as empty.
func (repository Repository) LoadColumnIndex(project string) (index.ColumnIndex, error) {
	return LoadColumnIndex(repository.ColumnIndexPath(project))
}

// LoadSettingIndex loads a Column Setting index, accepting an absent file as empty.
func (repository Repository) LoadSettingIndex(project string, column string) (index.SettingIndex, error) {
	return LoadSettingIndex(repository.SettingIndexPath(project, column))
}

// LoadModeIndex loads a Project Mode index, accepting an absent file as empty.
func (repository Repository) LoadModeIndex(project string) (index.ModeIndex, error) {
	return LoadModeIndex(repository.ModeIndexPath(project))
}

// SaveProjectIndex atomically saves the warehouse Project index.
func (repository Repository) SaveProjectIndex(value index.ProjectIndex) error {
	return SaveProjectIndex(repository.ProjectIndexPath(), value)
}

// SaveColumnIndex atomically saves a Project Column index.
func (repository Repository) SaveColumnIndex(project string, value index.ColumnIndex) error {
	return SaveColumnIndex(repository.ColumnIndexPath(project), value)
}

// SaveSettingIndex atomically saves a Column Setting index.
func (repository Repository) SaveSettingIndex(project string, column string, value index.SettingIndex) error {
	return SaveSettingIndex(repository.SettingIndexPath(project, column), value)
}

// SaveModeIndex atomically saves a Project Mode index.
func (repository Repository) SaveModeIndex(project string, value index.ModeIndex) error {
	return SaveModeIndex(repository.ModeIndexPath(project), value)
}

// LoadCurrentState loads one Project's current state through this repository.
func (repository Repository) LoadCurrentState(project string) (CurrentState, error) {
	return LoadCurrentState(repository.CurrentStatePath(project))
}

// SaveCurrentState saves one Project's current state through this repository.
func (repository Repository) SaveCurrentState(project string, state CurrentState) error {
	return SaveCurrentState(repository.CurrentStatePath(project), state)
}

// LoadHistory loads one Project's complete history through this repository.
func (repository Repository) LoadHistory(project string) ([]HistoryEntry, error) {
	return LoadHistory(repository.HistoryPath(project))
}

// SaveHistory saves one Project's complete history through this repository.
func (repository Repository) SaveHistory(project string, entries []HistoryEntry) error {
	return SaveHistory(repository.HistoryPath(project), entries)
}

// LoadSession loads one PPID-scoped context through this repository.
func (repository Repository) LoadSession(ppid int) (SessionRecord, bool, error) {
	return LoadSession(repository.SessionPath(ppid))
}

// SaveSession saves one PPID-scoped context through this repository.
func (repository Repository) SaveSession(ppid int, record SessionRecord) error {
	return SaveSession(repository.SessionPath(ppid), record)
}

// LoadProjectIndex loads one index from a path and accepts an absent file as empty.
func LoadProjectIndex(path string) (index.ProjectIndex, error) {
	data, err := readOptional(path)
	if err != nil {
		return index.ProjectIndex{}, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return index.ProjectIndex{Projects: map[string]index.ProjectEntry{}, Extra: map[string]json.RawMessage{}}, nil
	}
	return index.ParseProjectIndex(data)
}

// LoadColumnIndex loads one index from a path and accepts an absent file as empty.
func LoadColumnIndex(path string) (index.ColumnIndex, error) {
	data, err := readOptional(path)
	if err != nil {
		return index.ColumnIndex{}, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return index.ColumnIndex{Columns: map[string]index.ColumnEntry{}, Extra: map[string]json.RawMessage{}}, nil
	}
	return index.ParseColumnIndex(data)
}

// LoadSettingIndex loads one index from a path and accepts an absent file as empty.
func LoadSettingIndex(path string) (index.SettingIndex, error) {
	data, err := readOptional(path)
	if err != nil {
		return index.SettingIndex{}, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return index.SettingIndex{Settings: map[string]index.SettingEntry{}, Extra: map[string]json.RawMessage{}}, nil
	}
	return index.ParseSettingIndex(data)
}

// LoadModeIndex loads one index from a path and accepts an absent file as empty.
func LoadModeIndex(path string) (index.ModeIndex, error) {
	data, err := readOptional(path)
	if err != nil {
		return index.ModeIndex{}, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return index.ModeIndex{Modes: map[string]index.ModeEntry{}, Extra: map[string]json.RawMessage{}}, nil
	}
	return index.ParseModeIndex(data)
}

// SaveProjectIndex atomically saves an index through its schema-aware MarshalJSON implementation.
func SaveProjectIndex(path string, value index.ProjectIndex) error { return saveIndex(path, value) }

// SaveColumnIndex atomically saves an index through its schema-aware MarshalJSON implementation.
func SaveColumnIndex(path string, value index.ColumnIndex) error { return saveIndex(path, value) }

// SaveSettingIndex atomically saves an index through its schema-aware MarshalJSON implementation.
func SaveSettingIndex(path string, value index.SettingIndex) error { return saveIndex(path, value) }

// SaveModeIndex atomically saves an index through its schema-aware MarshalJSON implementation.
func SaveModeIndex(path string, value index.ModeIndex) error { return saveIndex(path, value) }

// LoadCurrentState loads one runtime current-state record from a path.
func LoadCurrentState(path string) (CurrentState, error) {
	data, err := readOptional(path)
	if err != nil {
		return CurrentState{}, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return CurrentState{Mappings: []Mapping{}, Extra: map[string]json.RawMessage{}}, nil
	}
	var state CurrentState
	if err := json.Unmarshal(data, &state); err != nil {
		return CurrentState{}, err
	}
	if state.Columns == nil {
		return CurrentState{}, ErrUnsupportedCurrentSchema
	}
	if state.Mappings == nil {
		state.Mappings = []Mapping{}
	}
	return state, nil
}

// SaveCurrentState atomically saves one runtime current-state record.
func SaveCurrentState(path string, state CurrentState) error { return saveJSON(path, state) }

// LoadHistory loads all newline-delimited history records from a path.
func LoadHistory(path string) ([]HistoryEntry, error) {
	data, err := readOptional(path)
	if err != nil {
		return nil, err
	}
	return ReadHistory(bytes.NewReader(data))
}

// SaveHistory atomically saves all newline-delimited history records.
func SaveHistory(path string, entries []HistoryEntry) error {
	var data bytes.Buffer
	for _, entry := range entries {
		encoded, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		data.Write(encoded)
		data.WriteByte('\n')
	}
	return WriteFileAtomic(path, data.Bytes(), 0o644)
}

// ReadHistory parses newline-delimited history records from a reader.
func ReadHistory(reader io.Reader) ([]HistoryEntry, error) {
	return ReadHistoryFrom(reader)
}

// LoadSession loads one PPID context record from a path.
func LoadSession(path string) (SessionRecord, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SessionRecord{}, false, nil
		}
		return SessionRecord{}, false, err
	}
	var record SessionRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return SessionRecord{}, false, err
	}
	if strings.TrimSpace(record.Project) == "" {
		return SessionRecord{}, false, nil
	}
	return record, true, nil
}

// SaveSession atomically saves one PPID context record.
func SaveSession(path string, record SessionRecord) error { return saveJSON(path, record) }

// WriteFileAtomic writes bytes through a same-directory synced temporary file and rename.
func WriteFileAtomic(path string, data []byte, requestedPerm os.FileMode) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	perm := requestedPerm.Perm()
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to replace symlink %q", path)
		}
		perm = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".cfgfc-tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(perm); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	keep = true
	return syncDirectory(directory)
}

// saveIndex uses an index type's schema-aware marshaler and writes one normalized document.
func saveIndex(path string, value json.Marshaler) error {
	data, err := value.MarshalJSON()
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, append(data, '\n'), 0o644)
}

// saveJSON marshals a runtime repository record and writes it with a trailing newline.
func saveJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return WriteFileAtomic(path, data, 0o644)
}

// readOptional reads a file and maps an absent path to empty bytes.
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

// mergeRaw adds unknown fields without allowing them to override schema fields.
func mergeRaw(destination map[string]any, extra map[string]json.RawMessage) {
	for key, value := range extra {
		if _, exists := destination[key]; !exists {
			destination[key] = json.RawMessage(value)
		}
	}
}

// MappingsOrEmpty returns a non-nil mapping slice for stable JSON output.
func (state CurrentState) MappingsOrEmpty() []Mapping { return mappingsOrEmpty(state.Mappings) }

// mappingsOrEmpty returns a non-nil mapping slice for stable JSON output.
func mappingsOrEmpty(mappings []Mapping) []Mapping {
	if mappings == nil {
		return []Mapping{}
	}
	return mappings
}

// columnsOrEmpty returns a non-nil columns map for stable JSON output.
func columnsOrEmpty(columns map[string]ColumnSelection) map[string]ColumnSelection {
	if columns == nil {
		return map[string]ColumnSelection{}
	}
	return columns
}

// syncDirectory flushes a containing directory where the platform supports it.
func syncDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

// sortedNames returns deterministic names for transaction and diagnostic output.
func sortedNames(values []string) []string {
	result := append([]string{}, values...)
	sort.Strings(result)
	return result
}

// nowUTC returns the transaction timestamp used by manifests.
func nowUTC() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// WriteJSONAtomic marshals any JSON value and writes it atomically with a trailing newline.
func WriteJSONAtomic(path string, value any) error {
	return saveJSON(path, value)
}
