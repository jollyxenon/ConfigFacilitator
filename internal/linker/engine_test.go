package linker

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

func TestInspectOwnershipClassifiesAbsentOwnedAndUnmanaged(t *testing.T) {
	engine := New()
	root := t.TempDir()
	source := writeFile(t, root, "warehouse/source.txt", "alpha")
	target := filepath.Join(root, "target.txt")

	ownership, err := engine.InspectOwnership(Mapping{Source: source, Target: target})
	if err != nil {
		t.Fatalf("inspect absent: %v", err)
	}
	if ownership != OwnershipAbsent {
		t.Fatalf("ownership = %s, want absent", ownership)
	}

	if err := os.Symlink(source, target); err != nil {
		t.Fatalf("create owned symlink: %v", err)
	}
	ownership, err = engine.InspectOwnership(Mapping{Source: source, Target: target})
	if err != nil {
		t.Fatalf("inspect owned: %v", err)
	}
	if ownership != OwnershipOwned {
		t.Fatalf("ownership = %s, want owned", ownership)
	}

	if err := os.Remove(target); err != nil {
		t.Fatalf("remove target: %v", err)
	}
	otherSource := writeFile(t, root, "warehouse/other.txt", "beta")
	if err := os.Symlink(otherSource, target); err != nil {
		t.Fatalf("create unmanaged symlink: %v", err)
	}
	ownership, err = engine.InspectOwnership(Mapping{Source: source, Target: target})
	if err != nil {
		t.Fatalf("inspect unmanaged: %v", err)
	}
	if ownership != OwnershipUnmanaged {
		t.Fatalf("ownership = %s, want unmanaged", ownership)
	}
}

func TestReplaceMappingsPersistsCurrentStateAndHistory(t *testing.T) {
	engine := New()
	project, root := newProjectPaths(t)
	firstSource := writeFile(t, root, "warehouse/first.txt", "one")
	secondSource := writeFile(t, root, "warehouse/second.txt", "two")
	target := filepath.Join(root, "target.txt")

	if err := engine.ReplaceMappings(project, []Mapping{{Source: firstSource, Target: target}}); err != nil {
		t.Fatalf("initial replace: %v", err)
	}
	assertFileSymlinkTarget(t, target, firstSource)

	if err := engine.ReplaceMappings(project, []Mapping{{Source: secondSource, Target: target}}); err != nil {
		t.Fatalf("second replace: %v", err)
	}
	assertFileSymlinkTarget(t, target, secondSource)

	state, err := engine.LoadCurrentState(project)
	if err != nil {
		t.Fatalf("load current state: %v", err)
	}
	if len(state.Mappings) != 1 || state.Mappings[0].Source != secondSource || state.Mappings[0].Target != target {
		t.Fatalf("unexpected current state: %#v", state.Mappings)
	}

	previous, err := engine.LoadPreviousSnapshot(project)
	if err != nil {
		t.Fatalf("load previous snapshot: %v", err)
	}
	if len(previous) != 1 || previous[0].Source != firstSource || previous[0].Target != target {
		t.Fatalf("unexpected previous snapshot: %#v", previous)
	}

	historyData, err := os.ReadFile(project.HistoryLogPath)
	if err != nil {
		t.Fatalf("read history log: %v", err)
	}
	entries, err := ReadHistoryEntries(bytes.NewReader(historyData))
	if err != nil {
		t.Fatalf("parse history entries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("history entries = %d, want 2", len(entries))
	}
	if len(entries[1].PreviousMappings) != 1 || entries[1].PreviousMappings[0].Source != firstSource {
		t.Fatalf("unexpected previous mappings in history: %#v", entries[1].PreviousMappings)
	}
}

func TestLoadCurrentStateReadsLegacyMappingOnlyState(t *testing.T) {
	engine := New()
	project, root := newProjectPaths(t)
	source := writeFile(t, root, "warehouse/source.txt", "alpha")
	target := filepath.Join(root, "target.txt")
	legacyState := []byte(fmt.Sprintf("{\n  \"mappings\": [{\"source\": %q, \"target\": %q}]\n}\n", source, target))
	if err := os.WriteFile(project.CurrentStatePath, legacyState, 0o644); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}

	state, err := engine.LoadCurrentState(project)
	if err != nil {
		t.Fatalf("load legacy state: %v", err)
	}
	if state.Intent != nil {
		t.Fatalf("legacy state intent = %#v, want nil", state.Intent)
	}
	if len(state.Mappings) != 1 || state.Mappings[0].Source != source || state.Mappings[0].Target != target {
		t.Fatalf("unexpected legacy mappings: %#v", state.Mappings)
	}
}

