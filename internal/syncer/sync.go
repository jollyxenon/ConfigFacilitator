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
		if column.Missing {
			continue
		}
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
	settingIndex.DefaultTargetDir = normalizeTargetArray(settingIndex.DefaultTargetDir, settingIndex.TargetNumber)
	settingIndex.DefaultTargetName = normalizeTargetArray(settingIndex.DefaultTargetName, settingIndex.TargetNumber)
	settingIndex.Settings = map[string]index.SettingEntry{}
	for _, setting := range column.Settings {
		if setting.Missing {
			continue
		}
		entry := setting.Metadata
		if entry.DisplayName == "" {
			entry.DisplayName = setting.Name
		}
		if entry.Aliases == nil {
			entry.Aliases = []string{}
		}
		entry.TargetDir = normalizeTargetArray(entry.TargetDir, settingIndex.TargetNumber)
		entry.TargetName = normalizeTargetArray(entry.TargetName, settingIndex.TargetNumber)
		settingIndex.Settings[setting.Name] = entry
	}
	return writeJSON(settingIndex, column.SettingIndexPath)
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
