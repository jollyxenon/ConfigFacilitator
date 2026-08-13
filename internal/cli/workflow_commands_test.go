package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/linker"
	"github.com/xenon/ConfigFacilitator/internal/repository"
)

// TestWorkflowContextStatusApplyRefreshAndMissingDiagnostics covers context precedence, canonical intents, status, full-Mode refresh, and read-only diagnostics.
func TestWorkflowContextStatusApplyRefreshAndMissingDiagnostics(t *testing.T) {
	dependencies := testDependencies(t)
	firstTargets := filepath.Join(dependencies.HomeDir, "targets-a")
	secondTargets := filepath.Join(dependencies.HomeDir, "targets-b")
	prepareWorkflowProject(t, dependencies, "ProjectA", "pa", firstTargets)
	prepareWorkflowProject(t, dependencies, "ProjectB", "pb", secondTargets)

	runResourceCommand(t, dependencies, []string{"use", "pa"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"apply", "column", "models", "alpha", "-p", "pb"}, ExitSuccess, "")
	assertManagedLink(t, filepath.Join(secondTargets, "Alpha.txt"), workflowSettingPath(dependencies, "ProjectB", "Models", "Alpha.txt"))
	if _, err := os.Lstat(filepath.Join(firstTargets, "Alpha.txt")); !os.IsNotExist(err) {
		t.Fatalf("explicit Project did not override selected context: %v", err)
	}

	runResourceCommand(t, dependencies, []string{"apply", "mode", "maximum"}, ExitSuccess, "")
	assertManagedLink(t, filepath.Join(firstTargets, "Alpha.txt"), workflowSettingPath(dependencies, "ProjectA", "Models", "Alpha.txt"))
	state := loadWorkflowState(t, dependencies, "ProjectA")
	if state.Intent == nil || state.Intent.Kind != "mode" || state.Intent.Mode != "Max" {
		t.Fatalf("mode alias was not persisted canonically: %#v", state.Intent)
	}

	runResourceCommand(t, dependencies, []string{"setting", "create", "Beta.txt", "-p", "ProjectA", "-c", "Models", "--kind", "file", "--text", "beta"}, ExitSuccess, "")
	if _, err := os.Lstat(filepath.Join(firstTargets, "Beta.txt")); !os.IsNotExist(err) {
		t.Fatalf("new full-Mode Setting was linked before refresh: %v", err)
	}
	runResourceCommand(t, dependencies, []string{"refresh"}, ExitSuccess, "")
	assertManagedLink(t, filepath.Join(firstTargets, "Beta.txt"), workflowSettingPath(dependencies, "ProjectA", "Models", "Beta.txt"))

	missingPath := workflowSettingPath(dependencies, "ProjectA", "Models", "Beta.txt")
	if err := os.Remove(missingPath); err != nil {
		t.Fatalf("remove Setting source: %v", err)
	}
	rootPath := filepath.Join(dependencies.HomeDir, ".configfacilitator")
	transaction, err := repository.New(rootPath).BeginMutation("status-probe", filepath.Join(rootPath, "probe.txt"))
	if err != nil {
		t.Fatalf("begin diagnostic transaction: %v", err)
	}
	if err := transaction.LeavePrepared(); err != nil {
		t.Fatalf("leave diagnostic transaction prepared: %v", err)
	}

	human, _ := runResourceCommand(t, dependencies, []string{"status", "-p", "ProjectA"}, ExitSuccess, "")
	for _, expected := range []string{"Active Mode:", "Intent: mode Max", "Mappings:", "Columns:", "Missing resources:", "Models/Beta.txt", "Incomplete transactions:", "status-probe"} {
		if !strings.Contains(human, expected) {
			t.Fatalf("status human output missing %q: %q", expected, human)
		}
	}
	if strings.Contains(human, "\x1b[") {
		t.Fatalf("buffered human status contained ANSI: %q", human)
	}
	jsonOutput, _ := runResourceCommand(t, dependencies, []string{"status", "-p", "ProjectA", "--json"}, ExitSuccess, "")
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Project      string                       `json:"project"`
			Intent       *linker.ApplyIntent          `json:"intent"`
			Mappings     []linker.Mapping             `json:"mappings"`
			Columns      []statusColumnSummary        `json:"columns"`
			Missing      []statusMissingResource      `json:"missing"`
			Transactions []repository.TransactionInfo `json:"transactions"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(jsonOutput), &envelope); err != nil {
		t.Fatalf("decode status JSON: %v; output=%q", err, jsonOutput)
	}
	if !envelope.OK || envelope.Data.Project != "ProjectA" || envelope.Data.Intent == nil || envelope.Data.Intent.Mode != "Max" || len(envelope.Data.Mappings) == 0 || len(envelope.Data.Columns) != 1 || len(envelope.Data.Missing) != 1 || len(envelope.Data.Transactions) != 1 {
		t.Fatalf("status JSON data = %#v", envelope.Data)
	}
	if _, err := os.Stat(transaction.Directory()); err != nil {
		t.Fatalf("read-only status recovered or removed transaction: %v", err)
	}
	if err := repository.New(rootPath).Recover(); err != nil {
		t.Fatalf("recover diagnostic transaction: %v", err)
	}

	runResourceCommand(t, dependencies, []string{"use", "global"}, ExitSuccess, "")
	globalHuman, _ := runResourceCommand(t, dependencies, []string{"status"}, ExitSuccess, "")
	if !strings.Contains(globalHuman, "ProjectA (Max)") || !strings.Contains(globalHuman, "ProjectB (Unmatched)") {
		t.Fatalf("warehouse status summaries = %q", globalHuman)
	}
	globalJSON, _ := runResourceCommand(t, dependencies, []string{"status", "--json"}, ExitSuccess, "")
	var globalEnvelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Projects []statusProjectSummary  `json:"projects"`
			Missing  []statusMissingResource `json:"missing"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(globalJSON), &globalEnvelope); err != nil {
		t.Fatalf("decode warehouse status JSON: %v; output=%q", err, globalJSON)
	}
	if !globalEnvelope.OK || len(globalEnvelope.Data.Projects) != 2 || len(globalEnvelope.Data.Missing) != 1 {
		t.Fatalf("warehouse status JSON = %#v", globalEnvelope.Data)
	}
}