func TestReplaceStatePersistsIntentAndHistory(t *testing.T) {
	engine := New()
	engine.now = func() time.Time { return time.Unix(456, 0) }
	project, root := newProjectPaths(t)
	firstSource := writeFile(t, root, "warehouse/first.txt", "one")
	secondSource := writeFile(t, root, "warehouse/second.txt", "two")
	target := filepath.Join(root, "target.txt")
	modeIntent := &ApplyIntent{Kind: "mode", Mode: "Max"}
	columnIntent := &ApplyIntent{Kind: "column", Column: "opencode.json", Settings: []string{"GPT.json"}}

	if err := engine.ReplaceState(project, CurrentState{Mappings: []Mapping{{Source: firstSource, Target: target}}, Intent: modeIntent}); err != nil {
		t.Fatalf("initial replace state: %v", err)
	}
	if err := engine.ReplaceState(project, CurrentState{Mappings: []Mapping{{Source: secondSource, Target: target}}, Intent: columnIntent}); err != nil {
		t.Fatalf("second replace state: %v", err)
	}

	state, err := engine.LoadCurrentState(project)
	if err != nil {
		t.Fatalf("load current state: %v", err)
	}
	if state.Intent == nil || state.Intent.Kind != "column" || state.Intent.Column != "opencode.json" || len(state.Intent.Settings) != 1 || state.Intent.Settings[0] != "GPT.json" {
		t.Fatalf("unexpected current intent: %#v", state.Intent)
	}
	previous, err := engine.LoadPreviousState(project)
	if err != nil {
		t.Fatalf("load previous state: %v", err)
	}
	if previous.Intent == nil || previous.Intent.Kind != "mode" || previous.Intent.Mode != "Max" {
		t.Fatalf("unexpected previous intent: %#v", previous.Intent)
	}

	historyData, err := os.ReadFile(project.HistoryLogPath)
	if err != nil {
		t.Fatalf("read history log: %v", err)
	}
	entries, err := ReadHistoryEntries(bytes.NewReader(historyData))
	if err != nil {
		t.Fatalf("parse history entries: %v", err)
	}
	if len(entries) != 2 || entries[1].PreviousIntent == nil || entries[1].NextIntent == nil {
		t.Fatalf("history did not record intents: %#v", entries)
	}
}

func TestResetClearsIntentAndLoadPreviousStateRestoresIt(t *testing.T) {
	engine := New()
	project, root := newProjectPaths(t)
	source := writeFile(t, root, "warehouse/source.txt", "alpha")
	target := filepath.Join(root, "target.txt")
	intent := &ApplyIntent{Kind: "mode", Mode: "Max"}

	if err := engine.ReplaceState(project, CurrentState{Mappings: []Mapping{{Source: source, Target: target}}, Intent: intent}); err != nil {
		t.Fatalf("replace state: %v", err)
	}
	if err := engine.Reset(project); err != nil {
		t.Fatalf("reset: %v", err)
	}

	state, err := engine.LoadCurrentState(project)
	if err != nil {
		t.Fatalf("load state after reset: %v", err)
	}
	if state.Intent != nil || len(state.Mappings) != 0 {
		t.Fatalf("reset state = %#v, want empty without intent", state)
	}
	previous, err := engine.LoadPreviousState(project)
	if err != nil {
		t.Fatalf("load previous state: %v", err)
	}
	if previous.Intent == nil || previous.Intent.Kind != "mode" || previous.Intent.Mode != "Max" {
		t.Fatalf("previous state did not preserve intent: %#v", previous.Intent)
	}
}

