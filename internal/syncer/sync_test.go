package syncer

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/index"
)

func TestNormalizeTargetArray(t *testing.T) {
	tests := []struct {
		name         string
		values       []string
		targetNumber int
		want         []string
	}{
		{name: "truncates surplus values", values: []string{"a", "b", "c"}, targetNumber: 2, want: []string{"a", "b"}},
		{name: "broadcasts uniform values", values: []string{"a", "a"}, targetNumber: 4, want: []string{"a", "a", "a", "a"}},
		{name: "fills varied values", values: []string{"a", "b"}, targetNumber: 4, want: []string{"a", "b", "", ""}},
		{name: "fills empty values", values: nil, targetNumber: 2, want: []string{"", ""}},
		{name: "supports zero target number", values: []string{"a"}, targetNumber: 0, want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeTargetArray(tt.values, tt.targetNumber); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("normalizeTargetArray(%#v, %d) = %#v, want %#v", tt.values, tt.targetNumber, got, tt.want)
			}
		})
	}
}

func TestSyncNormalizesDefaultsAndNewSettingTargets(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "OpenCode")
	columnRoot := filepath.Join(projectRoot, "Column", "Skills")
	for _, directory := range []string{columnRoot, filepath.Join(projectRoot, "Mode"), filepath.Join(projectRoot, "Backup")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatalf("create %s: %v", directory, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ProjectIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write project index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write column index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(columnRoot, "SettingIndex.jsonc"), []byte(`{
  "targetNumber": 3,
  "defaultTargetDir": ["~/.config/skills"],
  "defaultTargetName": ["first", "second"]
}`), 0o644); err != nil {
		t.Fatalf("write setting index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(columnRoot, "NewSkill"), []byte("setting"), 0o644); err != nil {
		t.Fatalf("write setting source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Mode", "ModeIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write mode index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "current_state.json"), []byte("{\"mappings\": []}\n"), 0o644); err != nil {
		t.Fatalf("write current state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "history.log"), nil, 0o644); err != nil {
		t.Fatalf("write history log: %v", err)
	}

	if err := SyncAll(root); err != nil {
		t.Fatalf("sync warehouse: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(columnRoot, "SettingIndex.jsonc"))
	if err != nil {
		t.Fatalf("read synced setting index: %v", err)
	}
	settingIndex, err := index.ParseSettingIndex(data)
	if err != nil {
		t.Fatalf("parse synced setting index: %v", err)
	}
	if !reflect.DeepEqual(settingIndex.DefaultTargetDir, []string{"~/.config/skills", "~/.config/skills", "~/.config/skills"}) {
		t.Fatalf("unexpected defaultTargetDir: %#v", settingIndex.DefaultTargetDir)
	}
	if !reflect.DeepEqual(settingIndex.DefaultTargetName, []string{"first", "second", ""}) {
		t.Fatalf("unexpected defaultTargetName: %#v", settingIndex.DefaultTargetName)
	}
	newSetting := settingIndex.Settings["NewSkill"]
	if !reflect.DeepEqual(newSetting.TargetDir, []string{"", "", ""}) || !reflect.DeepEqual(newSetting.TargetName, []string{"", "", ""}) {
		t.Fatalf("unexpected new setting targets: %#v", newSetting)
	}
}

func TestSyncRemovesDeletedColumnDirectory(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "OpenCode")
	keepColumnRoot := filepath.Join(projectRoot, "Column", "KeepColumn")
	deleteColumnRoot := filepath.Join(projectRoot, "Column", "DeleteColumn")
	for _, directory := range []string{keepColumnRoot, deleteColumnRoot, filepath.Join(projectRoot, "Mode"), filepath.Join(projectRoot, "Backup")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatalf("create %s: %v", directory, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ProjectIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write project index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte(`{
  "KeepColumn": {"displayName": "Keep Display", "description": "keep metadata", "aliases": ["keep"]},
  "DeleteColumn": {"displayName": "Delete Display", "description": "delete metadata", "aliases": ["gone"]}
}`), 0o644); err != nil {
		t.Fatalf("write column index: %v", err)
	}
	for _, columnRoot := range []string{keepColumnRoot, deleteColumnRoot} {
		if err := os.WriteFile(filepath.Join(columnRoot, "SettingIndex.jsonc"), []byte("{\"targetNumber\": 1}\n"), 0o644); err != nil {
			t.Fatalf("write setting index: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Mode", "ModeIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write mode index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "current_state.json"), []byte("{\"mappings\": []}\n"), 0o644); err != nil {
		t.Fatalf("write current state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "history.log"), nil, 0o644); err != nil {
		t.Fatalf("write history log: %v", err)
	}

	if err := SyncAll(root); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	if err := os.RemoveAll(deleteColumnRoot); err != nil {
		t.Fatalf("remove deleted column directory: %v", err)
	}
	if err := SyncAll(root); err != nil {
		t.Fatalf("sync after deletion: %v", err)
	}

	if _, err := os.Stat(deleteColumnRoot); !os.IsNotExist(err) {
		t.Fatalf("deleted column directory was recreated: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"))
	if err != nil {
		t.Fatalf("read synced column index: %v", err)
	}
	columnIndex, err := index.ParseColumnIndex(data)
	if err != nil {
		t.Fatalf("parse synced column index: %v", err)
	}
	if _, ok := columnIndex.Columns["DeleteColumn"]; ok {
		t.Fatalf("column index retained deleted column: %#v", columnIndex.Columns)
	}
	if bytes.Contains(data, []byte("delete metadata")) {
		t.Fatalf("column index retained deleted column metadata: %q", string(data))
	}
	keep := columnIndex.Columns["KeepColumn"]
	if keep.Description != "keep metadata" || keep.DisplayName != "Keep Display" || len(keep.Aliases) != 1 || keep.Aliases[0] != "keep" {
		t.Fatalf("extant column lost authored metadata: %#v", keep)
	}
}
