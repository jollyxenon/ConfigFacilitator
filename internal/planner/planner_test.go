package planner

import (
	"path/filepath"
	"testing"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/linker"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

func TestPlanColumnMappingsUsesExplicitAndDefaultTargets(t *testing.T) {
	project := sampleProject()
	mappings, err := PlanColumnMappings(project, "config", []string{"claude", "Special.json"}, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"})
	if err != nil {
		t.Fatalf("plan column mappings: %v", err)
	}
	if len(mappings) != 2 {
		t.Fatalf("len(mappings) = %d, want 2", len(mappings))
	}
	if mappings[0].Target != "/home/test/.config/opencode/opencode.json" {
		t.Fatalf("default target = %q", mappings[0].Target)
	}
	if mappings[1].Target != "/home/test/.config/opencode/special/special.json" {
		t.Fatalf("explicit target = %q", mappings[1].Target)
	}
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
	if len(mappings) != 2 {
		t.Fatalf("len(mappings) = %d, want 2", len(mappings))
	}
	assertContainsTarget(t, mappings, "/home/test/.config/opencode/opencode.json", filepath.Join(project.Columns["opencode.json"].Path, "GPT.json"))
	assertContainsTarget(t, mappings, "/home/test/.config/opencode/special/special.json", filepath.Join(project.Columns["opencode.json"].Path, "Special.json"))
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
				SettingIndex:  index.SettingIndex{DefaultTarget: "~/.config/opencode/opencode.json"},
				Settings: map[string]warehouse.Setting{
					"CLAUDE.json":  {Name: "CLAUDE.json", WarehouseName: "CLAUDE.json", Path: filepath.Join(projectPath, "Column", "opencode.json", "CLAUDE.json"), Metadata: index.SettingEntry{WarehouseName: "CLAUDE.json", Aliases: []string{"claude"}}},
					"GPT.json":     {Name: "GPT.json", WarehouseName: "GPT.json", Path: filepath.Join(projectPath, "Column", "opencode.json", "GPT.json"), Metadata: index.SettingEntry{WarehouseName: "GPT.json"}},
					"Special.json": {Name: "Special.json", WarehouseName: "Special.json", Path: filepath.Join(projectPath, "Column", "opencode.json", "Special.json"), Metadata: index.SettingEntry{WarehouseName: "Special.json", Target: "~/.config/opencode/special/special.json"}},
				},
			},
			"Skills": {
				Name:          "Skills",
				WarehouseName: "Skills",
				Path:          filepath.Join(projectPath, "Column", "Skills"),
				Metadata:      index.ColumnEntry{WarehouseName: "Skills", Aliases: []string{"skills"}},
				Settings: map[string]warehouse.Setting{
					"Skill-A":   {Name: "Skill-A", WarehouseName: "Skill-A", Path: filepath.Join(projectPath, "Column", "Skills", "Skill-A"), Metadata: index.SettingEntry{WarehouseName: "Skill-A", Aliases: []string{"alpha"}, Target: "/tmp/skills/a"}},
					"Skill-Old": {Name: "Skill-Old", WarehouseName: "Skill-Old", Path: filepath.Join(projectPath, "Column", "Skills", "Skill-Old"), Metadata: index.SettingEntry{WarehouseName: "Skill-Old", Target: "/tmp/skills/old"}},
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
