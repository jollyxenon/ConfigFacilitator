package scaffold

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// CreateProject creates the standard project directories and baseline index/state files.
func CreateProject(rootPath string, projectName string) error {
	if projectName == "" {
		return fmt.Errorf("project name is required")
	}
	projectPath := filepath.Join(rootPath, projectName)
	for _, path := range []string{
		filepath.Join(projectPath, "Column"),
		filepath.Join(projectPath, "Mode"),
		filepath.Join(projectPath, "Backup"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}
	if err := ensureProjectIndex(rootPath, projectName); err != nil {
		return err
	}
	if err := writeIfMissing(filepath.Join(projectPath, "Column", "ColumnIndex.jsonc"), []byte(columnIndexTemplate())); err != nil {
		return err
	}
	if err := writeIfMissing(filepath.Join(projectPath, "Mode", "ModeIndex.jsonc"), []byte(modeIndexTemplate(index.ModeIndex{}))); err != nil {
		return err
	}
	if err := writeIfMissing(filepath.Join(projectPath, "Backup", "current_state.json"), []byte("{\n  \"mappings\": []\n}\n")); err != nil {
		return err
	}
	if err := writeIfMissing(filepath.Join(projectPath, "Backup", "history.log"), []byte{}); err != nil {
		return err
	}
	return nil
}

// CreateColumn creates the column directory and its template setting index.
func CreateColumn(rootPath string, projectName string, columnName string) error {
	if projectName == "" || columnName == "" {
		return fmt.Errorf("project and column names are required")
	}
	projectPath := filepath.Join(rootPath, projectName)
	if err := os.MkdirAll(filepath.Join(projectPath, "Column", columnName), 0o755); err != nil {
		return err
	}
	if err := ensureProjectIndex(rootPath, projectName); err != nil {
		return err
	}
	if err := ensureColumnIndex(projectPath, columnName); err != nil {
		return err
	}
	return writeIfMissing(filepath.Join(projectPath, "Column", columnName, "SettingIndex.jsonc"), []byte(settingIndexTemplate()))
}

// CreateMode adds or refreshes one guided mode entry inside ModeIndex.jsonc.
func CreateMode(rootPath string, projectName string, modeName string) error {
	if projectName == "" || modeName == "" {
		return fmt.Errorf("project and mode names are required")
	}
	projectPath := filepath.Join(rootPath, projectName)
	modeIndexPath := filepath.Join(projectPath, "Mode", "ModeIndex.jsonc")
	columnIndexPath := filepath.Join(projectPath, "Column", "ColumnIndex.jsonc")
	if err := os.MkdirAll(filepath.Dir(modeIndexPath), 0o755); err != nil {
		return err
	}
	if err := ensureProjectIndex(rootPath, projectName); err != nil {
		return err
	}
	columnIndex, err := readColumnIndex(columnIndexPath)
	if err != nil {
		return err
	}
	modeIndex, err := readModeIndex(modeIndexPath)
	if err != nil {
		return err
	}
	columns := map[string]index.ModeColumnSelection{}
	for _, columnName := range sortedColumnNames(columnIndex) {
		columns[columnName] = index.ModeColumnSelection{Settings: []string{}, Strategy: "full", Extra: map[string]json.RawMessage{}}
	}
	modeIndex.Modes[modeName] = index.ModeEntry{DisplayName: modeName, Description: "", Columns: columns, Extra: map[string]json.RawMessage{}}
	return os.WriteFile(modeIndexPath, []byte(modeIndexTemplate(modeIndex)), 0o644)
}

func ensureProjectIndex(rootPath string, projectName string) error {
	projectIndexPath := filepath.Join(rootPath, "ProjectIndex.jsonc")
	if err := os.MkdirAll(rootPath, 0o755); err != nil {
		return err
	}
	projectIndex, err := readProjectIndex(projectIndexPath)
	if err != nil {
		return err
	}
	entry := projectIndex.Projects[projectName]
	if entry.FolderName == "" {
		entry.FolderName = projectName
	}
	if entry.DisplayName == "" {
		entry.DisplayName = projectName
	}
	projectIndex.Projects[projectName] = entry
	data, err := json.MarshalIndent(projectIndex, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(projectIndexPath, data, 0o644)
}

func ensureColumnIndex(projectPath string, columnName string) error {
	columnIndexPath := filepath.Join(projectPath, "Column", "ColumnIndex.jsonc")
	columnIndex, err := readColumnIndex(columnIndexPath)
	if err != nil {
		return err
	}
	entry := columnIndex.Columns[columnName]
	if entry.FolderName == "" {
		entry.FolderName = columnName
	}
	if entry.DisplayName == "" {
		entry.DisplayName = columnName
	}
	columnIndex.Columns[columnName] = entry
	data, err := json.MarshalIndent(columnIndex, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(columnIndexPath, data, 0o644)
}

func readProjectIndex(path string) (index.ProjectIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return index.ProjectIndex{Projects: map[string]index.ProjectEntry{}, Extra: map[string]json.RawMessage{}}, nil
		}
		return index.ProjectIndex{}, err
	}
	return index.ParseProjectIndex(data)
}

func readColumnIndex(path string) (index.ColumnIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return index.ColumnIndex{Columns: map[string]index.ColumnEntry{}, Extra: map[string]json.RawMessage{}}, nil
		}
		return index.ColumnIndex{}, err
	}
	return index.ParseColumnIndex(data)
}

func readModeIndex(path string) (index.ModeIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return index.ModeIndex{Modes: map[string]index.ModeEntry{}, Extra: map[string]json.RawMessage{}}, nil
		}
		return index.ModeIndex{}, err
	}
	return index.ParseModeIndex(data)
}

func writeIfMissing(path string, contents []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, contents, 0o644)
}

func sortedColumnNames(columnIndex index.ColumnIndex) []string {
	names := make([]string, 0, len(columnIndex.Columns))
	for name := range columnIndex.Columns {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func columnIndexTemplate() string {
	return "{\n  // Add columns keyed by folder name.\n}\n"
}

func settingIndexTemplate() string {
	return "{\n  \"description\": \"\",\n  \"defaultTarget\": \"\",\n  \"settings\": {\n    // \"ExampleSetting\": {\n    //   \"displayName\": \"Example Setting\",\n    //   \"description\": \"\",\n    //   \"target\": \"\"\n    // }\n  }\n}\n"
}

func modeIndexTemplate(modeIndex index.ModeIndex) string {
	if len(modeIndex.Modes) == 0 {
		return "{\n  // Add modes keyed by mode name.\n}\n"
	}
	data, err := json.MarshalIndent(modeIndex, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(append(data, '\n'))
}

// WarehouseRoot exposes the command-facing warehouse path convention.
func WarehouseRoot() (string, error) {
	return warehouse.DefaultWarehouseRoot()
}
