package warehouse

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultWarehouseRootUsesHomeConfigDirectory(t *testing.T) {
	homeDir := filepath.Join("/tmp", "cfgfc-home")
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home dir: %v", err)
	}
	got, err := DefaultWarehouseRoot()
	if err != nil {
		t.Fatalf("DefaultWarehouseRoot returned error: %v", err)
	}
	want := filepath.Join(homeDir, ".configfacilitator")
	if got != want {
		t.Fatalf("expected warehouse path %q, got %q", want, got)
	}
}

func TestLoadWarehouseBuildsProjectColumnAndModeRelationships(t *testing.T) {
	root := filepath.Join("testdata", "basic")
	warehouse, err := LoadWarehouse(root)
	if err != nil {
		t.Fatalf("LoadWarehouse returned error: %v", err)
	}

	project, ok := warehouse.Projects["OpenCode"]
	if !ok {
		t.Fatalf("expected OpenCode project to be present")
	}
	if project.Metadata.DisplayName != "OpenCode" {
		t.Fatalf("expected project display name preserved, got %q", project.Metadata.DisplayName)
	}
	if project.CurrentStatePath != filepath.Join(root, "OpenCode", "Backup", "current_state.json") {
		t.Fatalf("unexpected current state path: %q", project.CurrentStatePath)
	}

	column, ok := project.Columns["opencode.json"]
	if !ok {
		t.Fatalf("expected opencode.json column to be present")
	}
	if !reflect.DeepEqual(column.SettingIndex.DefaultTargetDir, []string{"~/.config/opencode"}) {
		t.Fatalf("expected defaultTargetDir preserved, got %#v", column.SettingIndex.DefaultTargetDir)
	}
	if !reflect.DeepEqual(column.SettingIndex.DefaultTargetName, []string{"opencode.json"}) {
		t.Fatalf("expected defaultTargetName preserved, got %#v", column.SettingIndex.DefaultTargetName)
	}
	if !column.Settings["GPT.json"].Exists {
		t.Fatalf("expected GPT.json setting to exist on disk")
	}

	skills, ok := project.Columns["Skills"]
	if !ok {
		t.Fatalf("expected Skills column to be present")
	}
	if !skills.Settings["Skill-A"].IsDir {
		t.Fatalf("expected Skill-A to be recognized as a directory setting")
	}

	mode, ok := project.Modes["Max"]
	if !ok {
		t.Fatalf("expected Max mode to be present")
	}
	selection, ok := mode.Metadata.Columns["Skills"]
	if !ok {
		t.Fatalf("expected mode to preserve Skills selection")
	}
	if selection.Strategy != "increment" {
		t.Fatalf("expected increment strategy preserved, got %q", selection.Strategy)
	}
	if len(selection.Settings) != 2 {
		t.Fatalf("expected two selected settings, got %d", len(selection.Settings))
	}
}

func TestLoadWarehousePreservesMissingEntriesAndBackupReferences(t *testing.T) {
	root := filepath.Join("testdata", "missing")
	warehouse, err := LoadWarehouse(root)
	if err != nil {
		t.Fatalf("LoadWarehouse returned error: %v", err)
	}

	project := warehouse.Projects["OpenCode"]
	if project.HistoryLogPath != filepath.Join(root, "OpenCode", "Backup", "history.log") {
		t.Fatalf("unexpected history log path: %q", project.HistoryLogPath)
	}

	column := project.Columns["opencode.json"]
	missingSetting, ok := column.Settings["CLAUDE.json"]
	if !ok {
		t.Fatalf("expected missing CLAUDE.json entry to remain present")
	}
	if !missingSetting.Missing {
		t.Fatalf("expected missing CLAUDE.json to be marked missing")
	}
	if missingSetting.Exists {
		t.Fatalf("expected missing CLAUDE.json not to exist on disk")
	}

	orphanedMode, ok := project.Modes["orphanedMode"]
	if !ok {
		t.Fatalf("expected orphaned mode to remain present")
	}
	if !orphanedMode.Missing {
		t.Fatalf("expected orphaned mode to retain missing marker")
	}
}

