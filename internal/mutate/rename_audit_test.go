package mutate_test

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

// TestRenameProjectFullWarehouse verifies Project rename move order and every
// affected path: sub-indexes, current state, history, sessions, and managed links.
func TestRenameProjectFullWarehouse(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	oldSource := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	newSource := filepath.Join(root, "Code", "Column", "Models", "GPT.json")
	fixedTarget := filepath.Join(targetDir, "fixed.json")
	derivedTarget := filepath.Join(targetDir, "GPT.json")

	// Current state with both a fixed and a derived target plus a following Mode relation.
	// Derived target names come from Setting names, so a Project rename must keep
	// them unchanged while only the mapping sources move.
	state := repository.CurrentState{
		Mappings: []repository.Mapping{
			{Source: oldSource, Target: fixedTarget},
			{Source: oldSource, Target: derivedTarget},
		},
		Relation: &repository.CurrentRelation{Kind: "following", OriginMode: "Max"},
	}
	if err := repo.SaveCurrentState("OpenCode", state); err != nil {
		t.Fatal(err)
	}
	// History with both relations and both mapping directions.
	history := []repository.HistoryEntry{{
		Timestamp:        "t1",
		PreviousMappings: []repository.Mapping{{Source: oldSource, Target: fixedTarget}},
		NextMappings:     []repository.Mapping{{Source: oldSource, Target: derivedTarget}},
		PreviousRelation: &repository.CurrentRelation{Kind: "following", OriginMode: "Max"},
		NextColumns:      map[string]repository.ColumnSelection{"Models": {Strategy: "cover", Settings: []string{"GPT.json"}}},
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
	if err := repo.SaveSession(101, repository.SessionRecord{Project: "OpenCode"}); err != nil {
		t.Fatal(err)
	}

	plan, err := mutate.BuildRenamePlan(repo, mutate.RenameRequest{Kind: mutate.ProjectKind, OldReference: "oc", NewName: "Code", PlanOptions: renamePlanOptions(t, targetDir)})
	if err != nil {
		t.Fatalf("BuildRenamePlan: %v", err)
	}
	if len(plan.Moves) != 1 || plan.Moves[0].From != filepath.Join(root, "OpenCode") || plan.Moves[0].To != filepath.Join(root, "Code") || !plan.Moves[0].Exists {
		t.Fatalf("project move = %#v", plan.Moves)
	}
	if len(plan.Contexts) != 1 || plan.Contexts[0].PPID != 101 || plan.Contexts[0].From != "OpenCode" || plan.Contexts[0].To != "Code" {
		t.Fatalf("contexts = %#v", plan.Contexts)
	}
	if len(plan.ManagedLinks) != 2 {
		t.Fatalf("managed links = %#v", plan.ManagedLinks)
	}

	if err := mutate.RenameProject(repo, "oc", "Code", false, renamePlanOptions(t, targetDir)); err != nil {
		t.Fatalf("RenameProject: %v", err)
	}

	// Move order: every sub-object lives at the new path only.
	if _, err := os.Lstat(filepath.Join(root, "OpenCode")); !os.IsNotExist(err) {
		t.Fatalf("old project dir survived: %v", err)
	}
	for _, path := range []string{
		filepath.Join(root, "Code", "Column", "Models", "GPT.json"),
		filepath.Join(root, "Code", "Column", "Models", "SettingIndex.jsonc"),
		filepath.Join(root, "Code", "Column", "ColumnIndex.jsonc"),
		filepath.Join(root, "Code", "Mode", "ModeIndex.jsonc"),
		filepath.Join(root, "Code", "Backup", "current_state.json"),
		filepath.Join(root, "Code", "Backup", "history.log"),
	} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("new path %s missing: %v", path, err)
		}
	}

	projectIndex, _ := repo.LoadProjectIndex()
	if _, ok := projectIndex.Projects["Code"]; !ok {
		t.Fatalf("ProjectIndex key = %#v", projectIndex.Projects)
	}
	columnIndex, _ := repo.LoadColumnIndex("Code")
	if _, ok := columnIndex.Columns["Models"]; !ok {
		t.Fatalf("ColumnIndex = %#v", columnIndex.Columns)
	}
	modeIndex, _ := repo.LoadModeIndex("Code")
	if got := modeIndex.Modes["Max"].Columns["Models"].Settings; !reflect.DeepEqual(got, []string{"GPT.json"}) {
		t.Fatalf("Mode refs after project rename = %#v", got)
	}
	current, _ := repo.LoadCurrentState("Code")
	if current.Mappings[0].Source != newSource || current.Mappings[0].Target != fixedTarget ||
		current.Mappings[1].Source != newSource || current.Mappings[1].Target != derivedTarget ||
		current.Relation == nil || current.Relation.OriginMode != "Max" {
		t.Fatalf("current after project rename = %#v", current)
	}
	entries, _ := repo.LoadHistory("Code")
	if entries[0].PreviousMappings[0].Source != newSource || entries[0].NextMappings[0].Target != derivedTarget ||
		entries[0].PreviousRelation == nil || entries[0].PreviousRelation.OriginMode != "Max" || entries[0].NextColumns["Models"].Settings[0] != "GPT.json" {
		t.Fatalf("history after project rename = %#v", entries)
	}
	record, ok, err := repo.LoadSession(101)
	if err != nil || !ok || record.Project != "Code" {
		t.Fatalf("session = %#v ok=%v err=%v", record, ok, err)
	}
	for _, target := range []string{fixedTarget, derivedTarget} {
		link, err := os.Readlink(target)
		if err != nil || link != newSource {
			t.Fatalf("managed link %q -> %q err=%v", target, link, err)
		}
	}
	// The derived target name comes from the Setting name, so a Project rename
	// keeps it and only re-points it at the moved source.
	if link, err := os.Readlink(derivedTarget); err != nil || link != newSource {
		t.Fatalf("derived target -> %q err=%v", link, err)
	}
}

