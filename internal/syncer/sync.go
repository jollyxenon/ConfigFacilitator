// Package syncer reconciles the warehouse with external filesystem and index
// changes. It immediately removes Index metadata for disappeared Project,
// Column, and Setting sources, rebuilds missing Index entries from the
// filesystem, and syncs each Project's Current state atomically per Project.
package syncer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/linker"
	"github.com/xenon/ConfigFacilitator/internal/planner"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// SyncAll reconciles every discovered or indexed Project. The ProjectIndex is
// committed first, then each Project commits independently; a failing Project
// rolls back only its own index, Current, and link changes.
func SyncAll(rootPath string, options planner.PlanOptions) error {
	loaded, err := warehouse.LoadWarehouse(rootPath)
	if err != nil {
		return err
	}
	if err := syncProjectIndex(rootPath, loaded); err != nil {
		return err
	}
	for _, projectName := range sortedKeys(loaded.Projects) {
		if loaded.Projects[projectName].Missing {
			continue
		}
		if err := SyncProject(rootPath, projectName, options); err != nil {
			return err
		}
	}
	return nil
}

// SyncProjectIndexOnly commits Project discovery and removal without touching
// per-Project indexes or Current states.
func SyncProjectIndexOnly(rootPath string) error {
	loaded, err := warehouse.LoadWarehouse(rootPath)
	if err != nil {
		return err
	}
	rebuilt, err := warehouse.RebuildProjectIndex(rootPath, loaded.ProjectIndex)
	if err != nil {
		return err
	}
	return repository.New(rootPath).WithMutation("sync-project-index", []string{loaded.ProjectIndexPath}, func() error {
		return rewriteProjectIndex(loaded, rebuilt, "")
	})
}

// SyncProject atomically reconciles one canonical or aliased Project scope,
// removing disappeared resources and rebuilding missing indexes in one
// transaction, then syncing the Current state through the link engine's own
// transaction.
func SyncProject(rootPath string, projectName string, options planner.PlanOptions) error {
	loaded, err := warehouse.LoadWarehouse(rootPath)
	if err != nil {
		return err
	}
	project, err := loaded.ResolveProject(projectName)
	if err != nil {
		return err
	}
	if project.Missing {
		return repository.New(rootPath).WithMutation("sync-project", []string{loaded.ProjectIndexPath}, func() error {
			return rewriteProjectIndex(loaded, loaded.ProjectIndex, project.Name)
		})
	}
	// Rebuild missing Column and Setting Index entries from the filesystem.
	if err := rebuildProjectIndexes(rootPath, project); err != nil {
		return err
	}
	reloaded, err := warehouse.LoadWarehouse(rootPath)
	if err != nil {
		return err
	}
	project, err = reloaded.ResolveProject(project.Name)
	if err != nil {
		return err
	}
	if project.Missing {
		return nil
	}
	if err := repository.New(rootPath).WithMutation("sync-project", projectIndexPaths(project), func() error {
		return rewriteProject(project)
	}); err != nil {
		return err
	}
	return syncCurrent(project, options)
}

// syncProjectIndex commits discovery and removal of Projects only.
func syncProjectIndex(rootPath string, loaded warehouse.Warehouse) error {
	rebuilt, err := warehouse.RebuildProjectIndex(rootPath, loaded.ProjectIndex)
	if err != nil {
		return err
	}
	reloaded, err := warehouse.LoadWarehouse(rootPath)
	if err != nil {
		return err
	}
	return repository.New(rootPath).WithMutation("sync-project-index", []string{loaded.ProjectIndexPath}, func() error {
		return rewriteProjectIndex(reloaded, rebuilt, "")
	})
}

// rebuildProjectIndexes writes missing Column and Setting Index entries for one
// present Project before Current reconciliation.
func rebuildProjectIndexes(rootPath string, project warehouse.Project) error {
	columnIndex, err := warehouse.RebuildColumnIndex(project.ColumnDirPath, project.ColumnIndex)
	if err != nil {
		return err
	}
	if err := repository.SaveColumnIndex(project.ColumnIndexPath, columnIndex); err != nil {
		return err
	}
	for _, columnName := range sortedKeys(columnIndex.Columns) {
		columnPath := filepath.Join(project.ColumnDirPath, columnName)
		if _, err := os.Lstat(columnPath); err != nil {
			continue
		}
		settingIndex, err := warehouse.RebuildSettingIndex(columnPath, project.Columns[columnName].SettingIndex)
		if err != nil {
			return err
		}
		if err := repository.SaveSettingIndex(filepath.Join(project.ColumnDirPath, columnName, "SettingIndex.jsonc"), settingIndex); err != nil {
			return err
		}
	}
	return nil
}

