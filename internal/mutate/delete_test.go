package mutate_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/mutate"
	"github.com/xenon/ConfigFacilitator/internal/repository"
)

// TestDependencyReportCoversEveryCategory verifies stable dependency discovery before deletion.
func TestDependencyReportCoversEveryCategory(t *testing.T) {
	root, repo, _ := createRenameFixture(t)
	source := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	target := filepath.Join(root, "managed", "GPT.json")
	state := repository.CurrentState{Mappings: []repository.Mapping{{Source: source, Target: target}}, Intent: &repository.ApplyIntent{Kind: "column", Column: "Models", Settings: []string{"GPT.json"}}}
	if err := repo.SaveCurrentState("OpenCode", state); err != nil { t.Fatal(err) }
	if err := repo.SaveHistory("OpenCode", []repository.HistoryEntry{{Timestamp: "t", PreviousMappings: state.Mappings, NextMappings: state.Mappings, PreviousIntent: state.Intent, NextIntent: state.Intent}}); err != nil { t.Fatal(err) }
	if err := repo.SaveSession(77, repository.SessionRecord{Project: "OpenCode"}); err != nil { t.Fatal(err) }

	report, err := mutate.BuildDependencyReport(repo, mutate.DeleteRequest{Kind: mutate.SettingKind, ProjectReference: "OpenCode", ColumnReference: "Models", Reference: "GPT.json"})
	if err != nil { t.Fatal(err) }
	if len(report.ModeSelections) != 2 || len(report.CurrentMappings) != 1 || report.CurrentIntent == nil || len(report.HistoryReferences) != 1 {
		t.Fatalf("setting dependency report = %#v", report)
	}
	projectReport, err := mutate.BuildDependencyReport(repo, mutate.DeleteRequest{Kind: mutate.ProjectKind, Reference: "OpenCode"})
	if err != nil { t.Fatal(err) }
	if len(projectReport.PPIDContexts) != 1 || projectReport.PPIDContexts[0].PPID != 77 {
		t.Fatalf("project contexts = %#v", projectReport.PPIDContexts)
	}
}

