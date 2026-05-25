package scaffold

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
		columns[columnName] = index.ModeColumnSelection{Settings: []string{}, Strategy: "cover", Extra: map[string]json.RawMessage{}}
	}
	modeIndex.Modes[modeName] = index.ModeEntry{DisplayName: modeName, Aliases: []string{}, Description: "", Columns: columns, Extra: map[string]json.RawMessage{}}
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
	if entry.WarehouseName == "" {
		entry.WarehouseName = projectName
	}
	if entry.DisplayName == "" {
		entry.DisplayName = projectName
	}
	if entry.Aliases == nil {
		entry.Aliases = []string{}
	}
	projectIndex.Projects[projectName] = entry
	data, err := json.MarshalIndent(projectIndex, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(projectIndexPath, []byte(formatJSONCWithExample(string(data), projectIndexExampleComment())), 0o644)
}

func ensureColumnIndex(projectPath string, columnName string) error {
	columnIndexPath := filepath.Join(projectPath, "Column", "ColumnIndex.jsonc")
	columnIndex, err := readColumnIndex(columnIndexPath)
	if err != nil {
		return err
	}
	entry := columnIndex.Columns[columnName]
	if entry.WarehouseName == "" {
		entry.WarehouseName = columnName
	}
	if entry.DisplayName == "" {
		entry.DisplayName = columnName
	}
	if entry.Aliases == nil {
		entry.Aliases = []string{}
	}
	columnIndex.Columns[columnName] = entry
	data, err := json.MarshalIndent(columnIndex, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(columnIndexPath, []byte(formatJSONCWithExample(string(data), columnIndexExampleComment())), 0o644)
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
	return formatJSONCWithExample("{}", columnIndexExampleComment())
}

func settingIndexTemplate() string {
	return formatJSONCWithExample("{\n  \"description\": \"\",\n  \"defaultTarget\": \"\",\n  \"settings\": {}\n}", settingIndexExampleComment())
}

func modeIndexTemplate(modeIndex index.ModeIndex) string {
	if len(modeIndex.Modes) == 0 {
		return formatJSONCWithExample("{}", modeIndexExampleComment())
	}
	data, err := json.MarshalIndent(modeIndex, "", "  ")
	if err != nil {
		panic(err)
	}
	return formatJSONCWithExample(string(data), modeIndexExampleComment())
}

// formatJSONCWithExample appends one trailing example comment block to a JSON body.
func formatJSONCWithExample(body string, exampleComment string) string {
	trimmedBody := strings.TrimRight(body, "\n")
	trimmedComment := strings.TrimRight(exampleComment, "\n")
	return trimmedBody + "\n\n" + trimmedComment + "\n"
}

// projectIndexExampleComment returns the generated example block for ProjectIndex.jsonc.
func projectIndexExampleComment() string {
	return `/*
Example:
{
  "OpenCode": {
	    "displayName": "OpenCode",
	    "aliases": [],
	    "description": "Optional note about this project"
  }
}

Keep durable notes in the "description" field.
*/`
}

// columnIndexExampleComment returns the generated example block for ColumnIndex.jsonc.
func columnIndexExampleComment() string {
	return `/*
Example:
{
  "Skills": {
	    "displayName": "Skills",
	    "aliases": [],
	    "description": "Optional note about this column"
  }
}

Add columns keyed by folder name and keep permanent notes in "description".
*/`
}

// settingIndexExampleComment returns the generated example block for SettingIndex.jsonc.
func settingIndexExampleComment() string {
	return `/*
Example:
{
  "description": "Optional note about this column",
  "defaultTarget": "~/.config/opencode/opencode.json",
  "settings": {
    "ExampleSetting": {
	      "displayName": "Example Setting",
	      "aliases": [],
	      "description": "Optional note about this setting",
	      "target": "~/.config/opencode/special/special.json"
    }
  }
}

Use "description" for permanent notes. Set "target" only when one setting needs a custom destination.
*/`
}

// modeIndexExampleComment returns the generated example block for ModeIndex.jsonc.
func modeIndexExampleComment() string {
	return `/*
Example:
{
  "Max": {
	    "displayName": "Max",
	    "aliases": [],
	    "description": "Optional note about this mode",
	    "columns": {
	      "Skills": {
        "settings": ["Skill-A", "Skill-B"],
	        "strategy": "increment"
	      },
	      "Agents": {
	        "strategy": "full"
      }
    }
  }
}

List modes keyed by mode name. Use "cover" for explicit replacement, "increment" to append to current managed links, "none" to link nothing, and "full" to link every known setting in a column. The "settings" field may be omitted for "none" and "full".
*/`
}

// WarehouseRoot exposes the command-facing warehouse path convention.
func WarehouseRoot() (string, error) {
	return warehouse.DefaultWarehouseRoot()
}