// TestWorkflowRefreshMappingOnlyColumnIsolationAndAllProjects covers every refresh planning scope.
func TestWorkflowRefreshMappingOnlyColumnIsolationAndAllProjects(t *testing.T) {
	dependencies := testDependencies(t)
	projectATargets := filepath.Join(dependencies.HomeDir, "project-a")
	projectBTargets := filepath.Join(dependencies.HomeDir, "project-b")
	prepareWorkflowProject(t, dependencies, "ProjectA", "pa", projectATargets)
	prepareWorkflowProject(t, dependencies, "ProjectB", "pb", projectBTargets)

	runResourceCommand(t, dependencies, []string{"apply", "column", "Models", "Alpha.txt", "-p", "ProjectA"}, ExitSuccess, "")
	state := loadWorkflowState(t, dependencies, "ProjectA")
	state.Intent = nil
	saveWorkflowState(t, dependencies, "ProjectA", state)
	mappingOnlyTargets := filepath.Join(dependencies.HomeDir, "mapping-only")
	runResourceCommand(t, dependencies, []string{"column", "target", "set", "Models", "0", "-p", "ProjectA", "--dir", mappingOnlyTargets}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"refresh", "-p", "ProjectA"}, ExitSuccess, "")
	assertManagedLink(t, filepath.Join(mappingOnlyTargets, "Alpha.txt"), workflowSettingPath(dependencies, "ProjectA", "Models", "Alpha.txt"))
	if loadWorkflowState(t, dependencies, "ProjectA").Intent != nil {
		t.Fatal("mapping-only refresh invented an apply intent")
	}

	addWorkflowColumn(t, dependencies, "ProjectA", "Skills", filepath.Join(projectATargets, "skills"), "Skill-A")
	runResourceCommand(t, dependencies, []string{"mode", "column", "set", "Max", "Skills", "-p", "ProjectA", "--strategy", "full"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"apply", "mode", "Max", "-p", "ProjectA"}, ExitSuccess, "")
	newModelsTargets := filepath.Join(dependencies.HomeDir, "models-new")
	newSkillsTargets := filepath.Join(dependencies.HomeDir, "skills-new")
	runResourceCommand(t, dependencies, []string{"column", "target", "set", "Models", "0", "-p", "ProjectA", "--dir", newModelsTargets}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "target", "set", "Skills", "0", "-p", "ProjectA", "--dir", newSkillsTargets}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"refresh", "-p", "ProjectA", "--column", "Models"}, ExitSuccess, "")
	assertManagedLink(t, filepath.Join(newModelsTargets, "Alpha.txt"), workflowSettingPath(dependencies, "ProjectA", "Models", "Alpha.txt"))
	assertManagedLink(t, filepath.Join(projectATargets, "skills", "Skill-A"), workflowSettingPath(dependencies, "ProjectA", "Skills", "Skill-A"))
	if _, err := os.Lstat(filepath.Join(newSkillsTargets, "Skill-A")); !os.IsNotExist(err) {
		t.Fatalf("Column refresh changed another Column: %v", err)
	}

	runResourceCommand(t, dependencies, []string{"apply", "column", "Models", "Alpha.txt", "-p", "ProjectB"}, ExitSuccess, "")
	allATargets := filepath.Join(dependencies.HomeDir, "all-a")
	allBTargets := filepath.Join(dependencies.HomeDir, "all-b")
	runResourceCommand(t, dependencies, []string{"column", "target", "set", "Models", "0", "-p", "ProjectA", "--dir", allATargets}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "target", "set", "Models", "0", "-p", "ProjectB", "--dir", allBTargets}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"use", "ProjectB"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"refresh", "--all"}, ExitSuccess, "")
	assertManagedLink(t, filepath.Join(allATargets, "Alpha.txt"), workflowSettingPath(dependencies, "ProjectA", "Models", "Alpha.txt"))
	assertManagedLink(t, filepath.Join(allBTargets, "Alpha.txt"), workflowSettingPath(dependencies, "ProjectB", "Models", "Alpha.txt"))
}

