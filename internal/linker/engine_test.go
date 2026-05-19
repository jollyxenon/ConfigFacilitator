package linker

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
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

func newProjectPaths(t *testing.T) (warehouse.Project, string) {
	t.Helper()
	root := t.TempDir()
	projectPath := filepath.Join(root, ".configfacilitator", "SettingWarehouse", "OpenCode")
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
