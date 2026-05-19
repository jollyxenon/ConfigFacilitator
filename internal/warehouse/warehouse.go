package warehouse

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"

	"github.com/xenon/ConfigFacilitator/internal/index"
)

const warehouseDirName = "SettingWarehouse"

// Warehouse stores the parsed warehouse root and all discovered projects.
type Warehouse struct {
	RootPath         string
	ProjectIndexPath string
	ProjectIndex     index.ProjectIndex
	Projects         map[string]Project
}

// Project stores one project and its filesystem/index relationships.
type Project struct {
	Name             string
	Path             string
	Missing          bool
	Metadata         index.ProjectEntry
	ColumnDirPath    string
	ModeDirPath      string
	BackupDirPath    string
	ColumnIndexPath  string
	ModeIndexPath    string
	CurrentStatePath string
	HistoryLogPath   string
	ColumnIndex      index.ColumnIndex
	ModeIndex        index.ModeIndex
	Columns          map[string]Column
	Modes            map[string]Mode
}

// Column stores one column and its declared settings.
type Column struct {
	Name             string
	Path             string
	Missing          bool
	Metadata         index.ColumnEntry
	SettingIndexPath string
	SettingIndex     index.SettingIndex
	Settings         map[string]Setting
}

// Setting stores one setting entry and its discovered filesystem state.
type Setting struct {
	Name     string
	Path     string
	Exists   bool
	Missing  bool
	IsDir    bool
	Metadata index.SettingEntry
}

// Mode stores one mode entry and its preserved state markers.
type Mode struct {
	Name     string
	Missing  bool
	Metadata index.ModeEntry
}

// DefaultWarehouseRoot returns the warehouse root in the user's config directory.
func DefaultWarehouseRoot() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".configfacilitator", warehouseDirName), nil
}

// LoadWarehouse loads the warehouse root, project index, and project models.
func LoadWarehouse(rootPath string) (Warehouse, error) {
	warehouse := Warehouse{
		RootPath:         rootPath,
		ProjectIndexPath: filepath.Join(rootPath, "ProjectIndex.jsonc"),
		Projects:         map[string]Project{},
	}

	projectIndex, err := loadProjectIndex(warehouse.ProjectIndexPath)
	if err != nil {
		return Warehouse{}, err
	}
	warehouse.ProjectIndex = projectIndex

	projectDirs, err := listSubdirectories(rootPath)
	if err != nil {
		return Warehouse{}, err
	}

	projectNames := unionStringKeys(mapKeys(projectDirs), mapKeys(projectIndex.Projects))
	for _, projectName := range projectNames {
		project, err := loadProject(rootPath, projectName, projectIndex.Projects[projectName], projectDirs[projectName])
		if err != nil {
			return Warehouse{}, err
		}
		warehouse.Projects[projectName] = project
	}

	return warehouse, nil
}

// loadProject loads one project model from the warehouse root.
func loadProject(rootPath string, projectName string, entry index.ProjectEntry, dirPresent bool) (Project, error) {
	projectPath := filepath.Join(rootPath, projectName)
	project := Project{
		Name:             projectName,
		Path:             projectPath,
		Missing:          !dirPresent,
		Metadata:         entry,
		ColumnDirPath:    filepath.Join(projectPath, "Column"),
		ModeDirPath:      filepath.Join(projectPath, "Mode"),
		BackupDirPath:    filepath.Join(projectPath, "Backup"),
		ColumnIndexPath:  filepath.Join(projectPath, "Column", "ColumnIndex.jsonc"),
		ModeIndexPath:    filepath.Join(projectPath, "Mode", "ModeIndex.jsonc"),
		CurrentStatePath: filepath.Join(projectPath, "Backup", "current_state.json"),
		HistoryLogPath:   filepath.Join(projectPath, "Backup", "history.log"),
		Columns:          map[string]Column{},
		Modes:            map[string]Mode{},
	}

	columnIndex, err := loadColumnIndex(project.ColumnIndexPath)
	if err != nil {
		return Project{}, err
	}
	project.ColumnIndex = columnIndex

	modeIndex, err := loadModeIndex(project.ModeIndexPath)
	if err != nil {
		return Project{}, err
	}
	project.ModeIndex = modeIndex

	columnDirs, err := listSubdirectories(project.ColumnDirPath)
	if err != nil {
		return Project{}, err
	}

	columnNames := unionStringKeys(mapKeys(columnDirs), mapKeys(columnIndex.Columns))
	for _, columnName := range columnNames {
		column, err := loadColumn(project.ColumnDirPath, columnName, columnIndex.Columns[columnName], columnDirs[columnName])
		if err != nil {
			return Project{}, err
		}
		project.Columns[columnName] = column
	}

	for _, modeName := range mapKeys(modeIndex.Modes) {
		modeEntry := modeIndex.Modes[modeName]
		project.Modes[modeName] = Mode{
			Name:     modeName,
			Missing:  hasMissingMarker(modeEntry.Extra),
			Metadata: modeEntry,
		}
	}

	return project, nil
}

