// Package mutate implements validated resource metadata mutations.
package mutate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/xenon/ConfigFacilitator/internal/content"
	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// ResourceKind identifies one configuration resource type.
type ResourceKind string

// Resource kinds select scope and reserved-name rules.
const (
	ProjectKind ResourceKind = "project"
	ColumnKind  ResourceKind = "column"
	SettingKind ResourceKind = "setting"
	ModeKind    ResourceKind = "mode"
)

// ErrorKind identifies a mutation failure that the CLI can classify.
type ErrorKind string

// Mutation error kinds separate invalid input, resource conflicts, and persistence failures.
const (
	InvalidError     ErrorKind = "invalid"
	ConflictError    ErrorKind = "conflict"
	MissingError     ErrorKind = "missing"
	PersistenceError ErrorKind = "persistence"
	RefusalError     ErrorKind = "refusal"
)

// Error is a typed resource-mutation failure.
type Error struct {
	Kind    ErrorKind
	Code    string
	Message string
	Cause   error
	Details any
}

// Error returns the concise mutation failure message.
func (err *Error) Error() string { return err.Message }

// Unwrap returns the underlying implementation error.
func (err *Error) Unwrap() error { return err.Cause }

// ErrorDetails exposes structured domain details to human and JSON renderers.
func (err *Error) ErrorDetails() any { return err.Details }

// Metadata stores the common resource metadata fields.
type Metadata struct {
	DisplayName string
	Description string
	Aliases     []string
}

// MetadataPatch replaces only explicitly supplied metadata fields.
type MetadataPatch struct {
	DisplayName *string
	Description *string
	Aliases     *[]string
}

// Identity is one canonical name and its aliases in a resolution scope.
type Identity struct {
	CanonicalName string
	Aliases       []string
}

// NewMetadata validates and normalizes metadata for a newly created resource.
func NewMetadata(kind ResourceKind, canonicalName string, displayName string, description string, aliases []string) (Metadata, error) {
	if err := ValidateCanonicalName(kind, canonicalName); err != nil {
		return Metadata{}, err
	}
	if displayName == "" {
		displayName = canonicalName
	}
	normalizedAliases, err := NormalizeAliases(kind, aliases)
	if err != nil {
		return Metadata{}, err
	}
	metadata := Metadata{DisplayName: displayName, Description: description, Aliases: normalizedAliases}
	if err := ValidateIdentityScope(kind, Identity{CanonicalName: canonicalName, Aliases: metadata.Aliases}, nil, ""); err != nil {
		return Metadata{}, err
	}
	return metadata, nil
}

// ValidateCanonicalName rejects names that cannot be one safe warehouse path key.
func ValidateCanonicalName(kind ResourceKind, name string) error {
	if name == "" {
		return invalid("invalid_name", fmt.Sprintf("%s canonical name cannot be empty", kind), nil)
	}
	if strings.TrimSpace(name) != name {
		return invalid("invalid_name", fmt.Sprintf("%s canonical name cannot start or end with whitespace", kind), nil)
	}
	if name == "." || name == ".." || filepath.IsAbs(name) || strings.ContainsAny(name, `/\\`) {
		return invalid("invalid_name", fmt.Sprintf("%s canonical name %q must be one relative path component", kind, name), nil)
	}
	for _, character := range name {
		if unicode.IsControl(character) {
			return invalid("invalid_name", fmt.Sprintf("%s canonical name %q contains a control character", kind, name), nil)
		}
	}
	if isReservedReference(kind, name) {
		return conflict("reserved_name", fmt.Sprintf("%s name %q is reserved", kind, name), nil)
	}
	return nil
}

