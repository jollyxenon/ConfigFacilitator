package content

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/repository"
)

// TestValidateRelativePathRejectsCrossPlatformEscapeForms verifies lexical validation on every host.
func TestValidateRelativePathRejectsCrossPlatformEscapeForms(t *testing.T) {
	root := t.TempDir()
	invalid := []string{"", ".", "..", "../outside", "a/../outside", "a//b", "/absolute", `\\server\share`, `\rooted`, `C:\Windows`, `C:/Windows`, `C:relative`, `//server/share`, `\\?\C:\Windows`}
	for _, value := range invalid {
		if _, err := ValidateRelativePath(root, value, false); err == nil {
			t.Fatalf("ValidateRelativePath(%q) succeeded", value)
		}
	}
	for _, value := range []string{"nested/file.txt", `nested\file.txt`, "unicodé/文件.txt"} {
		if _, err := ValidateRelativePath(root, value, false); err != nil {
			t.Fatalf("ValidateRelativePath(%q): %v", value, err)
		}
	}
}

// TestValidateRelativePathRejectsSymlinkComponents verifies existing links are never followed.
func TestValidateRelativePathRejectsSymlinkComponents(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require Windows Developer Mode")
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateRelativePath(root, "linked/escaped.txt", false); err == nil {
		t.Fatal("symlink component was accepted")
	}
}

// TestStageCreationCopiesRegularTreesAndRejectsUnsafeObjects verifies staged imports are complete and bounded.
func TestStageCreationCopiesRegularTreesAndRejectsUnsafeObjects(t *testing.T) {
	parent := t.TempDir()
	importRoot := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(filepath.Join(importRoot, "nested", "empty"), 0o750); err != nil {
		t.Fatal(err)
	}
	binary := []byte{0, 1, 2, 255}
	if err := os.WriteFile(filepath.Join(importRoot, "nested", "data.bin"), binary, 0o640); err != nil {
		t.Fatal(err)
	}
	staged, cleanup, err := StageCreation(parent, KindDirectory, Source{Mode: SourcePath, Path: importRoot})
	if err != nil {
		t.Fatalf("StageCreation: %v", err)
	}
	defer cleanup()
	got, err := os.ReadFile(filepath.Join(staged, "nested", "data.bin"))
	if err != nil || !reflect.DeepEqual(got, binary) {
		t.Fatalf("staged bytes=%v err=%v", got, err)
	}
	if info, err := os.Stat(filepath.Join(staged, "nested", "empty")); err != nil || !info.IsDir() {
		t.Fatalf("empty directory missing: info=%v err=%v", info, err)
	}

	if runtime.GOOS != "windows" {
		unsafeRoot := filepath.Join(t.TempDir(), "unsafe")
		if err := os.Mkdir(unsafeRoot, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(importRoot, filepath.Join(unsafeRoot, "link")); err != nil {
			t.Fatal(err)
		}
		if _, _, err := StageCreation(parent, KindDirectory, Source{Mode: SourcePath, Path: unsafeRoot}); err == nil {
			t.Fatal("symlink import succeeded")
		}
	}
	if runtime.GOOS != "windows" {
		fifoRoot := filepath.Join(t.TempDir(), "fifo")
		if err := os.Mkdir(fifoRoot, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := makeFIFO(filepath.Join(fifoRoot, "pipe")); err != nil {
			t.Skipf("cannot create FIFO: %v", err)
		}
		if _, _, err := StageCreation(parent, KindDirectory, Source{Mode: SourcePath, Path: fifoRoot}); err == nil {
			t.Fatal("special-file import succeeded")
		}
	}
}

// TestContentMutationsAreAtomicTransactionalAndLexical verifies nested operations and rollback.
func TestContentMutationsAreAtomicTransactionalAndLexical(t *testing.T) {
	root := t.TempDir()
	settingRoot := filepath.Join(root, "Setting")
	if err := os.Mkdir(settingRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	repo := repository.New(root)
	path := "prompts/system.txt"
	if err := Write(repo, settingRoot, KindDirectory, &path, Source{Mode: SourceBytes, Bytes: []byte("no newline")}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	data, err := Read(settingRoot, KindDirectory, &path)
	if err != nil || string(data) != "no newline" {
		t.Fatalf("Read=%q err=%v", data, err)
	}
	if err := Mkdir(repo, settingRoot, KindDirectory, "empty/nested"); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := Move(repo, settingRoot, KindDirectory, "prompts", "moved/prompts"); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if err := Move(repo, settingRoot, KindDirectory, "moved/prompts", "empty"); err == nil {
		t.Fatal("move conflict succeeded")
	}
	entries, err := List(settingRoot, KindDirectory)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []Entry{{Path: "empty", Kind: "directory", Size: 0}, {Path: "empty/nested", Kind: "directory", Size: 0}, {Path: "moved", Kind: "directory", Size: 0}, {Path: "moved/prompts", Kind: "directory", Size: 0}, {Path: "moved/prompts/system.txt", Kind: "file", Size: 10}}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries=%#v want=%#v", entries, want)
	}
	if err := Delete(repo, settingRoot, KindDirectory, "moved", false); err == nil {
		t.Fatal("unconfirmed delete succeeded")
	}
	if err := Delete(repo, settingRoot, KindDirectory, "moved", true); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	rollbackRepo := repository.New(root, repository.WithHooks(repository.Hooks{BeforeStage: func(stage repository.Stage) error {
		if stage == repository.StageCommitted {
			return errors.New("injected")
		}
		return nil
	}}))
	fileRoot := filepath.Join(root, "FileSetting")
	if err := os.WriteFile(fileRoot, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(rollbackRepo, fileRoot, KindFile, nil, Source{Mode: SourceBytes, Bytes: []byte("after")}); err == nil {
		t.Fatal("expected injected write failure")
	}
	if got, err := os.ReadFile(fileRoot); err != nil || string(got) != "before" {
		t.Fatalf("rollback bytes=%q err=%v", got, err)
	}

	if err := os.MkdirAll(filepath.Join(settingRoot, "rollback", "source"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(settingRoot, "rollback", "source", "value.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Move(rollbackRepo, settingRoot, KindDirectory, "rollback/source", "created/parents/destination"); err == nil {
		t.Fatal("expected injected move failure")
	}
	if got, err := os.ReadFile(filepath.Join(settingRoot, "rollback", "source", "value.txt")); err != nil || string(got) != "keep" {
		t.Fatalf("move rollback source=%q err=%v", got, err)
	}
	if _, err := os.Lstat(filepath.Join(settingRoot, "created")); !os.IsNotExist(err) {
		t.Fatalf("move rollback left parents: %v", err)
	}
	if err := Mkdir(rollbackRepo, settingRoot, KindDirectory, "new/deep/tree"); err == nil {
		t.Fatal("expected injected mkdir failure")
	}
	if _, err := os.Lstat(filepath.Join(settingRoot, "new")); !os.IsNotExist(err) {
		t.Fatalf("mkdir rollback left parents: %v", err)
	}
}
