package cli

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestHelpSweepCoversEveryVisibleCommand verifies every command has structured standalone help.
func TestHelpSweepCoversEveryVisibleCommand(t *testing.T) {
	dependencies := testDependencies(t)
	root := NewRootCommand(dependencies)
	paths := visibleCommandPaths(root)
	if len(paths) < 50 {
		t.Fatalf("help sweep found only %d commands", len(paths))
	}
	for _, path := range paths {
		t.Run(strings.ReplaceAll(path, " ", "_"), func(t *testing.T) {
			args := strings.Fields(strings.TrimPrefix(path, rootCommandName))
			args = append(args, "--help")
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			dependencies.Stdout = &stdout
			dependencies.Stderr = &stderr
			if exitCode := RunWithDependencies(args, dependencies); exitCode != ExitSuccess {
				t.Fatalf("exit code = %d, stderr=%q", exitCode, stderr.String())
			}
			help := stdout.String()
			for _, section := range []string{"Purpose:", "Arguments:", "Destructive behavior:", "Usage:", "Examples:", "Flags:"} {
				if !strings.Contains(help, section) {
					t.Fatalf("help missing %q: %q", section, help)
				}
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

// TestSpecializedHelpExplainsRequiredContracts verifies high-risk help wording.
func TestSpecializedHelpExplainsRequiredContracts(t *testing.T) {
	dependencies := testDependencies(t)
	tests := []struct {
		name  string
		args  []string
		terms []string
	}{
		{name: "root", args: []string{"--help"}, terms: []string{"selected for the current PPID", "--json", "--yes", "--cascade", "--force-targets", "root <Path>"}},
		{name: "setting create", args: []string{"setting", "create", "--help"}, terms: []string{"--kind", "--from", "--stdin", "--text", "at most one", "--project", "--column"}},
		{name: "column delete", args: []string{"column", "delete", "--help"}, terms: []string{"--yes", "--cascade", "--force-targets", "separately"}},
		{name: "sync", args: []string{"sync", "--help"}, terms: []string{"no longer exists", "--all", "--project"}},
		{name: "refresh", args: []string{"refresh", "--help"}, terms: []string{"persisted intent", "mapping-only", "--column", "--all", "--force-targets"}},
		{name: "revert", args: []string{"revert", "--help"}, terms: []string{"previous snapshot", "--force-targets"}},
		{name: "root path", args: []string{"root", "--help"}, terms: []string{"does not migrate", "[Path]"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			dependencies.Stdout = &stdout
			dependencies.Stderr = &stderr
			if exitCode := RunWithDependencies(tt.args, dependencies); exitCode != ExitSuccess {
				t.Fatalf("exit code = %d, stderr=%q", exitCode, stderr.String())
			}
			for _, term := range tt.terms {
				if !strings.Contains(stdout.String(), term) {
					t.Fatalf("help missing %q: %q", term, stdout.String())
				}
			}
		})
	}
}

// visibleCommandPaths returns every non-hidden command path in deterministic order.
func visibleCommandPaths(root *cobra.Command) []string {
	paths := []string{}
	walkCommands(root, func(command *cobra.Command) {
		paths = append(paths, command.CommandPath())
	})
	return paths
}

// TestHelpExecutionDoesNotMutate verifies a complete help sweep remains read-only.
func TestHelpExecutionDoesNotMutate(t *testing.T) {
	dependencies := testDependencies(t)
	before := snapshotTree(t, dependencies.HomeDir)
	for _, path := range visibleCommandPaths(NewRootCommand(dependencies)) {
		args := append(strings.Fields(strings.TrimPrefix(path, rootCommandName)), "--help")
		dependencies.Stdout = &bytes.Buffer{}
		dependencies.Stderr = &bytes.Buffer{}
		if exitCode := RunWithDependencies(args, dependencies); exitCode != ExitSuccess {
			t.Fatalf("%s help exit code = %d", path, exitCode)
		}
	}
	if after := snapshotTree(t, dependencies.HomeDir); !reflect.DeepEqual(after, before) {
		t.Fatalf("help sweep mutated fixture\nbefore=%#v\nafter=%#v", before, after)
	}
}
