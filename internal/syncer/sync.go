package syncer

import (
	"encoding/json"
	"fmt"
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
	project, err := loaded.ResolveProject(projectName)
	if err != nil {
		return err
	}
	return rewriteProject(project)
}

func rewriteProjectIndex(loaded warehouse.Warehouse) error {
	projectIndex := loaded.ProjectIndex
	projectIndex.Projects = map[string]index.ProjectEntry{}
	for _, project := range loaded.Projects {
		entry := project.Metadata
		if entry.DisplayName == "" {
			entry.DisplayName = project.Name
		}
		if entry.Aliases == nil {
			entry.Aliases = []string{}
		}
		projectIndex.Projects[project.Name] = entry
	}
	return writeJSON(projectIndex, loaded.ProjectIndexPath)
}

func rewriteProject(project warehouse.Project) error {
	columnIndex := project.ColumnIndex
	columnIndex.Columns = map[string]index.ColumnEntry{}
	for _, column := range project.Columns {
		entry := column.Metadata
		if entry.DisplayName == "" {
			entry.DisplayName = column.Name
		}
		if entry.Aliases == nil {
			entry.Aliases = []string{}
		}
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
		if mode.Missing {
			entry.Extra = withMissingMarker(entry.Extra)
		}
		modeIndex.Modes[mode.Name] = entry
	}
	return writeJSON(modeIndex, project.ModeIndexPath)
}

func rewriteSettingIndex(column warehouse.Column) error {
	settingIndex := column.SettingIndex
	settingIndex.Settings = map[string]index.SettingEntry{}
	for _, setting := range column.Settings {
		entry := setting.Metadata
		if entry.DisplayName == "" {
			entry.DisplayName = setting.Name
		}
		if entry.Aliases == nil {
			entry.Aliases = []string{}
		}
		if setting.Missing {
			entry.Extra = withMissingMarker(entry.Extra)
		}
		entry, err := withGeneratedTargetFields(column, setting, entry)
		if err != nil {
			return err
		}
		settingIndex.Settings[setting.Name] = entry
	}
	return writeJSON(settingIndex, column.SettingIndexPath)
}

func withGeneratedTargetFields(column warehouse.Column, setting warehouse.Setting, entry index.SettingEntry) (index.SettingEntry, error) {
	targetCount, err := generatedTargetCount(column.SettingIndex)
	if err != nil {
		return index.SettingEntry{}, err
	}
	if len(entry.TargetDir) == 0 {
		entry.TargetDir = repeatedString("", targetCount)
	}
	if len(entry.TargetName) == 0 {
		entry.TargetName = generatedTargetNames(column.SettingIndex, setting.WarehouseName, targetCount)
	}
	return entry, nil
}

func generatedTargetCount(settingIndex index.SettingIndex) (int, error) {
	if len(settingIndex.DefaultTargetDir) != len(settingIndex.DefaultTargetName) {
		return 0, fmt.Errorf("defaultTargetDir and defaultTargetName lengths differ")
	}
	if len(settingIndex.DefaultTargetDir) > 0 {
		return len(settingIndex.DefaultTargetDir), nil
	}
	return 1, nil
}

func generatedTargetNames(settingIndex index.SettingIndex, settingName string, targetCount int) []string {
	names := make([]string, targetCount)
	for index := range names {
		if index < len(settingIndex.DefaultTargetName) && settingIndex.DefaultTargetName[index] != "" {
			names[index] = ""
		} else {
			names[index] = settingName
		}
	}
	return names
}

func repeatedString(value string, count int) []string {
	values := make([]string, count)
	for index := range values {
		values[index] = value
	}
	return values
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
