package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/repository"
)

// TestRenameCommandsTransactionalSurface verifies the CLI rename surface for
// success, alias resolution, error classes, drift refusal, forced reclamation,
// and PPID context rewriting.
func TestRenameCommandsTransactionalSurface(t *testing.T) {
	dependencies := testDependencies(t)
	rootPath := filepath.Join(dependencies.HomeDir, ".configfacilitator")
	targetDir := filepath.Join(dependencies.HomeDir, "targets")

	runResourceCommand(t, dependencies, []string{"project", "create", "OpenCode", "--aliases", "oc"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "create", "Models", "-p", "oc", "--aliases", "models"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "create", "GPT.json", "-p", "OpenCode", "-c", "models", "--kind", "file", "--aliases", "gpt"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "target", "add", "Models", "-p", "OpenCode", "--dir", targetDir, "--name-from-setting"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"apply", "column", "Models", "GPT.json", "-p", "OpenCode"}, ExitSuccess, "")

	// Inactive-alias rename of an actively mapped Setting: target recreated.
	runResourceCommand(t, dependencies, []string{"setting", "rename", "gpt", "Primary.json", "-p", "OpenCode", "-c", "Models"}, ExitSuccess, "")
	primaryTarget := filepath.Join(targetDir, "Primary.json")
	if link, err := os.Readlink(primaryTarget); err != nil || link != filepath.Join(rootPath, "OpenCode", "Column", "Models", "Primary.json") {
		t.Fatalf("renamed target -> %q err=%v", link, err)
	}
	if _, err := os.Lstat(filepath.Join(targetDir, "GPT.json")); !os.IsNotExist(err) {
		t.Fatalf("old derived target survived: %v", err)
	}
	if data, err := os.ReadFile(primaryTarget); err != nil || len(data) != 0 {
		t.Fatalf("renamed target content = %q err=%v", data, err)
	}

	// Missing and same-name failures classify as resource conflicts.
	runResourceCommand(t, dependencies, []string{"setting", "rename", "Nope", "X.json", "-p", "OpenCode", "-c", "Models", "--json"}, ExitResource, "setting_not_found")
	runResourceCommand(t, dependencies, []string{"setting", "rename", "Primary.json", "Primary.json", "-p", "OpenCode", "-c", "Models", "--json"}, ExitResource, "rename_same_name")

	// Drift blocks rename without --force-targets as a refusal.
	driftPath := filepath.Join(targetDir, "Drift.json")
	if err := os.WriteFile(driftPath, []byte("drift"), 0o644); err != nil {
		t.Fatal(err)
	}
	runResourceCommand(t, dependencies, []string{"setting", "rename", "Primary.json", "Drift.json", "-p", "OpenCode", "-c", "Models", "--json"}, ExitRefusal, "unsafe_target")
	if data, err := os.ReadFile(driftPath); err != nil || string(data) != "drift" {
		t.Fatalf("refused rename changed drift: %q err=%v", data, err)
	}

	// Forced reclamation removes the drifted file and installs the new link.
	runResourceCommand(t, dependencies, []string{"setting", "rename", "Primary.json", "Drift.json", "-p", "OpenCode", "-c", "Models", "--force-targets"}, ExitSuccess, "")
	if link, err := os.Readlink(driftPath); err != nil || link != filepath.Join(rootPath, "OpenCode", "Column", "Models", "Drift.json") {
		t.Fatalf("forced target -> %q err=%v", link, err)
	}

	// Mode rename through alias preserves selections.
	runResourceCommand(t, dependencies, []string{"mode", "create", "Max", "-p", "OpenCode", "--aliases", "max"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"mode", "column", "set", "Max", "Models", "-p", "OpenCode", "--strategy", "cover", "--setting", "Drift.json"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"mode", "rename", "max", "Maximum", "-p", "OpenCode"}, ExitSuccess, "")
	modeIndex, err := repository.New(rootPath).LoadModeIndex("OpenCode")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := modeIndex.Modes["Maximum"]; !ok {
		t.Fatalf("renamed Mode missing: %#v", modeIndex.Modes)
	}
	if got := modeIndex.Modes["Maximum"].Columns["Models"].Settings; len(got) != 1 || got[0] != "Drift.json" {
		t.Fatalf("Mode selection after rename = %#v", got)
	}

	// Column rename rewrites Mode references and keeps the Setting index.
	runResourceCommand(t, dependencies, []string{"column", "rename", "models", "Configurations", "-p", "OpenCode"}, ExitSuccess, "")
	if _, err := os.Lstat(filepath.Join(rootPath, "OpenCode", "Column", "Models")); !os.IsNotExist(err) {
		t.Fatalf("old Column dir survived: %v", err)
	}
	columnIndex, err := repository.New(rootPath).LoadColumnIndex("OpenCode")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := columnIndex.Columns["Configurations"]; !ok {
		t.Fatalf("renamed Column missing: %#v", columnIndex.Columns)
	}
	if _, err := os.Lstat(filepath.Join(rootPath, "OpenCode", "Column", "Configurations", "SettingIndex.jsonc")); err != nil {
		t.Fatalf("SettingIndex did not move: %v", err)
	}
	runResourceCommand(t, dependencies, []string{"mode", "column", "list", "Maximum", "-p", "OpenCode", "--json"}, ExitSuccess, "")
	modeIndex, _ = repository.New(rootPath).LoadModeIndex("OpenCode")
	if _, ok := modeIndex.Modes["Maximum"].Columns["Configurations"]; !ok {
		t.Fatalf("Mode Column ref not rewritten: %#v", modeIndex.Modes["Maximum"].Columns)
	}

	// Project rename rewrites the selected PPID context.
	runResourceCommand(t, dependencies, []string{"use", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"project", "rename", "oc", "Code"}, ExitSuccess, "")
	record, ok, err := repository.New(rootPath).LoadSession(dependencies.PPID)
	if err != nil || !ok || record.Project != "Code" {
		t.Fatalf("session after Project rename = %#v ok=%v err=%v", record, ok, err)
	}
	if _, err := os.Lstat(filepath.Join(rootPath, "OpenCode")); !os.IsNotExist(err) {
		t.Fatalf("old Project dir survived: %v", err)
	}
	if link, err := os.Readlink(driftPath); err != nil || link != filepath.Join(rootPath, "Code", "Column", "Configurations", "Drift.json") {
		t.Fatalf("target after Project rename -> %q err=%v", link, err)
	}
}

// TestRenameCommandJSONEnvelope verifies the JSON success envelope shape.
func TestRenameCommandJSONEnvelope(t *testing.T) {
	dependencies := testDependencies(t)
	runResourceCommand(t, dependencies, []string{"project", "create", "OpenCode"}, ExitSuccess, "")
	stdout, _ := runResourceCommand(t, dependencies, []string{"project", "rename", "OpenCode", "Code", "--json"}, ExitSuccess, "")
	if !strings.Contains(stdout, `"ok":true`) || !strings.Contains(stdout, `"project":"Code"`) || !strings.Contains(stdout, `"previousName":"OpenCode"`) {
		t.Fatalf("rename JSON envelope = %q", stdout)
	}
}
