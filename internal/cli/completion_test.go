package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/content"
	"github.com/xenon/ConfigFacilitator/internal/mutate"
	"github.com/xenon/ConfigFacilitator/internal/repository"
)

// TestCompletionGenerationSupportsFourShells verifies every documented script generator.
func TestCompletionGenerationSupportsFourShells(t *testing.T) {
	dependencies := testDependencies(t)
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			dependencies.Stdout = &stdout
			dependencies.Stderr = &stderr
			if exitCode := RunWithDependencies([]string{"completion", shell}, dependencies); exitCode != ExitSuccess {
				t.Fatalf("exit code = %d, stderr=%q", exitCode, stderr.String())
			}
			if stdout.Len() < 100 || !strings.Contains(strings.ToLower(stdout.String()), "completion") {
				t.Fatalf("unexpected script: %q", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

// TestCompletionGenerationFailureDoesNotMutate verifies output failure is classified without recovery writes.
func TestCompletionGenerationFailureDoesNotMutate(t *testing.T) {
	dependencies := testDependencies(t)
	before := snapshotTree(t, dependencies.HomeDir)
	dependencies.Stdout = failingWriter{}
	var stderr bytes.Buffer
	dependencies.Stderr = &stderr
	if exitCode := RunWithDependencies([]string{"completion", "bash"}, dependencies); exitCode != ExitPersistence {
		t.Fatalf("exit code = %d, stderr=%q", exitCode, stderr.String())
	}
	if after := snapshotTree(t, dependencies.HomeDir); !reflect.DeepEqual(after, before) {
		t.Fatalf("completion failure mutated fixture\nbefore=%#v\nafter=%#v", before, after)
	}
}

// TestShellCompletionRequestDoesNotMutate verifies Cobra's hidden dynamic request path is read-only.
func TestShellCompletionRequestDoesNotMutate(t *testing.T) {
	dependencies := testDependencies(t)
	seedCompletionWarehouse(t, dependencies)
	before := snapshotTree(t, dependencies.HomeDir)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dependencies.Stdout = &stdout
	dependencies.Stderr = &stderr
	if exitCode := RunWithDependencies([]string{"__complete", "project", "show", "O"}, dependencies); exitCode != ExitSuccess {
		t.Fatalf("exit code = %d, stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "OpenCode") {
		t.Fatalf("completion output = %q", stdout.String())
	}
	if after := snapshotTree(t, dependencies.HomeDir); !reflect.DeepEqual(after, before) {
		t.Fatalf("dynamic completion mutated fixture\nbefore=%#v\nafter=%#v", before, after)
	}
}

// TestDynamicCompletionReturnsCanonicalNamesAndAliases verifies all resource kinds and scopes.
func TestDynamicCompletionReturnsCanonicalNamesAndAliases(t *testing.T) {
	dependencies := testDependencies(t)
	seedCompletionWarehouse(t, dependencies)
	tests := []struct {
		name     string
		path     []string
		flag     string
		args     []string
		expected []string
	}{
		{name: "project positional", path: []string{"project", "show"}, expected: []string{"OpenCode", "oc"}},
		{name: "project flag", path: []string{"status"}, flag: "project", expected: []string{"OpenCode", "oc"}},
		{name: "column positional", path: []string{"column", "show"}, args: []string{"--project", "oc"}, expected: []string{"Models", "models"}},
		{name: "column flag", path: []string{"setting", "list"}, flag: "column", args: []string{"--project", "OpenCode"}, expected: []string{"Models", "models"}},
		{name: "setting positional", path: []string{"setting", "show"}, args: []string{"--project", "OpenCode", "--column", "models"}, expected: []string{"GPT.json", "gpt"}},
		{name: "mode positional", path: []string{"mode", "show"}, args: []string{"--project", "OpenCode"}, expected: []string{"Max", "max"}},
		{name: "use global", path: []string{"use"}, expected: []string{"OpenCode", "oc", "global"}},
		{name: "apply column settings", path: []string{"apply", "column"}, args: []string{"--project", "OpenCode", "Models"}, expected: []string{"GPT.json", "gpt"}},
		{name: "mode selection setting flag", path: []string{"mode", "column", "set"}, flag: "setting", args: []string{"--project", "OpenCode", "Max", "Models"}, expected: []string{"GPT.json", "gpt"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := NewRootCommand(dependencies)
			command := findCommand(t, root, tt.path)
			if err := command.ParseFlags(tt.args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}
			var values []string
			var directive cobra.ShellCompDirective
			if tt.flag != "" {
				completion, ok := command.GetFlagCompletionFunc(tt.flag)
				if !ok {
					t.Fatalf("missing %s completion", tt.flag)
				}
				values, directive = completion(command, command.Flags().Args(), "")
			} else {
				values, directive = command.ValidArgsFunction(command, command.Flags().Args(), "")
			}
			if directive&cobra.ShellCompDirectiveNoFileComp == 0 {
				t.Fatalf("directive = %v", directive)
			}
			for _, expected := range tt.expected {
				if !containsCompletion(values, expected) {
					t.Fatalf("completion missing %q: %#v", expected, values)
				}
			}
		})
	}
}

// TestDynamicCompletionLoadFailureIsSilentAndReadOnly verifies broken indexes do not emit or mutate.
func TestDynamicCompletionLoadFailureIsSilentAndReadOnly(t *testing.T) {
	dependencies := testDependencies(t)
	rootPath := filepath.Join(dependencies.HomeDir, ".configfacilitator")
	if err := os.MkdirAll(rootPath, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootPath, "ProjectIndex.jsonc"), []byte("{"), 0o644); err != nil {
		t.Fatalf("write invalid index: %v", err)
	}
	before := snapshotTree(t, dependencies.HomeDir)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dependencies.Stdout = &stdout
	dependencies.Stderr = &stderr
	command := findCommand(t, NewRootCommand(dependencies), []string{"project", "show"})
	values, directive := command.ValidArgsFunction(command, nil, "")
	if len(values) != 0 || directive&cobra.ShellCompDirectiveNoFileComp == 0 {
		t.Fatalf("values=%#v directive=%v", values, directive)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if after := snapshotTree(t, dependencies.HomeDir); !reflect.DeepEqual(after, before) {
		t.Fatalf("completion mutated fixture\nbefore=%#v\nafter=%#v", before, after)
	}
}

// findCommand resolves one visible child command for direct completion testing.
func findCommand(t *testing.T, root *cobra.Command, path []string) *cobra.Command {
	t.Helper()
	command, _, err := root.Find(path)
	if err != nil {
		t.Fatalf("find %v: %v", path, err)
	}
	return command
}

// containsCompletion reports whether a described completion has the requested value.
func containsCompletion(values []string, expected string) bool {
	for _, value := range values {
		if strings.SplitN(value, "\t", 2)[0] == expected {
			return true
		}
	}
	return false
}

// seedCompletionWarehouse creates aliased resources through production mutations.
func seedCompletionWarehouse(t *testing.T, dependencies Dependencies) {
	t.Helper()
	repo := repository.New(filepath.Join(dependencies.HomeDir, ".configfacilitator"))
	projectMetadata, err := mutate.NewMetadata(mutate.ProjectKind, "OpenCode", "OpenCode", "", []string{"oc"})
	if err != nil {
		t.Fatalf("project metadata: %v", err)
	}
	if err := mutate.CreateProject(repo, "OpenCode", projectMetadata); err != nil {
		t.Fatalf("create project: %v", err)
	}
	columnMetadata, err := mutate.NewMetadata(mutate.ColumnKind, "Models", "Models", "", []string{"models"})
	if err != nil {
		t.Fatalf("column metadata: %v", err)
	}
	if err := mutate.CreateColumn(repo, "OpenCode", "Models", columnMetadata); err != nil {
		t.Fatalf("create column: %v", err)
	}
	settingMetadata, err := mutate.NewMetadata(mutate.SettingKind, "GPT.json", "GPT.json", "", []string{"gpt"})
	if err != nil {
		t.Fatalf("setting metadata: %v", err)
	}
	if err := mutate.CreateSetting(repo, "OpenCode", "Models", "GPT.json", "file", settingMetadata, content.Source{Mode: content.SourceEmpty}); err != nil {
		t.Fatalf("create setting: %v", err)
	}
	modeMetadata, err := mutate.NewMetadata(mutate.ModeKind, "Max", "Max", "", []string{"max"})
	if err != nil {
		t.Fatalf("mode metadata: %v", err)
	}
	if err := mutate.CreateMode(repo, "OpenCode", "Max", modeMetadata); err != nil {
		t.Fatalf("create mode: %v", err)
	}
}

// failingWriter forces generated completion output to fail.
type failingWriter struct{}

// Write rejects every output write.
func (failingWriter) Write(data []byte) (int, error) { return 0, os.ErrPermission }