// syncCurrent replans and persists one Project's Current state after index
// reconciliation. A missing Current file is recreated empty and any stale
// history is removed; a legacy or corrupt Current keeps the Project
// unavailable and is left untouched.
func syncCurrent(project warehouse.Project, options planner.PlanOptions) error {
	repo := repository.New(filepath.Dir(project.Path))
	if _, err := os.Lstat(project.CurrentStatePath); os.IsNotExist(err) {
		_ = os.MkdirAll(project.BackupDirPath, 0o755)
		_ = os.Remove(project.HistoryLogPath)
		return repo.SaveCurrentState(project.Name, repository.CurrentState{
			Columns:  map[string]repository.ColumnSelection{},
			Mappings: []repository.Mapping{},
		})
	}
	state, err := repo.LoadCurrentState(project.Name)
	if err != nil {
		return nil
	}
	if err := validateRelation(project, state.Relation); err != nil {
		return nil
	}
	columns, err := currentColumns(project, state)
	if err != nil {
		return nil
	}
	mappings, err := planner.PlanColumns(project, columns, state.Mappings, options)
	if err != nil {
		return err
	}
	next := repository.CurrentState{Columns: state.Columns, Relation: state.Relation, Mappings: mappings}
	if state.Relation != nil && state.Relation.Kind == "following" {
		next.Columns = repositoryColumns(columns)
	}
	return linker.New().ReplaceState(project, next)
}

// validateRelation accepts only following/detached relations with a resolvable origin.
func validateRelation(project warehouse.Project, relation *repository.CurrentRelation) error {
	if relation == nil {
		return nil
	}
	if relation.Kind != "following" && relation.Kind != "detached" {
		return fmt.Errorf("unsupported relation kind %q", relation.Kind)
	}
	if relation.OriginMode == "" {
		return fmt.Errorf("relation originMode cannot be empty")
	}
	_, err := project.ResolveMode(relation.OriginMode)
	return err
}

// currentColumns returns the columns that drive Current planning for sync.
func currentColumns(project warehouse.Project, state repository.CurrentState) (map[string]index.ModeColumnSelection, error) {
	if state.Relation != nil && state.Relation.Kind == "following" {
		mode, err := project.ResolveMode(state.Relation.OriginMode)
		if err != nil {
			return nil, err
		}
		return mode.Metadata.Columns, nil
	}
	return repositoryColumnsToIndex(state.Columns), nil
}

// repositoryColumns converts persisted Current columns to planner selections.
func repositoryColumnsToIndex(columns map[string]repository.ColumnSelection) map[string]index.ModeColumnSelection {
	result := make(map[string]index.ModeColumnSelection, len(columns))
	for name, selection := range columns {
		result[name] = index.ModeColumnSelection{Strategy: selection.Strategy, Settings: append([]string{}, selection.Settings...)}
	}
	return result
}

// repositoryColumns converts planner selections back to persisted Current columns.
func repositoryColumns(columns map[string]index.ModeColumnSelection) map[string]repository.ColumnSelection {
	result := make(map[string]repository.ColumnSelection, len(columns))
	for name, selection := range columns {
		result[name] = repository.ColumnSelection{Strategy: selection.Strategy, Settings: append([]string{}, selection.Settings...)}
	}
	return result
}

// rewriteProjectIndex serializes discovered Projects, removing any indexed
// Project whose source directory no longer exists and merging rebuilt entries.
func rewriteProjectIndex(loaded warehouse.Warehouse, projectIndex index.ProjectIndex, scope string) error {
	projectIndex.Projects = map[string]index.ProjectEntry{}
	for _, project := range loaded.Projects {
		if project.Missing && (scope == "" || project.Name == scope) {
			continue
		}
		entry := normalizeProjectEntry(project.Metadata, project.Name)
		entry.Extra = withoutMissingMarker(entry.Extra)
		projectIndex.Projects[project.Name] = entry
	}
	return writeJSON(projectIndex, loaded.ProjectIndexPath)
}

// rewriteProject serializes one present Project without recreating missing paths.
func rewriteProject(project warehouse.Project) error {
	if project.Missing {
		return nil
	}
	columnIndex := project.ColumnIndex
	columnIndex.Columns = map[string]index.ColumnEntry{}
	for _, column := range project.Columns {
		if column.Missing {
			continue
		}
		entry := normalizeColumnEntry(column.Metadata, column.Name)
		entry.Extra = withoutMissingMarker(entry.Extra)
		columnIndex.Columns[column.Name] = entry
		if err := rewriteSettingIndex(column); err != nil {
			return err
		}
	}
	if err := writeJSON(columnIndex, project.ColumnIndexPath); err != nil {
		return err
	}

	modeIndex := project.ModeIndex
	modeIndex.Modes = map[string]index.ModeEntry{}
	for _, mode := range project.Modes {
		entry := mode.Metadata
		if entry.DisplayName == "" {
			entry.DisplayName = mode.Name
		}
		if entry.Aliases == nil {
			entry.Aliases = []string{}
		}
		entry.Extra = withoutMissingMarker(entry.Extra)
		modeIndex.Modes[mode.Name] = entry
	}
	return writeJSON(modeIndex, project.ModeIndexPath)
}

