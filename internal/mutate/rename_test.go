package mutate_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/linker"
	"github.com/xenon/ConfigFacilitator/internal/mutate"
	"github.com/xenon/ConfigFacilitator/internal/planner"
	"github.com/xenon/ConfigFacilitator/internal/repository"
)

// TestRenameSettingRewritesEverySchemaReferenceAndPreservesMetadata verifies the full canonical plan.
func TestRenameSettingRewritesEverySchemaReferenceAndPreservesMetadata(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	oldSource := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	fixedTarget := filepath.Join(targetDir, "fixed.json")
	derivedTarget := filepath.Join(targetDir, "GPT.json")
	state := repository.CurrentState{
		Columns:  map[string]repository.ColumnSelection{"Models": {Strategy: "cover", Settings: []string{"GPT.json"}}},
		Mappings: []repository.Mapping{{Source: oldSource, Target: fixedTarget}, {Source: oldSource, Target: derivedTarget}},
		Extra:    map[string]json.RawMessage{"stateExtra": json.RawMessage(`{"keep":1}`)},
	}
	if err := repo.SaveCurrentState("OpenCode", state); err != nil {
		t.Fatal(err)
	}
	history := []repository.HistoryEntry{{
		Timestamp:        "one",
		PreviousColumns:  map[string]repository.ColumnSelection{"Models": {Strategy: "cover", Settings: []string{"GPT.json"}}},
		PreviousMappings: []repository.Mapping{{Source: oldSource, Target: fixedTarget}},
		NextMappings:     []repository.Mapping{{Source: oldSource, Target: derivedTarget}},
		NextRelation:     &repository.CurrentRelation{Kind: "following", OriginMode: "Max"},
		Extra:            map[string]json.RawMessage{"historyExtra": json.RawMessage(`"keep"`)},
	}}
	if err := repo.SaveHistory("OpenCode", history); err != nil {
		t.Fatal(err)
	}
	for _, mapping := range state.Mappings {
		if err := os.MkdirAll(filepath.Dir(mapping.Target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(mapping.Source, mapping.Target); err != nil {
			t.Fatal(err)
		}
	}

	plan, err := mutate.BuildRenamePlan(repo, mutate.RenameRequest{Kind: mutate.SettingKind, ProjectReference: "OpenCode", ColumnReference: "Models", OldReference: "gpt", NewName: "Primary.json", PlanOptions: renamePlanOptions(t, targetDir)})
	if err != nil {
		t.Fatalf("BuildRenamePlan: %v", err)
	}
	if len(plan.Moves) != 1 || len(plan.IndexReferences) < 3 || len(plan.ManagedLinks) != 2 || plan.HistoryEntries != 1 {
		t.Fatalf("incomplete plan = %#v", plan)
	}
	if err := mutate.RenameSetting(repo, "OpenCode", "Models", "gpt", "Primary.json", false, renamePlanOptions(t, targetDir)); err != nil {
		t.Fatalf("RenameSetting: %v", err)
	}

	newSource := filepath.Join(root, "OpenCode", "Column", "Models", "Primary.json")
	if data, err := os.ReadFile(newSource); err != nil || string(data) != "content" {
		t.Fatalf("renamed content = %q err=%v", data, err)
	}
	if _, err := os.Lstat(oldSource); !os.IsNotExist(err) {
		t.Fatalf("old source survived: %v", err)
	}
	settingIndex, err := repo.LoadSettingIndex("OpenCode", "Models")
	if err != nil {
		t.Fatal(err)
	}
	entry := settingIndex.Settings["Primary.json"]
	if entry.DisplayName != "GPT display" || entry.Description != "description" || !reflect.DeepEqual(entry.Aliases, []string{"gpt"}) || string(entry.Extra["settingExtra"]) != `{"keep":true}` || entry.TargetName[0] != "" || entry.TargetName[1] != "" {
		t.Fatalf("Setting metadata changed = %#v", entry)
	}
	modeIndex, _ := repo.LoadModeIndex("OpenCode")
	if got := modeIndex.Modes["Max"].Columns["Models"].Settings; !reflect.DeepEqual(got, []string{"Primary.json"}) {
		t.Fatalf("mode refs = %#v", got)
	}
	current, _ := repo.LoadCurrentState("OpenCode")
	if current.Columns["Models"].Settings[0] != "Primary.json" || current.Mappings[0].Target != fixedTarget || current.Mappings[1].Target != filepath.Join(targetDir, "Primary.json") || !jsonRawEqual(current.Extra["stateExtra"], `{"keep":1}`) {
		t.Fatalf("current state = %#v", current)
	}
	entries, _ := repo.LoadHistory("OpenCode")
	if entries[0].PreviousMappings[0].Source != newSource || entries[0].NextMappings[0].Target != filepath.Join(targetDir, "Primary.json") || entries[0].PreviousColumns["Models"].Settings[0] != "Primary.json" || !jsonRawEqual(entries[0].Extra["historyExtra"], `"keep"`) {
		t.Fatalf("history = %#v", entries)
	}
	for _, target := range []string{fixedTarget, filepath.Join(targetDir, "Primary.json")} {
		link, err := os.Readlink(target)
		if err != nil || link != newSource {
			t.Fatalf("managed link %q -> %q err=%v", target, link, err)
		}
	}
	if _, err := os.Lstat(derivedTarget); !os.IsNotExist(err) {
		t.Fatalf("old derived target survived: %v", err)
	}
}

// TestRenameProjectColumnModeAndMappingOnlyState verifies the remaining canonical scopes.
func TestRenameProjectColumnModeAndMappingOnlyState(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	oldSource := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	target := filepath.Join(targetDir, "fixed.json")
	if err := repo.SaveCurrentState("OpenCode", repository.CurrentState{Mappings: []repository.Mapping{{Source: oldSource, Target: target}}}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(oldSource, target); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveSession(101, repository.SessionRecord{Project: "OpenCode", Extra: map[string]json.RawMessage{"contextExtra": json.RawMessage(`1`)}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveSession(202, repository.SessionRecord{Project: "OpenCode"}); err != nil {
		t.Fatal(err)
	}
	if err := mutate.RenameMode(repo, "OpenCode", "max", "Maximum", false, renamePlanOptions(t, targetDir)); err != nil {
		t.Fatal(err)
	}
	if err := mutate.RenameColumn(repo, "OpenCode", "models", "Configurations", false, renamePlanOptions(t, targetDir)); err != nil {
		t.Fatal(err)
	}
	if err := mutate.RenameProject(repo, "OpenCode", "Code", false, renamePlanOptions(t, targetDir)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "Code", "Column", "Configurations", "GPT.json")); err != nil {
		t.Fatalf("renamed project tree: %v", err)
	}
	projectIndex, _ := repo.LoadProjectIndex()
	if _, ok := projectIndex.Projects["Code"]; !ok {
		t.Fatalf("ProjectIndex = %#v", projectIndex.Projects)
	}
	columnIndex, _ := repo.LoadColumnIndex("Code")
	if _, ok := columnIndex.Columns["Configurations"]; !ok {
		t.Fatalf("ColumnIndex = %#v", columnIndex.Columns)
	}
	modeIndex, _ := repo.LoadModeIndex("Code")
	mode := modeIndex.Modes["Maximum"]
	if _, ok := mode.Columns["Configurations"]; !ok || string(mode.Extra["modeExtra"]) != `{"keep":true}` {
		t.Fatalf("ModeIndex = %#v", modeIndex)
	}
	current, _ := repo.LoadCurrentState("Code")
	wantSource := filepath.Join(root, "Code", "Column", "Configurations", "GPT.json")
	if current.Relation != nil || current.Mappings[0].Source != wantSource {
		t.Fatalf("mapping-only state = %#v", current)
	}
	if link, err := os.Readlink(target); err != nil || link != wantSource {
		t.Fatalf("target -> %q err=%v", link, err)
	}
	for _, ppid := range []int{101, 202} {
		record, ok, err := repo.LoadSession(ppid)
		if err != nil || !ok || record.Project != "Code" {
			t.Fatalf("context %d = %#v ok=%v err=%v", ppid, record, ok, err)
		}
	}
}

// TestRenameMissingResourcesDoesNotCreatePaths verifies indexed missing resources remain missing.
func TestRenameMissingResourcesDoesNotCreatePaths(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	settingPath := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	if err := os.Remove(settingPath); err != nil {
		t.Fatal(err)
	}
	if err := mutate.RenameSetting(repo, "OpenCode", "Models", "GPT.json", "Missing.json", false, renamePlanOptions(t, targetDir)); err != nil {
		t.Fatalf("rename missing inactive Setting: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "OpenCode", "Column", "Models", "Missing.json")); !os.IsNotExist(err) {
		t.Fatalf("rename created missing source: %v", err)
	}
	settingIndex, _ := repo.LoadSettingIndex("OpenCode", "Models")
	if _, ok := settingIndex.Settings["Missing.json"]; !ok {
		t.Fatalf("missing index key was not renamed: %#v", settingIndex.Settings)
	}
	missingSource := filepath.Join(root, "OpenCode", "Column", "Models", "Missing.json")
	if err := repo.SaveCurrentState("OpenCode", repository.CurrentState{Mappings: []repository.Mapping{{Source: missingSource, Target: filepath.Join(targetDir, "fixed.json")}}}); err != nil {
		t.Fatal(err)
	}
	if err := mutate.RenameSetting(repo, "OpenCode", "Models", "Missing.json", "Other.json", true, renamePlanOptions(t, targetDir)); err == nil {
		t.Fatal("expected active missing source refusal")
	}
}

// TestRenameDriftForceAndRollback verifies ownership refusal, forced reclamation, and exact transaction rollback.
func TestRenameDriftForceAndRollback(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	oldSource := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	target := filepath.Join(targetDir, "fixed.json")
	if err := repo.SaveCurrentState("OpenCode", repository.CurrentState{Mappings: []repository.Mapping{{Source: oldSource, Target: target}}}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("drift"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := mutate.RenameSetting(repo, "OpenCode", "Models", "GPT.json", "Blocked.json", false, renamePlanOptions(t, targetDir)); err == nil {
		t.Fatal("expected drift refusal")
	}
	if _, err := os.Stat(oldSource); err != nil {
		t.Fatalf("refused rename changed source: %v", err)
	}
	if err := mutate.RenameSetting(repo, "OpenCode", "Models", "GPT.json", "Forced.json", true, renamePlanOptions(t, targetDir)); err != nil {
		t.Fatalf("forced rename: %v", err)
	}
	forcedSource := filepath.Join(root, "OpenCode", "Column", "Models", "Forced.json")
	if link, err := os.Readlink(target); err != nil || link != forcedSource {
		t.Fatalf("forced target -> %q err=%v", link, err)
	}

	rollbackRepo := repository.New(root, repository.WithHooks(repository.Hooks{BeforeStage: func(stage repository.Stage) error {
		if stage == repository.StageCommitted {
			return errors.New("injected rename failure")
		}
		return nil
	}}))
	beforeIndex, _ := os.ReadFile(repo.SettingIndexPath("OpenCode", "Models"))
	beforeState, _ := os.ReadFile(repo.CurrentStatePath("OpenCode"))
	if err := mutate.RenameSetting(rollbackRepo, "OpenCode", "Models", "Forced.json", "Rollback.json", false, renamePlanOptions(t, targetDir)); err == nil {
		t.Fatal("expected injected rollback")
	}
	afterIndex, _ := os.ReadFile(repo.SettingIndexPath("OpenCode", "Models"))
	afterState, _ := os.ReadFile(repo.CurrentStatePath("OpenCode"))
	if !reflect.DeepEqual(beforeIndex, afterIndex) || !reflect.DeepEqual(beforeState, afterState) {
		t.Fatal("rollback did not restore runtime/index bytes")
	}
	if _, err := os.Stat(forcedSource); err != nil {
		t.Fatalf("rollback source missing: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "OpenCode", "Column", "Models", "Rollback.json")); !os.IsNotExist(err) {
		t.Fatalf("rollback destination survived: %v", err)
	}
	if link, err := os.Readlink(target); err != nil || link != forcedSource {
		t.Fatalf("rollback target -> %q err=%v", link, err)
	}
}

// createRenameFixture creates two target positions and two Mode references with extension metadata.
func createRenameFixture(t *testing.T) (string, repository.Repository, string) {
	t.Helper()
	root := t.TempDir()
	repo := repository.New(root)
	if err := mutate.CreateProject(repo, "OpenCode", renameMetadata(t, mutate.ProjectKind, "OpenCode", []string{"oc"})); err != nil {
		t.Fatal(err)
	}
	if err := mutate.CreateColumn(repo, "OpenCode", "Models", renameMetadata(t, mutate.ColumnKind, "Models", []string{"models"})); err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(root, "targets")
	options := renamePlanOptions(t, targetDir)
	if _, _, err := mutate.AddColumnTarget(repo, "OpenCode", "Models", mutate.TargetPosition{Dir: targetDir, Name: "fixed.json", DirMode: "fixed", NameMode: "fixed"}, options); err != nil {
		t.Fatal(err)
	}
	if _, _, err := mutate.AddColumnTarget(repo, "OpenCode", "Models", mutate.TargetPosition{Dir: targetDir, NameMode: "setting"}, options); err != nil {
		t.Fatal(err)
	}
	metadata := renameMetadata(t, mutate.SettingKind, "GPT.json", []string{"gpt"})
	metadata.DisplayName = "GPT display"
	metadata.Description = "description"
	if err := mutate.CreateSetting(repo, "OpenCode", "Models", "GPT.json", "file", metadata); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	settingIndex, _ := repo.LoadSettingIndex("OpenCode", "Models")
	entry := settingIndex.Settings["GPT.json"]
	entry.Extra["settingExtra"] = json.RawMessage(`{"keep":true}`)
	settingIndex.Settings["GPT.json"] = entry
	if err := repo.SaveSettingIndex("OpenCode", "Models", settingIndex); err != nil {
		t.Fatal(err)
	}
	if err := mutate.CreateMode(repo, "OpenCode", "Max", renameMetadata(t, mutate.ModeKind, "Max", []string{"max"})); err != nil {
		t.Fatal(err)
	}
	if err := mutate.CreateMode(repo, "OpenCode", "Other", renameMetadata(t, mutate.ModeKind, "Other", nil)); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"Max", "Other"} {
		if _, _, _, err := mutate.SetModeColumnSelection(repo, "OpenCode", mode, "Models", "cover", []string{"GPT.json"}); err != nil {
			t.Fatal(err)
		}
	}
	modeIndex, _ := repo.LoadModeIndex("OpenCode")
	modeEntry := modeIndex.Modes["Max"]
	modeEntry.Extra["modeExtra"] = json.RawMessage(`{"keep":true}`)
	modeEntry.Columns["Models"] = index.ModeColumnSelection{Strategy: "cover", Settings: []string{"GPT.json"}, Extra: map[string]json.RawMessage{"selectionExtra": json.RawMessage(`true`)}}
	modeIndex.Modes["Max"] = modeEntry
	if err := repo.SaveModeIndex("OpenCode", modeIndex); err != nil {
		t.Fatal(err)
	}
	return root, repo, targetDir
}

// renameMetadata constructs common validated resource metadata.
func renameMetadata(t *testing.T, kind mutate.ResourceKind, name string, aliases []string) mutate.Metadata {
	t.Helper()
	metadata, err := mutate.NewMetadata(kind, name, name, "", aliases)
	if err != nil {
		t.Fatal(err)
	}
	return metadata
}

// renamePlanOptions returns deterministic target expansion options.
func renamePlanOptions(t *testing.T, targetDir string) planner.PlanOptions {
	t.Helper()
	return planner.PlanOptions{HomeDir: filepath.Dir(targetDir), Env: map[string]string{}, OS: "linux"}
}

// jsonRawEqual compares preserved unknown JSON fields independent of formatting.
func jsonRawEqual(raw json.RawMessage, expected string) bool {
	var left any
	var right any
	return json.Unmarshal(raw, &left) == nil && json.Unmarshal([]byte(expected), &right) == nil && reflect.DeepEqual(left, right)
}

// ensure the linker import remains part of compile-time task coverage.
var _ linker.Ownership = linker.OwnershipOwned
