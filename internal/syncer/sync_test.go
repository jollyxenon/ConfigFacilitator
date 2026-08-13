package syncer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/planner"
	"github.com/xenon/ConfigFacilitator/internal/repository"
)

// TestNormalizeTargetArray covers target-array normalization during synchronization.
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
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeTargetArray(test.values, test.targetNumber); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("normalizeTargetArray(%#v, %d) = %#v, want %#v", test.values, test.targetNumber, got, test.want)
			}
		})
	}
}

// TestSyncNormalizesDefaultsAndNewSettingTargets verifies discovery and target inheritance.
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
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(columnRoot, "SettingIndex.jsonc"), []byte(`{
  "targetNumber": 3,
  "defaultTargetDir": ["~/.config/skills"],
  "defaultTargetName": ["first", "second"]
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(columnRoot, "NewSkill"), []byte("setting"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProjectRuntime(t, projectRoot)
	if err := SyncAll(root, testPlanOptions()); err != nil {
		t.Fatalf("sync warehouse: %v", err)
	}
	settingIndex, err := repository.New(root).LoadSettingIndex("OpenCode", "Skills")
	if err != nil {
		t.Fatal(err)
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

// TestSyncDeletesDisappearedResources verifies sync removes index entries for
// disappeared file Settings, directory Settings, and Columns without recreating
// their source paths, and that external restoration rediscovers them.
func TestSyncDeletesDisappearedResources(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "OpenCode")
	keepColumnRoot := filepath.Join(projectRoot, "Column", "KeepColumn")
	missingColumnRoot := filepath.Join(projectRoot, "Column", "MissingColumn")
	missingSettingPath := filepath.Join(keepColumnRoot, "MissingSetting")
	missingDirSettingPath := filepath.Join(keepColumnRoot, "MissingDirSetting")
	for _, directory := range []string{keepColumnRoot, missingColumnRoot, missingDirSettingPath, filepath.Join(projectRoot, "Mode"), filepath.Join(projectRoot, "Backup")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	projectExtra := `{"keep":"project"}`
	columnExtra := `{"keep":"column"}`
	settingExtra := `{"keep":"setting"}`
	if err := os.WriteFile(filepath.Join(root, "ProjectIndex.jsonc"), []byte(`{
  "OpenCode":{"description":"project description","aliases":["oc"],"extension":`+projectExtra+`}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte(`{
  "KeepColumn":{"description":"keep metadata","aliases":["keep"],"extension":`+columnExtra+`},
  "MissingColumn":{"description":"missing column","aliases":["gone"]}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keepColumnRoot, "SettingIndex.jsonc"), []byte(`{
  "description":"targets","targetNumber":1,"defaultTargetDir":["/targets"],"defaultTargetName":["fixed"],
  "settings":{"Present":{"description":"present description","targetDir":["custom"],"targetName":["name"],"extension":`+settingExtra+`},"MissingSetting":{"description":"missing setting","aliases":["ms"],"targetDir":["custom"],"targetName":["name"],"extension":`+settingExtra+`},"MissingDirSetting":{"description":"missing dir setting","aliases":["mds"]}}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keepColumnRoot, "Present"), []byte("present"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keepColumnRoot, "MissingSetting"), []byte("missing"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(missingDirSettingPath, "placeholder"), []byte("dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(missingColumnRoot, "SettingIndex.jsonc"), []byte(`{"targetNumber":0,"settings":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProjectRuntime(t, projectRoot)

	if err := SyncAll(root, testPlanOptions()); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	repo := repository.New(root)
	columnIndex, err := repo.LoadColumnIndex("OpenCode")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := columnIndex.Columns["MissingColumn"]; !ok {
		t.Fatalf("initial sync lost indexed Column: %#v", columnIndex.Columns)
	}
	settingIndex, err := repo.LoadSettingIndex("OpenCode", "KeepColumn")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"MissingSetting", "MissingDirSetting"} {
		if _, ok := settingIndex.Settings[name]; !ok {
			t.Fatalf("initial sync lost indexed Setting %q: %#v", name, settingIndex.Settings)
		}
	}

	if err := os.RemoveAll(missingColumnRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(missingSettingPath); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(missingDirSettingPath); err != nil {
		t.Fatal(err)
	}
	if err := SyncAll(root, testPlanOptions()); err != nil {
		t.Fatalf("removal sync: %v", err)
	}
	columnIndex, _ = repo.LoadColumnIndex("OpenCode")
	if _, ok := columnIndex.Columns["MissingColumn"]; ok {
		t.Fatalf("disappeared Column retained: %#v", columnIndex.Columns)
	}
	keep := columnIndex.Columns["KeepColumn"]
	if keep.Description != "keep metadata" || !reflect.DeepEqual(keep.Aliases, []string{"keep"}) || string(keep.Extra["extension"]) != columnExtra {
		t.Fatalf("sync lost retained Column metadata: %#v", keep)
	}
	settingIndex, _ = repo.LoadSettingIndex("OpenCode", "KeepColumn")
	if _, ok := settingIndex.Settings["MissingSetting"]; ok {
		t.Fatalf("disappeared file Setting retained: %#v", settingIndex.Settings)
	}
	if _, ok := settingIndex.Settings["MissingDirSetting"]; ok {
		t.Fatalf("disappeared directory Setting retained: %#v", settingIndex.Settings)
	}
	present := settingIndex.Settings["Present"]
	if present.Description != "present description" || !reflect.DeepEqual(present.TargetDir, []string{"custom"}) || string(present.Extra["extension"]) != settingExtra {
		t.Fatalf("sync lost retained Setting metadata: %#v", present)
	}
	for _, path := range []string{missingColumnRoot, missingSettingPath, missingDirSettingPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("sync recreated disappeared path %s: %v", path, err)
		}
	}

	if err := os.MkdirAll(missingColumnRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(missingColumnRoot, "SettingIndex.jsonc"), []byte(`{"targetNumber":0,"settings":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(missingSettingPath, []byte("restored"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(missingDirSettingPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SyncAll(root, testPlanOptions()); err != nil {
		t.Fatalf("restoration sync: %v", err)
	}
	columnIndex, _ = repo.LoadColumnIndex("OpenCode")
	settingIndex, _ = repo.LoadSettingIndex("OpenCode", "KeepColumn")
	if _, ok := columnIndex.Columns["MissingColumn"]; !ok {
		t.Fatal("restored Column was not rediscovered")
	}
	for _, name := range []string{"MissingSetting", "MissingDirSetting"} {
		if rawBool(settingIndex.Settings[name].Extra["missing"]) {
			t.Fatalf("restored Setting %q carries stale missing marker", name)
		}
	}
	projectIndex, _ := repo.LoadProjectIndex()
	project := projectIndex.Projects["OpenCode"]
	if project.Description != "project description" || string(project.Extra["extension"]) != projectExtra {
		t.Fatalf("sync lost authored Project metadata: %#v", project)
	}
}

// TestSyncAllDiscoversRootSettingWarehouse verifies that the legacy-looking name is ordinary.
func TestSyncAllDiscoversRootSettingWarehouse(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "SettingWarehouse")
	columnRoot := filepath.Join(projectRoot, "Column", "Legacy")
	if err := os.MkdirAll(columnRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(columnRoot, "SettingIndex.jsonc"), []byte(`{"targetNumber":0,"settings":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProjectRuntime(t, projectRoot)
	if err := SyncAll(root, testPlanOptions()); err != nil {
		t.Fatal(err)
	}
	projectIndex, err := repository.New(root).LoadProjectIndex()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := projectIndex.Projects["SettingWarehouse"]; !ok {
		t.Fatalf("SettingWarehouse not discovered: %#v", projectIndex.Projects)
	}
}

// writeProjectRuntime creates present Project-owned indexes and runtime files for sync tests.
func writeProjectRuntime(t *testing.T, projectRoot string) {
	t.Helper()
	for _, directory := range []string{filepath.Join(projectRoot, "Mode"), filepath.Join(projectRoot, "Backup")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, data := range map[string][]byte{
		filepath.Join(projectRoot, "Mode", "ModeIndex.jsonc"):      []byte(`{"Max":{"columns":{}}}`),
		filepath.Join(projectRoot, "Backup", "current_state.json"): []byte(`{"columns":{},"mappings":[]}`),
		filepath.Join(projectRoot, "Backup", "history.log"):        nil,
	} {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// rawBool decodes one durable boolean extension field for assertions.
func rawBool(raw json.RawMessage) bool {
	var value bool
	_ = json.Unmarshal(raw, &value)
	return value
}

// TestProjectSyncDoesNotTouchOtherProjects verifies Project-scoped sync removes
// disappeared resources only inside the selected Project and never removes
// out-of-scope missing Projects.
func TestProjectSyncDoesNotTouchOtherProjects(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{
		filepath.Join(root, "OpenCode", "Column", "Keep"),
		filepath.Join(root, "OpenCode", "Mode"),
		filepath.Join(root, "OpenCode", "Backup"),
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ProjectIndex.jsonc"), []byte(`{
  "OpenCode": {"description": "present"},
  "GoneProject": {"description": "missing elsewhere"}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "OpenCode", "Column", "ColumnIndex.jsonc"), []byte(`{"Keep":{"description":"keep"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "OpenCode", "Column", "Keep", "SettingIndex.jsonc"), []byte(`{"targetNumber":0,"settings":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "OpenCode", "Mode", "ModeIndex.jsonc"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SyncProject(root, "OpenCode", testPlanOptions()); err != nil {
		t.Fatal(err)
	}
	projectIndex, err := repository.LoadProjectIndex(filepath.Join(root, "ProjectIndex.jsonc"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := projectIndex.Projects["OpenCode"]; !ok {
		t.Fatal("selected project was removed")
	}
	if _, ok := projectIndex.Projects["GoneProject"]; !ok {
		t.Fatal("Project-scoped sync removed an out-of-scope missing project")
	}

	if err := SyncAll(root, testPlanOptions()); err != nil {
		t.Fatal(err)
	}
	projectIndex, _ = repository.LoadProjectIndex(filepath.Join(root, "ProjectIndex.jsonc"))
	if _, ok := projectIndex.Projects["GoneProject"]; ok {
		t.Fatal("warehouse-wide sync kept a missing project")
	}
	if _, ok := projectIndex.Projects["OpenCode"]; !ok {
		t.Fatal("warehouse-wide sync removed a present project")
	}
}

// TestSyncAllRemovesMissingProjects verifies warehouse-wide sync removes every
// indexed Project whose source directory no longer exists.
func TestSyncAllRemovesMissingProjects(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "OpenCode", "Backup"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "OpenCode", "Mode"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ProjectIndex.jsonc"), []byte(`{
  "OpenCode": {"description": "present"},
  "GoneProject": {"description": "missing"}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "OpenCode", "Column", "Keep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "OpenCode", "Column", "ColumnIndex.jsonc"), []byte(`{"Keep":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "OpenCode", "Column", "Keep", "SettingIndex.jsonc"), []byte(`{"targetNumber":0,"settings":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "OpenCode", "Mode", "ModeIndex.jsonc"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SyncAll(root, testPlanOptions()); err != nil {
		t.Fatal(err)
	}
	projectIndex, err := repository.LoadProjectIndex(filepath.Join(root, "ProjectIndex.jsonc"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := projectIndex.Projects["OpenCode"]; !ok {
		t.Fatal("present project was removed")
	}
	if _, ok := projectIndex.Projects["GoneProject"]; ok {
		t.Fatal("warehouse-wide sync kept a missing project")
	}
}

// TestSyncProjectRemovesScopedMissingProject verifies Project-scoped sync of a
// disappeared Project removes it from the Project index without failing
// resolution or recreating its source path, while leaving other missing and
// present Projects untouched.
func TestSyncProjectRemovesScopedMissingProject(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{
		filepath.Join(root, "Keep", "Column", "Keep"),
		filepath.Join(root, "Keep", "Mode"),
		filepath.Join(root, "Keep", "Backup"),
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ProjectIndex.jsonc"), []byte(`{
  "GoneA": {"description": "missing scoped"},
  "GoneB": {"description": "missing elsewhere"},
  "Keep": {"description": "present"}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Keep", "Column", "ColumnIndex.jsonc"), []byte(`{"Keep":{"description":"keep"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Keep", "Column", "Keep", "SettingIndex.jsonc"), []byte(`{"targetNumber":0,"settings":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Keep", "Mode", "ModeIndex.jsonc"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SyncProject(root, "GoneA", testPlanOptions()); err != nil {
		t.Fatalf("scoped sync of missing project: %v", err)
	}
	projectIndex, err := repository.LoadProjectIndex(filepath.Join(root, "ProjectIndex.jsonc"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := projectIndex.Projects["GoneA"]; ok {
		t.Fatal("scoped sync kept the selected disappeared project")
	}
	if _, ok := projectIndex.Projects["GoneB"]; !ok {
		t.Fatal("scoped sync removed an out-of-scope disappeared project")
	}
	if _, ok := projectIndex.Projects["Keep"]; !ok {
		t.Fatal("scoped sync removed a present project")
	}
	if _, err := os.Stat(filepath.Join(root, "GoneA")); !os.IsNotExist(err) {
		t.Fatalf("scoped sync recreated the disappeared path: %v", err)
	}

	if err := SyncAll(root, testPlanOptions()); err != nil {
		t.Fatal(err)
	}
	projectIndex, _ = repository.LoadProjectIndex(filepath.Join(root, "ProjectIndex.jsonc"))
	if _, ok := projectIndex.Projects["GoneB"]; ok {
		t.Fatal("warehouse-wide sync kept a missing project")
	}
	if _, ok := projectIndex.Projects["Keep"]; !ok {
		t.Fatal("warehouse-wide sync removed a present project")
	}
}

// TestSyncCleansStaleMissingMarkers verifies sync strips legacy durable missing
// markers from existing Project, Column, Setting, and Mode entries while
// preserving other extension fields.
func TestSyncCleansStaleMissingMarkers(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "OpenCode")
	columnRoot := filepath.Join(projectRoot, "Column", "Skills")
	for _, directory := range []string{columnRoot, filepath.Join(projectRoot, "Mode")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ProjectIndex.jsonc"), []byte(`{
  "OpenCode": {"missing": true, "keep": "project"}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte(`{
  "Skills": {"missing": true, "keep": "column"}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(columnRoot, "SettingIndex.jsonc"), []byte(`{
  "targetNumber": 0,
  "settings": {"Tool.json": {"missing": true, "keep": "setting"}}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Mode", "ModeIndex.jsonc"), []byte(`{
  "Max": {"missing": true, "keep": "mode"}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(columnRoot, "Tool.json"), []byte("tool"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SyncAll(root, testPlanOptions()); err != nil {
		t.Fatal(err)
	}
	repo := repository.New(root)
	projectIndex, err := repo.LoadProjectIndex()
	if err != nil {
		t.Fatal(err)
	}
	if rawBool(projectIndex.Projects["OpenCode"].Extra["missing"]) {
		t.Fatal("Project entry retains stale missing marker")
	}
	if string(projectIndex.Projects["OpenCode"].Extra["keep"]) != `"project"` {
		t.Fatalf("Project extension fields lost: %#v", projectIndex.Projects["OpenCode"].Extra)
	}
	columnIndex, err := repo.LoadColumnIndex("OpenCode")
	if err != nil {
		t.Fatal(err)
	}
	if rawBool(columnIndex.Columns["Skills"].Extra["missing"]) {
		t.Fatal("Column entry retains stale missing marker")
	}
	if string(columnIndex.Columns["Skills"].Extra["keep"]) != `"column"` {
		t.Fatalf("Column extension fields lost: %#v", columnIndex.Columns["Skills"].Extra)
	}
	settingIndex, err := repo.LoadSettingIndex("OpenCode", "Skills")
	if err != nil {
		t.Fatal(err)
	}
	if rawBool(settingIndex.Settings["Tool.json"].Extra["missing"]) {
		t.Fatal("Setting entry retains stale missing marker")
	}
	if string(settingIndex.Settings["Tool.json"].Extra["keep"]) != `"setting"` {
		t.Fatalf("Setting extension fields lost: %#v", settingIndex.Settings["Tool.json"].Extra)
	}
	modeIndex, err := repo.LoadModeIndex("OpenCode")
	if err != nil {
		t.Fatal(err)
	}
	if rawBool(modeIndex.Modes["Max"].Extra["missing"]) {
		t.Fatal("Mode entry retains stale missing marker")
	}
	if string(modeIndex.Modes["Max"].Extra["keep"]) != `"mode"` {
		t.Fatalf("Mode extension fields lost: %#v", modeIndex.Modes["Max"].Extra)
	}
}

// testPlanOptions returns deterministic environment-sensitive plan options for sync tests.
func testPlanOptions() planner.PlanOptions {
	return planner.PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"}
}

// TestSyncRecreatesMissingCurrentState verifies sync creates an empty Current
// state and removes stale history when current_state.json is absent.
func TestSyncRecreatesMissingCurrentState(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "OpenCode")
	columnRoot := filepath.Join(projectRoot, "Column", "Keep")
	for _, directory := range []string{columnRoot, filepath.Join(projectRoot, "Mode"), filepath.Join(projectRoot, "Backup")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ProjectIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte(`{"Keep":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(columnRoot, "SettingIndex.jsonc"), []byte(`{"targetNumber":0,"settings":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Mode", "ModeIndex.jsonc"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "history.log"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SyncAll(root, testPlanOptions()); err != nil {
		t.Fatalf("sync: %v", err)
	}

	repo := repository.New(root)
	if _, err := os.Lstat(repo.CurrentStatePath("OpenCode")); err != nil {
		t.Fatalf("current_state.json not created: %v", err)
	}
	state, err := repo.LoadCurrentState("OpenCode")
	if err != nil {
		t.Fatalf("load recreated current state: %v", err)
	}
	if len(state.Columns) != 0 || state.Relation != nil || len(state.Mappings) != 0 {
		t.Fatalf("recreated current state = %#v, want empty", state)
	}
	if _, err := os.Lstat(filepath.Join(projectRoot, "Backup", "history.log")); !os.IsNotExist(err) {
		t.Fatalf("stale history.log not removed, err=%v", err)
	}
}

// TestSyncReplansFollowingRelationFromOriginMode verifies sync replans mappings
// from the followed Mode's latest column selections when the Current state
// follows that Mode but its persisted mappings are stale.
func TestSyncReplansFollowingRelationFromOriginMode(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	projectRoot := filepath.Join(root, "OpenCode")
	columnRoot := filepath.Join(projectRoot, "Column", "opencode.json")
	for _, directory := range []string{columnRoot, filepath.Join(projectRoot, "Mode"), filepath.Join(projectRoot, "Backup")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "ProjectIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte(`{"opencode.json":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(columnRoot, "SettingIndex.jsonc"), []byte(`{
	"targetNumber": 1,
	"defaultTargetDir": ["~/.config/opencode"],
	"defaultTargetName": ["opencode.json"],
	"settings": {
		"GPT.json": {},
		"CLAUDE.json": {}
	}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(columnRoot, "GPT.json"), []byte("gpt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(columnRoot, "CLAUDE.json"), []byte("claude"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Mode Max selects CLAUDE.json; the Current state follows Max but still
	// carries the stale GPT.json mapping from an earlier selection.
	if err := os.WriteFile(filepath.Join(projectRoot, "Mode", "ModeIndex.jsonc"), []byte(`{
	"Max": {"columns": {"opencode.json": {"strategy": "cover", "settings": ["CLAUDE.json"]}}}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	staleTarget := filepath.Join(home, "stale", "opencode.json")
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "current_state.json"), []byte(`{
	"columns": {"opencode.json": {"strategy": "cover", "settings": ["GPT.json"]}},
	"relation": {"kind": "following", "originMode": "Max"},
	"mappings": [{"source": "`+filepath.Join(columnRoot, "GPT.json")+`", "target": "`+staleTarget+`"}]
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	options := planner.PlanOptions{HomeDir: home, Env: map[string]string{}, OS: "linux"}
	if err := SyncAll(root, options); err != nil {
		t.Fatalf("sync: %v", err)
	}

	repo := repository.New(root)
	state, err := repo.LoadCurrentState("OpenCode")
	if err != nil {
		t.Fatalf("load current state: %v", err)
	}
	if state.Relation == nil || state.Relation.Kind != "following" || state.Relation.OriginMode != "Max" {
		t.Fatalf("following relation lost: %#v", state.Relation)
	}
	if len(state.Mappings) != 1 || state.Mappings[0].Source != filepath.Join(columnRoot, "CLAUDE.json") {
		t.Fatalf("mappings not replanned from Mode Max: %#v", state.Mappings)
	}
	wantTarget := filepath.Join(home, ".config", "opencode", "opencode.json")
	if state.Mappings[0].Target != wantTarget {
		t.Fatalf("replanned target = %q, want %q", state.Mappings[0].Target, wantTarget)
	}
	if got := state.Columns["opencode.json"]; got.Strategy != "cover" || len(got.Settings) != 1 || got.Settings[0] != "CLAUDE.json" {
		t.Fatalf("columns not refreshed from Mode Max: %#v", state.Columns)
	}
}