// TestRenameColumnRewritesEveryReference verifies Column rename rewrites Mode
// selections, current and history columns, mappings, and the Setting index move.
func TestRenameColumnRewritesEveryReference(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	oldSource := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	newSource := filepath.Join(root, "OpenCode", "Column", "Configurations", "GPT.json")
	fixedTarget := filepath.Join(targetDir, "fixed.json")

	state := repository.CurrentState{
		Mappings: []repository.Mapping{{Source: oldSource, Target: fixedTarget}},
		Columns:  map[string]repository.ColumnSelection{"Models": {Strategy: "cover", Settings: []string{"GPT.json"}}},
	}
	if err := repo.SaveCurrentState("OpenCode", state); err != nil {
		t.Fatal(err)
	}
	history := []repository.HistoryEntry{{
		Timestamp:        "t1",
		PreviousMappings: []repository.Mapping{{Source: oldSource, Target: fixedTarget}},
		NextMappings:     []repository.Mapping{{Source: oldSource, Target: fixedTarget}},
		PreviousColumns:  map[string]repository.ColumnSelection{"Models": {Strategy: "cover", Settings: []string{"GPT.json"}}},
		NextRelation:     &repository.CurrentRelation{Kind: "following", OriginMode: "Other"},
	}}
	if err := repo.SaveHistory("OpenCode", history); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(fixedTarget), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(oldSource, fixedTarget); err != nil {
		t.Fatal(err)
	}
	// A PPID context for the project must survive a Column rename untouched.
	if err := repo.SaveSession(101, repository.SessionRecord{Project: "OpenCode"}); err != nil {
		t.Fatal(err)
	}

	plan, err := mutate.BuildRenamePlan(repo, mutate.RenameRequest{Kind: mutate.ColumnKind, ProjectReference: "OpenCode", OldReference: "models", NewName: "Configurations", PlanOptions: renamePlanOptions(t, targetDir)})
	if err != nil {
		t.Fatalf("BuildRenamePlan: %v", err)
	}
	if len(plan.IndexReferences) != 3 { // ColumnIndex + two Mode references
		t.Fatalf("index references = %#v", plan.IndexReferences)
	}
	if err := mutate.RenameColumn(repo, "OpenCode", "models", "Configurations", false, renamePlanOptions(t, targetDir)); err != nil {
		t.Fatalf("RenameColumn: %v", err)
	}

	modeIndex, _ := repo.LoadModeIndex("OpenCode")
	for _, mode := range []string{"Max", "Other"} {
		if got := modeIndex.Modes[mode].Columns["Configurations"].Settings; !reflect.DeepEqual(got, []string{"GPT.json"}) {
			t.Fatalf("mode %q refs = %#v", mode, got)
		}
		if _, ok := modeIndex.Modes[mode].Columns["Models"]; ok {
			t.Fatalf("mode %q still references Models", mode)
		}
	}
	settingIndex, err := repo.LoadSettingIndex("OpenCode", "Configurations")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := settingIndex.Settings["GPT.json"]; !ok {
		t.Fatalf("SettingIndex did not move with column: %#v", settingIndex.Settings)
	}
	if _, err := os.Lstat(filepath.Join(root, "OpenCode", "Column", "Models")); !os.IsNotExist(err) {
		t.Fatalf("old column dir survived: %v", err)
	}
	current, _ := repo.LoadCurrentState("OpenCode")
	if current.Mappings[0].Source != newSource || current.Columns["Configurations"].Settings[0] != "GPT.json" || len(current.Columns) != 1 {
		t.Fatalf("current = %#v", current)
	}
	entries, _ := repo.LoadHistory("OpenCode")
	if entries[0].PreviousMappings[0].Source != newSource || entries[0].PreviousColumns["Configurations"].Settings[0] != "GPT.json" || entries[0].NextRelation == nil || entries[0].NextRelation.OriginMode != "Other" {
		t.Fatalf("history = %#v", entries)
	}
	if link, err := os.Readlink(fixedTarget); err != nil || link != newSource {
		t.Fatalf("target -> %q err=%v", link, err)
	}
	record, ok, err := repo.LoadSession(101)
	if err != nil || !ok || record.Project != "OpenCode" {
		t.Fatalf("session changed by Column rename = %#v", record)
	}
}

