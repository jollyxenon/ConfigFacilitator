package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/session"
)

// TestResourceCommandsCoverExplicitScopeContextAliasesAndJSON verifies task 3 inspection flows.
func TestResourceCommandsCoverExplicitScopeContextAliasesAndJSON(t *testing.T) {
	dependencies := testDependencies(t)
	runResourceCommand(t, dependencies, []string{"project", "create", "ProjectA", "--aliases", "pa"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"project", "create", "ProjectB", "--aliases", "pb"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "create", "SelectedColumn", "-p", "pa"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "create", "ExplicitColumn", "-p", "pb", "--aliases", "ec"}, ExitSuccess, "")
	rootPath := filepath.Join(dependencies.HomeDir, ".configfacilitator")
	if err := session.NewStore(rootPath).Set(dependencies.PPID, "ProjectA"); err != nil {
		t.Fatalf("set selected context: %v", err)
	}

	stdout, _ := runResourceCommand(t, dependencies, []string{"column", "list", "--json"}, ExitSuccess, "")
	var selected struct {
		OK   bool `json:"ok"`
		Data struct {
			Project string `json:"project"`
			Columns []struct {
				Name string `json:"name"`
			} `json:"columns"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &selected); err != nil {
		t.Fatalf("decode selected list: %v", err)
	}
	if !selected.OK || selected.Data.Project != "ProjectA" || len(selected.Data.Columns) != 1 || selected.Data.Columns[0].Name != "SelectedColumn" {
		t.Fatalf("selected context result = %#v", selected)
	}

	stdout, _ = runResourceCommand(t, dependencies, []string{"column", "show", "ec", "-p", "pb", "--json"}, ExitSuccess, "")
	var explicit struct {
		OK   bool `json:"ok"`
		Data struct {
			Project string `json:"project"`
			Column  struct {
				Name string `json:"name"`
			} `json:"column"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &explicit); err != nil {
		t.Fatalf("decode explicit show: %v", err)
	}
	if explicit.Data.Project != "ProjectB" || explicit.Data.Column.Name != "ExplicitColumn" {
		t.Fatalf("explicit scope result = %#v", explicit)
	}
}

// TestResourceCreateSetDescriptionAliasesAndMissingInspection verifies common metadata and source kinds.
func TestResourceCreateSetDescriptionAliasesAndMissingInspection(t *testing.T) {
	dependencies := testDependencies(t)
	runResourceCommand(t, dependencies, []string{"project", "create", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "create", "Skills", "-p", "OpenCode"}, ExitSuccess, "")
	dependencies.Stdin = strings.NewReader("complete stdin description\nsecond line\n")
	runResourceCommand(t, dependencies, []string{"setting", "create", "Skill-A", "-p", "OpenCode", "-c", "Skills", "--kind", "file", "--aliases", "alpha"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "set", "alpha", "-p", "OpenCode", "-c", "Skills", "--description-file", "-", "--aliases", "beta,gamma"}, ExitSuccess, "")
	stdout, _ := runResourceCommand(t, dependencies, []string{"setting", "show", "beta", "-p", "OpenCode", "-c", "Skills", "--json"}, ExitSuccess, "")
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Setting settingView `json:"setting"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode Setting show: %v", err)
	}
	if envelope.Data.Setting.Kind != "file" || envelope.Data.Setting.Description != "complete stdin description\nsecond line\n" || !reflect.DeepEqual(envelope.Data.Setting.Aliases, []string{"beta", "gamma"}) {
		t.Fatalf("Setting view = %#v", envelope.Data.Setting)
	}

	rootPath := filepath.Join(dependencies.HomeDir, ".configfacilitator")
	if err := os.Remove(filepath.Join(rootPath, "OpenCode", "Column", "Skills", "Skill-A")); err != nil {
		t.Fatalf("remove Setting source: %v", err)
	}
	stdout, _ = runResourceCommand(t, dependencies, []string{"setting", "show", "gamma", "-p", "OpenCode", "-c", "Skills", "--json"}, ExitSuccess, "")
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode missing Setting show: %v", err)
	}
	if !envelope.Data.Setting.Missing || envelope.Data.Setting.Kind != "missing" {
		t.Fatalf("missing Setting view = %#v", envelope.Data.Setting)
	}

	runResourceCommand(t, dependencies, []string{"setting", "set", "gamma", "-p", "OpenCode", "-c", "Skills", "--clear-aliases"}, ExitSuccess, "")
	settingIndex, err := repository.New(rootPath).LoadSettingIndex("OpenCode", "Skills")
	if err != nil {
		t.Fatalf("LoadSettingIndex: %v", err)
	}
	if aliases := settingIndex.Settings["Skill-A"].Aliases; aliases == nil || len(aliases) != 0 {
		t.Fatalf("cleared aliases = %#v", aliases)
	}
}

// TestResourceCreateShapesAndModeSetPreservesExtensions verifies zero targets and empty Modes.
func TestResourceCreateShapesAndModeSetPreservesExtensions(t *testing.T) {
	dependencies := testDependencies(t)
	runResourceCommand(t, dependencies, []string{"project", "create", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "create", "Skills", "-p", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "create", "Skill-A", "-p", "OpenCode", "-c", "Skills", "--kind", "directory"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"mode", "create", "Max", "-p", "OpenCode"}, ExitSuccess, "")
	rootPath := filepath.Join(dependencies.HomeDir, ".configfacilitator")
	repo := repository.New(rootPath)
	settingIndex, err := repo.LoadSettingIndex("OpenCode", "Skills")
	if err != nil {
		t.Fatal(err)
	}
	if settingIndex.TargetNumber != 0 || len(settingIndex.Settings["Skill-A"].TargetDir) != 0 || len(settingIndex.Settings["Skill-A"].TargetName) != 0 {
		t.Fatalf("zero-target Setting index = %#v", settingIndex)
	}
	modeIndex, err := repo.LoadModeIndex("OpenCode")
	if err != nil {
		t.Fatal(err)
	}
	entry := modeIndex.Modes["Max"]
	entry.Columns["Skills"] = index.ModeColumnSelection{Strategy: "full", Extra: map[string]json.RawMessage{"selectionExtra": json.RawMessage(`true`)}}
	entry.Extra["modeExtra"] = json.RawMessage(`{"keep":true}`)
	modeIndex.Modes["Max"] = entry
	if err := repo.SaveModeIndex("OpenCode", modeIndex); err != nil {
		t.Fatal(err)
	}
	runResourceCommand(t, dependencies, []string{"mode", "set", "Max", "-p", "OpenCode", "--description", "updated"}, ExitSuccess, "")
	modeIndex, err = repo.LoadModeIndex("OpenCode")
	if err != nil {
		t.Fatal(err)
	}
	entry = modeIndex.Modes["Max"]
	if entry.Description != "updated" || entry.Columns["Skills"].Strategy != "full" || string(entry.Extra["modeExtra"]) != `{"keep":true}` || string(entry.Columns["Skills"].Extra["selectionExtra"]) != "true" {
		t.Fatalf("Mode metadata set lost data: %#v", entry)
	}
}

// TestTargetCommandsCoverStructuralCRUDAndValidation verifies task 4 CLI target workflows.
func TestTargetCommandsCoverStructuralCRUDAndValidation(t *testing.T) {
	dependencies := testDependencies(t)
	runResourceCommand(t, dependencies, []string{"project", "create", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "create", "Skills", "-p", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "create", "Skill-A", "-p", "OpenCode", "-c", "Skills", "--kind", "directory"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "target", "add", "Skills", "-p", "OpenCode", "--dir", "${HOME}/skills", "--name-from-setting"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "target", "add", "Skills", "-p", "OpenCode", "--dir", "/tmp/skills", "--name", "fixed"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "target", "set", "Skill-A", "0", "-p", "OpenCode", "-c", "Skills", "--inherit-dir", "--name", "skill"}, ExitSuccess, "")
	stdout, _ := runResourceCommand(t, dependencies, []string{"setting", "target", "list", "Skill-A", "-p", "OpenCode", "-c", "Skills", "--json"}, ExitSuccess, "")
	if !strings.Contains(stdout, `"dirMode":"inherit"`) || !strings.Contains(stdout, `"nameMode":"explicit"`) {
		t.Fatalf("target list = %q", stdout)
	}
	runResourceCommand(t, dependencies, []string{"setting", "target", "reset", "Skill-A", "0", "-p", "OpenCode", "-c", "Skills"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "target", "delete", "Skills", "0", "-p", "OpenCode", "--json"}, ExitRefusal, "confirmation_required")
	runResourceCommand(t, dependencies, []string{"column", "target", "delete", "Skills", "0", "-p", "OpenCode", "--yes"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "target", "set", "Skills", "2", "-p", "OpenCode", "--name-from-setting", "--json"}, ExitInvalidData, "invalid_target_index")
	runResourceCommand(t, dependencies, []string{"column", "target", "add", "Skills", "-p", "OpenCode", "--dir", "${MISSING}/skills", "--name", "bad", "--json"}, ExitInvalidData, "target_plan")
}

// TestResourceCommandsClassifyDocumentedExitCodes verifies 2-6 error classes on task 3 routes.
func TestResourceCommandsClassifyDocumentedExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		prepare  func(t *testing.T, dependencies Dependencies)
		args     []string
		exitCode int
		code     string
	}{
		{name: "usage", args: []string{"project", "create", "--json"}, exitCode: ExitUsage, code: "invalid_arguments"},
		{name: "invalid", prepare: prepareProjectFixture, args: []string{"column", "create", "../bad", "-p", "OpenCode", "--json"}, exitCode: ExitInvalidData, code: "invalid_name"},
		{name: "resource", prepare: prepareProjectFixture, args: []string{"project", "create", "Second", "--aliases", "OpenCode", "--json"}, exitCode: ExitResource, code: "reference_conflict"},
		{name: "persistence", prepare: func(t *testing.T, dependencies Dependencies) {
			rootPath := filepath.Join(dependencies.HomeDir, ".configfacilitator")
			if err := os.WriteFile(rootPath, []byte("occupied"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, args: []string{"project", "create", "OpenCode", "--json"}, exitCode: ExitPersistence, code: "transaction_recovery"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dependencies := testDependencies(t)
			if test.prepare != nil {
				test.prepare(t, dependencies)
			}
			_, stderr := runResourceCommand(t, dependencies, test.args, test.exitCode, test.code)
			if strings.Contains(stderr, "\x1b[") || strings.Count(strings.TrimSpace(stderr), "\n") != 0 {
				t.Fatalf("error JSON has extra output: %q", stderr)
			}
		})
	}

	commandErr := NewRefusalError("confirmation_required", "confirmation required", nil, nil)
	var stderr bytes.Buffer
	renderCommandError(&stderr, commandErr, true)
	if commandErr.ExitCode() != ExitRefusal || !strings.Contains(stderr.String(), `"code":"confirmation_required"`) {
		t.Fatalf("refusal classification = exit %d, stderr %q", commandErr.ExitCode(), stderr.String())
	}
}

// prepareProjectFixture creates one Project for command classification tests.
func prepareProjectFixture(t *testing.T, dependencies Dependencies) {
	t.Helper()
	runResourceCommand(t, dependencies, []string{"project", "create", "OpenCode"}, ExitSuccess, "")
}

// runResourceCommand executes one isolated resource command and checks its stable result class.
func runResourceCommand(t *testing.T, dependencies Dependencies, args []string, expectedExit int, expectedErrorCode string) (string, string) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dependencies.Stdout = &stdout
	dependencies.Stderr = &stderr
	exitCode := RunWithDependencies(args, dependencies)
	if exitCode != expectedExit {
		t.Fatalf("args=%v exit=%d want=%d stdout=%q stderr=%q", args, exitCode, expectedExit, stdout.String(), stderr.String())
	}
	if expectedErrorCode != "" && !strings.Contains(stderr.String(), `"code":"`+expectedErrorCode+`"`) {
		t.Fatalf("args=%v error code missing from %q", args, stderr.String())
	}
	return stdout.String(), stderr.String()
}

// TestModeColumnSelectionCommandsAreImmediatelyApplicable verifies CLI selection CRUD, strategy validation, and canonical persistence.
func TestModeColumnSelectionCommandsAreImmediatelyApplicable(t *testing.T) {
	dependencies := testDependencies(t)
	runResourceCommand(t, dependencies, []string{"project", "create", "OpenCode", "--aliases", "oc"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "create", "Models", "-p", "oc", "--aliases", "models-alias"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "create", "GPT.json", "-p", "OpenCode", "-c", "Models", "--kind", "file", "--aliases", "gpt-alias"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "target", "add", "Models", "-p", "OpenCode", "--dir", filepath.Join(dependencies.HomeDir, "targets"), "--name-from-setting"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "create", "Tools.json", "-p", "OpenCode", "-c", "Models", "--kind", "file", "--aliases", "tools-alias"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"mode", "create", "Max", "-p", "OpenCode", "--description", "keep"}, ExitSuccess, "")

	runResourceCommand(t, dependencies, []string{"mode", "column", "set", "Max", "models-alias", "-p", "oc", "--strategy", "cover", "--setting", "gpt-alias"}, ExitSuccess, "")
	stdout, _ := runResourceCommand(t, dependencies, []string{"mode", "column", "list", "Max", "-p", "OpenCode", "--json"}, ExitSuccess, "")
	if !strings.Contains(stdout, `"Models":{"strategy":"cover","settings":["GPT.json"]}`) {
		t.Fatalf("canonical Mode Column selection = %q", stdout)
	}
	runResourceCommand(t, dependencies, []string{"apply", "mode", "Max", "-p", "OpenCode"}, ExitSuccess, "")
	targetPath := filepath.Join(dependencies.HomeDir, "targets", "GPT.json")
	if data, err := os.ReadFile(targetPath); err != nil {
		t.Fatalf("managed target was not immediately applicable: %v", err)
	} else if len(data) != 0 {
		t.Fatalf("managed target contents = %q, want empty source", data)
	}
	runResourceCommand(t, dependencies, []string{"mode", "column", "set", "Max", "Models", "-p", "OpenCode", "--strategy", "increment", "--setting", "gpt-alias", "--setting", "tools-alias"}, ExitSuccess, "")
	stdout, _ = runResourceCommand(t, dependencies, []string{"mode", "column", "list", "Max", "-p", "OpenCode", "--json"}, ExitSuccess, "")
	if !strings.Contains(stdout, `"settings":["GPT.json","Tools.json"]`) {
		t.Fatalf("repeated --setting values were not preserved: %q", stdout)
	}

	runResourceCommand(t, dependencies, []string{"mode", "column", "set", "Max", "Models", "-p", "OpenCode", "--strategy", "full"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"mode", "column", "set", "Max", "Models", "-p", "OpenCode", "--strategy", "none"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"mode", "column", "set", "Max", "Models", "-p", "OpenCode", "--strategy", "cover", "--json"}, ExitInvalidData, "mode_settings_required")
	runResourceCommand(t, dependencies, []string{"mode", "column", "set", "Max", "Models", "-p", "OpenCode", "--strategy", "full", "--setting", "GPT.json", "--json"}, ExitInvalidData, "mode_settings_forbidden")
	runResourceCommand(t, dependencies, []string{"mode", "column", "set", "Max", "Models", "-p", "OpenCode", "--strategy", "cover", "--setting", "missing.json", "--json"}, ExitResource, "setting_not_found")

	rootPath := filepath.Join(dependencies.HomeDir, ".configfacilitator")
	modeIndex, err := repository.New(rootPath).LoadModeIndex("OpenCode")
	if err != nil {
		t.Fatal(err)
	}
	entry := modeIndex.Modes["Max"]
	if entry.Description != "keep" || len(entry.Columns) != 1 {
		t.Fatalf("failed selection validation changed unrelated Mode data: %#v", entry)
	}
	runResourceCommand(t, dependencies, []string{"mode", "column", "delete", "Max", "Models", "-p", "OpenCode", "--json"}, ExitSuccess, "")
	modeIndex, err = repository.New(rootPath).LoadModeIndex("OpenCode")
	if err != nil {
		t.Fatal(err)
	}
	if len(modeIndex.Modes["Max"].Columns) != 0 || modeIndex.Modes["Max"].Description != "keep" {
		t.Fatalf("selection delete did not preserve Mode metadata: %#v", modeIndex.Modes["Max"])
	}
}