// NormalizeAliases trims and validates one complete alias replacement.
func NormalizeAliases(kind ResourceKind, aliases []string) ([]string, error) {
	if aliases == nil {
		return []string{}, nil
	}
	normalized := make([]string, 0, len(aliases))
	seen := map[string]struct{}{}
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			return nil, invalid("invalid_alias", fmt.Sprintf("%s aliases cannot contain an empty value", kind), nil)
		}
		if err := ValidateCanonicalName(kind, alias); err != nil {
			var mutationErr *Error
			if errors.As(err, &mutationErr) && mutationErr.Code == "reserved_name" {
				return nil, err
			}
			return nil, invalid("invalid_alias", fmt.Sprintf("invalid %s alias %q", kind, alias), err)
		}
		if _, exists := seen[alias]; exists {
			return nil, invalid("duplicate_alias", fmt.Sprintf("%s alias %q is duplicated", kind, alias), nil)
		}
		seen[alias] = struct{}{}
		normalized = append(normalized, alias)
	}
	return normalized, nil
}

// ValidateIdentityScope proves a canonical name and aliases are unique in one resolution scope.
func ValidateIdentityScope(kind ResourceKind, candidate Identity, existing []Identity, replacingCanonical string) error {
	if err := ValidateCanonicalName(kind, candidate.CanonicalName); err != nil {
		return err
	}
	aliases, err := NormalizeAliases(kind, candidate.Aliases)
	if err != nil {
		return err
	}
	candidateReferences := append([]string{candidate.CanonicalName}, aliases...)
	seen := map[string]struct{}{}
	for _, reference := range candidateReferences {
		if _, exists := seen[reference]; exists {
			return invalid("duplicate_reference", fmt.Sprintf("%s reference %q is duplicated", kind, reference), nil)
		}
		seen[reference] = struct{}{}
	}
	for _, identity := range existing {
		if identity.CanonicalName == replacingCanonical {
			continue
		}
		for _, occupied := range append([]string{identity.CanonicalName}, identity.Aliases...) {
			for _, reference := range candidateReferences {
				if reference == occupied {
					return conflict("reference_conflict", fmt.Sprintf("%s reference %q conflicts with %q", kind, reference, identity.CanonicalName), nil)
				}
			}
		}
	}
	return nil
}

// CreateProject creates one complete immediately usable Project transactionally.
func CreateProject(repo repository.Repository, canonicalName string, metadata Metadata) error {
	loaded, err := loadWarehouse(repo.RootPath)
	if err != nil {
		return err
	}
	if err := ValidateIdentityScope(ProjectKind, Identity{CanonicalName: canonicalName, Aliases: metadata.Aliases}, projectIdentities(loaded), ""); err != nil {
		return err
	}
	projectPath := filepath.Join(repo.RootPath, canonicalName)
	paths := []string{repo.ProjectIndexPath(), projectPath}
	if err := repo.WithMutation("project-create", paths, func() error {
		if err := os.MkdirAll(filepath.Join(projectPath, "Column"), 0o755); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(projectPath, "Mode"), 0o755); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(projectPath, "Backup"), 0o755); err != nil {
			return err
		}
		projectIndex := loaded.ProjectIndex
		projectIndex.Projects[canonicalName] = projectEntry(canonicalName, metadata)
		if err := repo.SaveProjectIndex(projectIndex); err != nil {
			return err
		}
		if err := repo.SaveColumnIndex(canonicalName, emptyColumnIndex()); err != nil {
			return err
		}
		if err := repo.SaveModeIndex(canonicalName, emptyModeIndex()); err != nil {
			return err
		}
		if err := repo.SaveCurrentState(canonicalName, repository.CurrentState{Mappings: []repository.Mapping{}, Extra: map[string]json.RawMessage{}}); err != nil {
			return err
		}
		return repo.SaveHistory(canonicalName, []repository.HistoryEntry{})
	}); err != nil {
		return persistence("project_create", fmt.Sprintf("create project %q", canonicalName), err)
	}
	return nil
}

