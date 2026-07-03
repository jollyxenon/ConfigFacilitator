package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/linker"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
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
	for _, required := range []string{"cfgfc manages portable configuration warehouses.", "new", "sync", "switch", "list", "apply", "update", "reset", "revert", "root", "Warehouse root:", "cfgfc root <path>", "Development:", "pixi run compile", "pixi run build", "dist/cfgfc"} {
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
		{name: "list --help", args: []string{"list", "--help"}, want: []string{"Inspect projects, columns, modes, and current usage status.", "cfgfc list -p <project>", "persisted mode name", "(Unmatched)", "`(Full)` / `(Partial)` / `(None)`", "highlights enabled settings", "cfgfc list -c Skills"}},
		{name: "apply --help", args: []string{"apply", "--help"}, want: []string{"Activate a mode or explicit settings selection.", "cfgfc apply -p <project> -m <mode>", "-s <settings>", "-f, --force", "last confirmed managed state", "cfgfc apply -p OpenCode -c opencode.json -s GPT.json"}},
		{name: "update --help", args: []string{"update", "--help"}, want: []string{"Refresh the last applied intent from current warehouse metadata.", "cfgfc update --project <project>", "--column <column>", "--all", "-f, --force", "persisted mode or column apply intent", "no apply intent is recorded", "cfgfc update -c Skills"}},
		{name: "reset --help", args: []string{"reset", "--help"}, want: []string{"Remove the current project's managed links.", "cfgfc reset -p <project>", "-f, --force", "recorded target path", "cfgfc reset -p OpenCode"}},
		{name: "revert --help", args: []string{"revert", "--help"}, want: []string{"Restore the previous apply state for a project.", "cfgfc revert -p <project>", "-f, --force", "overwritten unmanaged contents", "cfgfc revert -p OpenCode"}},
		{name: "root --help", args: []string{"root", "--help"}, want: []string{"Inspect or change the persistent warehouse root.", "cfgfc root", "cfgfc root <path>", "current effective warehouse root", "does not migrate, copy, or initialize warehouse contents", "cfgfc root ~/.configfacilitator-alt"}},
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

func TestRunRootPrintsEffectiveWarehouseRoot(t *testing.T) {
	workspace := t.TempDir()
	homeDir := setTempHome(t, workspace)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"root"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
	want := filepath.Join(homeDir, ".configfacilitator") + "\n"
	if stdout.String() != want {
		t.Fatalf("expected root output %q, got %q", want, stdout.String())
	}
}

func TestRunRootSetPersistsNormalizedRootAndRecoversFromBadBootstrap(t *testing.T) {
	workspace := t.TempDir()
	homeDir := setTempHome(t, workspace)
	bootstrapPath := filepath.Join(homeDir, ".cfgfc-root")
	if err := os.WriteFile(bootstrapPath, []byte("relative/bootstrap\n"), 0o644); err != nil {
		t.Fatalf("write malformed bootstrap: %v", err)
	}
	requestedRoot := "~/warehouse/../alternate-root"
	wantRoot := filepath.Join(homeDir, "alternate-root")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"root", requestedRoot}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(wantRoot)) {
		t.Fatalf("expected normalized root in stdout, got %q", stdout.String())
	}
	bootstrapData, err := os.ReadFile(bootstrapPath)
	if err != nil {
		t.Fatalf("read bootstrap: %v", err)
	}
	if string(bootstrapData) != wantRoot+"\n" {
		t.Fatalf("expected bootstrap contents %q, got %q", wantRoot+"\n", string(bootstrapData))
	}
}

func TestRunRootRejectsExtraPathArguments(t *testing.T) {
	workspace := t.TempDir()
	setTempHome(t, workspace)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"root", "one", "two"}, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("root accepts zero arguments")) {
		t.Fatalf("expected extra-arg rejection, got %q", stderr.String())
	}
}

func TestRunReportsMissingProjectForUpdateColumnWithoutContext(t *testing.T) {
	workspace := t.TempDir()
	setTempHome(t, workspace)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"update", "-c", "Skills"}, &stdout, &stderr)

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