// loadColumn loads one column model and its setting entries.
func loadColumn(columnRoot string, columnName string, entry index.ColumnEntry, dirPresent bool) (Column, error) {
	columnPath := filepath.Join(columnRoot, columnName)
	column := Column{
		Name:             columnName,
		Path:             columnPath,
		Missing:          !dirPresent,
		Metadata:         entry,
		SettingIndexPath: filepath.Join(columnPath, "SettingIndex.jsonc"),
		Settings:         map[string]Setting{},
	}

	settingIndex, err := loadSettingIndex(column.SettingIndexPath)
	if err != nil {
		return Column{}, err
	}
	column.SettingIndex = settingIndex

	settingEntries, err := listSettingEntries(columnPath)
	if err != nil {
		return Column{}, err
	}

	settingNames := unionStringKeys(mapKeys(settingEntries), mapKeys(settingIndex.Settings))
	for _, settingName := range settingNames {
		metadata := settingIndex.Settings[settingName]
		exists := settingEntries[settingName].exists
		column.Settings[settingName] = Setting{
			Name:     settingName,
			Path:     filepath.Join(columnPath, settingName),
			Exists:   exists,
			Missing:  !exists || hasMissingMarker(metadata.Extra),
			IsDir:    settingEntries[settingName].isDir,
			Metadata: metadata,
		}
	}

	return column, nil
}

// loadProjectIndex loads the optional project index file.
func loadProjectIndex(path string) (index.ProjectIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return index.ProjectIndex{Projects: map[string]index.ProjectEntry{}, Extra: map[string]json.RawMessage{}}, nil
		}
		return index.ProjectIndex{}, err
	}
	return index.ParseProjectIndex(data)
}

// loadColumnIndex loads the optional column index file.
func loadColumnIndex(path string) (index.ColumnIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return index.ColumnIndex{Columns: map[string]index.ColumnEntry{}, Extra: map[string]json.RawMessage{}}, nil
		}
		return index.ColumnIndex{}, err
	}
	return index.ParseColumnIndex(data)
}

// loadSettingIndex loads the optional setting index file.
func loadSettingIndex(path string) (index.SettingIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return index.SettingIndex{Settings: map[string]index.SettingEntry{}, Extra: map[string]json.RawMessage{}}, nil
		}
		return index.SettingIndex{}, err
	}
	return index.ParseSettingIndex(data)
}

// loadModeIndex loads the optional mode index file.
func loadModeIndex(path string) (index.ModeIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return index.ModeIndex{Modes: map[string]index.ModeEntry{}, Extra: map[string]json.RawMessage{}}, nil
		}
		return index.ModeIndex{}, err
	}
	return index.ParseModeIndex(data)
}

type discoveredEntry struct {
	exists bool
	isDir  bool
}

// listSubdirectories lists direct child directories, returning an empty map when the path does not exist.
func listSubdirectories(path string) (map[string]bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}

	directories := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != ".cfgfc-session" {
			directories[entry.Name()] = true
		}
	}
	return directories, nil
}

// listSettingEntries lists direct setting files/directories while excluding the setting index itself.
func listSettingEntries(path string) (map[string]discoveredEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]discoveredEntry{}, nil
		}
		return nil, err
	}

	settings := map[string]discoveredEntry{}
	for _, entry := range entries {
		if entry.Name() == "SettingIndex.jsonc" {
			continue
		}
		settings[entry.Name()] = discoveredEntry{exists: true, isDir: entry.IsDir()}
	}
	return settings, nil
}

// unionStringKeys returns the sorted union of two key slices.
func unionStringKeys(left []string, right []string) []string {
	keys := map[string]struct{}{}
	for _, key := range left {
		keys[key] = struct{}{}
	}
	for _, key := range right {
		keys[key] = struct{}{}
	}
	return sortedKeys(keys)
}

// mapKeys returns the sorted keys of a map.
func mapKeys[V any](input map[string]V) []string {
	keys := map[string]struct{}{}
	for key := range input {
		keys[key] = struct{}{}
	}
	return sortedKeys(keys)
}

// sortedKeys returns lexicographically sorted keys from a set map.
func sortedKeys(input map[string]struct{}) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// hasMissingMarker reports whether the extra-field map contains a truthy `missing` marker.
func hasMissingMarker(extra map[string]json.RawMessage) bool {
	raw, ok := extra["missing"]
	if !ok {
		return false
	}
	var missing bool
	if err := json.Unmarshal(raw, &missing); err != nil {
		return false
	}
	return missing
}