// TestWorkflowResetRevertAndForcedTargetRecovery covers one-step restore and every state-command target override.
func TestWorkflowResetRevertAndForcedTargetRecovery(t *testing.T) {
	dependencies := testDependencies(t)
	targets := filepath.Join(dependencies.HomeDir, "targets")
	prepareWorkflowProject(t, dependencies, "OpenCode", "oc", targets)
	runResourceCommand(t, dependencies, []string{"setting", "create", "Beta.txt", "-p", "OpenCode", "-c", "Models", "--kind", "file", "--text", "beta"}, ExitSuccess, "")

	runResourceCommand(t, dependencies, []string{"apply", "column", "Models", "Alpha.txt", "-p", "oc"}, ExitSuccess, "")
	alphaTarget := filepath.Join(targets, "Alpha.txt")
	replaceManagedTargetWithDirectory(t, alphaTarget)
	runResourceCommand(t, dependencies, []string{"refresh", "-p", "OpenCode", "--force-targets"}, ExitSuccess, "")
	assertManagedLink(t, alphaTarget, workflowSettingPath(dependencies, "OpenCode", "Models", "Alpha.txt"))

	runResourceCommand(t, dependencies, []string{"apply", "column", "Models", "Beta.txt", "-p", "OpenCode"}, ExitSuccess, "")
	if err := os.MkdirAll(alphaTarget, 0o755); err != nil {
		t.Fatalf("occupy revert target: %v", err)
	}
	runResourceCommand(t, dependencies, []string{"revert", "-p", "oc", "--force-targets", "--json"}, ExitSuccess, "")
	assertManagedLink(t, alphaTarget, workflowSettingPath(dependencies, "OpenCode", "Models", "Alpha.txt"))
	state := loadWorkflowState(t, dependencies, "OpenCode")
	if state.Intent == nil || state.Intent.Column != "Models" || len(state.Intent.Settings) != 1 || state.Intent.Settings[0] != "Alpha.txt" {
		t.Fatalf("one-step revert state = %#v", state)
	}

	replaceManagedTargetWithDirectory(t, alphaTarget)
	runResourceCommand(t, dependencies, []string{"reset", "-p", "OpenCode", "--force-targets", "--json"}, ExitSuccess, "")
	if _, err := os.Lstat(alphaTarget); !os.IsNotExist(err) {
		t.Fatalf("forced reset did not reclaim target: %v", err)
	}
	state = loadWorkflowState(t, dependencies, "OpenCode")
	if state.Intent != nil || len(state.Mappings) != 0 {
		t.Fatalf("reset state = %#v", state)
	}

	runResourceCommand(t, dependencies, []string{"revert", "-p", "OpenCode", "--force-targets"}, ExitSuccess, "")
	state = loadWorkflowState(t, dependencies, "OpenCode")
	if state.Intent == nil || state.Intent.Column != "Models" || len(state.Intent.Settings) != 1 || state.Intent.Settings[0] != "Alpha.txt" {
		t.Fatalf("revert after reset state = %#v", state)
	}
	assertManagedLink(t, alphaTarget, workflowSettingPath(dependencies, "OpenCode", "Models", "Alpha.txt"))
}

