package planner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/linker"
	"github.com/xenon/ConfigFacilitator/internal/pathvars"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

const (
	modeStrategyCover     = "cover"
	modeStrategyIncrement = "increment"
	modeStrategyNone      = "none"
	modeStrategyFull      = "full"
)

// PlanOptions controls environment-sensitive target resolution.
type PlanOptions struct {
	HomeDir string
	Env     map[string]string
	OS      string
}

// PlanColumnMappings builds the mapping set for one explicit column selection.
func PlanColumnMappings(project warehouse.Project, columnReference string, settingReferences []string, options PlanOptions) ([]linker.Mapping, error) {
	column, err := project.ResolveColumn(columnReference)
	if err != nil {
		return nil, err
	}
	mappings := make([]linker.Mapping, 0, len(settingReferences))
	for _, settingReference := range settingReferences {
		setting, err := column.ResolveSetting(settingReference)
		if err != nil {
			return nil, err
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
func PlanModeMappings(project warehouse.Project, modeReference string, current []linker.Mapping, options PlanOptions) ([]linker.Mapping, error) {
	mode, err := project.ResolveMode(modeReference)
	if err != nil {
		return nil, err
	}
	byColumn := groupCurrentMappingsByColumn(project, current)
	result := []linker.Mapping{}
	seenTargets := map[string]struct{}{}
	for columnReference, selection := range mode.Metadata.Columns {
		column, err := project.ResolveColumn(columnReference)
		if err != nil {
			return nil, err
		}
		columnMappings, err := planModeColumnMappings(column, selection, byColumn[column.Name], options)
		if err != nil {
			return nil, err
		}
		for _, mapping := range columnMappings {
			if _, exists := seenTargets[mapping.Target]; exists {
				result = upsertMapping(result, mapping)
				continue
			}
			result = append(result, mapping)
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

func planModeColumnMappings(column warehouse.Column, selection index.ModeColumnSelection, current []linker.Mapping, options PlanOptions) ([]linker.Mapping, error) {
	switch selection.Strategy {
	case modeStrategyCover:
		return resolveSelectedMappings(column, selection.Settings, options)
	case modeStrategyIncrement:
		selected, err := resolveSelectedMappings(column, selection.Settings, options)
		if err != nil {
			return nil, err
		}
		result := append([]linker.Mapping{}, current...)
		for _, mapping := range selected {
			result = upsertMapping(result, mapping)
		}
		return result, nil
	case modeStrategyNone:
		return []linker.Mapping{}, nil
	case modeStrategyFull:
		return resolveAllColumnMappings(column, options)
	default:
		return nil, fmt.Errorf("column %q uses unsupported mode strategy %q", column.Name, selection.Strategy)
	}
}

func resolveSelectedMappings(column warehouse.Column, settingReferences []string, options PlanOptions) ([]linker.Mapping, error) {
	if len(settingReferences) == 0 {
		return nil, fmt.Errorf("column %q requires at least one setting", column.Name)
	}
	mappings := make([]linker.Mapping, 0, len(settingReferences))
	for _, settingReference := range settingReferences {
		setting, err := column.ResolveSetting(settingReference)
		if err != nil {
			return nil, err
		}
		mapping, err := resolveSettingMapping(column, setting, options)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mapping)
	}
	return mappings, nil
}

func resolveAllColumnMappings(column warehouse.Column, options PlanOptions) ([]linker.Mapping, error) {
	settingNames := make([]string, 0, len(column.Settings))
	for name := range column.Settings {
		settingNames = append(settingNames, name)
	}
	sort.Strings(settingNames)
	mappings := make([]linker.Mapping, 0, len(settingNames))
	for _, settingName := range settingNames {
		mapping, err := resolveSettingMapping(column, column.Settings[settingName], options)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mapping)
	}
	return mappings, nil
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