// SetProject replaces selected metadata fields while preserving the canonical key and extension fields.
func SetProject(repo repository.Repository, reference string, patch MetadataPatch) (string, error) {
	loaded, err := loadWarehouse(repo.RootPath)
	if err != nil {
		return "", err
	}
	project, err := loaded.ResolveProject(reference)
	if err != nil {
		return "", missing("project_not_found", err.Error(), err)
	}
	entry := loaded.ProjectIndex.Projects[project.Name]
	metadata, err := ApplyPatch(ProjectKind, project.Name, Metadata{DisplayName: entry.DisplayName, Description: entry.Description, Aliases: entry.Aliases}, patch)
	if err != nil {
		return "", err
	}
	entry = projectEntryWithExtra(project.Name, metadata, entry.Extra)
	if err := ValidateIdentityScope(ProjectKind, Identity{CanonicalName: project.Name, Aliases: entry.Aliases}, projectIdentities(loaded), project.Name); err != nil {
		return "", err
	}
	loaded.ProjectIndex.Projects[project.Name] = entry
	if err := repo.WithMutation("project-set", []string{repo.ProjectIndexPath()}, func() error {
		return repo.SaveProjectIndex(loaded.ProjectIndex)
	}); err != nil {
		return "", persistence("project_set", fmt.Sprintf("set project %q", project.Name), err)
	}
	return project.Name, nil
}

// CreateColumn creates one zero-target Column and its empty Setting index transactionally.
func CreateColumn(repo repository.Repository, projectReference string, canonicalName string, metadata Metadata) error {
	loaded, project, err := loadProject(repo.RootPath, projectReference)
	if err != nil {
		return err
	}
	if project.Missing {
		return missing("project_missing", fmt.Sprintf("project %q is missing", project.Name), nil)
	}
	if err := ValidateIdentityScope(ColumnKind, Identity{CanonicalName: canonicalName, Aliases: metadata.Aliases}, columnIdentities(project), ""); err != nil {
		return err
	}
	columnPath := filepath.Join(project.ColumnDirPath, canonicalName)
	paths := []string{repo.ColumnIndexPath(project.Name), columnPath}
	if err := repo.WithMutation("column-create", paths, func() error {
		if err := os.MkdirAll(columnPath, 0o755); err != nil {
			return err
		}
		columnIndex := project.ColumnIndex
		columnIndex.Columns[canonicalName] = columnEntry(canonicalName, metadata)
		if err := repo.SaveColumnIndex(project.Name, columnIndex); err != nil {
			return err
		}
		return repo.SaveSettingIndex(project.Name, canonicalName, emptySettingIndex())
	}); err != nil {
		return persistence("column_create", fmt.Sprintf("create column %q", canonicalName), err)
	}
	_ = loaded
	return nil
}

// SetColumn replaces selected metadata while preserving target data in its Setting index.
func SetColumn(repo repository.Repository, projectReference string, reference string, patch MetadataPatch) (string, error) {
	_, project, err := loadProject(repo.RootPath, projectReference)
	if err != nil {
		return "", err
	}
	if project.Missing {
		return "", missing("project_missing", fmt.Sprintf("project %q is missing", project.Name), nil)
	}
	column, err := project.ResolveColumn(reference)
	if err != nil {
		return "", missing("column_not_found", err.Error(), err)
	}
	entry := project.ColumnIndex.Columns[column.Name]
	metadata, err := ApplyPatch(ColumnKind, column.Name, Metadata{DisplayName: entry.DisplayName, Description: entry.Description, Aliases: entry.Aliases}, patch)
	if err != nil {
		return "", err
	}
	entry = columnEntryWithExtra(column.Name, metadata, entry.Extra)
	if err := ValidateIdentityScope(ColumnKind, Identity{CanonicalName: column.Name, Aliases: entry.Aliases}, columnIdentities(project), column.Name); err != nil {
		return "", err
	}
	project.ColumnIndex.Columns[column.Name] = entry
	if err := repo.WithMutation("column-set", []string{repo.ColumnIndexPath(project.Name)}, func() error {
		return repo.SaveColumnIndex(project.Name, project.ColumnIndex)
	}); err != nil {
		return "", persistence("column_set", fmt.Sprintf("set column %q", column.Name), err)
	}
	return column.Name, nil
}