// rewriteSettingIndex serializes Settings of one present Column, removing any
// Setting whose source file or directory no longer exists.
func rewriteSettingIndex(column warehouse.Column) error {
	if column.Missing {
		return nil
	}
	settingIndex := column.SettingIndex
	settingIndex.DefaultTargetDir = normalizeTargetArray(settingIndex.DefaultTargetDir, settingIndex.TargetNumber)
	settingIndex.DefaultTargetName = normalizeTargetArray(settingIndex.DefaultTargetName, settingIndex.TargetNumber)
	settingIndex.Settings = map[string]index.SettingEntry{}
	for _, setting := range column.Settings {
		if setting.Missing {
			continue
		}
		entry := normalizeSettingEntry(setting.Metadata, setting.Name, settingIndex.TargetNumber)
		entry.Extra = withoutMissingMarker(entry.Extra)
		settingIndex.Settings[setting.Name] = entry
	}
	return writeJSON(settingIndex, column.SettingIndexPath)
}

// projectIndexPaths returns only index paths below a present Project or Column.
func projectIndexPaths(project warehouse.Project) []string {
	if project.Missing {
		return nil
	}
	paths := []string{project.ColumnIndexPath, project.ModeIndexPath}
	for _, column := range project.Columns {
		if !column.Missing {
			paths = append(paths, column.SettingIndexPath)
		}
	}
	return paths
}

// normalizeProjectEntry supplies stable display and alias defaults without losing metadata.
func normalizeProjectEntry(entry index.ProjectEntry, name string) index.ProjectEntry {
	if entry.DisplayName == "" {
		entry.DisplayName = name
	}
	if entry.Aliases == nil {
		entry.Aliases = []string{}
	}
	return entry
}

// normalizeColumnEntry supplies stable display and alias defaults without losing metadata.
func normalizeColumnEntry(entry index.ColumnEntry, name string) index.ColumnEntry {
	if entry.DisplayName == "" {
		entry.DisplayName = name
	}
	if entry.Aliases == nil {
		entry.Aliases = []string{}
	}
	return entry
}

// normalizeSettingEntry supplies stable metadata and lockstep target arrays.
func normalizeSettingEntry(entry index.SettingEntry, name string, targetNumber int) index.SettingEntry {
	if entry.DisplayName == "" {
		entry.DisplayName = name
	}
	if entry.Aliases == nil {
		entry.Aliases = []string{}
	}
	entry.TargetDir = normalizeTargetArray(entry.TargetDir, targetNumber)
	entry.TargetName = normalizeTargetArray(entry.TargetName, targetNumber)
	return entry
}

// normalizeTargetArray resizes one persisted target array to its declared count.
func normalizeTargetArray(values []string, targetNumber int) []string {
	if targetNumber == 0 {
		return []string{}
	}
	if len(values) >= targetNumber {
		return append([]string{}, values[:targetNumber]...)
	}

	normalized := append([]string{}, values...)
	fillValue := ""
	if len(values) > 0 && allValuesEqual(values) {
		fillValue = values[0]
	}
	for len(normalized) < targetNumber {
		normalized = append(normalized, fillValue)
	}
	return normalized
}

// allValuesEqual reports whether every target-array value is identical.
func allValuesEqual(values []string) bool {
	for _, value := range values[1:] {
		if value != values[0] {
			return false
		}
	}
	return true
}

// writeJSON routes each index type through its schema-aware repository serializer.
func writeJSON(value any, path string) error {
	switch typed := value.(type) {
	case index.ProjectIndex:
		return repository.SaveProjectIndex(path, typed)
	case index.ColumnIndex:
		return repository.SaveColumnIndex(path, typed)
	case index.SettingIndex:
		return repository.SaveSettingIndex(path, typed)
	case index.ModeIndex:
		return repository.SaveModeIndex(path, typed)
	default:
		return fmt.Errorf("unsupported sync index type %T", value)
	}
}

// withoutMissingMarker clones unknown fields while dropping any stale durable
// missing marker, which synchronization no longer writes.
func withoutMissingMarker(extra map[string]json.RawMessage) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage, len(extra))
	for key, value := range extra {
		if key != "missing" {
			result[key] = append(json.RawMessage(nil), value...)
		}
	}
	return result
}

// sortedKeys returns deterministic names for sync iteration.
func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
