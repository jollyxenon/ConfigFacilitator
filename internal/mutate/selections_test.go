package mutate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/repository"
)

// TestModeSelectionCRUDCanonicalizesReferencesAndPreservesExtras verifies selection persistence and isolation.
func TestModeSelectionCRUDCanonicalizesReferencesAndPreservesExtras(t *testing.T) {
	root := t.TempDir()
	repo := createModeSelectionFixture(t, root)

	modeName, columnName, settings, err := SetModeColumnSelection(repo, "project-alias", "mode-alias", "models-alias", "cover", []string{"gpt-alias", "tools-alias"})
	if err != nil {
		t.Fatalf("SetModeColumnSelection: %v", err)
	}
	if modeName != "Max" || columnName != "Models" || !reflect.DeepEqual(settings, []string{"GPT.json", "Tools.json"}) {
		t.Fatalf("canonical result = mode %q column %q settings %#v", modeName, columnName, settings)
	}
	modeIndex, err := repo.LoadModeIndex("OpenCode")
	if err != nil {
		t.Fatal(err)
	}
	entry := modeIndex.Modes["Max"]
	selection := entry.Columns["Models"]
	if selection.Strategy != "cover" || !reflect.DeepEqual(selection.Settings, []string{"GPT.json", "Tools.json"}) {
		t.Fatalf("canonical selection = %#v", selection)
	}
	if _, aliasPersisted := entry.Columns["models-alias"]; aliasPersisted {
		t.Fatal("Column alias was persisted as a selection key")
	}
	if entry.Description != "keep description" || string(entry.Extra["modeExtra"]) != `{"keep":true}` || string(selection.Extra["selectionExtra"]) != "1" {
		t.Fatalf("Mode or replaced selection metadata was lost: %#v", entry)
	}
	if unrelated := entry.Columns["Skills"]; unrelated.Strategy != "full" || string(unrelated.Extra["unrelatedExtra"]) != "true" {
		t.Fatalf("unrelated selection changed: %#v", unrelated)
	}

	deletedMode, deletedColumn, err := DeleteModeColumnSelection(repo, "OpenCode", "Max", "models-alias")
	if err != nil {
		t.Fatalf("DeleteModeColumnSelection: %v", err)
	}
	if deletedMode != "Max" || deletedColumn != "Models" {
		t.Fatalf("delete result = %q %q", deletedMode, deletedColumn)
	}
	modeIndex, err = repo.LoadModeIndex("OpenCode")
	if err != nil {
		t.Fatal(err)
	}
	entry = modeIndex.Modes["Max"]
	if _, exists := entry.Columns["Models"]; exists || entry.Columns["Skills"].Strategy != "full" || entry.Description != "keep description" || string(entry.Extra["modeExtra"]) != `{"keep":true}` {
		t.Fatalf("selection delete changed unrelated data: %#v", entry)
	}
}

// TestModeSelectionValidatesEveryStrategyBeforeWriting verifies strategy and reference failures leave exact bytes unchanged.
func TestModeSelectionValidatesEveryStrategyBeforeWriting(t *testing.T) {
	root := t.TempDir()
	repo := createModeSelectionFixture(t, root)
	modeIndexPath := repo.ModeIndexPath("OpenCode")
	before, err := os.ReadFile(modeIndexPath)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		strategy string
		settings []string
		code     string
	}{
		{name: "cover requires settings", strategy: "cover", code: "mode_settings_required"},
		{name: "increment requires settings", strategy: "increment", code: "mode_settings_required"},
		{name: "none rejects settings", strategy: "none", settings: []string{"GPT.json"}, code: "mode_settings_forbidden"},
		{name: "full rejects settings", strategy: "full", settings: []string{"GPT.json"}, code: "mode_settings_forbidden"},
		{name: "unknown strategy", strategy: "replace", code: "invalid_mode_strategy"},
		{name: "unknown setting", strategy: "cover", settings: []string{"Missing"}, code: "setting_not_found"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, _, mutationErr := SetModeColumnSelection(repo, "OpenCode", "Max", "Models", test.strategy, test.settings)
			assertMutationCode(t, mutationErr, test.code)
			after, readErr := os.ReadFile(modeIndexPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !reflect.DeepEqual(after, before) {
				t.Fatalf("Mode index changed after validation failure\nbefore=%s\nafter=%s", before, after)
			}
		})
	}

	for _, strategy := range []string{"none", "full"} {
		if _, _, settings, setErr := SetModeColumnSelection(repo, "OpenCode", "Max", "Models", strategy, nil); setErr != nil || len(settings) != 0 {
			t.Fatalf("strategy %q result settings=%#v err=%v", strategy, settings, setErr)
		}
	}
	for _, strategy := range []string{"cover", "increment"} {
		if _, _, settings, setErr := SetModeColumnSelection(repo, "OpenCode", "Max", "Models", strategy, []string{"GPT.json"}); setErr != nil || !reflect.DeepEqual(settings, []string{"GPT.json"}) {
			t.Fatalf("strategy %q result settings=%#v err=%v", strategy, settings, setErr)
		}
	}
}

