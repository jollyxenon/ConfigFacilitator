package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestRootHelpExposesOnlyNewCommandFamilies verifies the compatibility break at discovery time.
func TestRootHelpExposesOnlyNewCommandFamilies(t *testing.T) {
	dependencies := testDependencies(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dependencies.Stdout = &stdout
	dependencies.Stderr = &stderr

	exitCode := RunWithDependencies([]string{"--help"}, dependencies)

	if exitCode != ExitSuccess {
		t.Fatalf("exit code = %d, stderr=%q", exitCode, stderr.String())
	}
	for _, command := range []string{"project", "column", "setting", "mode", "use", "status", "apply", "refresh", "sync", "root", "reset", "revert"} {
		if !strings.Contains(stdout.String(), command) {
			t.Fatalf("root help missing %q: %q", command, stdout.String())
		}
	}
	for _, removed := range []string{"\n  new ", "\n  switch ", "\n  list ", "\n  update "} {
		if strings.Contains(stdout.String(), removed) {
			t.Fatalf("root help retained removed command %q: %q", removed, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

// TestNewCommandFamiliesExposeSharedFlags verifies Project scope and JSON flags are inherited consistently.
func TestNewCommandFamiliesExposeSharedFlags(t *testing.T) {
	dependencies := testDependencies(t)
	for _, args := range [][]string{
		{"column", "--help"},
		{"setting", "--help"},
		{"mode", "--help"},
		{"status", "--help"},
		{"apply", "--help"},
		{"refresh", "--help"},
		{"sync", "--help"},
		{"reset", "--help"},
		{"revert", "--help"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			dependencies.Stdout = &stdout
			dependencies.Stderr = &stderr
			if exitCode := RunWithDependencies(args, dependencies); exitCode != ExitSuccess {
				t.Fatalf("exit code = %d, stderr=%q", exitCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), "--json") {
				t.Fatalf("help missing --json: %q", stdout.String())
			}
			if !strings.Contains(stdout.String(), "--project") {
				t.Fatalf("help missing --project: %q", stdout.String())
			}
		})
	}
}

// TestRemovedSyntaxFailsWithoutMutation proves removed commands and flags remain hard failures.
func TestRemovedSyntaxFailsWithoutMutation(t *testing.T) {
	dependencies := testDependencies(t)
	warehouseRoot := filepath.Join(dependencies.HomeDir, ".configfacilitator")
	if err := os.MkdirAll(warehouseRoot, 0o755); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}
	fixturePath := filepath.Join(warehouseRoot, "sentinel.txt")
	if err := os.WriteFile(fixturePath, []byte("unchanged"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	before := snapshotTree(t, dependencies.HomeDir)

	tests := []struct {
		name string
		args []string
	}{
		{name: "new", args: []string{"new", "-p", "OpenCode"}},
		{name: "switch", args: []string{"switch", "OpenCode"}},
		{name: "list", args: []string{"list"}},
		{name: "update", args: []string{"update"}},
		{name: "flag-only apply mode", args: []string{"apply", "-m", "Max"}},
		{name: "flag-only apply column", args: []string{"apply", "-c", "Skills", "-s", "Skill-A"}},
		{name: "short force", args: []string{"reset", "-f"}},
		{name: "long force", args: []string{"revert", "--force"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			dependencies.Stdout = &stdout
			dependencies.Stderr = &stderr
			if exitCode := RunWithDependencies(tt.args, dependencies); exitCode != ExitUsage {
				t.Fatalf("exit code = %d, stderr=%q", exitCode, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q", stdout.String())
			}
			if after := snapshotTree(t, dependencies.HomeDir); !reflect.DeepEqual(after, before) {
				t.Fatalf("fixture mutated\nbefore=%#v\nafter=%#v", before, after)
			}
		})
	}
}

// TestJSONErrorEnvelopeIsStable verifies one error object, no stdout, and the usage exit class.
func TestJSONErrorEnvelopeIsStable(t *testing.T) {
	dependencies := testDependencies(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dependencies.Stdout = &stdout
	dependencies.Stderr = &stderr

	exitCode := RunWithDependencies([]string{"unknown", "--json"}, dependencies)

	if exitCode != ExitUsage {
		t.Fatalf("exit code = %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
	var envelope struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error envelope: %v; output=%q", err, stderr.String())
	}
	if envelope.OK || envelope.Error.Code == "" || envelope.Error.Message == "" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if strings.Contains(stderr.String(), "\x1b[") || strings.Count(strings.TrimSpace(stderr.String()), "\n") != 0 {
		t.Fatalf("JSON error contained ANSI or extra prose: %q", stderr.String())
	}
}

// TestInjectedRootAndContextDependencies verifies home, environment, PPID, streams, stdin, and executable injection.
func TestInjectedRootAndContextDependencies(t *testing.T) {
	dependencies := testDependencies(t)
	dependencies.ExecutablePath = "custom-cfgfc"
	dependencies.PPID = 4242
	dependencies.Environment = map[string]string{"CFGFC_TEST": "injected"}
	dependencies.Stdin = strings.NewReader("injected stdin")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dependencies.Stdout = &stdout
	dependencies.Stderr = &stderr

	root := NewRootCommand(dependencies)
	if root.Use != rootCommandName || root.InOrStdin() != dependencies.Stdin || root.OutOrStdout() != dependencies.Stdout || root.ErrOrStderr() != dependencies.Stderr {
		t.Fatalf("root did not retain injected process dependencies")
	}
	if root.Context() != nil {
		t.Fatalf("new root should not retain cross-execution state")
	}
	if exitCode := RunWithDependencies([]string{"root", "--json"}, dependencies); exitCode != ExitSuccess {
		t.Fatalf("root exit=%d stderr=%q", exitCode, stderr.String())
	}
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Root string `json:"root"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode root envelope: %v", err)
	}
	if !envelope.OK || envelope.Data.Root != filepath.Join(dependencies.HomeDir, ".configfacilitator") {
		t.Fatalf("unexpected root envelope: %#v", envelope)
	}
}

// TestScopeConflictsAreUsageErrors verifies shared mutually exclusive scope validation.
func TestScopeConflictsAreUsageErrors(t *testing.T) {
	dependencies := testDependencies(t)
	for _, args := range [][]string{
		{"sync", "--all", "-p", "OpenCode"},
		{"refresh", "--all", "-p", "OpenCode"},
		{"refresh", "--all", "--column", "Skills"},
	} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		dependencies.Stdout = &stdout
		dependencies.Stderr = &stderr
		if exitCode := RunWithDependencies(args, dependencies); exitCode != ExitUsage {
			t.Fatalf("args=%v exit=%d stderr=%q", args, exitCode, stderr.String())
		}
	}
}

// TestExistingOperationalHandlersRemainReachableThroughNewSyntax verifies core routing was not dropped.
func TestExistingOperationalHandlersRemainReachableThroughNewSyntax(t *testing.T) {
	dependencies := testDependencies(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dependencies.Stdout = &stdout
	dependencies.Stderr = &stderr

	if exitCode := RunWithDependencies([]string{"sync", "--all"}, dependencies); exitCode != ExitSuccess {
		t.Fatalf("sync exit=%d stderr=%q", exitCode, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithDependencies([]string{"apply", "mode", "Max"}, dependencies); exitCode != ExitResource {
		t.Fatalf("apply mode should reach project resolution, exit=%d stderr=%q", exitCode, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithDependencies([]string{"reset"}, dependencies); exitCode != ExitResource {
		t.Fatalf("reset should reach project resolution, exit=%d stderr=%q", exitCode, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if exitCode := RunWithDependencies([]string{"revert"}, dependencies); exitCode != ExitResource {
		t.Fatalf("revert should reach project resolution, exit=%d stderr=%q", exitCode, stderr.String())
	}
}

// testDependencies constructs one isolated fully injected command environment.
func testDependencies(t *testing.T) Dependencies {
	t.Helper()
	homeDir := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	return Dependencies{
		Stdin:           strings.NewReader(""),
		Stdout:          io.Discard,
		Stderr:          io.Discard,
		ExecutablePath:  "cfgfc",
		HomeDir:         homeDir,
		Environment:     map[string]string{"HOME": homeDir},
		PPID:            1001,
		OperatingSystem: "linux",
	}
}

// snapshotTree captures all fixture files and directories relative to one isolated home.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.Walk(root, func(currentPath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relativePath, err := filepath.Rel(root, currentPath)
		if err != nil {
			return err
		}
		if info.IsDir() {
			snapshot[relativePath] = "directory"
			return nil
		}
		data, err := os.ReadFile(currentPath)
		if err != nil {
			return err
		}
		snapshot[relativePath] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot tree: %v", err)
	}
	return snapshot
}
