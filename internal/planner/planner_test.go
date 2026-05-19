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
	mappings, err := PlanColumnMappings(project, "opencode.json", []string{"CLAUDE.json", "Special.json"}, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"})
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

func TestPlanModeMappingsHandlesFullAndIncrementalColumns(t *testing.T) {
	project := sampleProject()
	current := []linker.Mapping{{Source: filepath.Join(project.Columns["Skills"].Path, "Skill-Old"), Target: "/tmp/skills/old"}, {Source: filepath.Join(project.Columns["opencode.json"].Path, "GPT.json"), Target: "/home/test/.config/opencode/opencode.json"}}
	mappings, err := PlanModeMappings(project, "Max", current, PlanOptions{HomeDir: "/home/test", Env: map[string]string{}, OS: "linux"})
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

func sampleProject() warehouse.Project {
	projectPath := "/warehouse/OpenCode"
	return warehouse.Project{
		Name: "OpenCode",
		Columns: map[string]warehouse.Column{
			"opencode.json": {
				Name: "opencode.json",
				Path: filepath.Join(projectPath, "Column", "opencode.json"),
				SettingIndex: index.SettingIndex{DefaultTarget: "~/.config/opencode/opencode.json"},
				Settings: map[string]warehouse.Setting{
					"CLAUDE.json": {Name: "CLAUDE.json", Path: filepath.Join(projectPath, "Column", "opencode.json", "CLAUDE.json")},
					"GPT.json": {Name: "GPT.json", Path: filepath.Join(projectPath, "Column", "opencode.json", "GPT.json")},
					"Special.json": {Name: "Special.json", Path: filepath.Join(projectPath, "Column", "opencode.json", "Special.json"), Metadata: index.SettingEntry{Target: "~/.config/opencode/special/special.json"}},
				},
			},
			"Skills": {
				Name: "Skills",
				Path: filepath.Join(projectPath, "Column", "Skills"),
				Settings: map[string]warehouse.Setting{
					"Skill-A":   {Name: "Skill-A", Path: filepath.Join(projectPath, "Column", "Skills", "Skill-A"), Metadata: index.SettingEntry{Target: "/tmp/skills/a"}},
					"Skill-Old": {Name: "Skill-Old", Path: filepath.Join(projectPath, "Column", "Skills", "Skill-Old"), Metadata: index.SettingEntry{Target: "/tmp/skills/old"}},
				},
			},
		},
		Modes: map[string]warehouse.Mode{
			"Max": {
				Name: "Max",
				Metadata: index.ModeEntry{Columns: map[string]index.ModeColumnSelection{
					"opencode.json": {Settings: []string{"CLAUDE.json"}, Strategy: "full"},
					"Skills":        {Settings: []string{"Skill-A"}, Strategy: "incremental"},
				}},
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
