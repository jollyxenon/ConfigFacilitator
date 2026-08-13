package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/content"
	"github.com/xenon/ConfigFacilitator/internal/mutate"
	"github.com/xenon/ConfigFacilitator/internal/repository"
)

// TestSettingCreationContentSourcesAreExactExclusiveAndKindChecked verifies task 6 creation semantics.
func TestSettingCreationContentSourcesAreExactExclusiveAndKindChecked(t *testing.T) {
	dependencies := testDependencies(t)
	runResourceCommand(t, dependencies, []string{"project", "create", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "create", "Models", "-p", "OpenCode"}, ExitSuccess, "")

	runResourceCommand(t, dependencies, []string{"setting", "create", "literal.txt", "-p", "OpenCode", "-c", "Models", "--kind", "file", "--text", "no newline"}, ExitSuccess, "")
	dependencies.Stdin = bytes.NewReader([]byte{0, 1, 2, 255})
	runResourceCommand(t, dependencies, []string{"setting", "create", "stdin.bin", "-p", "OpenCode", "-c", "Models", "--kind", "file", "--stdin"}, ExitSuccess, "")

	importFile := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(importFile, []byte("copied"), 0o640); err != nil {
		t.Fatal(err)
	}
	runResourceCommand(t, dependencies, []string{"setting", "create", "copied.txt", "-p", "OpenCode", "-c", "Models", "--kind", "file", "--from", importFile}, ExitSuccess, "")
	if err := os.WriteFile(importFile, []byte("changed"), 0o640); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(dependencies.HomeDir, ".configfacilitator", "OpenCode", "Column", "Models")
	assertExactBytes(t, filepath.Join(root, "literal.txt"), []byte("no newline"))
	assertExactBytes(t, filepath.Join(root, "stdin.bin"), []byte{0, 1, 2, 255})
	assertExactBytes(t, filepath.Join(root, "copied.txt"), []byte("copied"))
	importDirectory := filepath.Join(t.TempDir(), "tree")
	if err := os.MkdirAll(filepath.Join(importDirectory, "nested", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(importDirectory, "nested", "value.bin"), []byte{0, 4, 8}, 0o644); err != nil {
		t.Fatal(err)
	}
	runResourceCommand(t, dependencies, []string{"setting", "create", "tree", "-p", "OpenCode", "-c", "Models", "--kind", "directory", "--from", importDirectory}, ExitSuccess, "")
	assertExactBytes(t, filepath.Join(root, "tree", "nested", "value.bin"), []byte{0, 4, 8})
	if info, err := os.Stat(filepath.Join(root, "tree", "nested", "empty")); err != nil || !info.IsDir() {
		t.Fatalf("recursive empty directory info=%v err=%v", info, err)
	}

	runResourceCommand(t, dependencies, []string{"setting", "create", "empty.txt", "-p", "OpenCode", "-c", "Models", "--kind", "file"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "create", "empty-dir", "-p", "OpenCode", "-c", "Models", "--kind", "directory"}, ExitSuccess, "")
	assertExactBytes(t, filepath.Join(root, "empty.txt"), []byte{})
	if info, err := os.Stat(filepath.Join(root, "empty-dir")); err != nil || !info.IsDir() {
		t.Fatalf("empty directory info=%v err=%v", info, err)
	}

	runResourceCommand(t, dependencies, []string{"setting", "create", "conflict.txt", "-p", "OpenCode", "-c", "Models", "--kind", "file", "--text", "x", "--from", importFile, "--json"}, ExitUsage, "conflicting_content_sources")
	runResourceCommand(t, dependencies, []string{"setting", "create", "bad-dir", "-p", "OpenCode", "-c", "Models", "--kind", "directory", "--text", "x", "--json"}, ExitInvalidData, "directory_bytes_forbidden")
	runResourceCommand(t, dependencies, []string{"setting", "create", "bad-kind", "-p", "OpenCode", "-c", "Models", "--kind", "directory", "--from", importFile, "--json"}, ExitInvalidData, "import_kind_mismatch")
	assertSettingAbsent(t, dependencies, "OpenCode", "Models", "conflict.txt")
	assertSettingAbsent(t, dependencies, "OpenCode", "Models", "bad-dir")
	assertSettingAbsent(t, dependencies, "OpenCode", "Models", "bad-kind")
}

// TestContentCommandsCoverLexicalListExactReadJSONAndMutations verifies task 6 command behavior.
func TestContentCommandsCoverLexicalListExactReadJSONAndMutations(t *testing.T) {
	dependencies := testDependencies(t)
	runResourceCommand(t, dependencies, []string{"project", "create", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "create", "Skills", "-p", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "create", "Skill-A", "-p", "OpenCode", "-c", "Skills", "--kind", "directory"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "content", "mkdir", "Skill-A", "z/empty", "-p", "OpenCode", "-c", "Skills"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "content", "write", "Skill-A", "a/text.txt", "-p", "OpenCode", "-c", "Skills", "--text", "exact"}, ExitSuccess, "")
	dependencies.Stdin = bytes.NewReader([]byte{0xff, 0, 1})
	runResourceCommand(t, dependencies, []string{"setting", "content", "write", "Skill-A", "a/binary.bin", "-p", "OpenCode", "-c", "Skills", "--stdin"}, ExitSuccess, "")

	stdout, _ := runResourceCommand(t, dependencies, []string{"setting", "content", "list", "Skill-A", "-p", "OpenCode", "-c", "Skills", "--json"}, ExitSuccess, "")
	var listed struct {
		Data struct {
			Entries []struct {
				Path string `json:"path"`
				Kind string `json:"kind"`
				Size int64  `json:"size"`
			} `json:"entries"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &listed); err != nil {
		t.Fatal(err)
	}
	paths := []string{}
	for _, entry := range listed.Data.Entries {
		paths = append(paths, entry.Path)
	}
	if strings.Join(paths, ",") != "a,a/binary.bin,a/text.txt,z,z/empty" {
		t.Fatalf("lexical entries=%#v", listed.Data.Entries)
	}

	stdout, _ = runResourceCommand(t, dependencies, []string{"setting", "content", "read", "Skill-A", "a/text.txt", "-p", "OpenCode", "-c", "Skills"}, ExitSuccess, "")
	if stdout != "exact" {
		t.Fatalf("human read added decoration: %q", stdout)
	}
	stdout, _ = runResourceCommand(t, dependencies, []string{"setting", "content", "read", "Skill-A", "a/binary.bin", "-p", "OpenCode", "-c", "Skills", "--json"}, ExitSuccess, "")
	var binaryEnvelope struct {
		Data struct {
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &binaryEnvelope); err != nil {
		t.Fatal(err)
	}
	if binaryEnvelope.Data.Encoding != "base64" || binaryEnvelope.Data.Content != base64.StdEncoding.EncodeToString([]byte{0xff, 0, 1}) {
		t.Fatalf("binary envelope=%#v", binaryEnvelope)
	}
	fileSource := filepath.Join(t.TempDir(), "replacement.txt")
	if err := os.WriteFile(fileSource, []byte("from file"), 0o644); err != nil {
		t.Fatal(err)
	}
	runResourceCommand(t, dependencies, []string{"setting", "create", "File", "-p", "OpenCode", "-c", "Skills", "--kind", "file", "--text", "old"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "content", "write", "File", "-p", "OpenCode", "-c", "Skills", "--from", fileSource}, ExitSuccess, "")
	stdout, _ = runResourceCommand(t, dependencies, []string{"setting", "content", "list", "File", "-p", "OpenCode", "-c", "Skills", "--json"}, ExitSuccess, "")
	if !strings.Contains(stdout, `"path":".","kind":"file","size":9`) {
		t.Fatalf("file-backed list=%q", stdout)
	}
	stdout, _ = runResourceCommand(t, dependencies, []string{"setting", "content", "read", "File", "-p", "OpenCode", "-c", "Skills", "--json"}, ExitSuccess, "")
	if !strings.Contains(stdout, `"content":"from file","encoding":"utf-8"`) {
		t.Fatalf("UTF-8 JSON read=%q", stdout)
	}
	runResourceCommand(t, dependencies, []string{"setting", "content", "read", "File", "unexpected", "-p", "OpenCode", "-c", "Skills", "--json"}, ExitInvalidData, "file_path_forbidden")
	runResourceCommand(t, dependencies, []string{"setting", "content", "read", "Skill-A", "-p", "OpenCode", "-c", "Skills", "--json"}, ExitInvalidData, "directory_path_required")
	runResourceCommand(t, dependencies, []string{"setting", "content", "mkdir", "File", "nested", "-p", "OpenCode", "-c", "Skills", "--json"}, ExitInvalidData, "directory_setting_required")

	runResourceCommand(t, dependencies, []string{"setting", "content", "move", "Skill-A", "a", "moved/a", "-p", "OpenCode", "-c", "Skills"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "content", "move", "Skill-A", "moved/a", "z", "-p", "OpenCode", "-c", "Skills", "--json"}, ExitResource, "content_destination_exists")
	runResourceCommand(t, dependencies, []string{"setting", "content", "delete", "Skill-A", "moved", "-p", "OpenCode", "-c", "Skills", "--json"}, ExitRefusal, "confirmation_required")
	runResourceCommand(t, dependencies, []string{"setting", "content", "delete", "Skill-A", "moved", "-p", "OpenCode", "-c", "Skills", "--yes"}, ExitSuccess, "")

	traversals := []struct {
		path string
		code string
	}{
		{path: "../outside", code: "unclean_content_path"},
		{path: "a/../../outside", code: "unclean_content_path"},
		{path: "/absolute", code: "absolute_content_path"},
		{path: `C:\outside`, code: "absolute_content_path"},
	}
	for _, traversal := range traversals {
		runResourceCommand(t, dependencies, []string{"setting", "content", "write", "Skill-A", traversal.path, "-p", "OpenCode", "-c", "Skills", "--text", "bad", "--json"}, ExitInvalidData, traversal.code)
	}
	runResourceCommand(t, dependencies, []string{"setting", "content", "delete", "Skill-A", "", "-p", "OpenCode", "-c", "Skills", "--yes", "--json"}, ExitInvalidData, "empty_content_path")

	if runtime.GOOS != "windows" {
		settingRoot := filepath.Join(dependencies.HomeDir, ".configfacilitator", "OpenCode", "Column", "Skills", "Skill-A")
		if err := os.Symlink(t.TempDir(), filepath.Join(settingRoot, "linked")); err != nil {
			t.Fatal(err)
		}
		runResourceCommand(t, dependencies, []string{"setting", "content", "write", "Skill-A", "linked/escape.txt", "-p", "OpenCode", "-c", "Skills", "--text", "bad", "--json"}, ExitInvalidData, "symlink_content_path")
	}
}

// TestContentMutationsAreVisibleThroughManagedFileAndDirectorySymlinks verifies no refresh is needed.
func TestContentMutationsAreVisibleThroughManagedFileAndDirectorySymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require Windows Developer Mode")
	}
	dependencies := testDependencies(t)
	runResourceCommand(t, dependencies, []string{"project", "create", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "create", "Models", "-p", "OpenCode"}, ExitSuccess, "")
	targetRoot := filepath.Join(dependencies.HomeDir, "targets")
	runResourceCommand(t, dependencies, []string{"column", "target", "add", "Models", "-p", "OpenCode", "--dir", targetRoot, "--name-from-setting"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "create", "file.json", "-p", "OpenCode", "-c", "Models", "--kind", "file", "--text", "before"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"apply", "column", "Models", "file.json", "-p", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "content", "write", "file.json", "-p", "OpenCode", "-c", "Models", "--text", "after"}, ExitSuccess, "")
	assertExactBytes(t, filepath.Join(targetRoot, "file.json"), []byte("after"))

	runResourceCommand(t, dependencies, []string{"setting", "create", "dir", "-p", "OpenCode", "-c", "Models", "--kind", "directory"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"apply", "column", "Models", "dir", "-p", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"setting", "content", "write", "dir", "nested/new.txt", "-p", "OpenCode", "-c", "Models", "--text", "visible"}, ExitSuccess, "")
	assertExactBytes(t, filepath.Join(targetRoot, "dir", "nested", "new.txt"), []byte("visible"))
}

// TestSettingImportAndContentRollbackLeaveNoPartialState verifies creation and mutation rollback.
func TestSettingImportAndContentRollbackLeaveNoPartialState(t *testing.T) {
	dependencies := testDependencies(t)
	runResourceCommand(t, dependencies, []string{"project", "create", "OpenCode"}, ExitSuccess, "")
	runResourceCommand(t, dependencies, []string{"column", "create", "Skills", "-p", "OpenCode"}, ExitSuccess, "")
	root := filepath.Join(dependencies.HomeDir, ".configfacilitator")
	importRoot := filepath.Join(t.TempDir(), "import")
	if err := os.Mkdir(importRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink(t.TempDir(), filepath.Join(importRoot, "link")); err != nil {
			t.Fatal(err)
		}
		runResourceCommand(t, dependencies, []string{"setting", "create", "unsafe", "-p", "OpenCode", "-c", "Skills", "--kind", "directory", "--from", importRoot, "--json"}, ExitInvalidData, "import_symlink")
		assertSettingAbsent(t, dependencies, "OpenCode", "Skills", "unsafe")
	}

	repo := repository.New(root, repository.WithHooks(repository.Hooks{BeforeStage: func(stage repository.Stage) error {
		if stage == repository.StageCommitted {
			return errors.New("injected")
		}
		return nil
	}}))
	settingPath := filepath.Join(root, "OpenCode", "Column", "Skills", "rollback.txt")
	metadata, err := mutate.NewMetadata(mutate.SettingKind, "rollback.txt", "rollback.txt", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := mutate.CreateSetting(repo, "OpenCode", "Skills", "rollback.txt", "file", metadata, content.Source{Mode: content.SourceBytes, Bytes: []byte("staged")}); err == nil {
		t.Fatal("expected creation rollback")
	}
	if _, err := os.Lstat(settingPath); !os.IsNotExist(err) {
		t.Fatalf("partial source survived: %v", err)
	}
	index, err := repository.New(root).LoadSettingIndex("OpenCode", "Skills")
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := index.Settings["rollback.txt"]; exists {
		t.Fatal("partial metadata survived")
	}
}

// assertExactBytes compares one filesystem object's complete bytes.
func assertExactBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s bytes=%v want=%v", path, got, want)
	}
}

// assertSettingAbsent verifies neither a source nor index metadata was committed.
func assertSettingAbsent(t *testing.T, dependencies Dependencies, project string, column string, setting string) {
	t.Helper()
	root := filepath.Join(dependencies.HomeDir, ".configfacilitator")
	if _, err := os.Lstat(filepath.Join(root, project, "Column", column, setting)); !os.IsNotExist(err) {
		t.Fatalf("partial Setting source %q survived: %v", setting, err)
	}
	index, err := repository.New(root).LoadSettingIndex(project, column)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := index.Settings[setting]; exists {
		t.Fatalf("partial Setting metadata %q survived", setting)
	}
}
