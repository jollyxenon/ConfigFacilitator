package syncer

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// SyncAll rewrites every project's indexes from the reconciled warehouse model.
func SyncAll(rootPath string) error {
	loaded, err := warehouse.LoadWarehouse(rootPath)
	if err != nil {
		return err
	}
	if err := rewriteProjectIndex(loaded); err != nil {
		return err
	}
	for _, project := range loaded.Projects {
		if err := rewriteProject(project); err != nil {
			return err
		}
	}
	return nil
}

// SyncProject rewrites one project's indexes from the reconciled warehouse model.
func SyncProject(rootPath string, projectName string) error {
	loaded, err := warehouse.LoadWarehouse(rootPath)
	if err != nil {
		return err
	}
	if err := rewriteProjectIndex(loaded); err != nil {
		return err
	}
	project, ok := loaded.Projects[projectName]
	if !ok {
		return os.ErrNotExist
	}
	return rewriteProject(project)
}

func rewriteProjectIndex(loaded warehouse.Warehouse) error {
	projectIndex := loaded.ProjectIndex
	if projectIndex.Projects == nil {
		projectIndex.Projects = map[string]index.ProjectEntry{}
	}
	for name, project := range loaded.Projects {
		entry := project.Metadata
		if entry.FolderName == "" {
			entry.FolderName = name
		}
		if entry.DisplayName == "" {
			entry.DisplayName = name
		}
		projectIndex.Projects[name] = entry
	}
	return writeJSON(projectIndex, loaded.ProjectIndexPath)
}

func rewriteProject(project warehouse.Project) error {
	columnIndex := project.ColumnIndex
	if columnIndex.Columns == nil {
		columnIndex.Columns = map[string]index.ColumnEntry{}
	}
	for name, column := range project.Columns {
		entry := column.Metadata
		if entry.FolderName == "" {
			entry.FolderName = name
		}
		if entry.DisplayName == "" {
			entry.DisplayName = name
		}
		columnIndex.Columns[name] = entry
		if err := rewriteSettingIndex(column); err != nil {
			return err
		}
	}
	if err := writeJSON(columnIndex, project.ColumnIndexPath); err != nil {
		return err
	}
	modeIndex := project.ModeIndex
	if modeIndex.Modes == nil {
		modeIndex.Modes = map[string]index.ModeEntry{}
	}
	for name, mode := range project.Modes {
		entry := mode.Metadata
		if mode.Missing {
			entry.Extra = withMissingMarker(entry.Extra)
		}
		modeIndex.Modes[name] = entry
	}
	return writeJSON(modeIndex, project.ModeIndexPath)
}

func rewriteSettingIndex(column warehouse.Column) error {
	settingIndex := column.SettingIndex
	if settingIndex.Settings == nil {
		settingIndex.Settings = map[string]index.SettingEntry{}
	}
	for name, setting := range column.Settings {
		entry := setting.Metadata
		if entry.DisplayName == "" {
			entry.DisplayName = name
		}
		if setting.Missing {
			entry.Extra = withMissingMarker(entry.Extra)
		}
		settingIndex.Settings[name] = entry
	}
	return writeJSON(settingIndex, column.SettingIndexPath)
}

func writeJSON(value any, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func withMissingMarker(extra map[string]json.RawMessage) map[string]json.RawMessage {
	if extra == nil {
		extra = map[string]json.RawMessage{}
	}
	extra["missing"] = json.RawMessage("true")
	return extra
}