// TestDeleteRequiresIndependentConfirmationAndCascade verifies --yes does not imply cascade.
func TestDeleteRequiresIndependentConfirmationAndCascade(t *testing.T) {
	root, repo, _ := createRenameFixture(t)
	_, err := mutate.DeleteSetting(repo, "OpenCode", "Models", "GPT.json", false, true, true)
	assertDeleteError(t, err, mutate.RefusalError, "confirmation_required")
	if _, statErr := os.Lstat(filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")); statErr != nil { t.Fatal(statErr) }

	report, err := mutate.DeleteSetting(repo, "OpenCode", "Models", "GPT.json", true, false, false)
	assertDeleteError(t, err, mutate.RefusalError, "dependencies_exist")
	if len(report.ModeSelections) != 2 { t.Fatalf("report = %#v", report) }
	if _, statErr := os.Lstat(filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")); statErr != nil { t.Fatal(statErr) }
}

// TestSettingCascadeRepairsSelectionsMappingsAndHistory verifies Setting cascade isolation.
func TestSettingCascadeRepairsSelectionsMappingsAndHistory(t *testing.T) {
	root, repo, _ := createRenameFixture(t)
	other := filepath.Join(root, "OpenCode", "Column", "Models", "Other.json")
	if err := mutate.CreateSetting(repo, "OpenCode", "Models", "Other.json", "file", renameMetadata(t, mutate.SettingKind, "Other.json", nil)); err != nil { t.Fatal(err) }
	source := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	state := repository.CurrentState{Mappings: []repository.Mapping{{Source: source, Target: filepath.Join(root, "targets-a")}, {Source: other, Target: filepath.Join(root, "targets-b")}}, Intent: &repository.ApplyIntent{Kind: "column", Column: "Models", Settings: []string{"GPT.json", "Other.json"}}}
	if err := repo.SaveCurrentState("OpenCode", state); err != nil { t.Fatal(err) }
	if err := repo.SaveHistory("OpenCode", []repository.HistoryEntry{{Timestamp: "t", PreviousMappings: state.Mappings, NextMappings: state.Mappings, PreviousIntent: state.Intent, NextIntent: state.Intent}}); err != nil { t.Fatal(err) }

	if _, err := mutate.DeleteSetting(repo, "OpenCode", "Models", "GPT.json", true, true, false); err != nil { t.Fatal(err) }
	settingIndex, _ := repo.LoadSettingIndex("OpenCode", "Models")
	if _, ok := settingIndex.Settings["GPT.json"]; ok { t.Fatal("deleted Setting metadata survived") }
	if _, ok := settingIndex.Settings["Other.json"]; !ok { t.Fatal("unrelated Setting removed") }
	modeIndex, _ := repo.LoadModeIndex("OpenCode")
	if len(modeIndex.Modes["Max"].Columns) != 0 || len(modeIndex.Modes["Other"].Columns) != 0 {
		t.Fatalf("empty selections survived: %#v", modeIndex)
	}
	current, _ := repo.LoadCurrentState("OpenCode")
	if len(current.Mappings) != 1 || current.Mappings[0].Source != other || current.Intent == nil || !reflect.DeepEqual(current.Intent.Settings, []string{"Other.json"}) {
		t.Fatalf("current = %#v", current)
	}
	history, _ := repo.LoadHistory("OpenCode")
	if len(history[0].PreviousMappings) != 1 || !reflect.DeepEqual(history[0].NextIntent.Settings, []string{"Other.json"}) {
		t.Fatalf("history = %#v", history)
	}
}

// TestColumnAndModeCascadeSemantics verifies Column isolation and Mode mapping-only state.
func TestColumnAndModeCascadeSemantics(t *testing.T) {
	root, repo, _ := createRenameFixture(t)
	if err := mutate.CreateColumn(repo, "OpenCode", "OtherColumn", renameMetadata(t, mutate.ColumnKind, "OtherColumn", nil)); err != nil { t.Fatal(err) }
	if err := mutate.CreateSetting(repo, "OpenCode", "OtherColumn", "Keep.json", "file", renameMetadata(t, mutate.SettingKind, "Keep.json", nil)); err != nil { t.Fatal(err) }
	modelsSource := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	otherSource := filepath.Join(root, "OpenCode", "Column", "OtherColumn", "Keep.json")
	state := repository.CurrentState{Mappings: []repository.Mapping{{Source: modelsSource, Target: filepath.Join(root, "m")}, {Source: otherSource, Target: filepath.Join(root, "o")}}, Intent: &repository.ApplyIntent{Kind: "column", Column: "Models", Settings: []string{"GPT.json"}}}
	if err := repo.SaveCurrentState("OpenCode", state); err != nil { t.Fatal(err) }
	if _, err := mutate.DeleteColumn(repo, "OpenCode", "Models", true, true, false); err != nil { t.Fatal(err) }
	current, _ := repo.LoadCurrentState("OpenCode")
	if len(current.Mappings) != 1 || current.Mappings[0].Source != otherSource || current.Intent != nil {
		t.Fatalf("column current = %#v", current)
	}
	if _, err := os.Stat(filepath.Join(root, "OpenCode", "Column", "OtherColumn", "Keep.json")); err != nil {
		t.Fatal(err)
	}

	if err := repo.SaveCurrentState("OpenCode", repository.CurrentState{Mappings: current.Mappings, Intent: &repository.ApplyIntent{Kind: "mode", Mode: "Max"}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveHistory("OpenCode", []repository.HistoryEntry{{Timestamp: "t", PreviousMappings: current.Mappings, NextMappings: current.Mappings, PreviousIntent: &repository.ApplyIntent{Kind: "mode", Mode: "Max"}, NextIntent: &repository.ApplyIntent{Kind: "mode", Mode: "Other"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := mutate.DeleteMode(repo, "OpenCode", "Max", true, true, false); err != nil {
		t.Fatal(err)
	}
	current, _ = repo.LoadCurrentState("OpenCode")
	if current.Intent != nil || len(current.Mappings) != 1 {
		t.Fatalf("mode current = %#v", current)
	}
	history, _ := repo.LoadHistory("OpenCode")
	if history[0].PreviousIntent != nil || history[0].NextIntent == nil || history[0].NextIntent.Mode != "Other" || len(history[0].PreviousMappings) != 1 {
		t.Fatalf("mode history = %#v", history)
	}
}

// TestProjectCascadeOwnershipForceAndContexts verifies drift refusal, recursive force, and context cleanup.
func TestProjectCascadeOwnershipForceAndContexts(t *testing.T) {
	root, repo, _ := createRenameFixture(t)
	source := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
	target := filepath.Join(root, "outside", "recorded")
	if err := os.MkdirAll(target, 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(target, "drift"), []byte("x"), 0o644); err != nil { t.Fatal(err) }
	if err := repo.SaveCurrentState("OpenCode", repository.CurrentState{Mappings: []repository.Mapping{{Source: source, Target: target}}}); err != nil { t.Fatal(err) }
	if err := repo.SaveSession(88, repository.SessionRecord{Project: "OpenCode"}); err != nil { t.Fatal(err) }

	_, err := mutate.DeleteProject(repo, "OpenCode", true, true, false)
	assertDeleteError(t, err, mutate.RefusalError, "unsafe_target")
	if _, err := os.Stat(filepath.Join(target, "drift")); err != nil {
		t.Fatal(err)
	}
	if _, err := mutate.DeleteProject(repo, "OpenCode", true, true, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("recorded directory survived: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "OpenCode")); !os.IsNotExist(err) {
		t.Fatalf("Project survived: %v", err)
	}
	if _, ok, err := repo.LoadSession(88); err != nil || ok {
		t.Fatalf("context survived ok=%v err=%v", ok, err)
	}
}

// TestDeleteRollbackAtEveryDestructiveStage verifies source, indexes, state, and links restore exactly.
func TestDeleteRollbackAtEveryDestructiveStage(t *testing.T) {
	for _, stage := range []repository.Stage{repository.StageWrite, repository.StageCommitted, repository.StageCleanup} {
		t.Run(string(stage), func(t *testing.T) {
			root, repo, _ := createRenameFixture(t)
			source := filepath.Join(root, "OpenCode", "Column", "Models", "GPT.json")
			target := filepath.Join(root, "managed", "GPT.json")
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(source, target); err != nil {
				t.Fatal(err)
			}
			if err := repo.SaveCurrentState("OpenCode", repository.CurrentState{Mappings: []repository.Mapping{{Source: source, Target: target}}}); err != nil {
				t.Fatal(err)
			}
			faulty := repository.New(root, repository.WithHooks(repository.Hooks{BeforeStage: func(got repository.Stage) error {
				if got == stage {
					return errors.New("injected delete failure")
				}
				return nil
			}}))
			if _, err := mutate.DeleteSetting(faulty, "OpenCode", "Models", "GPT.json", true, true, false); err == nil {
				t.Fatal("expected failure")
			}
			if data, err := os.ReadFile(source); err != nil || string(data) != "content" {
				t.Fatalf("source = %q err=%v", data, err)
			}
			if link, err := os.Readlink(target); err != nil || link != source {
				t.Fatalf("link = %q err=%v", link, err)
			}
			settingIndex, _ := repo.LoadSettingIndex("OpenCode", "Models")
			if _, ok := settingIndex.Settings["GPT.json"]; !ok {
				t.Fatal("metadata not restored")
			}
		})
	}
}

// TestNonCascadeDeleteRollbackKeepsSourceAndIndexAtomic verifies dependency-free delete rollback.
func TestNonCascadeDeleteRollbackKeepsSourceAndIndexAtomic(t *testing.T) {
	root := t.TempDir()
	repo := repository.New(root)
	if err := mutate.CreateProject(repo, "OpenCode", renameMetadata(t, mutate.ProjectKind, "OpenCode", nil)); err != nil {
		t.Fatal(err)
	}
	if err := mutate.CreateColumn(repo, "OpenCode", "Empty", renameMetadata(t, mutate.ColumnKind, "Empty", nil)); err != nil {
		t.Fatal(err)
	}
	if err := mutate.CreateSetting(repo, "OpenCode", "Empty", "Unused", "file", renameMetadata(t, mutate.SettingKind, "Unused", nil)); err != nil {
		t.Fatal(err)
	}
	faulty := repository.New(root, repository.WithHooks(repository.Hooks{BeforeStage: func(stage repository.Stage) error {
		if stage == repository.StageCommitted {
			return errors.New("injected non-cascade failure")
		}
		return nil
	}}))
	if _, err := mutate.DeleteSetting(faulty, "OpenCode", "Empty", "Unused", true, false, false); err == nil {
		t.Fatal("expected failure")
	}
	if _, err := os.Lstat(filepath.Join(root, "OpenCode", "Column", "Empty", "Unused")); err != nil {
		t.Fatalf("source not restored: %v", err)
	}
	settingIndex, err := repo.LoadSettingIndex("OpenCode", "Empty")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := settingIndex.Settings["Unused"]; !ok {
		t.Fatal("index entry not restored")
	}
}

// assertDeleteError checks stable deletion error classification.
func assertDeleteError(t *testing.T, err error, kind mutate.ErrorKind, code string) {
	t.Helper()
	var mutationErr *mutate.Error
	if !errors.As(err, &mutationErr) || mutationErr.Kind != kind || mutationErr.Code != code {
		t.Fatalf("error = %#v want kind=%s code=%s", err, kind, code)
	}
}

// Keep the index import in compile-time coverage for future selection assertions.
var _ = index.ModeEntry{}
