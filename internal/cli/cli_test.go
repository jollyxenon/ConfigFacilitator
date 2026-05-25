package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func assertGeneratedIndexHasTrailingExampleComment(t *testing.T, data []byte, want []string) {
	t.Helper()
	if bytes.Contains(data, []byte("//")) {
		t.Fatalf("expected generated index body without inline comments, got %q", string(data))
	}
	if !bytes.Contains(data, []byte("/*")) || !bytes.Contains(data, []byte("*/")) {
		t.Fatalf("expected generated index to end with a block comment, got %q", string(data))
	}
	if bytes.Count(data, []byte("/*")) != 1 || bytes.Count(data, []byte("*/")) != 1 {
		t.Fatalf("expected exactly one trailing example block, got %q", string(data))
	}
	for _, required := range want {
		if !bytes.Contains(data, []byte(required)) {
			t.Fatalf("expected generated index to contain %q, got %q", required, string(data))
		}
	}
	if !bytes.HasSuffix(data, []byte("*/\n")) {
		t.Fatalf("expected generated index block comment at file end, got %q", string(data))
	}
}

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

func TestRunShowsCommandHelpForRegisteredCommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{name: "new --help", args: []string{"new", "--help"}, want: []string{"Scaffold project, column, and mode templates.", "Usage:", "cfgfc new -p <project>", "Flags:", "-p <project>", "Examples:", "cfgfc new -p OpenCode"}},
		{name: "new -h", args: []string{"new", "-h"}, want: []string{"Scaffold project, column, and mode templates.", "cfgfc new -c <column>"}},
		{name: "sync --help", args: []string{"sync", "--help"}, want: []string{"Reconcile warehouse indexes with filesystem state.", "cfgfc sync --all", "Flags:", "--all", "-a", "Examples:", "cfgfc sync -a"}},
		{name: "switch --help", args: []string{"switch", "--help"}, want: []string{"Select the active project context for this session.", "cfgfc switch global", "Notes:", "PPID-scoped", "Examples:", "cfgfc switch OpenCode"}},
		{name: "list --help", args: []string{"list", "--help"}, want: []string{"Inspect projects, columns, modes, and settings.", "cfgfc list -p <project> -m <mode>", "`list` accepts only one detailed target at a time: `-c` or `-m`.", "cfgfc list -p OpenCode -c Skills"}},
		{name: "apply --help", args: []string{"apply", "--help"}, want: []string{"Activate a mode or explicit settings selection.", "cfgfc apply -p <project> -m <mode>", "-s <settings>", "cfgfc apply -p OpenCode -c opencode.json -s GPT.json"}},
		{name: "reset --help", args: []string{"reset", "--help"}, want: []string{"Remove the current project's managed links.", "cfgfc reset -p <project>", "cfgfc reset -p OpenCode"}},
		{name: "revert --help", args: []string{"revert", "--help"}, want: []string{"Restore the previous apply state for a project.", "cfgfc revert -p <project>", "cfgfc revert -p OpenCode"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			exitCode := Run(tt.args, &stdout, &stderr)

			if exitCode != 0 {
				t.Fatalf("expected exit code 0, got %d stderr=%q", exitCode, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected empty stderr, got %q", stderr.String())
			}
			output := stdout.String()
			for _, required := range tt.want {
				if !bytes.Contains(stdout.Bytes(), []byte(required)) {
					t.Fatalf("expected help output to contain %q, got %q", required, output)
				}
			}
		})
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
	if !bytes.Contains(stdout.Bytes(), []byte("Created project scaffold \"OpenCode\"")) {
		t.Fatalf("unexpected new project stdout %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".configfacilitator", "OpenCode", "Column", "ColumnIndex.jsonc")); err != nil {
		t.Fatalf("project scaffold missing column index: %v", err)
	}
	projectIndexData, err := os.ReadFile(filepath.Join(homeDir, ".configfacilitator", "ProjectIndex.jsonc"))
	if err != nil {
		t.Fatalf("read project index: %v", err)
	}
	assertGeneratedIndexHasTrailingExampleComment(t, projectIndexData, []string{"\"OpenCode\"", "Example:"})
	columnIndexData, err := os.ReadFile(filepath.Join(homeDir, ".configfacilitator", "OpenCode", "Column", "ColumnIndex.jsonc"))
	if err != nil {
		t.Fatalf("read column index: %v", err)
	}
	assertGeneratedIndexHasTrailingExampleComment(t, columnIndexData, []string{"Example:", "\"Skills\""})

	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithExecutable([]string{"new", "-p", "OpenCode", "-c", "Skills"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("new column exit code = %d, stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Created column scaffold \"Skills\" in project \"OpenCode\"")) {
		t.Fatalf("unexpected new column stdout %q", stdout.String())
	}
	settingIndexPath := filepath.Join(homeDir, ".configfacilitator", "OpenCode", "Column", "Skills", "SettingIndex.jsonc")
	settingIndexData, err := os.ReadFile(settingIndexPath)
	if err != nil {
		t.Fatalf("read setting index: %v", err)
	}
	assertGeneratedIndexHasTrailingExampleComment(t, settingIndexData, []string{"ExampleSetting", "\"settings\": {}"})

	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithExecutable([]string{"new", "-p", "OpenCode", "-m", "Max"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("new mode exit code = %d, stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Created mode scaffold \"Max\" in project \"OpenCode\"")) {
		t.Fatalf("unexpected new mode stdout %q", stdout.String())
	}
	modeIndexData, err := os.ReadFile(filepath.Join(homeDir, ".configfacilitator", "OpenCode", "Mode", "ModeIndex.jsonc"))
	if err != nil {
		t.Fatalf("read mode index: %v", err)
	}
	assertGeneratedIndexHasTrailingExampleComment(t, modeIndexData, []string{"\"Max\"", "\"Skills\"", "\"settings\": []", "Example:"})
}

func TestRunWithExecutableSyncsWarehouseIndexes(t *testing.T) {
	workspace := t.TempDir()
	homeDir := setTempHome(t, workspace)
	executablePath := filepath.Join(workspace, "cfgfc")
	warehouseRoot := filepath.Join(homeDir, ".configfacilitator")
	projectRoot := filepath.Join(warehouseRoot, "OpenCode")
	for _, path := range []string{
		filepath.Join(projectRoot, "Column", "Skills"),
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
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "Skills", "SettingIndex.jsonc"), []byte("{\n  \"description\": \"Skills\",\n  \"settings\": {\n    \"MissingSkill\": {\n      \"description\": \"keep me\",\n      \"missing\": true\n    }\n  }\n}\n\n/*\nExample block should disappear after sync.\n*/\n"), 0o644); err != nil {
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
	if bytes.Contains(settingIndexData, []byte("Example block should disappear after sync.")) {
		t.Fatalf("expected sync to discard generated example block, got %q", string(settingIndexData))
	}
}

func TestRunWithExecutableSyncIncludesSettingWarehouseDirectory(t *testing.T) {
	workspace := t.TempDir()
	homeDir := setTempHome(t, workspace)
	executablePath := filepath.Join(workspace, "cfgfc")
	warehouseRoot := filepath.Join(homeDir, ".configfacilitator")
	projectRoot := filepath.Join(warehouseRoot, "OpenCode")
	for _, path := range []string{
		filepath.Join(projectRoot, "Column", "Skills"),
		filepath.Join(projectRoot, "Column", "Skills"),
		filepath.Join(projectRoot, "Mode"),
		filepath.Join(projectRoot, "Backup"),
		filepath.Join(warehouseRoot, "SettingWarehouse", "LegacyProject", "Column"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}
	if err := os.WriteFile(filepath.Join(warehouseRoot, "ProjectIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write project index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write column index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "Skills", "SettingIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write setting index: %v", err)
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
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "Skills", "Skill-A"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write discovered setting file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := RunWithExecutable([]string{"sync", "--all"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("sync --all exit=%d stderr=%q", exitCode, stderr.String())
	}
	projectIndexData, err := os.ReadFile(filepath.Join(warehouseRoot, "ProjectIndex.jsonc"))
	if err != nil {
		t.Fatalf("read project index: %v", err)
	}
	if !bytes.Contains(projectIndexData, []byte("\"OpenCode\"")) {
		t.Fatalf("expected synced project index to include OpenCode, got %q", string(projectIndexData))
	}
	if !bytes.Contains(projectIndexData, []byte("SettingWarehouse")) {
		t.Fatalf("expected SettingWarehouse directory to be included, got %q", string(projectIndexData))
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
	if err := os.WriteFile(filepath.Join(homeDir, ".configfacilitator", "OpenCode", "Column", "Skills", "Skill-A"), []byte("x"), 0o644); err != nil {
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

func TestRunWithExecutableListUsesDisplayNamesAndCanonicalIdentifiers(t *testing.T) {
	workspace := t.TempDir()
	homeDir := setTempHome(t, workspace)
	executablePath := filepath.Join(workspace, "cfgfc")
	warehouseRoot := filepath.Join(homeDir, ".configfacilitator")
	projectRoot := filepath.Join(warehouseRoot, "OpenCode")
	for _, path := range []string{
		filepath.Join(projectRoot, "Column", "Skills"),
		filepath.Join(projectRoot, "Mode"),
		filepath.Join(projectRoot, "Backup"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}
	if err := os.WriteFile(filepath.Join(warehouseRoot, "ProjectIndex.jsonc"), []byte(`{
	  "OpenCode": {
	    "displayName": "Open Code",
	    "aliases": ["oc"]
	  }
	}
`), 0o644); err != nil {
		t.Fatalf("write project index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte(`{
	  "Skills": {
	    "displayName": "Skills Display"
	  }
	}
`), 0o644); err != nil {
		t.Fatalf("write column index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "Skills", "SettingIndex.jsonc"), []byte(`{
	  "settings": {
	    "Skill-A": {
	      "displayName": "Skill Alpha",
	      "target": "/tmp/skill-a"
	    }
	  }
	}
`), 0o644); err != nil {
		t.Fatalf("write setting index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "Skills", "Skill-A"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write setting file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Mode", "ModeIndex.jsonc"), []byte(`{
	  "Max": {
	    "displayName": "Maximum Mode",
	    "columns": {
	      "Skills": {
	        "settings": ["Skill-A"],
	        "strategy": "cover"
	      }
	    }
	  }
	}
`), 0o644); err != nil {
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
	if exitCode := RunWithExecutable([]string{"switch", "oc"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("switch exit=%d stderr=%q", exitCode, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithExecutable([]string{"list"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("list exit=%d stderr=%q", exitCode, stderr.String())
	}
	if got := stdout.String(); !bytes.Contains(stdout.Bytes(), []byte("Project: Open Code [OpenCode]")) || !bytes.Contains(stdout.Bytes(), []byte("Skills Display [Skills]")) || !bytes.Contains(stdout.Bytes(), []byte("Maximum Mode [Max]")) {
		t.Fatalf("expected list output to use display labels, got %q", got)
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithExecutable([]string{"list", "-c", "Skills"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("list column exit=%d stderr=%q", exitCode, stderr.String())
	}
	if got := stdout.String(); !bytes.Contains(stdout.Bytes(), []byte("Column: Skills Display [Skills]")) || !bytes.Contains(stdout.Bytes(), []byte("Skill Alpha [Skill-A] (present)")) {
		t.Fatalf("expected column list to use display labels, got %q", got)
	}
}

func TestRunWithExecutableSwitchGlobalClearsConvenienceContext(t *testing.T) {
	workspace := t.TempDir()
	homeDir := setTempHome(t, workspace)
	executablePath := filepath.Join(workspace, "cfgfc")
	mustRun(t, executablePath, []string{"new", "-p", "OpenCode"})
	mustRun(t, executablePath, []string{"new", "-p", "ClaudeCode"})
	mustRun(t, executablePath, []string{"switch", "OpenCode"})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := RunWithExecutable([]string{"switch", "global"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("switch global exit=%d stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Switched to global context")) {
		t.Fatalf("unexpected switch global stdout %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithExecutable([]string{"list"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("list after switch global exit=%d stderr=%q", exitCode, stderr.String())
	}
	if bytes.Contains(stdout.Bytes(), []byte("Project: OpenCode")) {
		t.Fatalf("expected global list view instead of project view, got %q", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("OpenCode")) || !bytes.Contains(stdout.Bytes(), []byte("ClaudeCode")) {
		t.Fatalf("expected project list after switch global, got %q", stdout.String())
	}

	if _, err := os.Stat(filepath.Join(homeDir, ".configfacilitator", ".cfgfc-session")); err != nil {
		t.Fatalf("expected session directory to remain addressable: %v", err)
	}
}

func TestRunWithExecutableNewUsesConvenienceContextAndExplicitProjectWins(t *testing.T) {
	workspace := t.TempDir()
	homeDir := setTempHome(t, workspace)
	executablePath := filepath.Join(workspace, "cfgfc")

	mustRun(t, executablePath, []string{"new", "-p", "OpenCode"})
	mustRun(t, executablePath, []string{"new", "-p", "ClaudeCode"})
	mustRun(t, executablePath, []string{"switch", "OpenCode"})
	mustRun(t, executablePath, []string{"new", "-c", "Skills"})
	mustRun(t, executablePath, []string{"new", "-p", "ClaudeCode", "-c", "Agents"})
	mustRun(t, executablePath, []string{"new", "-m", "Max"})

	if _, err := os.Stat(filepath.Join(homeDir, ".configfacilitator", "OpenCode", "Column", "Skills", "SettingIndex.jsonc")); err != nil {
		t.Fatalf("expected switched-context column scaffold in OpenCode: %v", err)
	}
	modeIndexData, err := os.ReadFile(filepath.Join(homeDir, ".configfacilitator", "OpenCode", "Mode", "ModeIndex.jsonc"))
	if err != nil {
		t.Fatalf("read switched-context mode scaffold: %v", err)
	}
	if !bytes.Contains(modeIndexData, []byte("\"Max\"")) {
		t.Fatalf("expected switched-context mode scaffold to include Max, got %q", string(modeIndexData))
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".configfacilitator", "ClaudeCode", "Column", "Agents", "SettingIndex.jsonc")); err != nil {
		t.Fatalf("expected explicit-project column scaffold in ClaudeCode: %v", err)
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".configfacilitator", "ClaudeCode", "Column", "Skills")); !os.IsNotExist(err) {
		t.Fatalf("did not expect switched-context scaffold to leak into ClaudeCode, err=%v", err)
	}
}

func TestRunWithExecutableNewRejectsReservedGlobalProjectName(t *testing.T) {
	workspace := t.TempDir()
	setTempHome(t, workspace)
	executablePath := filepath.Join(workspace, "cfgfc")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := RunWithExecutable([]string{"new", "-p", "global"}, &stdout, &stderr, executablePath); exitCode != 1 {
		t.Fatalf("new -p global exit=%d stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("project name \"global\" is reserved")) {
		t.Fatalf("expected reserved-name error, got %q", stderr.String())
	}
}

func TestRunWithExecutableSyncUsesConvenienceContextAndFallsBackToAllProjects(t *testing.T) {
	workspace := t.TempDir()
	homeDir := setTempHome(t, workspace)
	executablePath := filepath.Join(workspace, "cfgfc")

	mustRun(t, executablePath, []string{"new", "-p", "OpenCode"})
	mustRun(t, executablePath, []string{"new", "-p", "ClaudeCode"})
	mustRun(t, executablePath, []string{"new", "-p", "OpenCode", "-c", "Skills"})
	mustRun(t, executablePath, []string{"new", "-p", "ClaudeCode", "-c", "Agents"})

	openCodeSetting := filepath.Join(homeDir, ".configfacilitator", "OpenCode", "Column", "Skills", "Skill-A")
	claudeCodeSetting := filepath.Join(homeDir, ".configfacilitator", "ClaudeCode", "Column", "Agents", "Agent-A")
	if err := os.WriteFile(openCodeSetting, []byte("open"), 0o644); err != nil {
		t.Fatalf("write OpenCode setting: %v", err)
	}
	if err := os.WriteFile(claudeCodeSetting, []byte("claude"), 0o644); err != nil {
		t.Fatalf("write ClaudeCode setting: %v", err)
	}

	mustRun(t, executablePath, []string{"switch", "OpenCode"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := RunWithExecutable([]string{"sync"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("context sync exit=%d stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Synchronized project \"OpenCode\"")) {
		t.Fatalf("expected context sync to target OpenCode, got %q", stdout.String())
	}

	openCodeIndex, err := os.ReadFile(filepath.Join(homeDir, ".configfacilitator", "OpenCode", "Column", "Skills", "SettingIndex.jsonc"))
	if err != nil {
		t.Fatalf("read OpenCode setting index: %v", err)
	}
	if !bytes.Contains(openCodeIndex, []byte("Skill-A")) {
		t.Fatalf("expected OpenCode sync to discover Skill-A, got %q", string(openCodeIndex))
	}
	claudeCodeIndex, err := os.ReadFile(filepath.Join(homeDir, ".configfacilitator", "ClaudeCode", "Column", "Agents", "SettingIndex.jsonc"))
	if err != nil {
		t.Fatalf("read ClaudeCode setting index: %v", err)
	}
	if bytes.Contains(claudeCodeIndex, []byte("Agent-A")) {
		t.Fatalf("did not expect context sync to touch ClaudeCode, got %q", string(claudeCodeIndex))
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithExecutable([]string{"sync", "-p", "ClaudeCode"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("explicit sync exit=%d stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Synchronized project \"ClaudeCode\"")) {
		t.Fatalf("expected explicit sync override to target ClaudeCode, got %q", stdout.String())
	}
	claudeCodeIndex, err = os.ReadFile(filepath.Join(homeDir, ".configfacilitator", "ClaudeCode", "Column", "Agents", "SettingIndex.jsonc"))
	if err != nil {
		t.Fatalf("read ClaudeCode setting index after explicit override: %v", err)
	}
	if !bytes.Contains(claudeCodeIndex, []byte("Agent-A")) {
		t.Fatalf("expected explicit sync override to discover Agent-A, got %q", string(claudeCodeIndex))
	}

	otherWorkspace := t.TempDir()
	otherHome := setTempHome(t, otherWorkspace)
	otherExecutablePath := filepath.Join(otherWorkspace, "cfgfc")
	mustRun(t, otherExecutablePath, []string{"new", "-p", "OpenCode"})
	mustRun(t, otherExecutablePath, []string{"new", "-p", "ClaudeCode"})
	mustRun(t, otherExecutablePath, []string{"new", "-p", "OpenCode", "-c", "Skills"})
	mustRun(t, otherExecutablePath, []string{"new", "-p", "ClaudeCode", "-c", "Agents"})
	if err := os.WriteFile(filepath.Join(otherHome, ".configfacilitator", "OpenCode", "Column", "Skills", "Skill-A"), []byte("open"), 0o644); err != nil {
		t.Fatalf("write other OpenCode setting: %v", err)
	}
	if err := os.WriteFile(filepath.Join(otherHome, ".configfacilitator", "ClaudeCode", "Column", "Agents", "Agent-A"), []byte("claude"), 0o644); err != nil {
		t.Fatalf("write other ClaudeCode setting: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithExecutable([]string{"sync"}, &stdout, &stderr, otherExecutablePath); exitCode != 0 {
		t.Fatalf("global sync exit=%d stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Synchronized all projects")) {
		t.Fatalf("expected global sync fallback, got %q", stdout.String())
	}

	otherOpenCodeIndex, err := os.ReadFile(filepath.Join(otherHome, ".configfacilitator", "OpenCode", "Column", "Skills", "SettingIndex.jsonc"))
	if err != nil {
		t.Fatalf("read other OpenCode setting index: %v", err)
	}
	otherClaudeCodeIndex, err := os.ReadFile(filepath.Join(otherHome, ".configfacilitator", "ClaudeCode", "Column", "Agents", "SettingIndex.jsonc"))
	if err != nil {
		t.Fatalf("read other ClaudeCode setting index: %v", err)
	}
	if !bytes.Contains(otherOpenCodeIndex, []byte("Skill-A")) || !bytes.Contains(otherClaudeCodeIndex, []byte("Agent-A")) {
		t.Fatalf("expected global sync to update all projects, OpenCode=%q ClaudeCode=%q", string(otherOpenCodeIndex), string(otherClaudeCodeIndex))
	}
}

func TestRunWithExecutableSyncSupportsAllFlagAndRejectsReservedGlobalProject(t *testing.T) {
	workspace := t.TempDir()
	homeDir := setTempHome(t, workspace)
	executablePath := filepath.Join(workspace, "cfgfc")

	mustRun(t, executablePath, []string{"new", "-p", "OpenCode"})
	mustRun(t, executablePath, []string{"new", "-p", "ClaudeCode"})
	mustRun(t, executablePath, []string{"new", "-p", "OpenCode", "-c", "Skills"})
	mustRun(t, executablePath, []string{"new", "-p", "ClaudeCode", "-c", "Agents"})
	if err := os.WriteFile(filepath.Join(homeDir, ".configfacilitator", "OpenCode", "Column", "Skills", "Skill-A"), []byte("open"), 0o644); err != nil {
		t.Fatalf("write OpenCode setting: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".configfacilitator", "ClaudeCode", "Column", "Agents", "Agent-A"), []byte("claude"), 0o644); err != nil {
		t.Fatalf("write ClaudeCode setting: %v", err)
	}

	mustRun(t, executablePath, []string{"switch", "OpenCode"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := RunWithExecutable([]string{"sync", "--all"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("sync --all exit=%d stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Synchronized all projects")) {
		t.Fatalf("expected --all sync output, got %q", stdout.String())
	}

	openCodeIndex, err := os.ReadFile(filepath.Join(homeDir, ".configfacilitator", "OpenCode", "Column", "Skills", "SettingIndex.jsonc"))
	if err != nil {
		t.Fatalf("read OpenCode setting index: %v", err)
	}
	claudeCodeIndex, err := os.ReadFile(filepath.Join(homeDir, ".configfacilitator", "ClaudeCode", "Column", "Agents", "SettingIndex.jsonc"))
	if err != nil {
		t.Fatalf("read ClaudeCode setting index: %v", err)
	}
	if !bytes.Contains(openCodeIndex, []byte("Skill-A")) || !bytes.Contains(claudeCodeIndex, []byte("Agent-A")) {
		t.Fatalf("expected --all sync to update all projects, OpenCode=%q ClaudeCode=%q", string(openCodeIndex), string(claudeCodeIndex))
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithExecutable([]string{"sync", "-a"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("sync -a exit=%d stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Synchronized all projects")) {
		t.Fatalf("expected -a sync output, got %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithExecutable([]string{"sync", "-p", "global"}, &stdout, &stderr, executablePath); exitCode != 1 {
		t.Fatalf("sync -p global exit=%d stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("project name \"global\" is reserved")) {
		t.Fatalf("expected reserved-name error, got %q", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithExecutable([]string{"sync", "-p", "OpenCode", "--all"}, &stdout, &stderr, executablePath); exitCode != 1 {
		t.Fatalf("sync -p OpenCode --all exit=%d stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("sync does not accept -p together with --all or -a")) {
		t.Fatalf("expected conflict error, got %q", stderr.String())
	}
}

func TestRunWithExecutableSyncFallsBackToAllProjectsAfterSwitchGlobal(t *testing.T) {
	workspace := t.TempDir()
	homeDir := setTempHome(t, workspace)
	executablePath := filepath.Join(workspace, "cfgfc")

	mustRun(t, executablePath, []string{"new", "-p", "OpenCode"})
	mustRun(t, executablePath, []string{"new", "-p", "ClaudeCode"})
	mustRun(t, executablePath, []string{"new", "-p", "OpenCode", "-c", "Skills"})
	mustRun(t, executablePath, []string{"new", "-p", "ClaudeCode", "-c", "Agents"})
	if err := os.WriteFile(filepath.Join(homeDir, ".configfacilitator", "OpenCode", "Column", "Skills", "Skill-A"), []byte("open"), 0o644); err != nil {
		t.Fatalf("write OpenCode setting: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".configfacilitator", "ClaudeCode", "Column", "Agents", "Agent-A"), []byte("claude"), 0o644); err != nil {
		t.Fatalf("write ClaudeCode setting: %v", err)
	}

	mustRun(t, executablePath, []string{"switch", "OpenCode"})
	mustRun(t, executablePath, []string{"switch", "global"})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := RunWithExecutable([]string{"sync"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("sync after switch global exit=%d stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Synchronized all projects")) {
		t.Fatalf("expected sync fallback after switch global, got %q", stdout.String())
	}

	openCodeIndex, err := os.ReadFile(filepath.Join(homeDir, ".configfacilitator", "OpenCode", "Column", "Skills", "SettingIndex.jsonc"))
	if err != nil {
		t.Fatalf("read OpenCode setting index: %v", err)
	}
	claudeCodeIndex, err := os.ReadFile(filepath.Join(homeDir, ".configfacilitator", "ClaudeCode", "Column", "Agents", "SettingIndex.jsonc"))
	if err != nil {
		t.Fatalf("read ClaudeCode setting index: %v", err)
	}
	if !bytes.Contains(openCodeIndex, []byte("Skill-A")) || !bytes.Contains(claudeCodeIndex, []byte("Agent-A")) {
		t.Fatalf("expected fallback sync to update all projects, OpenCode=%q ClaudeCode=%q", string(openCodeIndex), string(claudeCodeIndex))
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
	if err := os.WriteFile(filepath.Join(homeDir, ".configfacilitator", "OpenCode", "Column", "Skills", "Skill-A"), []byte("x"), 0o644); err != nil {
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
	if !bytes.Contains(stdout.Bytes(), []byte("Mode: Max")) || !bytes.Contains(stdout.Bytes(), []byte("Skills: strategy=cover")) {
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

	warehouseRoot := filepath.Join(homeDir, ".configfacilitator", "OpenCode")
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
	if err := os.WriteFile(filepath.Join(warehouseRoot, "Mode", "ModeIndex.jsonc"), []byte("{\n  \"Max\": {\n    \"displayName\": \"Max\",\n    \"columns\": {\n      \"opencode.json\": {\n        \"settings\": [\"CLAUDE.json\"],\n        \"strategy\": \"cover\"\n      },\n      \"Skills\": {\n        \"settings\": [\"Skill-A\", \"Skill-B\"],\n        \"strategy\": \"increment\"\n      }\n    }\n  }\n}\n"), 0o644); err != nil {
		t.Fatalf("write mode index: %v", err)
	}

	mustRun(t, executablePath, []string{"sync", "-p", "OpenCode"})
	if got := mustRun(t, executablePath, []string{"switch", "OpenCode"}); !bytes.Contains([]byte(got), []byte("Switched active project to \"OpenCode\"")) {
		t.Fatalf("unexpected switch stdout %q", got)
	}
	if got := mustRun(t, executablePath, []string{"apply", "-c", "opencode.json", "-s", "GPT.json"}); !bytes.Contains([]byte(got), []byte("Applied column \"opencode.json\" for project \"OpenCode\"")) {
		t.Fatalf("unexpected apply-column stdout %q", got)
	}
	assertFileSymlinkPointsTo(t, filepath.Join(configDir, "opencode.json"), filepath.Join(warehouseRoot, "Column", "opencode.json", "GPT.json"))

	if got := mustRun(t, executablePath, []string{"apply", "-m", "Max"}); !bytes.Contains([]byte(got), []byte("Applied mode \"Max\" for project \"OpenCode\"")) {
		t.Fatalf("unexpected apply-mode stdout %q", got)
	}
	assertFileSymlinkPointsTo(t, filepath.Join(configDir, "opencode.json"), filepath.Join(warehouseRoot, "Column", "opencode.json", "CLAUDE.json"))
	assertSymlinkPointsTo(t, filepath.Join(configDir, "skills", "Skill-A"), filepath.Join(warehouseRoot, "Column", "Skills", "Skill-A"))
	assertSymlinkPointsTo(t, filepath.Join(configDir, "skills", "Skill-B"), filepath.Join(warehouseRoot, "Column", "Skills", "Skill-B"))

	if got := mustRun(t, executablePath, []string{"revert"}); !bytes.Contains([]byte(got), []byte("Reverted project \"OpenCode\"")) {
		t.Fatalf("unexpected revert stdout %q", got)
	}
	assertFileSymlinkPointsTo(t, filepath.Join(configDir, "opencode.json"), filepath.Join(warehouseRoot, "Column", "opencode.json", "GPT.json"))
	if _, err := os.Lstat(filepath.Join(configDir, "skills", "Skill-A")); !os.IsNotExist(err) {
		t.Fatalf("Skill-A should be removed after revert, err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(configDir, "skills", "Skill-B")); !os.IsNotExist(err) {
		t.Fatalf("Skill-B should be removed after revert, err=%v", err)
	}

	if got := mustRun(t, executablePath, []string{"reset"}); !bytes.Contains([]byte(got), []byte("Reset project \"OpenCode\"")) {
		t.Fatalf("unexpected reset stdout %q", got)
	}
	if _, err := os.Lstat(filepath.Join(configDir, "opencode.json")); !os.IsNotExist(err) {
		t.Fatalf("opencode.json should be removed after reset, err=%v", err)
	}
}

func TestRunWithExecutableResolvesAliasesAndStoresCanonicalProjectContext(t *testing.T) {
	workspace := t.TempDir()
	homeDir := setTempHome(t, workspace)
	executablePath := filepath.Join(workspace, "cfgfc")
	warehouseRoot := filepath.Join(homeDir, ".configfacilitator")
	projectRoot := filepath.Join(warehouseRoot, "OpenCodeFolder")
	for _, path := range []string{
		filepath.Join(projectRoot, "Column", "skills-dir"),
		filepath.Join(projectRoot, "Mode"),
		filepath.Join(projectRoot, "Backup"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}
	if err := os.WriteFile(filepath.Join(warehouseRoot, "ProjectIndex.jsonc"), []byte(`{
  "OpenCodeFolder": {
	    "displayName": "Open Code",
	    "aliases": ["oc"]
  }
}
`), 0o644); err != nil {
		t.Fatalf("write project index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte(`{
  "skills-dir": {
	    "displayName": "Skills",
	    "aliases": ["skills"]
  }
}
`), 0o644); err != nil {
		t.Fatalf("write column index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "skills-dir", "SettingIndex.jsonc"), []byte(`{
  "settings": {
    "Skill-A": {
	      "displayName": "Skill A",
	      "aliases": ["alpha"]
    }
  }
}
`), 0o644); err != nil {
		t.Fatalf("write setting index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Mode", "ModeIndex.jsonc"), []byte(`{
  "Max": {
	    "displayName": "Max",
	    "aliases": ["m"],
    "columns": {
	      "skills": {
	        "settings": ["alpha"],
	        "strategy": "cover"
	      }
    }
  }
}
`), 0o644); err != nil {
		t.Fatalf("write mode index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "current_state.json"), []byte("{\"mappings\": []}\n"), 0o644); err != nil {
		t.Fatalf("write current state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "history.log"), []byte{}, 0o644); err != nil {
		t.Fatalf("write history log: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "skills-dir", "Skill-A"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write setting file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := RunWithExecutable([]string{"switch", "oc"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("switch alias exit=%d stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`Open Code`)) {
		t.Fatalf("expected display name in switch output, got %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithExecutable([]string{"list", "-c", "skills"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("list alias exit=%d stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Column: Skills [skills-dir]")) || !bytes.Contains(stdout.Bytes(), []byte("Skill A [Skill-A]")) {
		t.Fatalf("unexpected list alias stdout %q", stdout.String())
	}
}

func TestRunWithExecutableRejectsExplicitIDField(t *testing.T) {
	workspace := t.TempDir()
	homeDir := setTempHome(t, workspace)
	executablePath := filepath.Join(workspace, "cfgfc")
	warehouseRoot := filepath.Join(homeDir, ".configfacilitator")
	projectRoot := filepath.Join(warehouseRoot, "OpenCode")
	for _, path := range []string{
		filepath.Join(projectRoot, "Column"),
		filepath.Join(projectRoot, "Mode"),
		filepath.Join(projectRoot, "Backup"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}
	if err := os.WriteFile(filepath.Join(warehouseRoot, "ProjectIndex.jsonc"), []byte(`{
  "OpenCode": {
    "id": "open-code"
  }
}
`), 0o644); err != nil {
		t.Fatalf("write project index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write column index: %v", err)
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
	if exitCode := RunWithExecutable([]string{"list"}, &stdout, &stderr, executablePath); exitCode != 1 {
		t.Fatalf("list exit=%d stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("id")) {
		t.Fatalf("expected explicit id rejection, got %q", stderr.String())
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

func assertFileSymlinkPointsTo(t *testing.T, path string, want string) {
	t.Helper()
	assertSymlinkPointsTo(t, path, want)

	gotContent, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file through symlink %s: %v", path, err)
	}
	wantContent, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read source file %s: %v", want, err)
	}
	if !bytes.Equal(gotContent, wantContent) {
		t.Fatalf("file content via symlink %s = %q, want source content %q", path, string(gotContent), string(wantContent))
	}
}