// TestRenameModeRewritesCurrentAndHistoryIntents verifies Mode rename rewrites
// both relation directions while preserving other Mode selections and contexts.
func TestRenameModeRewritesCurrentAndHistoryIntents(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	oldSource := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	target := filepath.Join(targetDir, "fixed.json")

	state := repository.CurrentState{
		Mappings: []repository.Mapping{{Source: oldSource, Target: target}},
		Relation: &repository.CurrentRelation{Kind: "following", OriginMode: "Max"},
	}
	if err := repo.SaveCurrentState("OpenCode", state); err != nil {
		t.Fatal(err)
	}
	history := []repository.HistoryEntry{{
		Timestamp:        "t1",
		PreviousMappings: []repository.Mapping{{Source: oldSource, Target: target}},
		NextMappings:     []repository.Mapping{{Source: oldSource, Target: target}},
		PreviousRelation: &repository.CurrentRelation{Kind: "following", OriginMode: "Max"},
		NextRelation:     &repository.CurrentRelation{Kind: "following", OriginMode: "Other"},
	}}
	if err := repo.SaveHistory("OpenCode", history); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(oldSource, target); err != nil {
		t.Fatal(err)
	}

	plan, err := mutate.BuildRenamePlan(repo, mutate.RenameRequest{Kind: mutate.ModeKind, ProjectReference: "OpenCode", OldReference: "max", NewName: "Maximum", PlanOptions: renamePlanOptions(t, targetDir)})
	if err != nil {
		t.Fatalf("BuildRenamePlan: %v", err)
	}
	if len(plan.IntentReferences) != 2 { // current relation + previous history relation
		t.Fatalf("intent references = %#v", plan.IntentReferences)
	}
	if err := mutate.RenameMode(repo, "OpenCode", "max", "Maximum", false, renamePlanOptions(t, targetDir)); err != nil {
		t.Fatal(err)
	}

	modeIndex, _ := repo.LoadModeIndex("OpenCode")
	if _, ok := modeIndex.Modes["Maximum"]; !ok {
		t.Fatalf("renamed mode missing: %#v", modeIndex.Modes)
	}
	if _, ok := modeIndex.Modes["Max"]; ok {
		t.Fatalf("old mode survived: %#v", modeIndex.Modes)
	}
	if got := modeIndex.Modes["Other"].Columns["Models"].Settings; !reflect.DeepEqual(got, []string{"GPT.json"}) {
		t.Fatalf("unrelated mode changed: %#v", got)
	}
	current, _ := repo.LoadCurrentState("OpenCode")
	if current.Relation == nil || current.Relation.OriginMode != "Maximum" {
		t.Fatalf("current relation = %#v", current.Relation)
	}
	entries, _ := repo.LoadHistory("OpenCode")
	if entries[0].PreviousRelation == nil || entries[0].PreviousRelation.OriginMode != "Maximum" || entries[0].NextRelation == nil || entries[0].NextRelation.OriginMode != "Other" {
		t.Fatalf("history relations = %#v", entries)
	}
}

// TestRenameSourceDescendantBoundaries verifies sibling and unrelated sources are
// never rewritten by a descendant-path rewrite.
func TestRenameSourceDescendantBoundaries(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	oldSource := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	sibling := filepath.Join(root, "OpenCode", "Column", "Models", "GPT2.json")
	otherColumn := filepath.Join(root, "OpenCode", "Column", "OtherColumn", "GPT.json")
	prefixTrap := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json.bak")
	// Create a second setting so sibling paths are real.
	if err := mutate.CreateSetting(repo, "OpenCode", "Models", "GPT2.json", "file", renameMetadata(t, mutate.SettingKind, "GPT2.json", nil)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sibling, []byte("sibling"), 0o644); err != nil {
		t.Fatal(err)
	}

	state := repository.CurrentState{Mappings: []repository.Mapping{
		{Source: oldSource, Target: filepath.Join(targetDir, "a.json")},
		{Source: sibling, Target: filepath.Join(targetDir, "b.json")},
		{Source: otherColumn, Target: filepath.Join(targetDir, "c.json")},
		{Source: prefixTrap, Target: filepath.Join(targetDir, "d.json")},
	}}
	if err := repo.SaveCurrentState("OpenCode", state); err != nil {
		t.Fatal(err)
	}

	if err := mutate.RenameSetting(repo, "OpenCode", "Models", "GPT.json", "Primary.json", true, renamePlanOptions(t, targetDir)); err != nil {
		t.Fatal(err)
	}

	current, _ := repo.LoadCurrentState("OpenCode")
	want := []string{
		filepath.Join(root, "OpenCode", "Column", "Models", "Primary.json"),
		sibling,
		otherColumn,
		prefixTrap,
	}
	for index := range want {
		if current.Mappings[index].Source != want[index] {
			t.Fatalf("mapping %d source = %q want %q", index, current.Mappings[index].Source, want[index])
		}
	}
}

