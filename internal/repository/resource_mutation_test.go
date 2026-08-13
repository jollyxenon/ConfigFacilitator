package repository_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/mutate"
	"github.com/xenon/ConfigFacilitator/internal/repository"
)

// TestResourceCreationCommitsCompleteRepositoryState verifies create resources need no later sync.
func TestResourceCreationCommitsCompleteRepositoryState(t *testing.T) {
	root := t.TempDir()
	repo := repository.New(root)
	projectMetadata := mustMetadata(t, mutate.ProjectKind, "OpenCode", []string{"oc"})
	if err := mutate.CreateProject(repo, "OpenCode", projectMetadata); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	columnMetadata := mustMetadata(t, mutate.ColumnKind, "Skills", []string{"skills"})
	if err := mutate.CreateColumn(repo, "oc", "Skills", columnMetadata); err != nil {
		t.Fatalf("CreateColumn: %v", err)
	}
	settingMetadata := mustMetadata(t, mutate.SettingKind, "Skill-A", []string{"alpha"})
	if err := mutate.CreateSetting(repo, "oc", "skills", "Skill-A", "directory", settingMetadata); err != nil {
		t.Fatalf("CreateSetting: %v", err)
	}
	modeMetadata := mustMetadata(t, mutate.ModeKind, "Max", []string{"maximal"})
	if err := mutate.CreateMode(repo, "oc", "Max", modeMetadata); err != nil {
		t.Fatalf("CreateMode: %v", err)
	}

	projectIndex, err := repo.LoadProjectIndex()
	if err != nil {
		t.Fatalf("LoadProjectIndex: %v", err)
	}
	if !reflect.DeepEqual(projectIndex.Projects["OpenCode"].Aliases, []string{"oc"}) {
		t.Fatalf("project aliases = %#v", projectIndex.Projects["OpenCode"].Aliases)
	}
	columnIndex, err := repo.LoadColumnIndex("OpenCode")
	if err != nil {
		t.Fatalf("LoadColumnIndex: %v", err)
	}
	if _, exists := columnIndex.Columns["Skills"]; !exists {
		t.Fatal("created Column is absent from its index")
	}
	settingIndex, err := repo.LoadSettingIndex("OpenCode", "Skills")
	if err != nil {
		t.Fatalf("LoadSettingIndex: %v", err)
	}
	if settingIndex.TargetNumber != 0 || len(settingIndex.DefaultTargetDir) != 0 || len(settingIndex.DefaultTargetName) != 0 {
		t.Fatalf("new Column targets = %#v", settingIndex)
	}
	setting := settingIndex.Settings["Skill-A"]
	if len(setting.TargetDir) != 0 || len(setting.TargetName) != 0 {
		t.Fatalf("new Setting overrides = %#v", setting)
	}
	if info, err := os.Stat(filepath.Join(root, "OpenCode", "Column", "Skills", "Skill-A")); err != nil || !info.IsDir() {
		t.Fatalf("directory Setting not immediately usable: info=%v err=%v", info, err)
	}
	modeIndex, err := repo.LoadModeIndex("OpenCode")
	if err != nil {
		t.Fatalf("LoadModeIndex: %v", err)
	}
	if columns := modeIndex.Modes["Max"].Columns; columns == nil || len(columns) != 0 {
		t.Fatalf("new Mode columns = %#v", columns)
	}
	if _, err := repo.LoadCurrentState("OpenCode"); err != nil {
		t.Fatalf("LoadCurrentState: %v", err)
	}
	if _, err := repo.LoadHistory("OpenCode"); err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
}

// TestModeMetadataMutationPreservesSelectionsAndExtra verifies set changes only requested metadata.
func TestModeMetadataMutationPreservesSelectionsAndExtra(t *testing.T) {
	root := t.TempDir()
	repo := repository.New(root)
	if err := mutate.CreateProject(repo, "OpenCode", mustMetadata(t, mutate.ProjectKind, "OpenCode", nil)); err != nil {
		t.Fatal(err)
	}
	modeIndex := index.ModeIndex{Modes: map[string]index.ModeEntry{
		"Max": {
			WarehouseName: "Max",
			DisplayName:   "Max",
			Aliases:       []string{"m"},
			Columns: map[string]index.ModeColumnSelection{
				"Skills": {Strategy: "full", Extra: map[string]json.RawMessage{"selectionExtra": json.RawMessage(`true`)}},
			},
			Extra: map[string]json.RawMessage{"modeExtra": json.RawMessage(`{"keep":1}`)},
		},
	}}
	if err := repo.SaveModeIndex("OpenCode", modeIndex); err != nil {
		t.Fatal(err)
	}
	description := "new description"
	aliases := []string{}
	if _, err := mutate.SetMode(repo, "OpenCode", "m", mutate.MetadataPatch{Description: &description, Aliases: &aliases}); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	updated, err := repo.LoadModeIndex("OpenCode")
	if err != nil {
		t.Fatal(err)
	}
	mode := updated.Modes["Max"]
	if mode.Description != description || len(mode.Aliases) != 0 || mode.Columns["Skills"].Strategy != "full" {
		t.Fatalf("known fields were not preserved/replaced correctly: %#v", mode)
	}
	if string(mode.Extra["modeExtra"]) != `{"keep":1}` || string(mode.Columns["Skills"].Extra["selectionExtra"]) != "true" {
		t.Fatalf("extension fields were not preserved: %#v", updated)
	}
}

// TestCreateRollbackRemovesEveryPartialProjectArtifact verifies repository fault rollback.
func TestCreateRollbackRemovesEveryPartialProjectArtifact(t *testing.T) {
	root := t.TempDir()
	repo := repository.New(root, repository.WithHooks(repository.Hooks{BeforeStage: func(stage repository.Stage) error {
		if stage == repository.StageCommitted {
			return errors.New("injected commit failure")
		}
		return nil
	}}))
	err := mutate.CreateProject(repo, "OpenCode", mustMetadata(t, mutate.ProjectKind, "OpenCode", nil))
	if err == nil {
		t.Fatal("expected injected create failure")
	}
	if _, err := os.Lstat(filepath.Join(root, "OpenCode")); !os.IsNotExist(err) {
		t.Fatalf("partial Project path survived rollback: %v", err)
	}
	projectIndex, loadErr := repository.New(root).LoadProjectIndex()
	if loadErr != nil {
		t.Fatalf("LoadProjectIndex after rollback: %v", loadErr)
	}
	if len(projectIndex.Projects) != 0 {
		t.Fatalf("partial Project index survived rollback: %#v", projectIndex.Projects)
	}
}

// mustMetadata constructs validated metadata for repository integration tests.
func mustMetadata(t *testing.T, kind mutate.ResourceKind, name string, aliases []string) mutate.Metadata {
	t.Helper()
	metadata, err := mutate.NewMetadata(kind, name, name, "", aliases)
	if err != nil {
		t.Fatalf("NewMetadata: %v", err)
	}
	return metadata
}