func TestReplaceMappingsRejectsUnmanagedTarget(t *testing.T) {
	engine := New()
	project, root := newProjectPaths(t)
	source := writeFile(t, root, "warehouse/source.txt", "alpha")
	target := writeFile(t, root, "target.txt", "real-file")

	err := engine.ReplaceMappings(project, []Mapping{{Source: source, Target: target}})
	if err == nil {
		t.Fatal("expected unmanaged target conflict")
	}
}

func TestReplaceMappingsRejectsDuplicateTargets(t *testing.T) {
	engine := New()
	project, root := newProjectPaths(t)
	firstSource := writeFile(t, root, "warehouse/first.txt", "one")
	secondSource := writeFile(t, root, "warehouse/second.txt", "two")
	target := filepath.Join(root, "target.txt")

	err := engine.ReplaceMappings(project, []Mapping{{Source: firstSource, Target: target}, {Source: secondSource, Target: target}})
	if err == nil {
		t.Fatal("expected duplicate target to fail")
	}
	if _, statErr := os.Lstat(target); !os.IsNotExist(statErr) {
		t.Fatalf("duplicate target should not be created, err=%v", statErr)
	}
}

func TestReplaceMappingsWithForceOverridesUnmanagedTarget(t *testing.T) {
	engine := New()
	project, root := newProjectPaths(t)
	source := writeFile(t, root, "warehouse/source.txt", "alpha")
	target := writeFile(t, root, "target.txt", "real-file")

	if err := engine.ReplaceMappings(project, []Mapping{{Source: source, Target: target}}, WithForce(true)); err != nil {
		t.Fatalf("forced replace: %v", err)
	}
	assertFileSymlinkTarget(t, target, source)
}

func TestReplaceMappingsFailsClearlyWhenSourceIsMissing(t *testing.T) {
	engine := New()
	project, root := newProjectPaths(t)
	source := filepath.Join(root, "warehouse", "missing.txt")
	target := filepath.Join(root, "target.txt")

	err := engine.ReplaceMappings(project, []Mapping{{Source: source, Target: target}})
	if err == nil {
		t.Fatal("expected missing source failure")
	}
	if !strings.Contains(err.Error(), "symlink source") || !strings.Contains(err.Error(), source) || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing source error = %q", err.Error())
	}
	if _, statErr := os.Lstat(target); !os.IsNotExist(statErr) {
		t.Fatalf("missing source should not create target, err=%v", statErr)
	}
}

func TestCreateSymlinkForOSWrapsWindowsCreationFailures(t *testing.T) {
	root := t.TempDir()
	source := writeFile(t, root, "warehouse/source.txt", "alpha")
	target := filepath.Join(root, "target.txt")
	creationErr := errors.New("privilege not held")

	err := createSymlinkForOS(source, target, "windows", os.Lstat, func(string, string) error {
		return creationErr
	})
	if err == nil {
		t.Fatal("expected Windows symlink creation failure")
	}
	message := err.Error()
	for _, want := range []string{"privilege not held", "real symlinks only", "hardlinks", "junctions", "copies", "shell fallbacks", "Developer Mode", "Administrator"} {
		if !strings.Contains(message, want) {
			t.Fatalf("Windows symlink error %q missing %q", message, want)
		}
	}
}

func TestReplaceMappingsCreatesDirectorySymlink(t *testing.T) {
	engine := New()
	project, root := newProjectPaths(t)
	sourceDir := filepath.Join(root, "warehouse", "Skill-A")
	targetDir := filepath.Join(root, "target-skill")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "README.md"), []byte("skill-a"), 0o644); err != nil {
		t.Fatalf("write source readme: %v", err)
	}

	if err := engine.ReplaceMappings(project, []Mapping{{Source: sourceDir, Target: targetDir}}); err != nil {
		t.Fatalf("replace directory mapping: %v", err)
	}
	assertSymlinkTarget(t, targetDir, sourceDir)
	got, err := os.ReadFile(filepath.Join(targetDir, "README.md"))
	if err != nil {
		t.Fatalf("read through directory symlink: %v", err)
	}
	if string(got) != "skill-a" {
		t.Fatalf("directory symlink content = %q, want skill-a", string(got))
	}
}

