package mutate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/planner"
	"github.com/xenon/ConfigFacilitator/internal/repository"
)

// TestTargetPositionRoundTrip verifies logical positions preserve fixed and inherited components.
func TestTargetPositionRoundTrip(t *testing.T) {
	settingIndex := index.SettingIndex{TargetNumber: 2, DefaultTargetDir: []string{"/tmp/a", "/tmp/b"}, DefaultTargetName: []string{"fixed.json", ""}, Settings: map[string]index.SettingEntry{"S": {TargetDir: []string{"", "/tmp/override"}, TargetName: []string{"", "name.json"}}}}
	columnPositions, err := ColumnTargetPositions(settingIndex)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(columnPositions, []TargetPosition{{Dir: "/tmp/a", Name: "fixed.json", DirMode: "fixed", NameMode: "fixed"}, {Dir: "/tmp/b", Name: "", DirMode: "fixed", NameMode: "setting"}}) {
		t.Fatalf("column positions = %#v", columnPositions)
	}
	settingPositions, err := SettingTargetPositions(settingIndex, settingIndex.Settings["S"])
	if err != nil {
		t.Fatal(err)
	}
	if settingPositions[0].DirMode != "inherit" || settingPositions[0].NameMode != "inherit" || settingPositions[1].DirMode != "explicit" || settingPositions[1].NameMode != "explicit" {
		t.Fatalf("setting positions = %#v", settingPositions)
	}
}

// TestTargetMutationsKeepArraysAndExtrasInLockstep verifies add, middle delete, and unknown fields.
func TestTargetMutationsKeepArraysAndExtrasInLockstep(t *testing.T) {
	root := t.TempDir()
	repo := repository.New(root)
	metadata, err := NewMetadata(ProjectKind, "P", "P", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := CreateProject(repo, "P", metadata); err != nil {
		t.Fatal(err)
	}
	columnMetadata, err := NewMetadata(ColumnKind, "C", "C", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := CreateColumn(repo, "P", "C", columnMetadata); err != nil {
		t.Fatal(err)
	}
	settingMetadata, err := NewMetadata(SettingKind, "S", "S", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := CreateSetting(repo, "P", "C", "S", "file", settingMetadata); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "P", "Column", "C", "SettingIndex.jsonc")
	settingIndex, err := repo.LoadSettingIndex("P", "C")
	if err != nil {
		t.Fatal(err)
	}
	settingIndex.Extra["keep"] = json.RawMessage(`true`)
	if err := repo.SaveSettingIndex("P", "C", settingIndex); err != nil {
		t.Fatal(err)
	}
	options := planner.PlanOptions{HomeDir: root, Env: map[string]string{}, OS: "linux"}
	if _, _, err := AddColumnTarget(repo, "P", "C", TargetPosition{Dir: "/tmp/one", Name: "one.json"}, options); err != nil {
		t.Fatal(err)
	}
	if _, _, err := AddColumnTarget(repo, "P", "C", TargetPosition{Dir: "/tmp/two", DirMode: "fixed", NameMode: "setting"}, options); err != nil {
		t.Fatal(err)
	}
	if _, _, err := AddColumnTarget(repo, "P", "C", TargetPosition{Dir: "/tmp/three", Name: "three.json"}, options); err != nil {
		t.Fatal(err)
	}
	name := "override.json"
	if _, err := SetSettingTarget(repo, "P", "C", "S", 1, nil, true, &name, false, options); err != nil {
		t.Fatal(err)
	}
	if err := DeleteColumnTarget(repo, "P", "C", 1, true, options); err != nil {
		t.Fatal(err)
	}
	updated, err := repo.LoadSettingIndex("P", "C")
	if err != nil {
		t.Fatal(err)
	}
	if updated.TargetNumber != 2 || !reflect.DeepEqual(updated.DefaultTargetDir, []string{"/tmp/one", "/tmp/three"}) || !reflect.DeepEqual(updated.DefaultTargetName, []string{"one.json", "three.json"}) {
		t.Fatalf("defaults = %#v", updated)
	}
	entry := updated.Settings["S"]
	if !reflect.DeepEqual(entry.TargetDir, []string{"", ""}) || !reflect.DeepEqual(entry.TargetName, []string{"", ""}) {
		t.Fatalf("override = %#v", entry)
	}
	if string(updated.Extra["keep"]) != "true" {
		t.Fatalf("extra lost: %#v", updated.Extra)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

// TestTargetMutationsRejectInvalidIndicesAndConfirmation verifies no mutation on invalid destructive input.
func TestTargetMutationsRejectInvalidIndicesAndConfirmation(t *testing.T) {
	root := t.TempDir()
	repo := repository.New(root)
	metadata, _ := NewMetadata(ProjectKind, "P", "P", "", nil)
	if err := CreateProject(repo, "P", metadata); err != nil {
		t.Fatal(err)
	}
	columnMetadata, _ := NewMetadata(ColumnKind, "C", "C", "", nil)
	if err := CreateColumn(repo, "P", "C", columnMetadata); err != nil {
		t.Fatal(err)
	}
	options := planner.PlanOptions{HomeDir: root, Env: map[string]string{}, OS: "linux"}
	if _, _, err := AddColumnTarget(repo, "P", "C", TargetPosition{Dir: "/tmp/a", Name: "a"}, options); err != nil {
		t.Fatal(err)
	}
	if err := DeleteColumnTarget(repo, "P", "C", 0, false, options); err == nil {
		t.Fatal("expected confirmation error")
	}
	if _, err := SetColumnTarget(repo, "P", "C", -1, nil, false, nil, true, options); err == nil {
		t.Fatal("expected negative index error")
	}
	if _, err := SetColumnTarget(repo, "P", "C", 1, nil, false, nil, true, options); err == nil {
		t.Fatal("expected out-of-range error")
	}
}