func TestLoadWarehouseIncludesSettingWarehouseAtRoot(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "OpenCode")
	for _, dir := range []string{
		filepath.Join(projectRoot, "Column", "Skills"),
		filepath.Join(projectRoot, "Mode"),
		filepath.Join(projectRoot, "Backup"),
		filepath.Join(root, ".cfgfc-session"),
		filepath.Join(root, "SettingWarehouse", "LegacyProject", "Column"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ProjectIndex.jsonc"), []byte(`{
  "OpenCode": {}
}`), 0o644); err != nil {
		t.Fatalf("write project index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte(`{
  "Skills": {}
}`), 0o644); err != nil {
		t.Fatalf("write column index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "Skills", "SettingIndex.jsonc"), []byte(`{
  "settings": {}
}`), 0o644); err != nil {
		t.Fatalf("write setting index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Mode", "ModeIndex.jsonc"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write mode index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "current_state.json"), []byte("{\"mappings\": []}\n"), 0o644); err != nil {
		t.Fatalf("write current state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "history.log"), []byte{}, 0o644); err != nil {
		t.Fatalf("write history log: %v", err)
	}

	loaded, err := LoadWarehouse(root)
	if err != nil {
		t.Fatalf("LoadWarehouse returned error: %v", err)
	}
	if _, ok := loaded.Projects["SettingWarehouse"]; !ok {
		t.Fatalf("expected SettingWarehouse project to load")
	}
	if _, ok := loaded.Projects["OpenCode"]; !ok {
		t.Fatalf("expected OpenCode project to load")
	}
	if len(loaded.Projects) != 2 {
		t.Fatalf("expected two discovered projects, got %d", len(loaded.Projects))
	}
}

func TestLoadWarehouseAllowsColumnNamedLegacyOrSessionMarkers(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "OpenCode")
	for _, dir := range []string{
		filepath.Join(projectRoot, "Column", "SettingWarehouse"),
		filepath.Join(projectRoot, "Column", ".cfgfc-session"),
		filepath.Join(projectRoot, "Mode"),
		filepath.Join(projectRoot, "Backup"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ProjectIndex.jsonc"), []byte(`{
  "OpenCode": {}
}`), 0o644); err != nil {
		t.Fatalf("write project index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte(`{
  "SettingWarehouse": {},
  ".cfgfc-session": {}
}`), 0o644); err != nil {
		t.Fatalf("write column index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "SettingWarehouse", "SettingIndex.jsonc"), []byte(`{
  "settings": {}
}`), 0o644); err != nil {
		t.Fatalf("write legacy-named setting index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", ".cfgfc-session", "SettingIndex.jsonc"), []byte(`{
  "settings": {}
}`), 0o644); err != nil {
		t.Fatalf("write session-named setting index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Mode", "ModeIndex.jsonc"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write mode index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "current_state.json"), []byte("{\"mappings\": []}\n"), 0o644); err != nil {
		t.Fatalf("write current state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "history.log"), []byte{}, 0o644); err != nil {
		t.Fatalf("write history log: %v", err)
	}

	loaded, err := LoadWarehouse(root)
	if err != nil {
		t.Fatalf("LoadWarehouse returned error: %v", err)
	}
	project, ok := loaded.Projects["OpenCode"]
	if !ok {
		t.Fatalf("expected OpenCode project to load")
	}
	if _, ok := project.Columns["SettingWarehouse"]; !ok {
		t.Fatalf("expected column named SettingWarehouse to load")
	}
	if _, ok := project.Columns[".cfgfc-session"]; !ok {
		t.Fatalf("expected column named .cfgfc-session to load")
	}
}

func TestLoadWarehouseExposesCanonicalIdentityAndAliases(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "OpenCodeFolder")
	for _, path := range []string{
		filepath.Join(projectRoot, "Column", "skills-dir", "Skill-A"),
		filepath.Join(projectRoot, "Mode"),
		filepath.Join(projectRoot, "Backup"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ProjectIndex.jsonc"), []byte(`{
  "OpenCodeFolder": {
	    "displayName": "Open Code",
	    "aliases": ["oc"]
  }
}`), 0o644); err != nil {
		t.Fatalf("write project index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte(`{
  "skills-dir": {
	    "displayName": "Skills",
	    "aliases": ["skills"]
  }
}`), 0o644); err != nil {
		t.Fatalf("write column index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "skills-dir", "SettingIndex.jsonc"), []byte(`{
  "defaultTargetDir": ["~/.config"],
  "defaultTargetName": ["test"],
  "settings": {
    "Skill-A": {
	      "displayName": "Skill A",
	      "aliases": ["alpha"]
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write setting index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Mode", "ModeIndex.jsonc"), []byte(`{
  "Max": {
	    "displayName": "Max",
	    "aliases": ["m"],
    "columns": {
      "skills": {
        "settings": ["alpha"],
        "strategy": "full"
      }
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write mode index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "current_state.json"), []byte("{\"mappings\": []}\n"), 0o644); err != nil {
		t.Fatalf("write current_state.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "history.log"), []byte{}, 0o644); err != nil {
		t.Fatalf("write history.log: %v", err)
	}

	loaded, err := LoadWarehouse(root)
	if err != nil {
		t.Fatalf("LoadWarehouse returned error: %v", err)
	}
	project, err := loaded.ResolveProject("oc")
	if err != nil {
		t.Fatalf("ResolveProject(alias) returned error: %v", err)
	}
	if project.Name != "OpenCodeFolder" {
		t.Fatalf("unexpected project identity: name=%q", project.Name)
	}
	column, err := project.ResolveColumn("skills")
	if err != nil {
		t.Fatalf("ResolveColumn(alias) returned error: %v", err)
	}
	if column.Name != "skills-dir" {
		t.Fatalf("unexpected column identity: name=%q", column.Name)
	}
	setting, err := column.ResolveSetting("alpha")
	if err != nil {
		t.Fatalf("ResolveSetting(alias) returned error: %v", err)
	}
	if setting.Name != "Skill-A" {
		t.Fatalf("unexpected setting identity: name=%q", setting.Name)
	}
	mode, err := project.ResolveMode("m")
	if err != nil {
		t.Fatalf("ResolveMode(alias) returned error: %v", err)
	}
	if mode.Name != "Max" {
		t.Fatalf("unexpected mode identity: name=%q", mode.Name)
	}
}

func TestLoadWarehousePreservesAdditionalIdentityField(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "OpenCode")
	for _, dir := range []string{
		filepath.Join(projectRoot, "Column"),
		filepath.Join(projectRoot, "Mode"),
		filepath.Join(projectRoot, "Backup"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ProjectIndex.jsonc"), []byte(`{
  "OpenCode": {
    "warehouseName": "OpenCode"
  }
}`), 0o644); err != nil {
		t.Fatalf("write project index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write column index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Mode", "ModeIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write mode index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "current_state.json"), []byte("{\"mappings\": []}\n"), 0o644); err != nil {
		t.Fatalf("write current state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "history.log"), []byte{}, 0o644); err != nil {
		t.Fatalf("write history log: %v", err)
	}

	loaded, err := LoadWarehouse(root)
	if err != nil {
		t.Fatalf("LoadWarehouse returned error: %v", err)
	}
	if got := string(loaded.ProjectIndex.Projects["OpenCode"].Extra["warehouseName"]); got != `"OpenCode"` {
		t.Fatalf("expected warehouseName preserved in project extra fields, got %q", got)
	}
}

func TestLoadWarehouseRejectsCollidingProjectReferences(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"OpenCode", "ClaudeCode"} {
		if err := os.MkdirAll(filepath.Join(root, dir, "Column"), 0o755); err != nil {
			t.Fatalf("mkdir project: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(root, dir, "Mode"), 0o755); err != nil {
			t.Fatalf("mkdir mode: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(root, dir, "Backup"), 0o755); err != nil {
			t.Fatalf("mkdir backup: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "Column", "ColumnIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write column index: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "Mode", "ModeIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write mode index: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "Backup", "current_state.json"), []byte("{\"mappings\": []}\n"), 0o644); err != nil {
			t.Fatalf("write current state: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "Backup", "history.log"), []byte{}, 0o644); err != nil {
			t.Fatalf("write history log: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ProjectIndex.jsonc"), []byte(`{
  "OpenCode": {"aliases": ["shared"]},
  "ClaudeCode": {"aliases": ["shared"]}
}`), 0o644); err != nil {
		t.Fatalf("write project index: %v", err)
	}

	if _, err := LoadWarehouse(root); err == nil {
		t.Fatalf("expected collision error")
	}
}

func TestLoadWarehouseRejectsReservedGlobalProjectReference(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "OpenCode")
	for _, dir := range []string{
		filepath.Join(projectRoot, "Column"),
		filepath.Join(projectRoot, "Mode"),
		filepath.Join(projectRoot, "Backup"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ProjectIndex.jsonc"), []byte(`{
  "OpenCode": {
	    "aliases": ["global"]
  }
}`), 0o644); err != nil {
		t.Fatalf("write project index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write column index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Mode", "ModeIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write mode index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "current_state.json"), []byte("{\"mappings\": []}\n"), 0o644); err != nil {
		t.Fatalf("write current state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "history.log"), []byte{}, 0o644); err != nil {
		t.Fatalf("write history log: %v", err)
	}

	if _, err := LoadWarehouse(root); err == nil {
		t.Fatalf("expected reserved reference error")
	}
}

func TestLoadWarehouseRejectsExplicitIDField(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "OpenCode")
	for _, dir := range []string{
		filepath.Join(projectRoot, "Column"),
		filepath.Join(projectRoot, "Mode"),
		filepath.Join(projectRoot, "Backup"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ProjectIndex.jsonc"), []byte(`{
  "OpenCode": {
    "id": "open-code"
  }
}`), 0o644); err != nil {
		t.Fatalf("write project index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write column index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Mode", "ModeIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write mode index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "current_state.json"), []byte("{\"mappings\": []}\n"), 0o644); err != nil {
		t.Fatalf("write current state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "history.log"), []byte{}, 0o644); err != nil {
		t.Fatalf("write history log: %v", err)
	}

	if _, err := LoadWarehouse(root); err == nil {
		t.Fatalf("expected explicit id field to be rejected")
	}
}