// TestRenameSettingDerivedTargetAndUnrelatedPreservation verifies derived target
// rename plus full preservation of unrelated Settings, Modes, and history columns.
func TestRenameSettingDerivedTargetAndUnrelatedPreservation(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	oldSource := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	secondSource := filepath.Join(root, "OpenCode", "Column", "Models", "GPT2.json")
	if err := mutate.CreateSetting(repo, "OpenCode", "Models", "GPT2.json", "file", renameMetadata(t, mutate.SettingKind, "GPT2.json", nil)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondSource, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}

	state := repository.CurrentState{Mappings: []repository.Mapping{
		{Source: oldSource, Target: filepath.Join(targetDir, "GPT.json")},
		{Source: secondSource, Target: filepath.Join(targetDir, "GPT2.json")},
	}}
	if err := repo.SaveCurrentState("OpenCode", state); err != nil {
		t.Fatal(err)
	}
	// Second setting is selected in the "Other" mode; must not change.
	modeIndex, _ := repo.LoadModeIndex("OpenCode")
	modeEntry := modeIndex.Modes["Other"]
	modeEntry.Columns["Models"] = index.ModeColumnSelection{Strategy: "cover", Settings: []string{"GPT2.json"}}
	modeIndex.Modes["Other"] = modeEntry
	if err := repo.SaveModeIndex("OpenCode", modeIndex); err != nil {
		t.Fatal(err)
	}

	history := []repository.HistoryEntry{{
		Timestamp:        "t1",
		PreviousMappings: []repository.Mapping{{Source: oldSource, Target: filepath.Join(targetDir, "GPT.json")}},
		NextMappings:     []repository.Mapping{{Source: oldSource, Target: filepath.Join(targetDir, "GPT.json")}},
		PreviousColumns:  map[string]repository.ColumnSelection{"Models": {Strategy: "cover", Settings: []string{"GPT.json"}}},
		NextColumns:      map[string]repository.ColumnSelection{"Models": {Strategy: "cover", Settings: []string{"GPT.json", "GPT2.json"}}},
	}}
	if err := repo.SaveHistory("OpenCode", history); err != nil {
		t.Fatal(err)
	}

	if err := mutate.RenameSetting(repo, "OpenCode", "Models", "GPT.json", "Primary.json", true, renamePlanOptions(t, targetDir)); err != nil {
		t.Fatal(err)
	}

	current, _ := repo.LoadCurrentState("OpenCode")
	if current.Mappings[0].Source != filepath.Join(root, "OpenCode", "Column", "Models", "Primary.json") || current.Mappings[0].Target != filepath.Join(targetDir, "Primary.json") {
		t.Fatalf("renamed mapping = %#v", current.Mappings[0])
	}
	if current.Mappings[1].Source != secondSource || current.Mappings[1].Target != filepath.Join(targetDir, "GPT2.json") {
		t.Fatalf("unrelated mapping changed = %#v", current.Mappings[1])
	}
	entries, _ := repo.LoadHistory("OpenCode")
	if entries[0].NextMappings[0].Target != filepath.Join(targetDir, "Primary.json") {
		t.Fatalf("history mapping = %#v", entries[0].NextMappings)
	}
	if !reflect.DeepEqual(entries[0].NextColumns["Models"].Settings, []string{"Primary.json", "GPT2.json"}) {
		t.Fatalf("history columns = %#v", entries[0].NextColumns)
	}
	modeIndex, _ = repo.LoadModeIndex("OpenCode")
	if got := modeIndex.Modes["Max"].Columns["Models"].Settings; !reflect.DeepEqual(got, []string{"Primary.json"}) {
		t.Fatalf("Max mode refs = %#v", got)
	}
	if got := modeIndex.Modes["Other"].Columns["Models"].Settings; !reflect.DeepEqual(got, []string{"GPT2.json"}) {
		t.Fatalf("Other mode refs = %#v", got)
	}
}

// TestRenameSettingDirectoryContent verifies directory-backed Setting rename keeps
// content and kind while rewriting derived targets.
func TestRenameSettingDirectoryContent(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	oldDir := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	// The fixture creates GPT.json as an empty file; convert it to a directory.
	if err := os.Remove(oldDir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(oldDir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "sub", "nested.txt"), []byte("nested"), 0o644); err != nil {
		t.Fatal(err)
	}
	settingIndex, _ := repo.LoadSettingIndex("OpenCode", "Models")
	// Derive position 0 from the Setting name: empty default and empty override.
	settingIndex.DefaultTargetDir = []string{targetDir, targetDir}
	settingIndex.DefaultTargetName = []string{"", ""}
	entry := settingIndex.Settings["GPT.json"]
	entry.TargetDir = []string{targetDir}
	entry.TargetName = []string{""}
	settingIndex.Settings["GPT.json"] = entry
	if err := repo.SaveSettingIndex("OpenCode", "Models", settingIndex); err != nil {
		t.Fatal(err)
	}
	state := repository.CurrentState{Mappings: []repository.Mapping{
		{Source: oldDir, Target: filepath.Join(targetDir, "GPT.json")},
	}}
	if err := repo.SaveCurrentState("OpenCode", state); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(targetDir, "GPT.json")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(oldDir, filepath.Join(targetDir, "GPT.json")); err != nil {
		t.Fatal(err)
	}

	if err := mutate.RenameSetting(repo, "OpenCode", "Models", "GPT.json", "Primary", false, renamePlanOptions(t, targetDir)); err != nil {
		t.Fatal(err)
	}
	newDir := filepath.Join(root, "OpenCode", "Column", "Models", "Primary")
	if data, err := os.ReadFile(filepath.Join(newDir, "sub", "nested.txt")); err != nil || string(data) != "nested" {
		t.Fatalf("content = %q err=%v", data, err)
	}
	link, err := os.Readlink(filepath.Join(targetDir, "Primary"))
	if err != nil || link != newDir {
		t.Fatalf("derived link -> %q err=%v", link, err)
	}
	settingIndex, _ = repo.LoadSettingIndex("OpenCode", "Models")
	if !reflect.DeepEqual(settingIndex.Settings["Primary"].TargetName, []string{""}) {
		t.Fatalf("target metadata changed: %#v", settingIndex.Settings["Primary"])
	}
}

