package planner

import (
	"fmt"
	"strings"

	"github.com/xenon/ConfigFacilitator/internal/linker"
	"github.com/xenon/ConfigFacilitator/internal/pathvars"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// PlanOptions controls environment-sensitive target resolution.
type PlanOptions struct {
	HomeDir string
	Env     map[string]string
	OS      string
}

// PlanColumnMappings builds the mapping set for one explicit column selection.
func PlanColumnMappings(project warehouse.Project, columnName string, settingNames []string, options PlanOptions) ([]linker.Mapping, error) {
	column, ok := project.Columns[columnName]
	if !ok {
		return nil, fmt.Errorf("column %q does not exist in project %q", columnName, project.Name)
	}
	mappings := make([]linker.Mapping, 0, len(settingNames))
	for _, settingName := range settingNames {
		setting, ok := column.Settings[settingName]
		if !ok {
			return nil, fmt.Errorf("setting %q does not exist in column %q", settingName, columnName)
		}
		mapping, err := resolveSettingMapping(column, setting, options)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mapping)
	}
	return mappings, nil
}

// PlanModeMappings builds the mapping set for a mode selection from current managed state.
func PlanModeMappings(project warehouse.Project, modeName string, current []linker.Mapping, options PlanOptions) ([]linker.Mapping, error) {
	mode, ok := project.Modes[modeName]
	if !ok {
		return nil, fmt.Errorf("mode %q does not exist in project %q", modeName, project.Name)
	}
	byColumn := groupCurrentMappingsByColumn(project, current)
	result := []linker.Mapping{}
	seenTargets := map[string]struct{}{}
	for columnName, column := range project.Columns {
		selection, declared := mode.Metadata.Columns[columnName]
		if !declared {
			continue
		}
		if selection.Strategy == "incremental" {
			for _, mapping := range byColumn[columnName] {
				if _, exists := seenTargets[mapping.Target]; !exists {
					result = append(result, mapping)
					seenTargets[mapping.Target] = struct{}{}
				}
			}
		}
		for _, settingName := range selection.Settings {
			setting, ok := column.Settings[settingName]
			if !ok {
				return nil, fmt.Errorf("setting %q does not exist in column %q", settingName, columnName)
			}
			mapping, err := resolveSettingMapping(column, setting, options)
			if err != nil {
				return nil, err
			}
			result = upsertMapping(result, mapping)
			seenTargets[mapping.Target] = struct{}{}
		}
	}
	return result, nil
}

// ParseSettingList parses one or more setting names from CLI input.
func ParseSettingList(input string) []string {
	parts := strings.Split(input, ",")
	settings := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			settings = append(settings, trimmed)
		}
	}
	return settings
}

func resolveSettingMapping(column warehouse.Column, setting warehouse.Setting, options PlanOptions) (linker.Mapping, error) {
	target := setting.Metadata.Target
	if target == "" {
		target = column.SettingIndex.DefaultTarget
	}
	if target == "" {
		return linker.Mapping{}, fmt.Errorf("setting %q in column %q has no target", setting.Name, column.Name)
	}
	resolvedTarget, err := pathvars.Expand(target, pathvars.Options{HomeDir: options.HomeDir, Env: options.Env, OS: options.OS})
	if err != nil {
		return linker.Mapping{}, err
	}
	return linker.Mapping{Source: setting.Path, Target: resolvedTarget}, nil
}

func groupCurrentMappingsByColumn(project warehouse.Project, current []linker.Mapping) map[string][]linker.Mapping {
	grouped := map[string][]linker.Mapping{}
	for _, mapping := range current {
		for columnName, column := range project.Columns {
			for _, setting := range column.Settings {
				if setting.Path == mapping.Source {
					grouped[columnName] = append(grouped[columnName], mapping)
				}
			}
		}
	}
	return grouped
}

func upsertMapping(current []linker.Mapping, next linker.Mapping) []linker.Mapping {
	for i, mapping := range current {
		if mapping.Target == next.Target {
			current[i] = next
			return current
		}
	}
	return append(current, next)
}
