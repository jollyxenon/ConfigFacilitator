package planner

import (
	"fmt"
	"path"
	"path/filepath"
	"runtime"
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
	mappings := []linker.Mapping{}
	for _, settingReference := range settingReferences {
		setting, err := column.ResolveSetting(settingReference)
		if err != nil {
			return nil, err
		}
		resolvedMappings, err := resolveSettingMappings(column, setting, options)
		if err != nil {
			return nil, err
		}
		mappings, err = appendUniqueMappings(mappings, resolvedMappings)
		if err != nil {
			return nil, err
		}
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
	for columnReference, selection := range mode.Metadata.Columns {
		column, err := project.ResolveColumn(columnReference)
		if err != nil {
			return nil, err
		}
		columnMappings, err := planModeColumnMappings(column, selection, byColumn[column.Name], options)
		if err != nil {
			return nil, err
		}
		var appendErr error
		result, appendErr = appendUniqueMappings(result, columnMappings)
		if appendErr != nil {
			return nil, appendErr
		}
	}
	return result, nil
}

// PlanUpdateMappings refreshes every currently active mapping from current project metadata.
func PlanUpdateMappings(project warehouse.Project, current []linker.Mapping, options PlanOptions) ([]linker.Mapping, error) {
	result := []linker.Mapping{}
	processedSources := map[string]struct{}{}
	for _, mapping := range current {
		if _, processed := processedSources[mapping.Source]; processed {
			continue
		}
		match, err := matchCurrentMapping(project, mapping)
		if err != nil {
			return nil, err
		}
		refreshed, err := resolveSettingMappings(match.column, match.setting, options)
		if err != nil {
			return nil, err
		}
		result, err = appendUniqueMappings(result, refreshed)
		if err != nil {
			return nil, err
		}
		processedSources[mapping.Source] = struct{}{}
	}
	return result, nil
}

// PlanColumnUpdateMappings refreshes one active column and preserves mappings from other columns.
func PlanColumnUpdateMappings(project warehouse.Project, columnReference string, current []linker.Mapping, options PlanOptions) ([]linker.Mapping, error) {
	selectedColumn, err := project.ResolveColumn(columnReference)
	if err != nil {
		return nil, err
	}
	result := make([]linker.Mapping, 0, len(current))
	selectedCount := 0
	processedSelectedSources := map[string]struct{}{}
	for _, mapping := range current {
		match, err := matchCurrentMapping(project, mapping)
		if err != nil {
			return nil, err
		}
		if match.column.Name != selectedColumn.Name {
			result = append(result, mapping)
			continue
		}
		if _, processed := processedSelectedSources[mapping.Source]; processed {
			continue
		}
		refreshed, err := resolveSettingMappings(match.column, match.setting, options)
		if err != nil {
			return nil, err
		}
		result, err = appendUniqueMappings(result, refreshed)
		if err != nil {
			return nil, err
		}
		selectedCount++
		processedSelectedSources[mapping.Source] = struct{}{}
	}
	if selectedCount == 0 {
		return nil, fmt.Errorf("column %q has no active mappings to update", selectedColumn.Name)
	}
	return result, nil
}

// PlanIntentUpdateMappings refreshes the whole project from a persisted apply intent.
func PlanIntentUpdateMappings(project warehouse.Project, intent linker.ApplyIntent, current []linker.Mapping, options PlanOptions) ([]linker.Mapping, error) {
	switch intent.Kind {
	case "mode":
		if intent.Mode == "" {
			return nil, fmt.Errorf("mode update intent requires a mode")
		}
		return PlanModeMappings(project, intent.Mode, current, options)
	case "column":
		if intent.Column == "" {
			return nil, fmt.Errorf("column update intent requires a column")
		}
		return PlanColumnMappings(project, intent.Column, intent.Settings, options)
	default:
		return nil, fmt.Errorf("unsupported update intent kind %q", intent.Kind)
	}
}

// PlanIntentColumnUpdateMappings refreshes one column from persisted intent and preserves other mappings.
func PlanIntentColumnUpdateMappings(project warehouse.Project, intent linker.ApplyIntent, columnReference string, current []linker.Mapping, options PlanOptions) ([]linker.Mapping, error) {
	selectedColumn, err := project.ResolveColumn(columnReference)
	if err != nil {
		return nil, err
	}
	switch intent.Kind {
	case "mode":
		return planModeIntentColumnUpdateMappings(project, intent, selectedColumn, current, options)
	case "column":
		if intent.Column == "" {
			return nil, fmt.Errorf("column update intent requires a column")
		}
		intentColumn, err := project.ResolveColumn(intent.Column)
		if err != nil {
			return nil, err
		}
		if intentColumn.Name != selectedColumn.Name {
			return PlanColumnUpdateMappings(project, selectedColumn.Name, current, options)
		}
		return PlanColumnMappings(project, intentColumn.Name, intent.Settings, options)
	default:
		return nil, fmt.Errorf("unsupported update intent kind %q", intent.Kind)
	}
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

func resolveSettingMappings(column warehouse.Column, setting warehouse.Setting, options PlanOptions) ([]linker.Mapping, error) {
	dirs, names, err := effectiveTargetParts(column, setting)
	if err != nil {
		return nil, err
	}
	mappings := make([]linker.Mapping, 0, len(dirs))
	for index := range dirs {
		resolvedDir, err := pathvars.Expand(dirs[index], pathvars.Options{HomeDir: options.HomeDir, Env: options.Env, OS: options.OS})
		if err != nil {
			return nil, err
		}
		if resolvedDir == "" {
			return nil, fmt.Errorf("setting %q in column %q has an empty target directory", setting.Name, column.Name)
		}
		resolvedName, err := pathvars.Expand(names[index], pathvars.Options{HomeDir: options.HomeDir, Env: options.Env, OS: options.OS})
		if err != nil {
			return nil, err
		}
		if err := validateTargetName(setting, column, resolvedName); err != nil {
			return nil, err
		}
		mappings = append(mappings, linker.Mapping{Source: setting.Path, Target: cleanJoinedPathForOS(options.OS, resolvedDir, resolvedName)})
	}
	return appendUniqueMappings(nil, mappings)
}

// cleanJoinedPathForOS keeps planned target syntax aligned with PlanOptions.OS
// rather than the host OS running the test suite.
func cleanJoinedPathForOS(operatingSystem string, dir string, name string) string {
	if operatingSystem == "" {
		operatingSystem = runtime.GOOS
	}
	if operatingSystem == "windows" {
		return filepath.Clean(filepath.Join(dir, name))
	}
	return path.Clean(path.Join(dir, name))
}

func effectiveTargetParts(column warehouse.Column, setting warehouse.Setting) ([]string, []string, error) {
	defaultDirs := column.SettingIndex.DefaultTargetDir
	defaultNames := column.SettingIndex.DefaultTargetName
	if len(defaultDirs) != len(defaultNames) {
		return nil, nil, fmt.Errorf("column %q defaultTargetDir and defaultTargetName lengths differ", column.Name)
	}
	targetDirs := setting.Metadata.TargetDir
	targetNames := setting.Metadata.TargetName
	if len(targetDirs) == 0 && len(targetNames) == 0 {
		targetDirs = defaultPlaceholders(defaultDirs)
		targetNames = defaultPlaceholders(defaultNames)
	}
	if len(targetDirs) == 0 && len(targetNames) > 0 {
		targetDirs = repeatedString("", len(targetNames))
	}
	if len(targetNames) == 0 && len(targetDirs) > 0 {
		targetNames = repeatedString("", len(targetDirs))
	}
	if len(targetDirs) != len(targetNames) {
		return nil, nil, fmt.Errorf("setting %q in column %q targetDir and targetName lengths differ", setting.Name, column.Name)
	}
	if len(targetDirs) == 0 {
		return nil, nil, fmt.Errorf("setting %q in column %q has no target", setting.Name, column.Name)
	}

	dirs := make([]string, len(targetDirs))
	names := make([]string, len(targetNames))
	for index := range targetDirs {
		dir := targetDirs[index]
		if dir == "" && index < len(defaultDirs) {
			dir = defaultDirs[index]
		}
		name := targetNames[index]
		if name == "" && index < len(defaultNames) {
			name = defaultNames[index]
		}
		if name == "" {
			name = setting.WarehouseName
		}
		dirs[index] = dir
		names[index] = name
	}
	return dirs, names, nil
}

func defaultPlaceholders(defaultValues []string) []string {
	if len(defaultValues) == 0 {
		return []string{""}
	}
	return repeatedString("", len(defaultValues))
}

func repeatedString(value string, count int) []string {
	values := make([]string, count)
	for index := range values {
		values[index] = value
	}
	return values
}

func validateTargetName(setting warehouse.Setting, column warehouse.Column, name string) error {
	if name == "" {
		return fmt.Errorf("setting %q in column %q has an empty target name", setting.Name, column.Name)
	}
	if filepath.IsAbs(name) || name == "." || name == ".." || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("setting %q in column %q has invalid target name %q", setting.Name, column.Name, name)
	}
	return nil
}

type currentMappingMatch struct {
	column  warehouse.Column
	setting warehouse.Setting
}

// matchCurrentMapping finds the project setting represented by one persisted mapping source.
func matchCurrentMapping(project warehouse.Project, mapping linker.Mapping) (currentMappingMatch, error) {
	for _, column := range project.Columns {
		for _, setting := range column.Settings {
			if setting.Path == mapping.Source {
				return currentMappingMatch{column: column, setting: setting}, nil
			}
		}
	}
	return currentMappingMatch{}, fmt.Errorf("current mapping source %q no longer matches project %q metadata", mapping.Source, project.Name)
}

func planModeIntentColumnUpdateMappings(project warehouse.Project, intent linker.ApplyIntent, selectedColumn warehouse.Column, current []linker.Mapping, options PlanOptions) ([]linker.Mapping, error) {
	if intent.Mode == "" {
		return nil, fmt.Errorf("mode update intent requires a mode")
	}
	mode, err := project.ResolveMode(intent.Mode)
	if err != nil {
		return nil, err
	}
	selection, ok, err := resolveModeColumnSelection(project, mode, selectedColumn.Name)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("mode %q has no active selection for column %q", mode.Name, selectedColumn.Name)
	}

	byColumn := groupCurrentMappingsByColumn(project, current)
	selectedMappings, err := planModeColumnMappings(selectedColumn, selection, byColumn[selectedColumn.Name], options)
	if err != nil {
		return nil, err
	}
	if len(selectedMappings) == 0 && len(byColumn[selectedColumn.Name]) == 0 {
		return nil, fmt.Errorf("column %q has no active mappings to update", selectedColumn.Name)
	}

	result := make([]linker.Mapping, 0, len(current)+len(selectedMappings))
	for _, mapping := range current {
		match, err := matchCurrentMapping(project, mapping)
		if err != nil {
			return nil, err
		}
		if match.column.Name != selectedColumn.Name {
			result = append(result, mapping)
		}
	}
	for _, mapping := range selectedMappings {
		result = upsertMapping(result, mapping)
	}
	if err := validateUniqueMappingTargets(result); err != nil {
		return nil, err
	}
	return result, nil
}