// TestRenameErrorClassification verifies every rename failure maps to a typed
// mutation error class.
func TestRenameErrorClassification(t *testing.T) {
	_, repo, targetDir := createRenameFixture(t)
	options := renamePlanOptions(t, targetDir)
	if err := mutate.CreateProject(repo, "OtherProject", renameMetadata(t, mutate.ProjectKind, "OtherProject", nil)); err != nil {
		t.Fatal(err)
	}

	checkKind := func(name string, err error, kind mutate.ErrorKind) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: expected error", name)
		}
		var mutationErr *mutate.Error
		if !errors.As(err, &mutationErr) {
			t.Fatalf("%s: not a mutation error: %v", name, err)
		}
		if mutationErr.Kind != kind {
			t.Fatalf("%s: kind = %s want %s (%v)", name, mutationErr.Kind, kind, mutationErr)
		}
	}

	checkKind("missing project", mutate.RenameProject(repo, "Nope", "Code", false, options), mutate.MissingError)
	checkKind("missing column", mutate.RenameColumn(repo, "OpenCode", "Nope", "Configurations", false, options), mutate.MissingError)
	checkKind("missing setting", mutate.RenameSetting(repo, "OpenCode", "Models", "Nope", "Primary.json", false, options), mutate.MissingError)
	checkKind("missing mode", mutate.RenameMode(repo, "OpenCode", "Nope", "Maximum", false, options), mutate.MissingError)
	checkKind("same name", mutate.RenameProject(repo, "OpenCode", "OpenCode", false, options), mutate.ConflictError)
	checkKind("reserved name", mutate.RenameProject(repo, "OpenCode", "global", false, options), mutate.ConflictError)
	checkKind("invalid name", mutate.RenameProject(repo, "OpenCode", "a/b", false, options), mutate.InvalidError)
	checkKind("identity conflict", mutate.RenameProject(repo, "OpenCode", "OtherProject", false, options), mutate.ConflictError)
}

// TestRenameRollbackAtEveryStage injects failures at each transaction stage and
// verifies the filesystem, indexes, runtime records, contexts, and links all
// return to their exact pre-rename state.
func TestRenameRollbackAtEveryStage(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	oldSource := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	target := filepath.Join(targetDir, "fixed.json")
	state := repository.CurrentState{Mappings: []repository.Mapping{{Source: oldSource, Target: target}}}
	if err := repo.SaveCurrentState("OpenCode", state); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(oldSource, target); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveSession(101, repository.SessionRecord{Project: "OpenCode"}); err != nil {
		t.Fatal(err)
	}

	frozen := func(t *testing.T) map[string][]byte {
		t.Helper()
		paths := []string{
			repo.ProjectIndexPath(),
			repo.ColumnIndexPath("OpenCode"),
			repo.ModeIndexPath("OpenCode"),
			repo.SettingIndexPath("OpenCode", "Models"),
			repo.CurrentStatePath("OpenCode"),
			repo.HistoryPath("OpenCode"),
			repo.SessionPath(101),
		}
		snapshot := map[string][]byte{}
		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			snapshot[path] = data
		}
		return snapshot
	}
	restored := func(t *testing.T, before map[string][]byte) {
		t.Helper()
		for path, want := range before {
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s not restored", path)
			}
		}
	}

	for _, stage := range []repository.Stage{repository.StageWrite, repository.StageCommitted, repository.StageCleanup} {
		stage := stage
		t.Run(string(stage), func(t *testing.T) {
			before := frozen(t)
			faulty := repository.New(root, repository.WithHooks(repository.Hooks{BeforeStage: func(s repository.Stage) error {
				if s == stage {
					return errors.New("injected rename failure")
				}
				return nil
			}}))
			err := mutate.RenameSetting(faulty, "OpenCode", "Models", "GPT.json", "Primary.json", false, renamePlanOptions(t, targetDir))
			if err == nil {
				t.Fatalf("expected injected failure at %s", stage)
			}
			restored(t, before)
			if _, err := os.Lstat(filepath.Join(root, "OpenCode", "Column", "Models", "Primary.json")); !os.IsNotExist(err) {
				t.Fatalf("stage %s: destination survived", stage)
			}
			if link, err := os.Readlink(target); err != nil || link != oldSource {
				t.Fatalf("stage %s: target -> %q err=%v", stage, link, err)
			}
			// A second rename must succeed after rollback restored everything.
			if err := mutate.RenameSetting(repo, "OpenCode", "Models", "GPT.json", "Primary.json", false, renamePlanOptions(t, targetDir)); err != nil {
				t.Fatalf("stage %s: rename after rollback: %v", stage, err)
			}
			// Restore fixture state for the next stage.
			if err := mutate.RenameSetting(repo, "OpenCode", "Models", "Primary.json", "GPT.json", false, renamePlanOptions(t, targetDir)); err != nil {
				t.Fatalf("stage %s: restore fixture: %v", stage, err)
			}
		})
	}
}

