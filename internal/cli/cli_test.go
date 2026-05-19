package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func setTempHome(t *testing.T, workspace string) string {
	t.Helper()
	homeDir := filepath.Join(workspace, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home dir: %v", err)
	}
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	return homeDir
}

func TestRunShowsRootHelpWithoutArguments(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(nil, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := stdout.String()
	for _, required := range []string{"cfgfc manages portable configuration warehouses.", "new", "sync", "switch", "list", "apply", "reset", "revert"} {
		if !bytes.Contains(stdout.Bytes(), []byte(required)) {
			t.Fatalf("expected help output to contain %q, got %q", required, output)
		}
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunReportsMissingProjectForApplyWithoutContext(t *testing.T) {
	workspace := t.TempDir()
	setTempHome(t, workspace)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"apply"}, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}

	if !bytes.Contains(stderr.Bytes(), []byte("no active project")) {
		t.Fatalf("expected missing-project error, got %q", stderr.String())
	}
}

func TestRunRejectsUnknownCommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"unknown"}, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}

	if got := stderr.String(); !bytes.Contains(stderr.Bytes(), []byte("unknown command")) || !bytes.Contains(stderr.Bytes(), []byte("Available Commands:")) {
		t.Fatalf("expected stderr to contain error and help text, got %q", got)
	}
}