// CreateSetting stages source content, then commits it with inherited Setting metadata.
func CreateSetting(repo repository.Repository, projectReference string, columnReference string, canonicalName string, kind string, metadata Metadata, sources ...content.Source) error {
	_, project, err := loadProject(repo.RootPath, projectReference)
	if err != nil {
		return err
	}
	if project.Missing {
		return missing("project_missing", fmt.Sprintf("project %q is missing", project.Name), nil)
	}
	column, err := project.ResolveColumn(columnReference)
	if err != nil {
		return missing("column_not_found", err.Error(), err)
	}
	if column.Missing {
		return missing("column_missing", fmt.Sprintf("column %q is missing", column.Name), nil)
	}
	if kind != "file" && kind != "directory" {
		return invalid("invalid_setting_kind", "setting kind must be file or directory", nil)
	}
	if err := ValidateIdentityScope(SettingKind, Identity{CanonicalName: canonicalName, Aliases: metadata.Aliases}, settingIdentities(column), ""); err != nil {
		return err
	}
	source := content.Source{Mode: content.SourceEmpty}
	if len(sources) > 1 {
		return invalid("invalid_content_source", "Setting creation accepts at most one content source", nil)
	}
	if len(sources) == 1 {
		source = sources[0]
	}
	staged, cleanup, err := content.StageCreation(column.Path, content.Kind(kind), source)
	if err != nil {
		return contentMutationError(err)
	}
	defer cleanup()
	settingPath := filepath.Join(column.Path, canonicalName)
	paths := []string{repo.SettingIndexPath(project.Name, column.Name), settingPath}
	if err := repo.WithMutation("setting-create", paths, func() error {
		if err := os.Rename(staged, settingPath); err != nil {
			return err
		}
		settingIndex := column.SettingIndex
		settingIndex.Settings[canonicalName] = settingEntry(canonicalName, metadata, settingIndex.TargetNumber)
		return repo.SaveSettingIndex(project.Name, column.Name, settingIndex)
	}); err != nil {
		return persistence("setting_create", fmt.Sprintf("create setting %q", canonicalName), err)
	}
	return nil
}

// SetSetting replaces selected metadata while preserving source content, targets, and extension fields.
func SetSetting(repo repository.Repository, projectReference string, columnReference string, reference string, patch MetadataPatch) (string, error) {
	_, project, err := loadProject(repo.RootPath, projectReference)
	if err != nil {
		return "", err
	}
	if project.Missing {
		return "", missing("project_missing", fmt.Sprintf("project %q is missing", project.Name), nil)
	}
	column, err := project.ResolveColumn(columnReference)
	if err != nil {
		return "", missing("column_not_found", err.Error(), err)
	}
	if column.Missing {
		return "", missing("column_missing", fmt.Sprintf("column %q is missing", column.Name), nil)
	}
	setting, err := column.ResolveSetting(reference)
	if err != nil {
		return "", missing("setting_not_found", err.Error(), err)
	}
	entry := column.SettingIndex.Settings[setting.Name]
	metadata, err := ApplyPatch(SettingKind, setting.Name, Metadata{DisplayName: entry.DisplayName, Description: entry.Description, Aliases: entry.Aliases}, patch)
	if err != nil {
		return "", err
	}
	entry = settingEntryWithExisting(setting.Name, metadata, entry)
	if err := ValidateIdentityScope(SettingKind, Identity{CanonicalName: setting.Name, Aliases: entry.Aliases}, settingIdentities(column), setting.Name); err != nil {
		return "", err
	}
	column.SettingIndex.Settings[setting.Name] = entry
	if err := repo.WithMutation("setting-set", []string{repo.SettingIndexPath(project.Name, column.Name)}, func() error {
		return repo.SaveSettingIndex(project.Name, column.Name, column.SettingIndex)
	}); err != nil {
		return "", persistence("setting_set", fmt.Sprintf("set setting %q", setting.Name), err)
	}
	return setting.Name, nil
}

