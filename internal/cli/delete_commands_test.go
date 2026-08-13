package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/repository"
)

// TestDeleteCommandsRequireYesAndExposeDependencyDetails verifies CLI flags and human/JSON errors.
func TestDeleteCommandsRequireYesAndExposeDependencyDetails(t *testing.T) {
	dependencies := testDependencies(t)
	runResourceCommand(t, dependencies, []string{"project", "create", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "create", "Models", "-p", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "create", "GPT.json", "-p", "OpenCode", "-c", "Models", "--kind", "file"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"mode", "create", "Max", "-p", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"mode", "column", "set", "Max", "Models", "-p", "OpenCode", "--strategy", "cover", "--setting", "GPT.json"}, ExitSuccess, "")

	runResourceCommand(t, dependencies, []string{"setting", "delete", "GPT.json", "-p", "OpenCode", "-c", "Models", "--cascade", "--force-targets", "--json"}, ExitRefusal, "confirmation_required")
	_, stderr := runResourceCommand(t, dependencies, []string{"setting", "delete", "GPT.json", "-p", "OpenCode", "-c", "Models", "--yes", "--json"}, ExitRefusal, "dependencies_exist")
	var envelope struct { Error struct { Details struct { ModeSelections []any `json:"modeSelections"` } `json:"details"` } `json:"error"` }
	if err := json.Unmarshal([]byte(stderr), &envelope); err != nil || len(envelope.Error.Details.ModeSelections) != 1 {
		t.Fatalf("dependency JSON = %q err=%v", stderr, err)
	}
	_, human := runResourceCommand(t, dependencies, []string{"setting", "delete", "GPT.json", "-p", "OpenCode", "-c", "Models", "--yes"}, ExitRefusal, "")
	if !strings.Contains(human, "mode selections=1") { t.Fatalf("human dependency error = %q", human) }
}

// TestDeleteCommandCascadeAndNonCascadeLifecycle verifies all resource delete routes and independent flags.
func TestDeleteCommandCascadeAndNonCascadeLifecycle(t *testing.T) {
	dependencies := testDependencies(t)
	root := filepath.Join(dependencies.HomeDir, ".configfacilitator")
	runResourceCommand(t, dependencies, []string{"project", "create", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "create", "Empty", "-p", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "create", "Unused", "-p", "OpenCode", "-c", "Empty", "--kind", "directory"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "delete", "Unused", "-p", "OpenCode", "-c", "Empty", "--yes"}, ExitSuccess, "")
	if _, err := os.Lstat(filepath.Join(root, "OpenCode", "Column", "Empty", "Unused")); !os.IsNotExist(err) { t.Fatalf("Setting survived: %v", err) }
	runResourceCommand(t, dependencies, []string{"column", "delete", "Empty", "-p", "OpenCode", "--yes"}, ExitSuccess, "")

	runResourceCommand(t, dependencies, []string{"mode", "create", "UnusedMode", "-p", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"mode", "delete", "UnusedMode", "-p", "OpenCode", "--yes"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"use", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"project", "delete", "OpenCode", "--yes", "--cascade"}, ExitSuccess, "")
	if _, ok, err := repository.New(root).LoadSession(dependencies.PPID); err != nil || ok { t.Fatalf("session survived ok=%v err=%v", ok, err) }
}