func TestReplaceStateWithForceRepairsDriftedRecordedTarget(t *testing.T) {
	engine := New()
	project, root := newProjectPaths(t)
	source := writeFile(t, root, "warehouse/source.txt", "alpha")
	target := filepath.Join(root, "target.txt")
	manualSource := writeFile(t, root, "warehouse/manual.txt", "manual")

	if err := engine.ReplaceState(project, CurrentState{Mappings: []Mapping{{Source: source, Target: target}}}); err != nil {
		t.Fatalf("initial replace state: %v", err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatalf("remove owned target: %v", err)
	}
	if err := os.Symlink(manualSource, target); err != nil {
		t.Fatalf("create drifted target: %v", err)
	}

	if err := engine.ReplaceState(project, CurrentState{Mappings: []Mapping{{Source: source, Target: target}}}, WithForce(true)); err != nil {
		t.Fatalf("forced replace state: %v", err)
	}
	assertFileSymlinkTarget(t, target, source)
}

func TestReplaceMappingsRollsBackOnHistoryWriteFailure(t *testing.T) {
	engine := New()
	engine.now = func() time.Time { return time.Unix(123, 0) }
	project, root := newProjectPaths(t)
	firstSource := writeFile(t, root, "warehouse/first.txt", "one")
	secondSource := writeFile(t, root, "warehouse/second.txt", "two")
	target := filepath.Join(root, "target.txt")

	if err := engine.ReplaceMappings(project, []Mapping{{Source: firstSource, Target: target}}); err != nil {
		t.Fatalf("initial replace: %v", err)
	}

	defaultWriter := engine.writeFile
	engine.writeFile = func(path string, data []byte, perm os.FileMode) error {
		if path == project.HistoryLogPath {
			return errors.New("boom")
		}
		return defaultWriter(path, data, perm)
	}
	err := engine.ReplaceMappings(project, []Mapping{{Source: secondSource, Target: target}})
	if err == nil {
		t.Fatal("expected persistence failure")
	}

	assertFileSymlinkTarget(t, target, firstSource)
	state, stateErr := engine.LoadCurrentState(project)
	if stateErr != nil {
		t.Fatalf("load current state after rollback: %v", stateErr)
	}
	if len(state.Mappings) != 1 || state.Mappings[0].Source != firstSource {
		t.Fatalf("unexpected state after rollback: %#v", state.Mappings)
	}
}

func TestResetRemovesOnlyOwnedTargets(t *testing.T) {
	engine := New()
	project, root := newProjectPaths(t)
	source := writeFile(t, root, "warehouse/source.txt", "alpha")
	otherSource := writeFile(t, root, "warehouse/other.txt", "beta")
	ownedTarget := filepath.Join(root, "owned.txt")
	unmanagedTarget := filepath.Join(root, "unmanaged.txt")

	if err := engine.ReplaceMappings(project, []Mapping{{Source: source, Target: ownedTarget}}); err != nil {
		t.Fatalf("replace for reset: %v", err)
	}
	if err := os.Symlink(otherSource, unmanagedTarget); err != nil {
		t.Fatalf("create unmanaged symlink: %v", err)
	}

	if err := engine.Reset(project); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if _, err := os.Lstat(ownedTarget); !os.IsNotExist(err) {
		t.Fatalf("owned target still exists, err=%v", err)
	}
	assertFileSymlinkTarget(t, unmanagedTarget, otherSource)
	state, err := engine.LoadCurrentState(project)
	if err != nil {
		t.Fatalf("load state after reset: %v", err)
	}
	if len(state.Mappings) != 0 {
		t.Fatalf("state mappings after reset = %#v, want empty", state.Mappings)
	}
}

func TestResetWithForceRemovesDriftedDirectoryTarget(t *testing.T) {
	engine := New()
	project, root := newProjectPaths(t)
	sourceDir := filepath.Join(root, "warehouse", "Skill-A")
	targetDir := filepath.Join(root, "target-skill")
	manualFile := filepath.Join(targetDir, "manual.txt")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "README.md"), []byte("skill-a"), 0o644); err != nil {
		t.Fatalf("write source readme: %v", err)
	}

	if err := engine.ReplaceState(project, CurrentState{Mappings: []Mapping{{Source: sourceDir, Target: targetDir}}}); err != nil {
		t.Fatalf("initial directory replace: %v", err)
	}
	if err := os.Remove(targetDir); err != nil {
		t.Fatalf("remove owned directory symlink: %v", err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir drifted target dir: %v", err)
	}
	if err := os.WriteFile(manualFile, []byte("manual"), 0o644); err != nil {
		t.Fatalf("write drifted target file: %v", err)
	}

	if err := engine.Reset(project, WithForce(true)); err != nil {
		t.Fatalf("forced reset: %v", err)
	}
	if _, err := os.Lstat(targetDir); !os.IsNotExist(err) {
		t.Fatalf("drifted directory target still exists, err=%v", err)
	}
}

