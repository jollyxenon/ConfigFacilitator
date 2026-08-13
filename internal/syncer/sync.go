package syncer

import (
	"encoding/json"
	"fmt"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// SyncAll atomically reconciles every discovered or indexed Project, removing
// index entries whose source Project, Column, or Setting path no longer exists.
func SyncAll(rootPath string) error {
	loaded, err := warehouse.LoadWarehouse(rootPath)
	if err != nil {
		return err
	}
	paths := syncAllPaths(loaded)
	return repository.New(rootPath).WithMutation("sync-all", paths, func() error {
		if err := rewriteProjectIndex(loaded, ""); err != nil {
			return err
		}
		for _, project := range loaded.Projects {
			if err := rewriteProject(project); err != nil {
				return err
			}
		}
		return nil
	})
}

// SyncProject atomically reconciles one canonical or aliased Project scope,
// removing disappeared resources only inside that Project.
func SyncProject(rootPath string, projectName string) error {
	loaded, err := warehouse.LoadWarehouse(rootPath)
	if err != nil {
		return err
	}
	project, err := loaded.ResolveProject(projectName)
	if err != nil {
		return err
	}
	paths := append([]string{loaded.ProjectIndexPath}, projectIndexPaths(project)...)
	return repository.New(rootPath).WithMutation("sync-project", paths, func() error {
		if err := rewriteProjectIndex(loaded, project.Name); err != nil {
			return err
		}
		return rewriteProject(project)
	})
}

// syncAllPaths returns only index paths that warehouse-wide reconciliation may write.
func syncAllPaths(loaded warehouse.Warehouse) []string {
	paths := []string{loaded.ProjectIndexPath}
	for _, project := range loaded.Projects {
		paths = append(paths, projectIndexPaths(project)...)
	}
	return paths
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

// rewriteProjectIndex serializes discovered Projects, removing any indexed
// Project whose source directory no longer exists. scope restricts removal to
// one named Project (empty means every Project); out-of-scope Projects are
// written back unchanged.
func rewriteProjectIndex(loaded warehouse.Warehouse, scope string) error {
	projectIndex := loaded.ProjectIndex
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