// CreateMode creates one Mode with no placeholder Column selections.
func CreateMode(repo repository.Repository, projectReference string, canonicalName string, metadata Metadata) error {
	_, project, err := loadProject(repo.RootPath, projectReference)
	if err != nil {
		return err
	}
	if project.Missing {
		return missing("project_missing", fmt.Sprintf("project %q is missing", project.Name), nil)
	}
	if err := ValidateIdentityScope(ModeKind, Identity{CanonicalName: canonicalName, Aliases: metadata.Aliases}, modeIdentities(project), ""); err != nil {
		return err
	}
	modeIndex := project.ModeIndex
	modeIndex.Modes[canonicalName] = modeEntry(canonicalName, metadata)
	if err := repo.WithMutation("mode-create", []string{repo.ModeIndexPath(project.Name)}, func() error {
		return repo.SaveModeIndex(project.Name, modeIndex)
	}); err != nil {
		return persistence("mode_create", fmt.Sprintf("create mode %q", canonicalName), err)
	}
	return nil
}

// SetMode replaces selected metadata while preserving selections and extension fields.
func SetMode(repo repository.Repository, projectReference string, reference string, patch MetadataPatch) (string, error) {
	_, project, err := loadProject(repo.RootPath, projectReference)
	if err != nil {
		return "", err
	}
	if project.Missing {
		return "", missing("project_missing", fmt.Sprintf("project %q is missing", project.Name), nil)
	}
	mode, err := project.ResolveMode(reference)
	if err != nil {
		return "", missing("mode_not_found", err.Error(), err)
	}
	entry := project.ModeIndex.Modes[mode.Name]
	metadata, err := ApplyPatch(ModeKind, mode.Name, Metadata{DisplayName: entry.DisplayName, Description: entry.Description, Aliases: entry.Aliases}, patch)
	if err != nil {
		return "", err
	}
	entry = modeEntryWithExisting(mode.Name, metadata, entry)
	if err := ValidateIdentityScope(ModeKind, Identity{CanonicalName: mode.Name, Aliases: entry.Aliases}, modeIdentities(project), mode.Name); err != nil {
		return "", err
	}
	project.ModeIndex.Modes[mode.Name] = entry
	if err := repo.WithMutation("mode-set", []string{repo.ModeIndexPath(project.Name)}, func() error {
		return repo.SaveModeIndex(project.Name, project.ModeIndex)
	}); err != nil {
		return "", persistence("mode_set", fmt.Sprintf("set mode %q", mode.Name), err)
	}
	return mode.Name, nil
}

// ApplyPatch validates and applies common metadata replacements.
func ApplyPatch(kind ResourceKind, canonicalName string, current Metadata, patch MetadataPatch) (Metadata, error) {
	if patch.DisplayName != nil {
		current.DisplayName = *patch.DisplayName
		if current.DisplayName == "" {
			current.DisplayName = canonicalName
		}
	}
	if patch.Description != nil {
		current.Description = *patch.Description
	}
	if patch.Aliases != nil {
		aliases, err := NormalizeAliases(kind, *patch.Aliases)
		if err != nil {
			return Metadata{}, err
		}
		current.Aliases = aliases
	}
	return current, nil
}

// isReservedReference reports names occupied by command context or repository implementation artifacts.
func isReservedReference(kind ResourceKind, name string) bool {
	if kind == ProjectKind && name == "global" {
		return true
	}
	if strings.HasPrefix(name, ".cfgfc-") {
		return true
	}
	switch kind {
	case ProjectKind:
		return name == "ProjectIndex.jsonc"
	case ColumnKind:
		return name == "ColumnIndex.jsonc"
	case SettingKind:
		return name == "SettingIndex.jsonc"
	default:
		return false
	}
}

