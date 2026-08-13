package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/repository"
)

// TestSyncScopesDeleteDisappearedAndSettingWarehouse covers CLI reconciliation
// scopes: selected context, explicit Project, --all, restoration, removed
// prune syntax, and root-level SettingWarehouse discovery.
func TestSyncScopesDeleteDisappearedAndSettingWarehouse(t *testing.T) {
	dependencies := testDependencies(t)
	prepareWorkflowProject(t, dependencies, "ProjectA", "pa", filepath.Join(dependencies.HomeDir, "targets-a"))
	prepareWorkflowProject(t, dependencies, "ProjectB", "pb", filepath.Join(dependencies.HomeDir, "targets-b"))
	root := filepath.Join(dependencies.HomeDir, ".configfacilitator")
	repo := repository.New(root)

	projectASetting := workflowSettingPath(dependencies, "ProjectA", "Models", "Alpha.txt")
	projectBSetting := workflowSettingPath(dependencies, "ProjectB", "Models", "Alpha.txt")
	if err := os.Remove(projectASetting); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(projectBSetting); err != nil {
		t.Fatal(err)
	}

	// Removed --prune/--yes syntax is rejected as unknown flags.
	runResourceCommand(t, dependencies, []string{"sync", "--prune", "--json"}, ExitUsage, "invalid_usage")
	runResourceCommand(t, dependencies, []string{"sync", "--yes", "--json"}, ExitUsage, "invalid_usage")

	// Selected-context sync removes only the active Project's disappeared Setting.
	runResourceCommand(t, dependencies, []string{"use", "pa"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"sync"}, ExitSuccess, "")
	projectAIndex, err := repo.LoadSettingIndex("ProjectA", "Models")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := projectAIndex.Settings["Alpha.txt"]; ok {
		t.Fatal("selected-context sync retained disappeared ProjectA Setting")
	}
	projectBIndex, err := repo.LoadSettingIndex("ProjectB", "Models")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := projectBIndex.Settings["Alpha.txt"]; !ok {
		t.Fatal("selected-context sync changed ProjectB")
	}

	// Explicit warehouse-wide sync removes the other Project's disappeared Setting.
	runResourceCommand(t, dependencies, []string{"sync", "--all"}, ExitSuccess, "")
	projectBIndex, _ = repo.LoadSettingIndex("ProjectB", "Models")
	if _, ok := projectBIndex.Settings["Alpha.txt"]; ok {
		t.Fatal("explicit all sync retained disappeared ProjectB Setting")
	}

	// External restoration plus alias-scoped sync rediscovers the Setting.
	if err := os.WriteFile(projectASetting, []byte("restored"), 0o644); err != nil {
		t.Fatal(err)
	}
	runResourceCommand(t, dependencies, []string{"sync", "-p", "pa"}, ExitSuccess, "")
	projectAIndex, _ = repo.LoadSettingIndex("ProjectA", "Models")
	if _, ok := projectAIndex.Settings["Alpha.txt"]; !ok {
		t.Fatal("alias-scoped sync did not rediscover restored ProjectA Setting")
	}
	if testMissingMarker(projectAIndex.Settings["Alpha.txt"].Extra) {
		t.Fatal("restored ProjectA Setting carries stale missing marker")
	}

	// No-context warehouse sync discovers a root-level SettingWarehouse.
	runResourceCommand(t, dependencies, []string{"use", "global"}, ExitSuccess, "")
	settingWarehouse := filepath.Join(root, "SettingWarehouse", "Column", "Legacy")
	if err := os.MkdirAll(settingWarehouse, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(settingWarehouse, "SettingIndex.jsonc"), []byte(`{"targetNumber":0,"settings":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "SettingWarehouse", "Mode"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "SettingWarehouse", "Mode", "ModeIndex.jsonc"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	runResourceCommand(t, dependencies, []string{"sync"}, ExitSuccess, "")
	projectIndex, err := repo.LoadProjectIndex()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := projectIndex.Projects["SettingWarehouse"]; !ok {
		t.Fatalf("no-context warehouse sync skipped SettingWarehouse: %#v", projectIndex.Projects)
	}
	runResourceCommand(t, dependencies, []string{"sync", "-p", "global", "--json"}, ExitResource, "reserved_project")
}

// TestApplyAndRefreshRejectMissingBeforeTargetMutation verifies missing plans preserve managed targets and state.
func TestApplyAndRefreshRejectMissingBeforeTargetMutation(t *testing.T) {
	dependencies := testDependencies(t)
	targetDirectory := filepath.Join(dependencies.HomeDir, "targets")
	prepareWorkflowProject(t, dependencies, "OpenCode", "oc", targetDirectory)
	runResourceCommand(t, dependencies, []string{"apply", "mode", "Max", "-p", "OpenCode"}, ExitSuccess, "")
	target := filepath.Join(targetDirectory, "Alpha.txt")
	source := workflowSettingPath(dependencies, "OpenCode", "Models", "Alpha.txt")
	beforeState := loadWorkflowState(t, dependencies, "OpenCode")
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	runResourceCommand(t, dependencies, []string{"apply", "mode", "Max", "-p", "OpenCode", "--json"}, ExitResource, "resource_missing")
	if info, err := os.Lstat(target); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("failed apply changed target: info=%v err=%v", info, err)
	}
	runResourceCommand(t, dependencies, []string{"refresh", "-p", "OpenCode", "--json"}, ExitResource, "resource_missing")
	afterState := loadWorkflowState(t, dependencies, "OpenCode")
	if len(afterState.Mappings) != len(beforeState.Mappings) || afterState.Intent == nil || beforeState.Intent == nil || afterState.Intent.Mode != beforeState.Intent.Mode {
		t.Fatalf("missing refresh changed state: before=%#v after=%#v", beforeState, afterState)
	}
	if info, err := os.Lstat(target); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("failed refresh changed target: info=%v err=%v", info, err)
	}
}

// testMissingMarker reports whether normalized test metadata carries missing=true.
func testMissingMarker(extra map[string]json.RawMessage) bool {
	var missing bool
	_ = json.Unmarshal(extra["missing"], &missing)
	return missing
}

// TestSyncProjectScopeRemovesDisappearedProject verifies selected-context and
// explicit -p sync remove a disappeared Project from the Project index without
// failing resolution or recreating its source path, and never remove other
// disappeared or present Projects.
func TestSyncProjectScopeRemovesDisappearedProject(t *testing.T) {
	dependencies := testDependencies(t)
	prepareWorkflowProject(t, dependencies, "ProjectA", "pa", filepath.Join(dependencies.HomeDir, "targets-a"))
	prepareWorkflowProject(t, dependencies, "ProjectB", "pb", filepath.Join(dependencies.HomeDir, "targets-b"))
	prepareWorkflowProject(t, dependencies, "ProjectC", "pc", filepath.Join(dependencies.HomeDir, "targets-c"))
	root := filepath.Join(dependencies.HomeDir, ".configfacilitator")
	repo := repository.New(root)

	// Selected-context sync of a disappeared Project removes only it.
	runResourceCommand(t, dependencies, []string{"use", "pc"}, ExitSuccess, "")
	if err := os.RemoveAll(filepath.Join(root, "ProjectC")); err != nil {
		t.Fatal(err)
	}
	runResourceCommand(t, dependencies, []string{"sync"}, ExitSuccess, "")
	projectIndex, err := repo.LoadProjectIndex()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := projectIndex.Projects["ProjectC"]; ok {
		t.Fatal("selected-context sync retained the disappeared Project")
	}
	for _, name := range []string{"ProjectA", "ProjectB"} {
		if _, ok := projectIndex.Projects[name]; !ok {
			t.Fatalf("selected-context sync removed present Project %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "ProjectC")); !os.IsNotExist(err) {
		t.Fatalf("selected-context sync recreated the disappeared Project path: %v", err)
	}

	// Explicit -p sync of another disappeared Project removes only it.
	if err := os.RemoveAll(filepath.Join(root, "ProjectB")); err != nil {
		t.Fatal(err)
	}
	runResourceCommand(t, dependencies, []string{"sync", "-p", "pb"}, ExitSuccess, "")
	projectIndex, _ = repo.LoadProjectIndex()
	if _, ok := projectIndex.Projects["ProjectB"]; ok {
		t.Fatal("explicit scoped sync retained the disappeared Project")
	}
	if _, ok := projectIndex.Projects["ProjectA"]; !ok {
		t.Fatal("explicit scoped sync removed a present Project")
	}
	if _, err := os.Stat(filepath.Join(root, "ProjectB")); !os.IsNotExist(err) {
		t.Fatalf("explicit scoped sync recreated the disappeared Project path: %v", err)
	}

	// Warehouse-wide sync removes every remaining disappeared Project.
	runResourceCommand(t, dependencies, []string{"sync", "--all"}, ExitSuccess, "")
	projectIndex, _ = repo.LoadProjectIndex()
	if _, ok := projectIndex.Projects["ProjectB"]; ok {
		t.Fatal("warehouse-wide sync retained a disappeared Project")
	}
	if _, ok := projectIndex.Projects["ProjectA"]; !ok {
		t.Fatal("warehouse-wide sync removed a present Project")
	}
}