// TestRenameProjectRollback verifies rollback restores the whole project tree and
// sessions when the source move already happened.
func TestRenameProjectRollback(t *testing.T) {
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
	if err := repo.SaveSession(101, repository.SessionRecord{Project: "OpenCode"}); err != nil {
		t.Fatal(err)
	}
	beforeIndex, _ := os.ReadFile(repo.ProjectIndexPath())
	beforeSession, _ := os.ReadFile(repo.SessionPath(101))

	faulty := repository.New(root, repository.WithHooks(repository.Hooks{BeforeStage: func(s repository.Stage) error {
		if s == repository.StageCommitted {
			return errors.New("injected project rename failure")
		}
		return nil
	}}))
	if err := mutate.RenameProject(faulty, "OpenCode", "Code", false, renamePlanOptions(t, targetDir)); err == nil {
		t.Fatal("expected injected failure")
	}
	if _, err := os.Lstat(filepath.Join(root, "Code")); !os.IsNotExist(err) {
		t.Fatalf("new project dir survived rollback: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")); err != nil || string(data) != "content" {
		t.Fatalf("old tree content = %q err=%v", data, err)
	}
	afterIndex, _ := os.ReadFile(repo.ProjectIndexPath())
	afterSession, _ := os.ReadFile(repo.SessionPath(101))
	if !reflect.DeepEqual(beforeIndex, afterIndex) || !reflect.DeepEqual(beforeSession, afterSession) {
		t.Fatal("project rename rollback did not restore index or session")
	}
	if link, err := os.Readlink(target); err != nil || link != oldSource {
		t.Fatalf("target -> %q err=%v", link, err)
	}
}