func TestRunWithExecutableCreatesProjectColumnAndModeScaffolds(t *testing.T) {
	workspace := t.TempDir()
	homeDir := setTempHome(t, workspace)
	executablePath := filepath.Join(workspace, "cfgfc")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := RunWithExecutable([]string{"new", "-p", "OpenCode"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("new project exit code = %d, stderr=%q", exitCode, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".configfacilitator", "SettingWarehouse", "OpenCode", "Column", "ColumnIndex.jsonc")); err != nil {
		t.Fatalf("project scaffold missing column index: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithExecutable([]string{"new", "-p", "OpenCode", "-c", "Skills"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("new column exit code = %d, stderr=%q", exitCode, stderr.String())
	}
	settingIndexPath := filepath.Join(homeDir, ".configfacilitator", "SettingWarehouse", "OpenCode", "Column", "Skills", "SettingIndex.jsonc")
	settingIndexData, err := os.ReadFile(settingIndexPath)
	if err != nil {
		t.Fatalf("read setting index: %v", err)
	}
	if !bytes.Contains(settingIndexData, []byte("ExampleSetting")) {
		t.Fatalf("setting index template missing guidance, got %q", string(settingIndexData))
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithExecutable([]string{"new", "-p", "OpenCode", "-m", "Max"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("new mode exit code = %d, stderr=%q", exitCode, stderr.String())
	}
	modeIndexData, err := os.ReadFile(filepath.Join(homeDir, ".configfacilitator", "SettingWarehouse", "OpenCode", "Mode", "ModeIndex.jsonc"))
	if err != nil {
		t.Fatalf("read mode index: %v", err)
	}
	if !bytes.Contains(modeIndexData, []byte("\"Max\"")) || !bytes.Contains(modeIndexData, []byte("\"Skills\"")) {
		t.Fatalf("mode index missing mode scaffold, got %q", string(modeIndexData))
	}
}

func TestRunWithExecutableSyncsWarehouseIndexes(t *testing.T) {
	workspace := t.TempDir()
	homeDir := setTempHome(t, workspace)
	executablePath := filepath.Join(workspace, "cfgfc")
	warehouseRoot := filepath.Join(homeDir, ".configfacilitator", "SettingWarehouse")
	projectRoot := filepath.Join(warehouseRoot, "OpenCode")
	for _, path := range []string{
		filepath.Join(projectRoot, "Column", "Skills", "Skill-A"),
		filepath.Join(projectRoot, "Mode"),
		filepath.Join(projectRoot, "Backup"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "ProjectIndex.jsonc"), []byte{}, 0o644); err == nil {
		// intentionally ignored; project index lives at warehouse root
	}
	if err := os.WriteFile(filepath.Join(warehouseRoot, "ProjectIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write project index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write column index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "Skills", "SettingIndex.jsonc"), []byte("{\n  \"description\": \"Skills\",\n  \"settings\": {\n    \"MissingSkill\": {\n      \"description\": \"keep me\",\n      \"missing\": true\n    }\n  }\n}\n"), 0o644); err != nil {
		t.Fatalf("write setting index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "Skills", "Skill-A", "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write discovered setting file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Mode", "ModeIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write mode index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "current_state.json"), []byte("{\"mappings\": []}\n"), 0o644); err != nil {
		t.Fatalf("write current state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "history.log"), []byte{}, 0o644); err != nil {
		t.Fatalf("write history log: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := RunWithExecutable([]string{"sync", "-p", "OpenCode"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("sync exit code = %d, stderr=%q", exitCode, stderr.String())
	}

	projectIndexData, err := os.ReadFile(filepath.Join(warehouseRoot, "ProjectIndex.jsonc"))
	if err != nil {
		t.Fatalf("read project index after sync: %v", err)
	}
	if !bytes.Contains(projectIndexData, []byte("\"OpenCode\"")) {
		t.Fatalf("project index missing synced project, got %q", string(projectIndexData))
	}

	columnIndexData, err := os.ReadFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"))
	if err != nil {
		t.Fatalf("read column index after sync: %v", err)
	}
	if !bytes.Contains(columnIndexData, []byte("\"Skills\"")) {
		t.Fatalf("column index missing synced column, got %q", string(columnIndexData))
	}

	settingIndexData, err := os.ReadFile(filepath.Join(projectRoot, "Column", "Skills", "SettingIndex.jsonc"))
	if err != nil {
		t.Fatalf("read setting index after sync: %v", err)
	}
	if !bytes.Contains(settingIndexData, []byte("\"Skill-A\"")) || !bytes.Contains(settingIndexData, []byte("\"MissingSkill\"")) {
		t.Fatalf("setting index missing synced entities, got %q", string(settingIndexData))
	}
	if !bytes.Contains(settingIndexData, []byte("\"description\": \"keep me\"")) || !bytes.Contains(settingIndexData, []byte("\"missing\": true")) {
		t.Fatalf("setting index lost missing metadata, got %q", string(settingIndexData))
	}
}

func TestRunWithExecutableSwitchAndListUseConvenienceContext(t *testing.T) {
	workspace := t.TempDir()
	homeDir := setTempHome(t, workspace)
	executablePath := filepath.Join(workspace, "cfgfc")
	if exitCode := RunWithExecutable([]string{"new", "-p", "OpenCode"}, &bytes.Buffer{}, &bytes.Buffer{}, executablePath); exitCode != 0 {
		t.Fatalf("create project failed")
	}
	if exitCode := RunWithExecutable([]string{"new", "-p", "OpenCode", "-c", "Skills"}, &bytes.Buffer{}, &bytes.Buffer{}, executablePath); exitCode != 0 {
		t.Fatalf("create column failed")
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".configfacilitator", "SettingWarehouse", "OpenCode", "Column", "Skills", "Skill-A"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write setting file: %v", err)
	}
	if exitCode := RunWithExecutable([]string{"sync", "-p", "OpenCode"}, &bytes.Buffer{}, &bytes.Buffer{}, executablePath); exitCode != 0 {
		t.Fatalf("sync failed")
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := RunWithExecutable([]string{"switch", "OpenCode"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("switch exit code = %d, stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Switched active project")) {
		t.Fatalf("unexpected switch stdout %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithExecutable([]string{"list"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("list exit code = %d, stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Project: OpenCode")) || !bytes.Contains(stdout.Bytes(), []byte("Columns:")) {
		t.Fatalf("unexpected list stdout %q", stdout.String())
	}
}

func TestRunWithExecutableListsColumnAndModeDetails(t *testing.T) {
	workspace := t.TempDir()
	homeDir := setTempHome(t, workspace)
	executablePath := filepath.Join(workspace, "cfgfc")
	if exitCode := RunWithExecutable([]string{"new", "-p", "OpenCode"}, &bytes.Buffer{}, &bytes.Buffer{}, executablePath); exitCode != 0 {
		t.Fatalf("create project failed")
	}
	if exitCode := RunWithExecutable([]string{"new", "-p", "OpenCode", "-c", "Skills"}, &bytes.Buffer{}, &bytes.Buffer{}, executablePath); exitCode != 0 {
		t.Fatalf("create column failed")
	}
	if exitCode := RunWithExecutable([]string{"new", "-p", "OpenCode", "-m", "Max"}, &bytes.Buffer{}, &bytes.Buffer{}, executablePath); exitCode != 0 {
		t.Fatalf("create mode failed")
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".configfacilitator", "SettingWarehouse", "OpenCode", "Column", "Skills", "Skill-A"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write setting file: %v", err)
	}
	if exitCode := RunWithExecutable([]string{"sync", "-p", "OpenCode"}, &bytes.Buffer{}, &bytes.Buffer{}, executablePath); exitCode != 0 {
		t.Fatalf("sync failed")
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := RunWithExecutable([]string{"list", "-p", "OpenCode", "-c", "Skills"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("list column exit code = %d, stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Column: Skills")) || !bytes.Contains(stdout.Bytes(), []byte("Skill-A")) {
		t.Fatalf("unexpected column output %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithExecutable([]string{"list", "-p", "OpenCode", "-m", "Max"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("list mode exit code = %d, stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Mode: Max")) || !bytes.Contains(stdout.Bytes(), []byte("Skills: strategy=full")) {
		t.Fatalf("unexpected mode output %q", stdout.String())
	}
}

func TestRunWithExecutableApplyResetAndRevertEndToEnd(t *testing.T) {
	workspace := t.TempDir()
	executablePath := filepath.Join(workspace, "cfgfc")
	homeDir := filepath.Join(workspace, "home")
	configDir := filepath.Join(homeDir, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	t.Setenv("HOME", homeDir)

	mustRun(t, executablePath, []string{"new", "-p", "OpenCode"})
	mustRun(t, executablePath, []string{"new", "-p", "OpenCode", "-c", "opencode.json"})
	mustRun(t, executablePath, []string{"new", "-p", "OpenCode", "-c", "Skills"})
	mustRun(t, executablePath, []string{"new", "-p", "OpenCode", "-m", "Max"})

	warehouseRoot := filepath.Join(homeDir, ".configfacilitator", "SettingWarehouse", "OpenCode")
	if err := os.WriteFile(filepath.Join(warehouseRoot, "Column", "opencode.json", "CLAUDE.json"), []byte("claude"), 0o644); err != nil {
		t.Fatalf("write CLAUDE.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(warehouseRoot, "Column", "opencode.json", "GPT.json"), []byte("gpt"), 0o644); err != nil {
		t.Fatalf("write GPT.json: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(warehouseRoot, "Column", "Skills", "Skill-A"), 0o755); err != nil {
		t.Fatalf("mkdir Skill-A: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(warehouseRoot, "Column", "Skills", "Skill-B"), 0o755); err != nil {
		t.Fatalf("mkdir Skill-B: %v", err)
	}
	if err := os.WriteFile(filepath.Join(warehouseRoot, "Column", "Skills", "Skill-A", "README.md"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write Skill-A readme: %v", err)
	}
	if err := os.WriteFile(filepath.Join(warehouseRoot, "Column", "Skills", "Skill-B", "README.md"), []byte("b"), 0o644); err != nil {
		t.Fatalf("write Skill-B readme: %v", err)
	}
	if err := os.WriteFile(filepath.Join(warehouseRoot, "Column", "opencode.json", "SettingIndex.jsonc"), []byte("{\n  \"description\": \"main config\",\n  \"defaultTarget\": \"~/.config/opencode/opencode.json\",\n  \"settings\": {\n    \"CLAUDE.json\": {\"displayName\": \"CLAUDE.json\"},\n    \"GPT.json\": {\"displayName\": \"GPT.json\"}\n  }\n}\n"), 0o644); err != nil {
		t.Fatalf("write opencode setting index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(warehouseRoot, "Column", "Skills", "SettingIndex.jsonc"), []byte("{\n  \"description\": \"skills\",\n  \"settings\": {\n    \"Skill-A\": {\"target\": \"~/.config/opencode/skills/Skill-A\"},\n    \"Skill-B\": {\"target\": \"~/.config/opencode/skills/Skill-B\"}\n  }\n}\n"), 0o644); err != nil {
		t.Fatalf("write skills setting index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(warehouseRoot, "Mode", "ModeIndex.jsonc"), []byte("{\n  \"Max\": {\n    \"displayName\": \"Max\",\n    \"columns\": {\n      \"opencode.json\": {\n        \"settings\": [\"CLAUDE.json\"],\n        \"strategy\": \"full\"\n      },\n      \"Skills\": {\n        \"settings\": [\"Skill-A\", \"Skill-B\"],\n        \"strategy\": \"incremental\"\n      }\n    }\n  }\n}\n"), 0o644); err != nil {
		t.Fatalf("write mode index: %v", err)
	}

	mustRun(t, executablePath, []string{"sync", "-p", "OpenCode"})
	mustRun(t, executablePath, []string{"switch", "OpenCode"})
	mustRun(t, executablePath, []string{"apply", "-c", "opencode.json", "-s", "GPT.json"})
	assertSymlinkPointsTo(t, filepath.Join(configDir, "opencode.json"), filepath.Join(warehouseRoot, "Column", "opencode.json", "GPT.json"))

	mustRun(t, executablePath, []string{"apply", "-m", "Max"})
	assertSymlinkPointsTo(t, filepath.Join(configDir, "opencode.json"), filepath.Join(warehouseRoot, "Column", "opencode.json", "CLAUDE.json"))
	assertSymlinkPointsTo(t, filepath.Join(configDir, "skills", "Skill-A"), filepath.Join(warehouseRoot, "Column", "Skills", "Skill-A"))
	assertSymlinkPointsTo(t, filepath.Join(configDir, "skills", "Skill-B"), filepath.Join(warehouseRoot, "Column", "Skills", "Skill-B"))

	mustRun(t, executablePath, []string{"revert"})
	assertSymlinkPointsTo(t, filepath.Join(configDir, "opencode.json"), filepath.Join(warehouseRoot, "Column", "opencode.json", "GPT.json"))
	if _, err := os.Lstat(filepath.Join(configDir, "skills", "Skill-A")); !os.IsNotExist(err) {
		t.Fatalf("Skill-A should be removed after revert, err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(configDir, "skills", "Skill-B")); !os.IsNotExist(err) {
		t.Fatalf("Skill-B should be removed after revert, err=%v", err)
	}

	mustRun(t, executablePath, []string{"reset"})
	if _, err := os.Lstat(filepath.Join(configDir, "opencode.json")); !os.IsNotExist(err) {
		t.Fatalf("opencode.json should be removed after reset, err=%v", err)
	}
}

func mustRun(t *testing.T, executablePath string, args []string) string {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := RunWithExecutable(args, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("run %v exit=%d stderr=%q", args, exitCode, stderr.String())
	}
	return stdout.String()
}

func assertSymlinkPointsTo(t *testing.T, path string, want string) {
	t.Helper()
	got, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("readlink %s: %v", path, err)
	}
	if got != want {
		t.Fatalf("readlink(%s) = %q, want %q", path, got, want)
	}
}