// loadWarehouse maps invalid durable data into a typed mutation error.
func loadWarehouse(rootPath string) (warehouse.Warehouse, error) {
	loaded, err := warehouse.LoadWarehouse(rootPath)
	if err != nil {
		return warehouse.Warehouse{}, invalid("warehouse_data", err.Error(), err)
	}
	return loaded, nil
}

// loadProject resolves one canonical or aliased Project from the current warehouse.
func loadProject(rootPath string, reference string) (warehouse.Warehouse, warehouse.Project, error) {
	loaded, err := loadWarehouse(rootPath)
	if err != nil {
		return warehouse.Warehouse{}, warehouse.Project{}, err
	}
	project, err := loaded.ResolveProject(reference)
	if err != nil {
		return warehouse.Warehouse{}, warehouse.Project{}, missing("project_not_found", err.Error(), err)
	}
	return loaded, project, nil
}

// projectIdentities returns deterministic identities in the warehouse Project scope.
func projectIdentities(loaded warehouse.Warehouse) []Identity {
	identities := make([]Identity, 0, len(loaded.Projects))
	for _, project := range loaded.Projects {
		identities = append(identities, Identity{CanonicalName: project.Name, Aliases: project.Metadata.Aliases})
	}
	return sortedIdentities(identities)
}

// columnIdentities returns deterministic identities in one Project's Column scope.
func columnIdentities(project warehouse.Project) []Identity {
	identities := make([]Identity, 0, len(project.Columns))
	for _, column := range project.Columns {
		identities = append(identities, Identity{CanonicalName: column.Name, Aliases: column.Metadata.Aliases})
	}
	return sortedIdentities(identities)
}

// settingIdentities returns deterministic identities in one Column's Setting scope.
func settingIdentities(column warehouse.Column) []Identity {
	identities := make([]Identity, 0, len(column.Settings))
	for _, setting := range column.Settings {
		identities = append(identities, Identity{CanonicalName: setting.Name, Aliases: setting.Metadata.Aliases})
	}
	return sortedIdentities(identities)
}

// modeIdentities returns deterministic identities in one Project's Mode scope.
func modeIdentities(project warehouse.Project) []Identity {
	identities := make([]Identity, 0, len(project.Modes))
	for _, mode := range project.Modes {
		identities = append(identities, Identity{CanonicalName: mode.Name, Aliases: mode.Metadata.Aliases})
	}
	return sortedIdentities(identities)
}

// sortedIdentities stabilizes validation and conflict diagnostics.
func sortedIdentities(identities []Identity) []Identity {
	sort.Slice(identities, func(left int, right int) bool {
		return identities[left].CanonicalName < identities[right].CanonicalName
	})
	return identities
}

// emptyColumnIndex returns a complete zero-entry Column index.
func emptyColumnIndex() index.ColumnIndex {
	return index.ColumnIndex{Columns: map[string]index.ColumnEntry{}, Extra: map[string]json.RawMessage{}}
}

// emptySettingIndex returns a complete zero-target Setting index.
func emptySettingIndex() index.SettingIndex {
	return index.SettingIndex{TargetNumber: 0, DefaultTargetDir: []string{}, DefaultTargetName: []string{}, Settings: map[string]index.SettingEntry{}, Extra: map[string]json.RawMessage{}}
}

// emptyModeIndex returns a complete zero-entry Mode index.
func emptyModeIndex() index.ModeIndex {
	return index.ModeIndex{Modes: map[string]index.ModeEntry{}, Extra: map[string]json.RawMessage{}}
}

// projectEntry converts common metadata to one Project index entry.
func projectEntry(canonicalName string, metadata Metadata) index.ProjectEntry {
	return index.ProjectEntry{WarehouseName: canonicalName, DisplayName: metadata.DisplayName, Aliases: append([]string{}, metadata.Aliases...), Description: metadata.Description, Extra: map[string]json.RawMessage{}}
}