func resolveModeColumnSelection(project warehouse.Project, mode warehouse.Mode, selectedColumnName string) (index.ModeColumnSelection, bool, error) {
	for columnReference, selection := range mode.Metadata.Columns {
		column, err := project.ResolveColumn(columnReference)
		if err != nil {
			return index.ModeColumnSelection{}, false, err
		}
		if column.Name == selectedColumnName {
			return selection, true, nil
		}
	}
	return index.ModeColumnSelection{}, false, nil
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
	mappings := []linker.Mapping{}
	for _, settingReference := range settingReferences {
		setting, err := column.ResolveSetting(settingReference)
		if err != nil {
			return nil, err
		}
		resolvedMappings, err := resolveSettingMappings(column, setting, options)
		if err != nil {
			return nil, err
		}
		mappings, err = appendUniqueMappings(mappings, resolvedMappings)
		if err != nil {
			return nil, err
		}
	}
	return mappings, nil
}

func resolveAllColumnMappings(column warehouse.Column, options PlanOptions) ([]linker.Mapping, error) {
	settingNames := make([]string, 0, len(column.Settings))
	for name := range column.Settings {
		settingNames = append(settingNames, name)
	}
	sort.Strings(settingNames)
	mappings := []linker.Mapping{}
	for _, settingName := range settingNames {
		resolvedMappings, err := resolveSettingMappings(column, column.Settings[settingName], options)
		if err != nil {
			return nil, err
		}
		mappings, err = appendUniqueMappings(mappings, resolvedMappings)
		if err != nil {
			return nil, err
		}
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

func appendUniqueMappings(current []linker.Mapping, next []linker.Mapping) ([]linker.Mapping, error) {
	result := append([]linker.Mapping{}, current...)
	seen := map[string]struct{}{}
	for _, mapping := range result {
		seen[mapping.Target] = struct{}{}
	}
	for _, mapping := range next {
		if _, exists := seen[mapping.Target]; exists {
			return nil, fmt.Errorf("duplicate target %s", mapping.Target)
		}
		result = append(result, mapping)
		seen[mapping.Target] = struct{}{}
	}
	return result, nil
}

func validateUniqueMappingTargets(mappings []linker.Mapping) error {
	_, err := appendUniqueMappings(nil, mappings)
	return err
}