func TestRunUpdateRejectsInvalidScopeArgs(t *testing.T) {
	workspace := t.TempDir()
	setTempHome(t, workspace)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "all with project", args: []string{"update", "--all", "--project", "OpenCode"}, want: "together with"},
		{name: "all with column", args: []string{"update", "--column", "Skills", "-a"}, want: "together with"},
		{name: "reserved global", args: []string{"update", "--project", "global"}, want: "reserved"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			exitCode := Run(tt.args, &stdout, &stderr)

			if exitCode != 1 {
				t.Fatalf("expected exit code 1, got %d", exitCode)
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected empty stdout, got %q", stdout.String())
			}
			if !bytes.Contains(stderr.Bytes(), []byte(tt.want)) {
				t.Fatalf("expected stderr to contain %q, got %q", tt.want, stderr.String())
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
	      "targetDir": ["/tmp"],
	      "targetName": ["skill-a"]
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

func TestRunWithExecutableRootSwitchesWarehouseAndSessionStores(t *testing.T) {
	workspace := t.TempDir()
	homeDir := setTempHome(t, workspace)
	executablePath := filepath.Join(workspace, "cfgfc")
	defaultRoot := filepath.Join(homeDir, ".configfacilitator")
	alternateRoot := filepath.Join(workspace, "alternate-root")

	mustRun(t, executablePath, []string{"new", "-p", "DefaultProject"})
	mustRun(t, executablePath, []string{"switch", "DefaultProject"})
	if _, err := os.Stat(filepath.Join(defaultRoot, ".cfgfc-session")); err != nil {
		t.Fatalf("expected default-root session store: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := RunWithExecutable([]string{"root", alternateRoot}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("root set exit=%d stderr=%q", exitCode, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(defaultRoot, "DefaultProject")); err != nil {
		t.Fatalf("expected previous warehouse contents to remain in default root: %v", err)
	}
	if _, err := os.Stat(alternateRoot); !os.IsNotExist(err) {
		t.Fatalf("expected root command not to create alternate root, err=%v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithExecutable([]string{"list"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("list on empty alternate root exit=%d stderr=%q", exitCode, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty global list on untouched alternate root, got %q", stdout.String())
	}

	mustRun(t, executablePath, []string{"new", "-p", "AlternateProject"})
	mustRun(t, executablePath, []string{"switch", "AlternateProject"})
	if _, err := os.Stat(filepath.Join(alternateRoot, ".cfgfc-session")); err != nil {
		t.Fatalf("expected alternate-root session store: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithExecutable([]string{"list"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("list on alternate root exit=%d stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Project: AlternateProject")) {
		t.Fatalf("expected alternate-root session context, got %q", stdout.String())
	}

	mustRun(t, executablePath, []string{"root", defaultRoot})
	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithExecutable([]string{"list"}, &stdout, &stderr, executablePath); exitCode != 0 {
		t.Fatalf("list after returning to default root exit=%d stderr=%q", exitCode, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Project: DefaultProject")) {
		t.Fatalf("expected default-root session context after switching back, got %q", stdout.String())
	}
	if bytes.Contains(stdout.Bytes(), []byte("AlternateProject")) {
		t.Fatalf("did not expect alternate-root session context to leak into default root, got %q", stdout.String())
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

func TestRunWithExecutableGlobalListShowsUsageSummariesWithoutANSI(t *testing.T) {
	fixture := setupListStatusFixture(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := RunWithExecutable([]string{"list"}, &stdout, &stderr, fixture.executablePath); exitCode != 0 {
		t.Fatalf("list exit=%d stderr=%q", exitCode, stderr.String())
	}

	output := stdout.String()
	for _, required := range []string{"Open Code [OpenCode] (Max)", "Manual (Unmatched)", "Empty (None)"} {
		if !strings.Contains(output, required) {
			t.Fatalf("expected global list output to contain %q, got %q", required, output)
		}
	}
	assertNoANSI(t, output)
}

func TestRunWithExecutableProjectListShowsColumnUsageSummariesWithoutANSI(t *testing.T) {
	fixture := setupListStatusFixture(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := RunWithExecutable([]string{"list", "-p", "OpenCode"}, &stdout, &stderr, fixture.executablePath); exitCode != 0 {
		t.Fatalf("list -p OpenCode exit=%d stderr=%q", exitCode, stderr.String())
	}

	output := stdout.String()
	for _, required := range []string{"Config [opencode.json] (Full)", "Skills (Partial)", "Extras (None)"} {
		if !strings.Contains(output, required) {
			t.Fatalf("expected project list output to contain %q, got %q", required, output)
		}
	}
	assertNoANSI(t, output)
}

func TestRunWithExecutableProjectListShowsActiveModeWithoutANSI(t *testing.T) {
	fixture := setupListStatusFixture(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := RunWithExecutable([]string{"list", "-p", "OpenCode"}, &stdout, &stderr, fixture.executablePath); exitCode != 0 {
		t.Fatalf("list -p OpenCode exit=%d stderr=%q", exitCode, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Modes:") || !strings.Contains(output, "  - Max") {
		t.Fatalf("expected active mode to remain readable in plain text, got %q", output)
	}
	assertNoANSI(t, output)
}

func TestRunWithExecutableColumnListShowsEnabledAndMissingSettingsWithoutANSI(t *testing.T) {
	fixture := setupListStatusFixture(t)
	mustRun(t, fixture.executablePath, []string{"switch", "OpenCode"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := RunWithExecutable([]string{"list", "-c", "Skills"}, &stdout, &stderr, fixture.executablePath); exitCode != 0 {
		t.Fatalf("list -c Skills exit=%d stderr=%q", exitCode, stderr.String())
	}

	output := stdout.String()
	for _, required := range []string{"Column: Skills", "Skill A [Skill-A] (present)", "Skill B [Skill-B] (present)", "Skill Missing [Skill-Missing] (missing)"} {
		if !strings.Contains(output, required) {
			t.Fatalf("expected column list output to contain %q, got %q", required, output)
		}
	}
	assertNoANSI(t, output)
}

func TestRunWithExecutableGlobalListShowsModeDriftAsUnmatchedWithoutANSI(t *testing.T) {
	fixture := setupUpdateFixture(t)
	mustRun(t, fixture.executablePath, []string{"apply", "-p", "OpenCode", "-m", "Max"})
	writeUpdateFixtureIndexes(t, fixture.projectRoot, "~/.config/opencode/opencode.drifted.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := RunWithExecutable([]string{"list"}, &stdout, &stderr, fixture.executablePath); exitCode != 0 {
		t.Fatalf("list exit=%d stderr=%q", exitCode, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Open Code [OpenCode] (Unmatched)") {
		t.Fatalf("expected mode drift to render as unmatched, got %q", output)
	}
	if strings.Contains(output, "Open Code [OpenCode] (Max)") {
		t.Fatalf("did not expect drifted mode to remain matched, got %q", output)
	}
	assertNoANSI(t, output)
}

func TestRunWithExecutableProjectListShowsMultiTargetPartialColumnCoverageWithoutANSI(t *testing.T) {
	fixture := setupUpdateFixture(t)
	writeMultiTargetConfigSettingIndex(t, fixture.projectRoot)
	writeProjectCurrentState(t, fixture.projectRoot, linker.CurrentState{
		Mappings: []linker.Mapping{{
			Source: filepath.Join(fixture.projectRoot, "Column", "opencode.json", "GPT.json"),
			Target: filepath.Join(fixture.configDir, "opencode.primary.json"),
		}},
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := RunWithExecutable([]string{"list", "-p", "OpenCode"}, &stdout, &stderr, fixture.executablePath); exitCode != 0 {
		t.Fatalf("list -p OpenCode exit=%d stderr=%q", exitCode, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Config [opencode.json] (Partial)") {
		t.Fatalf("expected multi-target partial column coverage, got %q", output)
	}
	assertNoANSI(t, output)
}

func TestRunWithExecutableColumnListKeepsMultiTargetPartialSettingUnhighlightedWithoutANSI(t *testing.T) {
	fixture := setupUpdateFixture(t)
	state := linker.CurrentState{
		Mappings: []linker.Mapping{{
			Source: filepath.Join(fixture.projectRoot, "Column", "opencode.json", "GPT.json"),
			Target: filepath.Join(fixture.configDir, "opencode.primary.json"),
		}},
	}
	writeMultiTargetConfigSettingIndex(t, fixture.projectRoot)
	writeProjectCurrentState(t, fixture.projectRoot, state)

	loaded, err := warehouse.LoadWarehouse(filepath.Dir(fixture.projectRoot))
	if err != nil {
		t.Fatalf("load warehouse: %v", err)
	}
	project, err := loaded.ResolveProject("OpenCode")
	if err != nil {
		t.Fatalf("resolve project: %v", err)
	}
	column, err := project.ResolveColumn("opencode.json")
	if err != nil {
		t.Fatalf("resolve column: %v", err)
	}
	setting, err := column.ResolveSetting("GPT.json")
	if err != nil {
		t.Fatalf("resolve setting: %v", err)
	}
	if got := settingCoverage(project, column, setting, mappingSet(state.Mappings), newListStatusContext()); got != coveragePartial {
		t.Fatalf("expected multi-target partial setting coverage, got %v", got)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := RunWithExecutable([]string{"list", "-p", "OpenCode", "-c", "opencode.json"}, &stdout, &stderr, fixture.executablePath); exitCode != 0 {
		t.Fatalf("list -c opencode.json exit=%d stderr=%q", exitCode, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "GPT [GPT.json] (present)") {
		t.Fatalf("expected partial setting to remain listed, got %q", output)
	}
	assertNoANSI(t, output)
}

func TestRunWithExecutableApplyResetAndRevertEndToEnd(t *testing.T) {
	workspace := t.TempDir()
	executablePath := filepath.Join(workspace, "cfgfc")
	homeDir := setTempHome(t, workspace)
	configDir := filepath.Join(homeDir, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

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
	if err := os.WriteFile(filepath.Join(warehouseRoot, "Column", "opencode.json", "SettingIndex.jsonc"), []byte("{\n  \"description\": \"main config\",\n  \"defaultTargetDir\": [\"~/.config/opencode\"],\n  \"defaultTargetName\": [\"opencode.json\"],\n  \"settings\": {\n    \"CLAUDE.json\": {\"displayName\": \"CLAUDE.json\"},\n    \"GPT.json\": {\"displayName\": \"GPT.json\"}\n  }\n}\n"), 0o644); err != nil {
		t.Fatalf("write opencode setting index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(warehouseRoot, "Column", "Skills", "SettingIndex.jsonc"), []byte("{\n  \"description\": \"skills\",\n  \"defaultTargetDir\": [\"~/.config/opencode/skills\"],\n  \"defaultTargetName\": [\"\"],\n  \"settings\": {\n    \"Skill-A\": {\"targetDir\": [\"\"], \"targetName\": [\"Skill-A\"]},\n    \"Skill-B\": {\"targetDir\": [\"\"], \"targetName\": [\"Skill-B\"]}\n  }\n}\n"), 0o644); err != nil {
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

func TestRunWithExecutableApplyForceOverridesUnmanagedTarget(t *testing.T) {
	fixture := setupUpdateFixture(t)
	target := filepath.Join(fixture.configDir, "opencode.json")
	if err := os.WriteFile(target, []byte("manual"), 0o644); err != nil {
		t.Fatalf("write unmanaged target: %v", err)
	}

	got := mustRun(t, fixture.executablePath, []string{"apply", "-p", "OpenCode", "-c", "opencode.json", "-s", "GPT.json", "-f"})
	if !bytes.Contains([]byte(got), []byte("Applied column \"Config\" for project \"Open Code\"")) {
		t.Fatalf("unexpected forced apply stdout %q", got)
	}
	assertFileSymlinkPointsTo(t, target, filepath.Join(fixture.projectRoot, "Column", "opencode.json", "GPT.json"))
}

func TestRunWithExecutableUpdateForceRepairsDriftedTarget(t *testing.T) {
	fixture := setupUpdateFixture(t)
	target := filepath.Join(fixture.configDir, "opencode.json")
	mustRun(t, fixture.executablePath, []string{"apply", "-p", "OpenCode", "-c", "opencode.json", "-s", "GPT.json"})
	if err := os.Remove(target); err != nil {
		t.Fatalf("remove managed target: %v", err)
	}
	if err := os.WriteFile(target, []byte("manual"), 0o644); err != nil {
		t.Fatalf("write drifted target: %v", err)
	}

	got := mustRun(t, fixture.executablePath, []string{"update", "-p", "OpenCode", "--force"})
	if !bytes.Contains([]byte(got), []byte("Updated project \"Open Code\"")) {
		t.Fatalf("unexpected forced update stdout %q", got)
	}
	assertFileSymlinkPointsTo(t, target, filepath.Join(fixture.projectRoot, "Column", "opencode.json", "GPT.json"))
}

func TestRunWithExecutableRevertForceReclaimsOccupiedTargets(t *testing.T) {
	fixture := setupUpdateFixture(t)
	configTarget := filepath.Join(fixture.configDir, "opencode.json")
	skillTarget := filepath.Join(fixture.configDir, "skills", "Skill-A")
	mustRun(t, fixture.executablePath, []string{"apply", "-p", "OpenCode", "-c", "opencode.json", "-s", "GPT.json"})
	mustRun(t, fixture.executablePath, []string{"apply", "-p", "OpenCode", "-m", "Max"})

	if err := os.Remove(configTarget); err != nil {
		t.Fatalf("remove managed config target: %v", err)
	}
	if err := os.WriteFile(configTarget, []byte("manual"), 0o644); err != nil {
		t.Fatalf("write unmanaged config target: %v", err)
	}
	if err := os.Remove(skillTarget); err != nil {
		t.Fatalf("remove managed skill target: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(skillTarget, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir unmanaged skill target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillTarget, "nested", "README.md"), []byte("manual"), 0o644); err != nil {
		t.Fatalf("write unmanaged skill file: %v", err)
	}

	got := mustRun(t, fixture.executablePath, []string{"revert", "-p", "OpenCode", "--force"})
	if !bytes.Contains([]byte(got), []byte("Reverted project \"Open Code\"")) {
		t.Fatalf("unexpected forced revert stdout %q", got)
	}
	assertFileSymlinkPointsTo(t, configTarget, filepath.Join(fixture.projectRoot, "Column", "opencode.json", "GPT.json"))
	if _, err := os.Lstat(skillTarget); !os.IsNotExist(err) {
		t.Fatalf("skill target should be removed after forced revert, err=%v", err)
	}
}

func TestRunWithExecutableResetForceRemovesDriftedTarget(t *testing.T) {
	fixture := setupUpdateFixture(t)
	target := filepath.Join(fixture.configDir, "opencode.json")
	mustRun(t, fixture.executablePath, []string{"apply", "-p", "OpenCode", "-c", "opencode.json", "-s", "GPT.json"})
	if err := os.Remove(target); err != nil {
		t.Fatalf("remove managed target: %v", err)
	}
	if err := os.WriteFile(target, []byte("manual"), 0o644); err != nil {
		t.Fatalf("write drifted target: %v", err)
	}

	got := mustRun(t, fixture.executablePath, []string{"reset", "-p", "OpenCode", "-f"})
	if !bytes.Contains([]byte(got), []byte("Reset project \"Open Code\"")) {
		t.Fatalf("unexpected forced reset stdout %q", got)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("target should be removed after forced reset, err=%v", err)
	}
}

func TestRunWithExecutableUpdateRefreshesExplicitAndSwitchedProjects(t *testing.T) {
	fixture := setupUpdateFixture(t)
	mustRun(t, fixture.executablePath, []string{"apply", "-p", "OpenCode", "-c", "opencode.json", "-s", "GPT.json"})
	assertFileSymlinkPointsTo(t, filepath.Join(fixture.configDir, "opencode.json"), filepath.Join(fixture.projectRoot, "Column", "opencode.json", "GPT.json"))
	state := readCurrentState(t, fixture.projectRoot)
	if state.Intent == nil || state.Intent.Kind != "column" || state.Intent.Column != "opencode.json" || len(state.Intent.Settings) != 1 || state.Intent.Settings[0] != "GPT.json" {
		t.Fatalf("apply column did not persist canonical intent: %#v", state.Intent)
	}

	writeUpdateFixtureIndexes(t, fixture.projectRoot, "~/.config/opencode/opencode.updated.json")
	if got := mustRun(t, fixture.executablePath, []string{"update", "--project", "OpenCode"}); !bytes.Contains([]byte(got), []byte("Updated project \"Open Code\"")) {
		t.Fatalf("unexpected explicit update stdout %q", got)
	}
	if _, err := os.Lstat(filepath.Join(fixture.configDir, "opencode.json")); !os.IsNotExist(err) {
		t.Fatalf("old target should be removed after update, err=%v", err)
	}
	assertFileSymlinkPointsTo(t, filepath.Join(fixture.configDir, "opencode.updated.json"), filepath.Join(fixture.projectRoot, "Column", "opencode.json", "GPT.json"))

	mustRun(t, fixture.executablePath, []string{"switch", "OpenCode"})
	writeUpdateFixtureIndexes(t, fixture.projectRoot, "~/.config/opencode/opencode.switched.json")
	if got := mustRun(t, fixture.executablePath, []string{"update"}); !bytes.Contains([]byte(got), []byte("Updated project \"Open Code\"")) {
		t.Fatalf("unexpected switched update stdout %q", got)
	}
	assertFileSymlinkPointsTo(t, filepath.Join(fixture.configDir, "opencode.switched.json"), filepath.Join(fixture.projectRoot, "Column", "opencode.json", "GPT.json"))
}

func TestRunWithExecutableUpdateColumnPreservesOtherColumnsAndAliases(t *testing.T) {
	fixture := setupUpdateFixture(t)
	mustRun(t, fixture.executablePath, []string{"apply", "-p", "OpenCode", "-m", "Max"})
	state := readCurrentState(t, fixture.projectRoot)
	if state.Intent == nil || state.Intent.Kind != "mode" || state.Intent.Mode != "Max" {
		t.Fatalf("apply mode did not persist canonical intent: %#v", state.Intent)
	}
	assertFileSymlinkPointsTo(t, filepath.Join(fixture.configDir, "opencode.json"), filepath.Join(fixture.projectRoot, "Column", "opencode.json", "GPT.json"))
	assertSymlinkPointsTo(t, filepath.Join(fixture.configDir, "skills", "Skill-A"), filepath.Join(fixture.projectRoot, "Column", "Skills", "Skill-A"))

	writeUpdateFixtureIndexes(t, fixture.projectRoot, "~/.config/opencode/opencode.column.json")
	if got := mustRun(t, fixture.executablePath, []string{"update", "-p", "oc", "-c", "config"}); !bytes.Contains([]byte(got), []byte("Updated column \"Config\" for project \"Open Code\"")) {
		t.Fatalf("unexpected column update stdout %q", got)
	}
	assertFileSymlinkPointsTo(t, filepath.Join(fixture.configDir, "opencode.column.json"), filepath.Join(fixture.projectRoot, "Column", "opencode.json", "GPT.json"))
	assertSymlinkPointsTo(t, filepath.Join(fixture.configDir, "skills", "Skill-A"), filepath.Join(fixture.projectRoot, "Column", "Skills", "Skill-A"))
}

func TestRunWithExecutableUpdateFullModeIncludesNewSyncedSkill(t *testing.T) {
	fixture := setupUpdateFixture(t)
	writeUpdateFixtureFullSkillsIndexes(t, fixture.projectRoot, "~/.config/opencode/opencode.json", false)
	mustRun(t, fixture.executablePath, []string{"sync", "-p", "OpenCode"})
	mustRun(t, fixture.executablePath, []string{"apply", "-p", "OpenCode", "-m", "Max"})
	assertSymlinkPointsTo(t, filepath.Join(fixture.configDir, "skills", "Skill-A"), filepath.Join(fixture.projectRoot, "Column", "Skills", "Skill-A"))

	newSkillPath := filepath.Join(fixture.projectRoot, "Column", "Skills", "Skill-New")
	if err := os.MkdirAll(newSkillPath, 0o755); err != nil {
		t.Fatalf("mkdir Skill-New: %v", err)
	}
	if err := os.WriteFile(filepath.Join(newSkillPath, "README.md"), []byte("skill-new"), 0o644); err != nil {
		t.Fatalf("write Skill-New readme: %v", err)
	}
	writeUpdateFixtureFullSkillsIndexes(t, fixture.projectRoot, "~/.config/opencode/opencode.json", true)
	mustRun(t, fixture.executablePath, []string{"sync", "-p", "OpenCode"})

	if got := mustRun(t, fixture.executablePath, []string{"update", "-p", "OpenCode"}); !bytes.Contains([]byte(got), []byte("Updated project \"Open Code\"")) {
		t.Fatalf("unexpected update stdout %q", got)
	}
	assertSymlinkPointsTo(t, filepath.Join(fixture.configDir, "skills", "Skill-A"), filepath.Join(fixture.projectRoot, "Column", "Skills", "Skill-A"))
	assertSymlinkPointsTo(t, filepath.Join(fixture.configDir, "skills", "Skill-New"), filepath.Join(fixture.projectRoot, "Column", "Skills", "Skill-New"))
	state := readCurrentState(t, fixture.projectRoot)
	if state.Intent == nil || state.Intent.Kind != "mode" || state.Intent.Mode != "Max" {
		t.Fatalf("update did not preserve mode intent: %#v", state.Intent)
	}
}

func TestRunWithExecutableUpdateFullModeColumnIncludesNewSyncedSkill(t *testing.T) {
	fixture := setupUpdateFixture(t)
	writeUpdateFixtureFullSkillsIndexes(t, fixture.projectRoot, "~/.config/opencode/opencode.json", false)
	mustRun(t, fixture.executablePath, []string{"sync", "-p", "OpenCode"})
	mustRun(t, fixture.executablePath, []string{"apply", "-p", "OpenCode", "-m", "Max"})

	newSkillPath := filepath.Join(fixture.projectRoot, "Column", "Skills", "Skill-New")
	if err := os.MkdirAll(newSkillPath, 0o755); err != nil {
		t.Fatalf("mkdir Skill-New: %v", err)
	}
	if err := os.WriteFile(filepath.Join(newSkillPath, "README.md"), []byte("skill-new"), 0o644); err != nil {
		t.Fatalf("write Skill-New readme: %v", err)
	}
	writeUpdateFixtureFullSkillsIndexes(t, fixture.projectRoot, "~/.config/opencode/opencode.json", true)
	mustRun(t, fixture.executablePath, []string{"sync", "-p", "OpenCode"})

	if got := mustRun(t, fixture.executablePath, []string{"update", "-p", "OpenCode", "-c", "skills"}); !bytes.Contains([]byte(got), []byte("Updated column \"Skills\" for project \"Open Code\"")) {
		t.Fatalf("unexpected column update stdout %q", got)
	}
	assertFileSymlinkPointsTo(t, filepath.Join(fixture.configDir, "opencode.json"), filepath.Join(fixture.projectRoot, "Column", "opencode.json", "GPT.json"))
	assertSymlinkPointsTo(t, filepath.Join(fixture.configDir, "skills", "Skill-New"), filepath.Join(fixture.projectRoot, "Column", "Skills", "Skill-New"))
}

func TestRunWithExecutableUpdateAllSkipsEmptyProjectsAndIgnoresContext(t *testing.T) {
	fixture := setupUpdateFixture(t)
	mustRun(t, fixture.executablePath, []string{"new", "-p", "Empty"})
	mustRun(t, fixture.executablePath, []string{"sync", "--all"})
	mustRun(t, fixture.executablePath, []string{"apply", "-p", "OpenCode", "-c", "opencode.json", "-s", "GPT.json"})
	mustRun(t, fixture.executablePath, []string{"switch", "Empty"})

	writeUpdateFixtureIndexes(t, fixture.projectRoot, "~/.config/opencode/opencode.all.json")
	got := mustRun(t, fixture.executablePath, []string{"update", "--all"})
	if !bytes.Contains([]byte(got), []byte("Updated project \"Open Code\"")) || !bytes.Contains([]byte(got), []byte("Updated 1 project(s)")) {
		t.Fatalf("unexpected update --all stdout %q", got)
	}
	assertFileSymlinkPointsTo(t, filepath.Join(fixture.configDir, "opencode.all.json"), filepath.Join(fixture.projectRoot, "Column", "opencode.json", "GPT.json"))

	writeUpdateFixtureIndexes(t, fixture.projectRoot, "~/.config/opencode/opencode.a.json")
	got = mustRun(t, fixture.executablePath, []string{"update", "-a"})
	if !bytes.Contains([]byte(got), []byte("Updated 1 project(s)")) {
		t.Fatalf("unexpected update -a stdout %q", got)
	}
	assertFileSymlinkPointsTo(t, filepath.Join(fixture.configDir, "opencode.a.json"), filepath.Join(fixture.projectRoot, "Column", "opencode.json", "GPT.json"))
}

func TestRunWithExecutableUpdateColumnReportsNoActiveMappings(t *testing.T) {
	fixture := setupUpdateFixture(t)
	mustRun(t, fixture.executablePath, []string{"apply", "-p", "OpenCode", "-c", "opencode.json", "-s", "GPT.json"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := RunWithExecutable([]string{"update", "-p", "OpenCode", "-c", "Skills"}, &stdout, &stderr, fixture.executablePath)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("no active mappings")) {
		t.Fatalf("expected no-active-mappings error, got %q", stderr.String())
	}
}

func TestRunWithExecutableUpdateRejectsMalformedIntentBeforeMutation(t *testing.T) {
	fixture := setupUpdateFixture(t)
	mustRun(t, fixture.executablePath, []string{"apply", "-p", "OpenCode", "-c", "opencode.json", "-s", "GPT.json"})
	originalTarget := filepath.Join(fixture.configDir, "opencode.json")
	originalSource := filepath.Join(fixture.projectRoot, "Column", "opencode.json", "GPT.json")
	assertFileSymlinkPointsTo(t, originalTarget, originalSource)

	state := readCurrentState(t, fixture.projectRoot)
	state.Intent = &linker.ApplyIntent{Kind: "unknown"}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("marshal malformed state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fixture.projectRoot, "Backup", "current_state.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write malformed state: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := RunWithExecutable([]string{"update", "-p", "OpenCode"}, &stdout, &stderr, fixture.executablePath)

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("unsupported update intent kind")) {
		t.Fatalf("expected malformed intent error, got %q", stderr.String())
	}
	assertFileSymlinkPointsTo(t, originalTarget, originalSource)
}

type updateFixture struct {
	executablePath string
	projectRoot    string
	configDir      string
}

type listStatusFixture struct {
	updateFixture
	warehouseRoot string
}

func setupUpdateFixture(t *testing.T) updateFixture {
	t.Helper()
	workspace := t.TempDir()
	homeDir := setTempHome(t, workspace)
	executablePath := filepath.Join(workspace, "cfgfc")
	configDir := filepath.Join(homeDir, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	mustRun(t, executablePath, []string{"new", "-p", "OpenCode"})
	mustRun(t, executablePath, []string{"new", "-p", "OpenCode", "-c", "opencode.json"})
	mustRun(t, executablePath, []string{"new", "-p", "OpenCode", "-c", "Skills"})
	mustRun(t, executablePath, []string{"new", "-p", "OpenCode", "-m", "Max"})
	projectRoot := filepath.Join(homeDir, ".configfacilitator", "OpenCode")
	if err := os.WriteFile(filepath.Join(homeDir, ".configfacilitator", "ProjectIndex.jsonc"), []byte(`{
  "OpenCode": {
    "displayName": "Open Code",
    "aliases": ["oc"]
  }
}
`), 0o644); err != nil {
		t.Fatalf("write project index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "opencode.json", "GPT.json"), []byte("gpt"), 0o644); err != nil {
		t.Fatalf("write GPT.json: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "Column", "Skills", "Skill-A"), 0o755); err != nil {
		t.Fatalf("mkdir Skill-A: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "Skills", "Skill-A", "README.md"), []byte("skill-a"), 0o644); err != nil {
		t.Fatalf("write Skill-A readme: %v", err)
	}
	writeUpdateFixtureIndexes(t, projectRoot, "~/.config/opencode/opencode.json")
	mustRun(t, executablePath, []string{"sync", "-p", "OpenCode"})
	return updateFixture{executablePath: executablePath, projectRoot: projectRoot, configDir: configDir}
}

// setupListStatusFixture provisions list-focused state for global, project, and column status assertions.
func setupListStatusFixture(t *testing.T) listStatusFixture {
	t.Helper()
	fixture := setupUpdateFixture(t)
	warehouseRoot := filepath.Dir(fixture.projectRoot)

	mustRun(t, fixture.executablePath, []string{"apply", "-p", "OpenCode", "-m", "Max"})
	writeListStatusFixtureIndexes(t, fixture.projectRoot)
	mustRun(t, fixture.executablePath, []string{"new", "-p", "Manual"})
	mustRun(t, fixture.executablePath, []string{"new", "-p", "Empty"})

	if err := os.WriteFile(filepath.Join(warehouseRoot, "ProjectIndex.jsonc"), []byte(`{
  "OpenCode": {
    "displayName": "Open Code",
    "aliases": ["oc"]
  },
  "Manual": {},
  "Empty": {}
}
`), 0o644); err != nil {
		t.Fatalf("write list project index: %v", err)
	}
	writeProjectCurrentState(t, filepath.Join(warehouseRoot, "Manual"), linker.CurrentState{
		Mappings: []linker.Mapping{{Source: "manual-source", Target: "manual-target"}},
		Intent:   &linker.ApplyIntent{Kind: "column", Column: "Solo", Settings: []string{"One"}},
	})
	return listStatusFixture{updateFixture: fixture, warehouseRoot: warehouseRoot}
}

func writeUpdateFixtureIndexes(t *testing.T, projectRoot string, configTarget string) {
	t.Helper()
	configTargetDir := path.Dir(configTarget)
	configTargetName := path.Base(configTarget)
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte(`{
  "opencode.json": {
    "displayName": "Config",
    "aliases": ["config"]
  },
  "Skills": {
    "displayName": "Skills",
    "aliases": ["skills"]
  }
}
`), 0o644); err != nil {
		t.Fatalf("write column index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "opencode.json", "SettingIndex.jsonc"), []byte(`{
  "defaultTargetDir": [`+jsonString(t, configTargetDir)+`],
  "defaultTargetName": [`+jsonString(t, configTargetName)+`],
  "settings": {
    "GPT.json": {"displayName": "GPT"}
  }
}
`), 0o644); err != nil {
		t.Fatalf("write opencode setting index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "Skills", "SettingIndex.jsonc"), []byte(`{
  "defaultTargetDir": ["~/.config/opencode/skills"],
  "defaultTargetName": [""],
  "settings": {
    "Skill-A": {
      "displayName": "Skill A",
      "aliases": ["alpha"],
      "targetDir": [""],
      "targetName": ["Skill-A"]
    }
  }
}
`), 0o644); err != nil {
		t.Fatalf("write skills setting index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Mode", "ModeIndex.jsonc"), []byte(`{
  "Max": {
    "displayName": "Max",
    "columns": {
      "config": {
        "settings": ["GPT.json"],
        "strategy": "cover"
      },
      "skills": {
        "settings": ["alpha"],
        "strategy": "increment"
      }
    }
  }
}
`), 0o644); err != nil {
		t.Fatalf("write mode index: %v", err)
	}
}

func writeMultiTargetConfigSettingIndex(t *testing.T, projectRoot string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "opencode.json", "SettingIndex.jsonc"), []byte(`{
  "defaultTargetDir": ["~/.config/opencode", "~/.config/opencode"],
  "defaultTargetName": ["opencode.primary.json", "opencode.secondary.json"],
  "settings": {
    "GPT.json": {"displayName": "GPT"}
  }
}
`), 0o644); err != nil {
		t.Fatalf("write multi-target config setting index: %v", err)
	}
}

// writeListStatusFixtureIndexes expands the OpenCode metadata so list can report full, partial, none, and missing states.
func writeListStatusFixtureIndexes(t *testing.T, projectRoot string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(projectRoot, "Column", "Extras"), 0o755); err != nil {
		t.Fatalf("mkdir Extras column: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "Column", "Skills", "Skill-B"), 0o755); err != nil {
		t.Fatalf("mkdir Skill-B: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "Skills", "Skill-B", "README.md"), []byte("skill-b"), 0o644); err != nil {
		t.Fatalf("write Skill-B readme: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "Extras", "Extra-A"), []byte("extra-a"), 0o644); err != nil {
		t.Fatalf("write Extra-A: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "ColumnIndex.jsonc"), []byte(`{
  "opencode.json": {
    "displayName": "Config",
    "aliases": ["config"]
  },
  "Skills": {
    "displayName": "Skills",
    "aliases": ["skills"]
  },
  "Extras": {
    "displayName": "Extras",
    "aliases": ["extras"]
  }
}
`), 0o644); err != nil {
		t.Fatalf("write list column index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "Skills", "SettingIndex.jsonc"), []byte(`{
  "defaultTargetDir": ["~/.config/opencode/skills"],
  "defaultTargetName": [""],
  "settings": {
    "Skill-A": {
      "displayName": "Skill A",
      "aliases": ["alpha"],
      "targetDir": [""],
      "targetName": ["Skill-A"]
    },
    "Skill-B": {
      "displayName": "Skill B",
      "targetDir": [""],
      "targetName": ["Skill-B"]
    },
    "Skill-Missing": {
      "displayName": "Skill Missing",
      "missing": true
    }
  }
}
`), 0o644); err != nil {
		t.Fatalf("write list skills setting index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "Extras", "SettingIndex.jsonc"), []byte(`{
  "defaultTargetDir": ["~/.config/opencode/extras"],
  "defaultTargetName": [""],
  "settings": {
    "Extra-A": {
      "displayName": "Extra A",
      "targetDir": [""],
      "targetName": ["Extra-A"]
    }
  }
}
`), 0o644); err != nil {
		t.Fatalf("write extras setting index: %v", err)
	}
}

func jsonString(t *testing.T, value string) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON string: %v", err)
	}
	return string(data)
}

func writeUpdateFixtureFullSkillsIndexes(t *testing.T, projectRoot string, configTarget string, includeNewSkill bool) {
	t.Helper()
	writeUpdateFixtureIndexes(t, projectRoot, configTarget)
	skillsSettings := `
    "Skill-A": {
      "displayName": "Skill A",
      "aliases": ["alpha"],
      "targetDir": [""],
      "targetName": ["Skill-A"]
    }`
	if includeNewSkill {
		skillsSettings += `,
    "Skill-New": {
      "displayName": "Skill New",
      "targetDir": [""],
      "targetName": ["Skill-New"]
    }`
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Column", "Skills", "SettingIndex.jsonc"), []byte(`{
  "defaultTargetDir": ["~/.config/opencode/skills"],
  "defaultTargetName": [""],
  "settings": {`+skillsSettings+`
  }
}
`), 0o644); err != nil {
		t.Fatalf("write full skills setting index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Mode", "ModeIndex.jsonc"), []byte(`{
  "Max": {
    "displayName": "Max",
    "columns": {
      "config": {
        "settings": ["GPT.json"],
        "strategy": "cover"
      },
      "skills": {
        "strategy": "full"
      }
    }
  }
}
`), 0o644); err != nil {
		t.Fatalf("write full skills mode index: %v", err)
	}
}

func readCurrentState(t *testing.T, projectRoot string) linker.CurrentState {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectRoot, "Backup", "current_state.json"))
	if err != nil {
		t.Fatalf("read current state: %v", err)
	}
	var state linker.CurrentState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("parse current state: %v", err)
	}
	return state
}

// writeProjectCurrentState stores one explicit current_state payload for list-focused state coverage.
func writeProjectCurrentState(t *testing.T, projectRoot string, state linker.CurrentState) {
	t.Helper()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("marshal current state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "Backup", "current_state.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write current state: %v", err)
	}
}

// assertNoANSI verifies that one plain-writer test path does not emit terminal color escapes.
func assertNoANSI(t *testing.T, output string) {
	t.Helper()
	if strings.Contains(output, "\x1b[") {
		t.Fatalf("expected plain-text output without ANSI escapes, got %q", output)
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

func TestShouldUseColorHonorsNoColorAndPipe(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "1")
	if shouldUseColor(os.Stdout) {
		t.Fatalf("expected NO_COLOR to disable ANSI output")
	}

	t.Setenv("NO_COLOR", "")
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	defer reader.Close()
	defer writer.Close()
	if shouldUseColor(writer) {
		t.Fatalf("expected pipe writer to disable ANSI output")
	}
}

func TestRunShowsVersionWithVersionFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"--version"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := stdout.String()
	if !bytes.Contains(stdout.Bytes(), []byte("dev")) {
		t.Fatalf("expected version output to contain 'dev', got %q", output)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunShowsVersionWithVFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"-v"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := stdout.String()
	if !bytes.Contains(stdout.Bytes(), []byte("dev")) {
		t.Fatalf("expected version output to contain 'dev', got %q", output)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestVersionFlagTakesPrecedence(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Test that --version takes precedence over --help
	exitCode := Run([]string{"--version", "--help"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := stdout.String()
	if !bytes.Contains(stdout.Bytes(), []byte("dev")) {
		t.Fatalf("expected version output to contain 'dev', got %q", output)
	}

	// Should not contain help text
	if bytes.Contains(stdout.Bytes(), []byte("cfgfc manages portable configuration warehouses.")) {
		t.Fatalf("expected version output, not help text, got %q", output)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestDefaultVersionIsDev(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"--version"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "dev" {
		t.Fatalf("expected default version 'dev', got %q", output)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}