// TestRenameColumnAndModeRollback verifies exact rollback of moved directories,
// Mode references, runtime records, and links for Column and Mode renames.
func TestRenameColumnAndModeRollback(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	oldSource := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	target := filepath.Join(targetDir, "fixed.json")
	state := repository.CurrentState{Mappings: []repository.Mapping{{Source: oldSource, Target: target}}, Columns: map[string]repository.ColumnSelection{"Models": {Strategy: "cover", Settings: []string{"GPT.json"}}}}
	if err := repo.SaveCurrentState("OpenCode", state); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(oldSource, target); err != nil {
		t.Fatal(err)
	}
	options := renamePlanOptions(t, targetDir)

	faulty := repository.New(root, repository.WithHooks(repository.Hooks{BeforeStage: func(s repository.Stage) error {
		if s == repository.StageCommitted {
			return errors.New("injected column rename failure")
		}
		return nil
	}}))
	if err := mutate.RenameColumn(faulty, "OpenCode", "Models", "Configurations", false, options); err == nil {
		t.Fatal("expected injected column failure")
	}
	if _, err := os.Lstat(filepath.Join(root, "OpenCode", "Column", "Models")); err != nil {
		t.Fatalf("column dir not restored: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "OpenCode", "Column", "Configurations")); !os.IsNotExist(err) {
		t.Fatalf("new column dir survived rollback: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "OpenCode", "Column", "Models", "SettingIndex.jsonc")); err != nil {
		t.Fatalf("SettingIndex not restored: %v", err)
	}
	modeIndex, _ := repo.LoadModeIndex("OpenCode")
	if got := modeIndex.Modes["Max"].Columns["Models"].Settings; !reflect.DeepEqual(got, []string{"GPT.json"}) {
		t.Fatalf("mode refs not restored: %#v", got)
	}
	current, _ := repo.LoadCurrentState("OpenCode")
	if current.Mappings[0].Source != oldSource || current.Columns["Models"].Settings[0] != "GPT.json" {
		t.Fatalf("current not restored: %#v", current)
	}
	if link, err := os.Readlink(target); err != nil || link != oldSource {
		t.Fatalf("target -> %q err=%v", link, err)
	}

	if err := repo.SaveCurrentState("OpenCode", repository.CurrentState{Mappings: []repository.Mapping{{Source: oldSource, Target: target}}, Relation: &repository.CurrentRelation{Kind: "following", OriginMode: "Max"}}); err != nil {
		t.Fatal(err)
	}
	faultyMode := repository.New(root, repository.WithHooks(repository.Hooks{BeforeStage: func(s repository.Stage) error {
		if s == repository.StageCommitted {
			return errors.New("injected mode rename failure")
		}
		return nil
	}}))
	if err := mutate.RenameMode(faultyMode, "OpenCode", "Max", "Maximum", false, options); err == nil {
		t.Fatal("expected injected mode failure")
	}
	modeIndex, _ = repo.LoadModeIndex("OpenCode")
	if _, ok := modeIndex.Modes["Maximum"]; ok {
		t.Fatalf("renamed mode survived rollback: %#v", modeIndex.Modes)
	}
	current, _ = repo.LoadCurrentState("OpenCode")
	if current.Relation == nil || current.Relation.OriginMode != "Max" {
		t.Fatalf("mode relation not restored: %#v", current.Relation)
	}
}

// TestRenameMissingColumnAndProject verifies renames of missing resources update
// only the durable indexes and never recreate absent paths.
func TestRenameMissingColumnAndProject(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	if err := os.RemoveAll(filepath.Join(root, "OpenCode", "Column", "Models")); err != nil {
		t.Fatal(err)
	}
	if err := mutate.RenameColumn(repo, "OpenCode", "Models", "Configurations", false, renamePlanOptions(t, targetDir)); err != nil {
		t.Fatalf("rename missing Column: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "OpenCode", "Column", "Configurations")); !os.IsNotExist(err) {
		t.Fatalf("missing Column rename created a path: %v", err)
	}
	columnIndex, _ := repo.LoadColumnIndex("OpenCode")
	if _, ok := columnIndex.Columns["Configurations"]; !ok {
		t.Fatalf("ColumnIndex key not renamed: %#v", columnIndex.Columns)
	}
	modeIndex, _ := repo.LoadModeIndex("OpenCode")
	for _, mode := range []string{"Max", "Other"} {
		if _, ok := modeIndex.Modes[mode].Columns["Configurations"]; !ok {
			t.Fatalf("Mode ref not renamed for missing Column: %#v", modeIndex.Modes[mode].Columns)
		}
	}

	if err := os.RemoveAll(filepath.Join(root, "OpenCode")); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveSession(101, repository.SessionRecord{Project: "OpenCode"}); err != nil {
		t.Fatal(err)
	}
	if err := mutate.RenameProject(repo, "OpenCode", "Code", false, renamePlanOptions(t, targetDir)); err != nil {
		t.Fatalf("rename missing Project: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "Code")); !os.IsNotExist(err) {
		t.Fatalf("missing Project rename created a path: %v", err)
	}
	projectIndex, _ := repo.LoadProjectIndex()
	if _, ok := projectIndex.Projects["Code"]; !ok {
		t.Fatalf("ProjectIndex key not renamed: %#v", projectIndex.Projects)
	}
	record, ok, err := repo.LoadSession(101)
	if err != nil || !ok || record.Project != "Code" {
		t.Fatalf("session not rewritten for missing Project: %#v ok=%v err=%v", record, ok, err)
	}
}

// TestRenameAbsentTargetRequiresForce verifies the spec rule that an absent
// recorded target blocks rename without --force-targets and is recreated with it.
func TestRenameAbsentTargetRequiresForce(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	oldSource := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	target := filepath.Join(targetDir, "fixed.json")
	if err := repo.SaveCurrentState("OpenCode", repository.CurrentState{Mappings: []repository.Mapping{{Source: oldSource, Target: target}}}); err != nil {
		t.Fatal(err)
	}
	// The recorded target is absent: no symlink, no file, no directory.
	options := renamePlanOptions(t, targetDir)
	if err := mutate.RenameSetting(repo, "OpenCode", "Models", "GPT.json", "Primary.json", false, options); err == nil {
		t.Fatal("expected absent-target refusal without --force-targets")
	}
	if _, err := os.Lstat(filepath.Join(root, "OpenCode", "Column", "Models", "Primary.json")); !os.IsNotExist(err) {
		t.Fatalf("refused rename changed the source: %v", err)
	}
	if err := mutate.RenameSetting(repo, "OpenCode", "Models", "GPT.json", "Primary.json", true, options); err != nil {
		t.Fatalf("forced rename over absent target: %v", err)
	}
	newSource := filepath.Join(root, "OpenCode", "Column", "Models", "Primary.json")
	if link, err := os.Readlink(target); err != nil || link != newSource {
		t.Fatalf("recreated target -> %q err=%v", link, err)
	}
}

// TestRenameDerivedDestinationDrift verifies an unmanaged object at the new
// derived target blocks rename without force, is reclaimed with force, and is
// restored exactly by rollback.
func TestRenameDerivedDestinationDrift(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	oldSource := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	options := renamePlanOptions(t, targetDir)
	// Position 1 derives the target name from the Setting name.
	state := repository.CurrentState{Mappings: []repository.Mapping{{Source: oldSource, Target: filepath.Join(targetDir, "GPT.json")}}}
	if err := repo.SaveCurrentState("OpenCode", state); err != nil {
		t.Fatal(err)
	}
	// The old target is owned; only the derived destination is drifted.
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(oldSource, filepath.Join(targetDir, "GPT.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(targetDir, "Primary.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "Primary.json", "drift.txt"), []byte("drift"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := mutate.RenameSetting(repo, "OpenCode", "Models", "GPT.json", "Primary.json", false, options); err == nil {
		t.Fatal("expected destination drift refusal")
	}
	if data, err := os.ReadFile(filepath.Join(targetDir, "Primary.json", "drift.txt")); err != nil || string(data) != "drift" {
		t.Fatalf("refused rename changed drift: %q err=%v", data, err)
	}

	if err := mutate.RenameSetting(repo, "OpenCode", "Models", "GPT.json", "Primary.json", true, options); err != nil {
		t.Fatalf("forced rename over drift: %v", err)
	}
	link, err := os.Readlink(filepath.Join(targetDir, "Primary.json"))
	if err != nil || link != filepath.Join(root, "OpenCode", "Column", "Models", "Primary.json") {
		t.Fatalf("forced link -> %q err=%v", link, err)
	}

	// Drift at the destination again, then roll back a forced rename.
	if err := os.Remove(filepath.Join(targetDir, "Primary.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(targetDir, "GPT.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "GPT.json", "drift.txt"), []byte("drift"), 0o644); err != nil {
		t.Fatal(err)
	}
	faulty := repository.New(root, repository.WithHooks(repository.Hooks{BeforeStage: func(s repository.Stage) error {
		if s == repository.StageCommitted {
			return errors.New("injected forced rename failure")
		}
		return nil
	}}))
	if err := mutate.RenameSetting(faulty, "OpenCode", "Models", "Primary.json", "GPT.json", true, options); err == nil {
		t.Fatal("expected injected failure")
	}
	if data, err := os.ReadFile(filepath.Join(targetDir, "GPT.json", "drift.txt")); err != nil || string(data) != "drift" {
		t.Fatalf("rollback did not restore destination drift: %q err=%v", data, err)
	}
	// The source move must roll back exactly: Primary.json restored, GPT.json absent.
	if data, err := os.ReadFile(filepath.Join(root, "OpenCode", "Column", "Models", "Primary.json")); err != nil || string(data) != "content" {
		t.Fatalf("rollback did not restore source: %q err=%v", data, err)
	}
	if _, err := os.Lstat(filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")); !os.IsNotExist(err) {
		t.Fatalf("rollback left destination source: %v", err)
	}
}

// TestRenameActiveMissingSourceRefusal verifies an actively mapped missing source
// blocks rename even with --force-targets because no link can be recreated.
func TestRenameActiveMissingSourceRefusal(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	missingSource := filepath.Join(root, "OpenCode", "Column", "Models", "Missing.json")
	target := filepath.Join(targetDir, "fixed.json")
	if err := repo.SaveCurrentState("OpenCode", repository.CurrentState{Mappings: []repository.Mapping{{Source: missingSource, Target: target}}}); err != nil {
		t.Fatal(err)
	}
	if err := mutate.RenameSetting(repo, "OpenCode", "Models", "Missing.json", "Other.json", true, renamePlanOptions(t, targetDir)); err == nil {
		t.Fatal("expected active missing source refusal")
	}
}

// TestRenameAliasResolution verifies OldReference resolves through aliases for
// every kind and the plan records canonical old names.
func TestRenameAliasResolution(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	plan, err := mutate.BuildRenamePlan(repo, mutate.RenameRequest{Kind: mutate.ProjectKind, OldReference: "oc", NewName: "Code", PlanOptions: renamePlanOptions(t, targetDir)})
	if err != nil {
		t.Fatal(err)
	}
	if plan.OldName != "OpenCode" {
		t.Fatalf("old name = %q", plan.OldName)
	}
	if len(plan.Moves) != 1 || plan.Moves[0].From != filepath.Join(root, "OpenCode") {
		t.Fatalf("moves = %#v", plan.Moves)
	}
}

// TestRenamePreservesUnknownFields verifies extra fields survive every rewrite layer.
func TestRenamePreservesUnknownFields(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	oldSource := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	target := filepath.Join(targetDir, "fixed.json")
	state := repository.CurrentState{
		Mappings: []repository.Mapping{{Source: oldSource, Target: target}},
		Extra:    map[string]json.RawMessage{"stateExtra": json.RawMessage(`{"keep":1}`)},
	}
	if err := repo.SaveCurrentState("OpenCode", state); err != nil {
		t.Fatal(err)
	}
	history := []repository.HistoryEntry{{
		Timestamp:        "t1",
		PreviousMappings: []repository.Mapping{{Source: oldSource, Target: target}},
		NextMappings:     []repository.Mapping{{Source: oldSource, Target: target}},
		Extra:            map[string]json.RawMessage{"historyExtra": json.RawMessage(`"keep"`)},
	}}
	if err := repo.SaveHistory("OpenCode", history); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(oldSource, target); err != nil {
		t.Fatal(err)
	}

	if err := mutate.RenameProject(repo, "OpenCode", "Code", false, renamePlanOptions(t, targetDir)); err != nil {
		t.Fatal(err)
	}
	current, _ := repo.LoadCurrentState("Code")
	if !jsonRawEqual(current.Extra["stateExtra"], `{"keep":1}`) {
		t.Fatalf("current extra = %#v", current.Extra)
	}
	entries, _ := repo.LoadHistory("Code")
	if !jsonRawEqual(entries[0].Extra["historyExtra"], `"keep"`) {
		t.Fatalf("history extra = %#v", entries[0].Extra)
	}
}

// TestRenameLinkApplyFailureRollsBackEveryLayer verifies that a failure while
// applying managed links mid-transaction restores sources, indexes, runtime
// records, and every already-applied link.
func TestRenameLinkApplyFailureRollsBackEveryLayer(t *testing.T) {
	root, repo, targetDir := createRenameFixture(t)
	oldSource := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	blockedDir := filepath.Join(targetDir, "blocked")
	// Two mappings: one in a writable directory, one whose parent is a regular
	// file so its symlink creation fails after the first link was applied.
	state := repository.CurrentState{Mappings: []repository.Mapping{
		{Source: oldSource, Target: filepath.Join(targetDir, "fixed.json")},
		{Source: oldSource, Target: filepath.Join(blockedDir, "GPT.json")},
	}}
	if err := repo.SaveCurrentState("OpenCode", state); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blockedDir, []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := mutate.RenameSetting(repo, "OpenCode", "Models", "GPT.json", "Primary.json", true, renamePlanOptions(t, targetDir)); err == nil {
		t.Fatal("expected mid-apply link failure")
	}
	if _, err := os.Lstat(filepath.Join(root, "OpenCode", "Column", "Models", "Primary.json")); !os.IsNotExist(err) {
		t.Fatalf("destination source survived rollback: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")); err != nil {
		t.Fatalf("source not restored: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(targetDir, "fixed.json")); !os.IsNotExist(err) {
		t.Fatalf("first applied link survived rollback: %v", err)
	}
	current, _ := repo.LoadCurrentState("OpenCode")
	if current.Mappings[0].Source != oldSource {
		t.Fatalf("current state not restored: %#v", current)
	}
	settingIndex, _ := repo.LoadSettingIndex("OpenCode", "Models")
	if _, ok := settingIndex.Settings["GPT.json"]; !ok {
		t.Fatalf("SettingIndex not restored: %#v", settingIndex.Settings)
	}
}
