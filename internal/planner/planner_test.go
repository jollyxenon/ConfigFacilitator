package planner

import (
	"path/filepath"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/linker"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

func TestPlanColumnMappingsUsesExplicitAndDefaultDirNameTargets(t *testing.T) {
	project := sampleProject()
	mappings, err := PlanColumnMappings(project, "config", []string{"claude", "Special.json"}, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"})
	if err != nil {
		t.Fatalf("plan column mappings: %v", err)
	}
	if len(mappings) != 3 {
		t.Fatalf("len(mappings) = %d, want 3", len(mappings))
	}
	if mappings[0].Target != "/home/test/.config/opencode/opencode.json" {
		t.Fatalf("default target = %q", mappings[0].Target)
	}
	if mappings[1].Target != "/home/test/.config/opencode/special/special.json" {
		t.Fatalf("explicit target = %q", mappings[1].Target)
	}
	if mappings[2].Target != "/home/test/.config/opencode/special/backup.json" {
		t.Fatalf("second explicit target = %q", mappings[2].Target)
	}
}

func TestPlanColumnMappingsExpandsMultipleDefaultDirNameTargets(t *testing.T) {
	project := sampleProject()
	column := project.Columns["opencode.json"]
	column.SettingIndex.DefaultTargetDir = []string{"~/.config/opencode", "~/.config/opencode"}
	column.SettingIndex.DefaultTargetName = []string{"opencode.json", "backup.json"}
	project.Columns["opencode.json"] = column

	mappings, err := PlanColumnMappings(project, "config", []string{"GPT.json"}, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"})
	if err != nil {
		t.Fatalf("plan column mappings: %v", err)
	}

	if len(mappings) != 2 {
		t.Fatalf("len(mappings) = %d, want 2", len(mappings))
	}
	assertContainsTarget(t, mappings, "/home/test/.config/opencode/opencode.json", filepath.Join(project.Columns["opencode.json"].Path, "GPT.json"))
	assertContainsTarget(t, mappings, "/home/test/.config/opencode/backup.json", filepath.Join(project.Columns["opencode.json"].Path, "GPT.json"))
}

func TestPlanModeMappingsHandlesCoverAndIncrementColumns(t *testing.T) {
	project := sampleProject()
	current := []linker.Mapping{{Source: filepath.Join(project.Columns["Skills"].Path, "Skill-Old"), Target: "/tmp/skills/old"}, {Source: filepath.Join(project.Columns["opencode.json"].Path, "GPT.json"), Target: "/home/test/.config/opencode/opencode.json"}}
	mappings, err := PlanModeMappings(project, "m", current, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"})
	if err != nil {
		t.Fatalf("plan mode mappings: %v", err)
	}
	if len(mappings) != 3 {
		t.Fatalf("len(mappings) = %d, want 3", len(mappings))
	}
	assertContainsTarget(t, mappings, "/home/test/.config/opencode/opencode.json", filepath.Join(project.Columns["opencode.json"].Path, "CLAUDE.json"))
	assertContainsTarget(t, mappings, "/tmp/skills/old", filepath.Join(project.Columns["Skills"].Path, "Skill-Old"))
	assertContainsTarget(t, mappings, "/tmp/skills/a", filepath.Join(project.Columns["Skills"].Path, "Skill-A"))
}

func TestPlanModeMappingsHandlesNoneAndFullColumns(t *testing.T) {
	project := sampleProject()
	column := project.Columns["opencode.json"]
	claude := column.Settings["CLAUDE.json"]
	claude.Metadata.TargetDir = []string{"~/.config/opencode"}
	claude.Metadata.TargetName = []string{"claude.json"}
	column.Settings["CLAUDE.json"] = claude
	project.Columns["opencode.json"] = column
	project.Modes["All"] = warehouse.Mode{
		Name:          "All",
		WarehouseName: "All",
		Metadata: index.ModeEntry{Columns: map[string]index.ModeColumnSelection{
			"opencode.json": {Strategy: "full"},
			"Skills":        {Strategy: "none"},
		}, WarehouseName: "All"},
	}
	current := []linker.Mapping{{Source: filepath.Join(project.Columns["Skills"].Path, "Skill-Old"), Target: "/tmp/skills/old"}}
	mappings, err := PlanModeMappings(project, "All", current, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"})
	if err != nil {
		t.Fatalf("plan mode mappings: %v", err)
	}
	if len(mappings) != 4 {
		t.Fatalf("len(mappings) = %d, want 4", len(mappings))
	}
	assertContainsTarget(t, mappings, "/home/test/.config/opencode/claude.json", filepath.Join(project.Columns["opencode.json"].Path, "CLAUDE.json"))
	assertContainsTarget(t, mappings, "/home/test/.config/opencode/opencode.json", filepath.Join(project.Columns["opencode.json"].Path, "GPT.json"))
	assertContainsTarget(t, mappings, "/home/test/.config/opencode/special/special.json", filepath.Join(project.Columns["opencode.json"].Path, "Special.json"))
	assertContainsTarget(t, mappings, "/home/test/.config/opencode/special/backup.json", filepath.Join(project.Columns["opencode.json"].Path, "Special.json"))
	assertDoesNotContainTarget(t, mappings, "/tmp/skills/old")
}

func TestPlanModeMappingsRejectsUnknownOrIncompleteStrategies(t *testing.T) {
	project := sampleProject()
	project.Modes["Broken"] = warehouse.Mode{
		Name:          "Broken",
		WarehouseName: "Broken",
		Metadata: index.ModeEntry{Columns: map[string]index.ModeColumnSelection{
			"Skills": {Strategy: "unknown"},
		}, WarehouseName: "Broken"},
	}
	if _, err := PlanModeMappings(project, "Broken", nil, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"}); err == nil {
		t.Fatalf("expected unknown strategy to fail")
	}

	project.Modes["Empty"] = warehouse.Mode{
		Name:          "Empty",
		WarehouseName: "Empty",
		Metadata: index.ModeEntry{Columns: map[string]index.ModeColumnSelection{
			"Skills": {Strategy: "cover"},
		}, WarehouseName: "Empty"},
	}
	if _, err := PlanModeMappings(project, "Empty", nil, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"}); err == nil {
		t.Fatalf("expected cover without settings to fail")
	}
}

func TestPlanColumnMappingsRejectsInvalidTargetParts(t *testing.T) {
	project := sampleProject()
	column := project.Columns["opencode.json"]
	setting := column.Settings["Special.json"]
	setting.Metadata.TargetDir = []string{"~/.config/opencode"}
	setting.Metadata.TargetName = []string{"special.json", "backup.json"}
	column.Settings["Special.json"] = setting
	project.Columns["opencode.json"] = column
	if _, err := PlanColumnMappings(project, "config", []string{"Special.json"}, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"}); err == nil {
		t.Fatalf("expected mismatched target arrays to fail")
	}

	setting.Metadata.TargetDir = []string{""}
	setting.Metadata.TargetName = []string{"special.json"}
	column.SettingIndex.DefaultTargetDir = []string{""}
	column.SettingIndex.DefaultTargetName = []string{""}
	column.Settings["Special.json"] = setting
	project.Columns["opencode.json"] = column
	if _, err := PlanColumnMappings(project, "config", []string{"Special.json"}, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"}); err == nil {
		t.Fatalf("expected empty resolved target directory to fail")
	}

	column.SettingIndex.DefaultTargetDir = []string{"~/.config/opencode"}
	column.SettingIndex.DefaultTargetName = []string{"opencode.json"}
	setting.Metadata.TargetDir = []string{"~/.config/opencode"}
	setting.Metadata.TargetName = []string{"../special.json"}
	column.Settings["Special.json"] = setting
	project.Columns["opencode.json"] = column
	if _, err := PlanColumnMappings(project, "config", []string{"Special.json"}, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"}); err == nil {
		t.Fatalf("expected invalid target name to fail")
	}

	setting.Metadata.TargetDir = []string{"~/.config/opencode", "${HOME}/.config/opencode"}
	setting.Metadata.TargetName = []string{"same.json", "same.json"}
	column.Settings["Special.json"] = setting
	project.Columns["opencode.json"] = column
	if _, err := PlanColumnMappings(project, "config", []string{"Special.json"}, PlanOptions{HomeDir: "/home/test", Env: map[string]string{"HOME": "/home/test"}, OS: "linux"}); err == nil {
		t.Fatalf("expected duplicate resolved targets to fail")
	}
}

func TestPlanColumnMappingsAllowsVariantsToShareTargetInSeparatePlans(t *testing.T) {
	project := sampleProject()
	column := project.Columns["Skills"]
	column.SettingIndex.DefaultTargetDir = []string{"/tmp/skills"}
	column.SettingIndex.DefaultTargetName = []string{""}
	variantA := warehouse.Setting{Name: "Skill-3-A", WarehouseName: "Skill-3-A", Path: filepath.Join(column.Path, "Skill-3-A"), Metadata: index.SettingEntry{WarehouseName: "Skill-3-A", TargetDir: []string{""}, TargetName: []string{"Skill-3"}}}
	variantB := warehouse.Setting{Name: "Skill-3-B", WarehouseName: "Skill-3-B", Path: filepath.Join(column.Path, "Skill-3-B"), Metadata: index.SettingEntry{WarehouseName: "Skill-3-B", TargetDir: []string{""}, TargetName: []string{"Skill-3"}}}
	column.Settings["Skill-3-A"] = variantA
	column.Settings["Skill-3-B"] = variantB
	project.Columns["Skills"] = column

	first, err := PlanColumnMappings(project, "skills", []string{"Skill-3-A"}, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"})
	if err != nil {
		t.Fatalf("plan first variant: %v", err)
	}
	assertContainsTarget(t, first, "/tmp/skills/Skill-3", variantA.Path)

	second, err := PlanColumnMappings(project, "skills", []string{"Skill-3-B"}, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"})
	if err != nil {
		t.Fatalf("plan second variant: %v", err)
	}
	assertContainsTarget(t, second, "/tmp/skills/Skill-3", variantB.Path)

	if _, err := PlanColumnMappings(project, "skills", []string{"Skill-3-A", "Skill-3-B"}, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"}); err == nil {
		t.Fatalf("expected duplicate variant targets to fail")
	}
}

func TestPlanUpdateMappingsRefreshesCurrentSources(t *testing.T) {
	project := sampleProject()
	current := []linker.Mapping{
		{Source: filepath.Join(project.Columns["opencode.json"].Path, "GPT.json"), Target: "/stale/target"},
		{Source: filepath.Join(project.Columns["Skills"].Path, "Skill-A"), Target: "/tmp/old-skills/a"},
	}

	mappings, err := PlanUpdateMappings(project, current, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"})
	if err != nil {
		t.Fatalf("plan update mappings: %v", err)
	}

	if len(mappings) != 2 {
		t.Fatalf("len(mappings) = %d, want 2", len(mappings))
	}
	assertContainsTarget(t, mappings, "/home/test/.config/opencode/opencode.json", filepath.Join(project.Columns["opencode.json"].Path, "GPT.json"))
	assertContainsTarget(t, mappings, "/tmp/skills/a", filepath.Join(project.Columns["Skills"].Path, "Skill-A"))
}

func TestPlanUpdateMappingsRejectsUnmatchedCurrentSource(t *testing.T) {
	project := sampleProject()
	current := []linker.Mapping{{Source: filepath.Join(project.Path, "Column", "Missing", "Ghost"), Target: "/tmp/ghost"}}

	if _, err := PlanUpdateMappings(project, current, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"}); err == nil {
		t.Fatalf("expected unmatched current source to fail")
	}
}

func TestPlanColumnUpdateMappingsRefreshesSelectedColumnAndPreservesOthers(t *testing.T) {
	project := sampleProject()
	current := []linker.Mapping{
		{Source: filepath.Join(project.Columns["opencode.json"].Path, "GPT.json"), Target: "/stale/target"},
		{Source: filepath.Join(project.Columns["Skills"].Path, "Skill-A"), Target: "/tmp/skills/a"},
	}

	mappings, err := PlanColumnUpdateMappings(project, "config", current, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"})
	if err != nil {
		t.Fatalf("plan column update mappings: %v", err)
	}

	if len(mappings) != 2 {
		t.Fatalf("len(mappings) = %d, want 2", len(mappings))
	}
	assertContainsTarget(t, mappings, "/home/test/.config/opencode/opencode.json", filepath.Join(project.Columns["opencode.json"].Path, "GPT.json"))
	assertContainsTarget(t, mappings, "/tmp/skills/a", filepath.Join(project.Columns["Skills"].Path, "Skill-A"))
}

func TestPlanColumnUpdateMappingsRejectsInactiveSelectedColumn(t *testing.T) {
	project := sampleProject()
	current := []linker.Mapping{{Source: filepath.Join(project.Columns["Skills"].Path, "Skill-A"), Target: "/tmp/skills/a"}}

	if _, err := PlanColumnUpdateMappings(project, "config", current, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"}); err == nil {
		t.Fatalf("expected inactive selected column to fail")
	}
}

func TestPlanIntentUpdateMappingsUsesFullModeCurrentMetadata(t *testing.T) {
	project := sampleProjectWithFullSkillsMode()
	current := []linker.Mapping{
		{Source: filepath.Join(project.Columns["opencode.json"].Path, "GPT.json"), Target: "/home/test/.config/opencode/opencode.json"},
		{Source: filepath.Join(project.Columns["Skills"].Path, "Skill-A"), Target: "/tmp/skills/a"},
	}

	mappings, err := PlanIntentUpdateMappings(project, linker.ApplyIntent{Kind: "mode", Mode: "Max"}, current, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"})
	if err != nil {
		t.Fatalf("plan intent update mappings: %v", err)
	}

	assertContainsTarget(t, mappings, "/home/test/.config/opencode/opencode.json", filepath.Join(project.Columns["opencode.json"].Path, "CLAUDE.json"))
	assertContainsTarget(t, mappings, "/tmp/skills/a", filepath.Join(project.Columns["Skills"].Path, "Skill-A"))
	assertContainsTarget(t, mappings, "/tmp/skills/new", filepath.Join(project.Columns["Skills"].Path, "Skill-New"))
	assertContainsTarget(t, mappings, "/tmp/skills/old", filepath.Join(project.Columns["Skills"].Path, "Skill-Old"))
}

func TestPlanIntentColumnUpdateMappingsRefreshesFullModeColumnAndPreservesOthers(t *testing.T) {
	project := sampleProjectWithFullSkillsMode()
	current := []linker.Mapping{
		{Source: filepath.Join(project.Columns["opencode.json"].Path, "GPT.json"), Target: "/stale/config-target"},
		{Source: filepath.Join(project.Columns["Skills"].Path, "Skill-A"), Target: "/tmp/skills/a"},
	}

	mappings, err := PlanIntentColumnUpdateMappings(project, linker.ApplyIntent{Kind: "mode", Mode: "Max"}, "skills", current, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"})
	if err != nil {
		t.Fatalf("plan intent column update mappings: %v", err)
	}

	assertContainsTarget(t, mappings, "/stale/config-target", filepath.Join(project.Columns["opencode.json"].Path, "GPT.json"))
	assertContainsTarget(t, mappings, "/tmp/skills/a", filepath.Join(project.Columns["Skills"].Path, "Skill-A"))
	assertContainsTarget(t, mappings, "/tmp/skills/new", filepath.Join(project.Columns["Skills"].Path, "Skill-New"))
	assertContainsTarget(t, mappings, "/tmp/skills/old", filepath.Join(project.Columns["Skills"].Path, "Skill-Old"))
}

func TestPlanIntentUpdateMappingsRejectsMalformedIntent(t *testing.T) {
	project := sampleProject()
	if _, err := PlanIntentUpdateMappings(project, linker.ApplyIntent{Kind: "mode"}, nil, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"}); err == nil {
		t.Fatalf("expected malformed mode intent to fail")
	}
	if _, err := PlanIntentColumnUpdateMappings(project, linker.ApplyIntent{Kind: "unknown"}, "skills", nil, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"}); err == nil {
		t.Fatalf("expected unsupported intent to fail")
	}
}

func sampleProject() warehouse.Project {
	projectPath := "/warehouse/OpenCode"
	return warehouse.Project{
		Name:          "OpenCode",
		WarehouseName: "OpenCode",
		Columns: map[string]warehouse.Column{
			"opencode.json": {
				Name:          "opencode.json",
				WarehouseName: "opencode.json",
				Path:          filepath.Join(projectPath, "Column", "opencode.json"),
				Metadata:      index.ColumnEntry{WarehouseName: "opencode.json", Aliases: []string{"config"}},
				SettingIndex:  index.SettingIndex{DefaultTargetDir: []string{"~/.config/opencode"}, DefaultTargetName: []string{"opencode.json"}},
				Settings: map[string]warehouse.Setting{
					"CLAUDE.json":  {Name: "CLAUDE.json", WarehouseName: "CLAUDE.json", Path: filepath.Join(projectPath, "Column", "opencode.json", "CLAUDE.json"), Metadata: index.SettingEntry{WarehouseName: "CLAUDE.json", Aliases: []string{"claude"}}},
					"GPT.json":     {Name: "GPT.json", WarehouseName: "GPT.json", Path: filepath.Join(projectPath, "Column", "opencode.json", "GPT.json"), Metadata: index.SettingEntry{WarehouseName: "GPT.json"}},
					"Special.json": {Name: "Special.json", WarehouseName: "Special.json", Path: filepath.Join(projectPath, "Column", "opencode.json", "Special.json"), Metadata: index.SettingEntry{WarehouseName: "Special.json", TargetDir: []string{"~/.config/opencode/special", "~/.config/opencode/special"}, TargetName: []string{"special.json", "backup.json"}}},
				},
			},
			"Skills": {
				Name:          "Skills",
				WarehouseName: "Skills",
				Path:          filepath.Join(projectPath, "Column", "Skills"),
				Metadata:      index.ColumnEntry{WarehouseName: "Skills", Aliases: []string{"skills"}},
				SettingIndex:  index.SettingIndex{DefaultTargetDir: []string{"/tmp/skills"}, DefaultTargetName: []string{""}},
				Settings: map[string]warehouse.Setting{
					"Skill-A":   {Name: "Skill-A", WarehouseName: "Skill-A", Path: filepath.Join(projectPath, "Column", "Skills", "Skill-A"), Metadata: index.SettingEntry{WarehouseName: "Skill-A", Aliases: []string{"alpha"}, TargetDir: []string{""}, TargetName: []string{"a"}}},
					"Skill-Old": {Name: "Skill-Old", WarehouseName: "Skill-Old", Path: filepath.Join(projectPath, "Column", "Skills", "Skill-Old"), Metadata: index.SettingEntry{WarehouseName: "Skill-Old", TargetDir: []string{""}, TargetName: []string{"old"}}},
				},
			},
		},
		Modes: map[string]warehouse.Mode{
			"Max": {
				Name:          "Max",
				WarehouseName: "Max",
				Metadata: index.ModeEntry{Columns: map[string]index.ModeColumnSelection{
					"opencode.json": {Settings: []string{"claude"}, Strategy: "cover"},
					"skills":        {Settings: []string{"alpha"}, Strategy: "increment"},
				}, WarehouseName: "Max", Aliases: []string{"m"}},
			},
		},
	}
}

func sampleProjectWithFullSkillsMode() warehouse.Project {
	project := sampleProject()
	project.Columns["Skills"].Settings["Skill-New"] = warehouse.Setting{Name: "Skill-New", WarehouseName: "Skill-New", Path: filepath.Join(project.Columns["Skills"].Path, "Skill-New"), Metadata: index.SettingEntry{WarehouseName: "Skill-New", TargetDir: []string{""}, TargetName: []string{"new"}}}
	mode := project.Modes["Max"]
	mode.Metadata.Columns["skills"] = index.ModeColumnSelection{Strategy: "full"}
	project.Modes["Max"] = mode
	return project
}

func assertContainsTarget(t *testing.T, mappings []linker.Mapping, target string, source string) {
	t.Helper()
	for _, mapping := range mappings {
		if mapping.Target == target && mapping.Source == source {
			return
		}
	}
	t.Fatalf("missing mapping target=%q source=%q in %#v", target, source, mappings)
}

func assertDoesNotContainTarget(t *testing.T, mappings []linker.Mapping, target string) {
	t.Helper()
	for _, mapping := range mappings {
		if mapping.Target == target {
			t.Fatalf("unexpected mapping target=%q in %#v", target, mappings)
		}
	}
}