// TestWorkflowRemovedOperationalSyntaxIsRejectedWithoutMutation expands old-parser rejection across task 9 commands.
func TestWorkflowRemovedOperationalSyntaxIsRejectedWithoutMutation(t *testing.T) {
	dependencies := testDependencies(t)
	prepareWorkflowProject(t, dependencies, "OpenCode", "oc", filepath.Join(dependencies.HomeDir, "targets"))
	before := snapshotTree(t, dependencies.HomeDir)
	invocations := [][]string{
		{"apply", "-m", "Max", "-p", "OpenCode"},
		{"apply", "-c", "Models", "-s", "Alpha.txt", "-p", "OpenCode"},
		{"apply", "mode", "Max", "-p", "OpenCode", "-f"},
		{"refresh", "-p", "OpenCode", "-c", "Models"},
		{"refresh", "-a"},
		{"refresh", "-p", "OpenCode", "--force"},
		{"reset", "-p", "OpenCode", "-f"},
		{"revert", "-p", "OpenCode", "--force"},
		{"update", "-p", "OpenCode"},
	}
	for _, args := range invocations {
		runResourceCommand(t, dependencies, args, ExitUsage, "")
		if after := snapshotTree(t, dependencies.HomeDir); !equalWorkflowSnapshots(before, after) {
			t.Fatalf("removed syntax mutated fixture: args=%v", args)
		}
	}
}

// prepareWorkflowProject creates one alias-addressable Project with a full Mode and one file Setting.
func prepareWorkflowProject(t *testing.T, dependencies Dependencies, project string, alias string, targetDirectory string) {
	t.Helper()
	runResourceCommand(t, dependencies, []string{"project", "create", project, "--aliases", alias}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "create", "Models", "-p", project, "--aliases", "models"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "target", "add", "Models", "-p", project, "--dir", targetDirectory, "--name-from-setting"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "create", "Alpha.txt", "-p", project, "-c", "Models", "--kind", "file", "--text", "alpha", "--aliases", "alpha"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"mode", "create", "Max", "-p", project, "--aliases", "maximum"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"mode", "column", "set", "Max", "Models", "-p", project, "--strategy", "full"}, ExitSuccess, "")
}

// addWorkflowColumn creates one full-Mode-ready directory-backed Column and Setting.
func addWorkflowColumn(t *testing.T, dependencies Dependencies, project string, column string, targetDirectory string, setting string) {
	t.Helper()
	runResourceCommand(t, dependencies, []string{"column", "create", column, "-p", project}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "target", "add", column, "-p", project, "--dir", targetDirectory, "--name-from-setting"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "create", setting, "-p", project, "-c", column, "--kind", "directory"}, ExitSuccess, "")
}

// workflowSettingPath returns one canonical Setting source path in the isolated warehouse.
func workflowSettingPath(dependencies Dependencies, project string, column string, setting string) string {
	return filepath.Join(dependencies.HomeDir, ".configfacilitator", project, "Column", column, setting)
}

// loadWorkflowState reads one Project's current runtime state.
func loadWorkflowState(t *testing.T, dependencies Dependencies, project string) linker.CurrentState {
	t.Helper()
	state, err := repository.New(filepath.Join(dependencies.HomeDir, ".configfacilitator")).LoadCurrentState(project)
	if err != nil {
		t.Fatalf("load current state: %v", err)
	}
	return state
}

// saveWorkflowState writes one Project's current runtime state for mapping-only compatibility coverage.
func saveWorkflowState(t *testing.T, dependencies Dependencies, project string, state linker.CurrentState) {
	t.Helper()
	if err := repository.New(filepath.Join(dependencies.HomeDir, ".configfacilitator")).SaveCurrentState(project, state); err != nil {
		t.Fatalf("save current state: %v", err)
	}
}

// assertManagedLink verifies one exact source-target symlink relationship.
func assertManagedLink(t *testing.T, target string, source string) {
	t.Helper()
	resolved, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("read managed link %q: %v", target, err)
	}
	if resolved != source {
		t.Fatalf("managed link %q -> %q, want %q", target, resolved, source)
	}
}

// replaceManagedTargetWithDirectory creates directory-backed drift at one recorded target.
func replaceManagedTargetWithDirectory(t *testing.T, target string) {
	t.Helper()
	if err := os.Remove(target); err != nil {
		t.Fatalf("remove managed target: %v", err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("create drifted target directory: %v", err)
	}
}

// equalWorkflowSnapshots compares fixture snapshots without adding another dependency.
func equalWorkflowSnapshots(left map[string]string, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for path, value := range left {
		if right[path] != value {
			return false
		}
	}
	return true
}
