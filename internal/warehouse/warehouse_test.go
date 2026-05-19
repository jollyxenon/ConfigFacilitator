package warehouse

import (
	"path/filepath"
	"testing"
)

func TestPathForExecutableUsesExecutableSiblingWarehouse(t *testing.T) {
	executablePath := filepath.Join("/tmp", "cfgfc-bin", "cfgfc")
	got := PathForExecutable(executablePath)
	want := filepath.Join("/tmp", "cfgfc-bin", "SettingWarehouse")
	if got != want {
		t.Fatalf("expected warehouse path %q, got %q", want, got)
	}
}

func TestLoadWarehouseBuildsProjectColumnAndModeRelationships(t *testing.T) {
	root := filepath.Join("testdata", "basic", "SettingWarehouse")
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
	if column.SettingIndex.DefaultTarget != "~/.config/opencode/opencode.json" {
		t.Fatalf("expected defaultTarget preserved, got %q", column.SettingIndex.DefaultTarget)
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
	if selection.Strategy != "incremental" {
		t.Fatalf("expected incremental strategy preserved, got %q", selection.Strategy)
	}
	if len(selection.Settings) != 2 {
		t.Fatalf("expected two selected settings, got %d", len(selection.Settings))
	}
}

func TestLoadWarehousePreservesMissingEntriesAndBackupReferences(t *testing.T) {
	root := filepath.Join("testdata", "missing", "SettingWarehouse")
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