// columnEntry converts common metadata to one Column index entry.
func columnEntry(canonicalName string, metadata Metadata) index.ColumnEntry {
	return index.ColumnEntry{WarehouseName: canonicalName, DisplayName: metadata.DisplayName, Aliases: append([]string{}, metadata.Aliases...), Description: metadata.Description, Extra: map[string]json.RawMessage{}}
}

// settingEntry converts common metadata to one Setting index entry with inherited target components.
func settingEntry(canonicalName string, metadata Metadata, targetNumber int) index.SettingEntry {
	return index.SettingEntry{WarehouseName: canonicalName, DisplayName: metadata.DisplayName, Aliases: append([]string{}, metadata.Aliases...), Description: metadata.Description, TargetDir: make([]string, targetNumber), TargetName: make([]string, targetNumber), Extra: map[string]json.RawMessage{}}
}

// modeEntry converts common metadata to one selection-free Mode index entry.
func modeEntry(canonicalName string, metadata Metadata) index.ModeEntry {
	return index.ModeEntry{WarehouseName: canonicalName, DisplayName: metadata.DisplayName, Aliases: append([]string{}, metadata.Aliases...), Description: metadata.Description, Columns: map[string]index.ModeColumnSelection{}, Extra: map[string]json.RawMessage{}}
}

// projectEntryWithExtra converts common metadata while preserving Project extension fields.
func projectEntryWithExtra(canonicalName string, metadata Metadata, extra map[string]json.RawMessage) index.ProjectEntry {
	entry := projectEntry(canonicalName, metadata)
	entry.Extra = extra
	return entry
}

// columnEntryWithExtra converts common metadata while preserving Column extension fields.
func columnEntryWithExtra(canonicalName string, metadata Metadata, extra map[string]json.RawMessage) index.ColumnEntry {
	entry := columnEntry(canonicalName, metadata)
	entry.Extra = extra
	return entry
}

// settingEntryWithExisting converts metadata while preserving targets and Setting extension fields.
func settingEntryWithExisting(canonicalName string, metadata Metadata, existing index.SettingEntry) index.SettingEntry {
	entry := settingEntry(canonicalName, metadata, len(existing.TargetDir))
	entry.TargetDir = existing.TargetDir
	entry.TargetName = existing.TargetName
	entry.Extra = existing.Extra
	return entry
}

// modeEntryWithExisting converts metadata while preserving selections and Mode extension fields.
func modeEntryWithExisting(canonicalName string, metadata Metadata, existing index.ModeEntry) index.ModeEntry {
	entry := modeEntry(canonicalName, metadata)
	entry.Columns = existing.Columns
	entry.Extra = existing.Extra
	return entry
}

// invalid constructs one invalid-resource-data mutation error.
func invalid(code string, message string, cause error) *Error {
	return &Error{Kind: InvalidError, Code: code, Message: message, Cause: cause}
}

// conflict constructs one identity collision or reserved-name mutation error.
func conflict(code string, message string, cause error) *Error {
	return &Error{Kind: ConflictError, Code: code, Message: message, Cause: cause}
}

// missing constructs one absent-resource mutation error.
func missing(code string, message string, cause error) *Error {
	return &Error{Kind: MissingError, Code: code, Message: message, Cause: cause}
}

// persistence constructs one filesystem or transaction mutation error.
func persistence(code string, message string, cause error) *Error {
	return &Error{Kind: PersistenceError, Code: code, Message: message + ": " + cause.Error(), Cause: cause}
}

// contentMutationError maps bounded-content failures into the mutation error contract.
func contentMutationError(err error) error {
	var contentErr *content.Error
	if !errors.As(err, &contentErr) {
		return persistence("content_stage", "stage Setting content", err)
	}
	switch contentErr.Kind {
	case content.InvalidError:
		return invalid(contentErr.Code, contentErr.Message, contentErr)
	case content.ConflictError:
		return conflict(contentErr.Code, contentErr.Message, contentErr)
	case content.MissingError:
		return missing(contentErr.Code, contentErr.Message, contentErr)
	default:
		return persistence(contentErr.Code, contentErr.Message, contentErr)
	}

}