func TestReplaceMappingsWithForceRollsBackManagedStateOnly(t *testing.T) {
	engine := New()
	engine.now = func() time.Time { return time.Unix(123, 0) }
	project, root := newProjectPaths(t)
	firstSource := writeFile(t, root, "warehouse/first.txt", "one")
	secondSource := writeFile(t, root, "warehouse/second.txt", "two")
	target := filepath.Join(root, "target.txt")

	if err := engine.ReplaceMappings(project, []Mapping{{Source: firstSource, Target: target}}); err != nil {
		t.Fatalf("initial replace: %v", err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatalf("remove owned target: %v", err)
	}
	if err := os.WriteFile(target, []byte("manual"), 0o644); err != nil {
		t.Fatalf("write unmanaged target: %v", err)
	}

	defaultWriter := engine.writeFile
	engine.writeFile = func(path string, data []byte, perm os.FileMode) error {
		if path == project.HistoryLogPath {
			return errors.New("boom")
		}
		return defaultWriter(path, data, perm)
	}
	err := engine.ReplaceMappings(project, []Mapping{{Source: secondSource, Target: target}}, WithForce(true))
	if err == nil {
		t.Fatal("expected persistence failure")
	}

	assertFileSymlinkTarget(t, target, firstSource)
	if gotContent, readErr := os.ReadFile(target); readErr != nil {
		t.Fatalf("read target after rollback: %v", readErr)
	} else if bytes.Equal(gotContent, []byte("manual")) {
		t.Fatalf("unexpected unmanaged content restored: %q", string(gotContent))
	}
}

func newProjectPaths(t *testing.T) (warehouse.Project, string) {
	t.Helper()
	root := t.TempDir()
	projectPath := filepath.Join(root, ".configfacilitator", "OpenCode")
	backupDir := filepath.Join(projectPath, "Backup")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("mkdir backup dir: %v", err)
	}
	return warehouse.Project{
		Name:             "OpenCode",
		Path:             projectPath,
		BackupDirPath:    backupDir,
		CurrentStatePath: filepath.Join(backupDir, "current_state.json"),
		HistoryLogPath:   filepath.Join(backupDir, "history.log"),
	}, root
}

func writeFile(t *testing.T, root string, rel string, contents string) string {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parents: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path
}

func assertSymlinkTarget(t *testing.T, path string, want string) {
	t.Helper()
	got, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("readlink %s: %v", path, err)
	}
	if got != want {
		t.Fatalf("readlink(%s) = %s, want %s", path, got, want)
	}
}

func assertFileSymlinkTarget(t *testing.T, path string, want string) {
	t.Helper()
	assertSymlinkTarget(t, path, want)

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