// TestModeSelectionRejectsMissingSourcesAndRollsBackFailures verifies pre-write missing checks and transaction rollback.
func TestModeSelectionRejectsMissingSourcesAndRollsBackFailures(t *testing.T) {
	root := t.TempDir()
	repo := createModeSelectionFixture(t, root)
	modeIndexPath := repo.ModeIndexPath("OpenCode")
	before, err := os.ReadFile(modeIndexPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = SetModeColumnSelection(repo, "OpenCode", "Max", "Models", "cover", []string{"GPT.json"})
	assertMutationCode(t, err, "setting_missing")
	afterMissing, err := os.ReadFile(modeIndexPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(afterMissing, before) {
		t.Fatal("Mode index changed after missing Setting rejection")
	}

	injected := repository.New(root, repository.WithHooks(repository.Hooks{BeforeStage: func(stage repository.Stage) error {
		if stage == repository.StageCommitted {
			return errors.New("injected selection commit failure")
		}
		return nil
	}}))
	_, _, _, err = SetModeColumnSelection(injected, "OpenCode", "Max", "Models", "full", nil)
	assertMutationCode(t, err, "mode_column_set")
	afterRollback, err := os.ReadFile(modeIndexPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(afterRollback, before) {
		t.Fatal("Mode index was not restored after transaction failure")
	}
}

// createModeSelectionFixture creates aliased resources and extension-bearing Mode selections.
func createModeSelectionFixture(t *testing.T, root string) repository.Repository {
	t.Helper()
	repo := repository.New(root)
	projectMetadata, err := NewMetadata(ProjectKind, "OpenCode", "OpenCode", "", []string{"project-alias"})
	if err != nil {
		t.Fatal(err)
	}
	if err := CreateProject(repo, "OpenCode", projectMetadata); err != nil {
		t.Fatal(err)
	}
	for _, definition := range []struct {
		name    string
		aliases []string
	}{{name: "Models", aliases: []string{"models-alias"}}, {name: "Skills", aliases: []string{"skills-alias"}}} {
		metadata, metadataErr := NewMetadata(ColumnKind, definition.name, definition.name, "", definition.aliases)
		if metadataErr != nil {
			t.Fatal(metadataErr)
		}
		if createErr := CreateColumn(repo, "OpenCode", definition.name, metadata); createErr != nil {
			t.Fatal(createErr)
		}
	}
	for _, definition := range []struct {
		name  string
		alias string
	}{{name: "GPT.json", alias: "gpt-alias"}, {name: "Tools.json", alias: "tools-alias"}} {
		metadata, metadataErr := NewMetadata(SettingKind, definition.name, definition.name, "", []string{definition.alias})
		if metadataErr != nil {
			t.Fatal(metadataErr)
		}
		if createErr := CreateSetting(repo, "OpenCode", "Models", definition.name, "file", metadata); createErr != nil {
			t.Fatal(createErr)
		}
	}
	modeMetadata, err := NewMetadata(ModeKind, "Max", "Max", "keep description", []string{"mode-alias"})
	if err != nil {
		t.Fatal(err)
	}
	if err := CreateMode(repo, "OpenCode", "Max", modeMetadata); err != nil {
		t.Fatal(err)
	}
	modeIndex, err := repo.LoadModeIndex("OpenCode")
	if err != nil {
		t.Fatal(err)
	}
	entry := modeIndex.Modes["Max"]
	entry.Extra["modeExtra"] = json.RawMessage(`{"keep":true}`)
	entry.Columns["Models"] = index.ModeColumnSelection{Strategy: "none", Extra: map[string]json.RawMessage{"selectionExtra": json.RawMessage(`1`)}}
	entry.Columns["Skills"] = index.ModeColumnSelection{Strategy: "full", Extra: map[string]json.RawMessage{"unrelatedExtra": json.RawMessage(`true`)}}
	modeIndex.Modes["Max"] = entry
	if err := repo.SaveModeIndex("OpenCode", modeIndex); err != nil {
		t.Fatal(err)
	}
	return repo
}

// assertMutationCode checks one typed mutation failure without depending on rendered text.
func assertMutationCode(t *testing.T, err error, code string) {
	t.Helper()
	var mutationErr *Error
	if !errors.As(err, &mutationErr) || mutationErr.Code != code {
		t.Fatalf("mutation error = %#v, want code %q", err, code)
	}
}
