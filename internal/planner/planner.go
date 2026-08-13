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

// MissingResourceError identifies a filesystem-backed model resource unavailable to planning.
type MissingResourceError struct {
	Kind    string
	Project string
	Column  string
	Name    string
}

// Error returns a stable, contextual missing-resource diagnostic.
func (err MissingResourceError) Error() string {
	switch err.Kind {
	case "project":
		return fmt.Sprintf("project %q is missing", err.Project)
	case "column":
		return fmt.Sprintf("column %q in project %q is missing", err.Column, err.Project)
	default:
		return fmt.Sprintf("setting %q in column %q is missing", err.Name, err.Column)
	}
}

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
	if err := requirePresentProject(project); err != nil {
		return nil, err
	}
	column, err := project.ResolveColumn(columnReference)
	if err != nil {
		return nil, err
	}
	if err := requirePresentColumn(project, column); err != nil {
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

// ValidateColumnTargets verifies defaults plus every Setting's effective target plan.
func ValidateColumnTargets(column warehouse.Column, options PlanOptions) error {
	if column.SettingIndex.TargetNumber != len(column.SettingIndex.DefaultTargetDir) || column.SettingIndex.TargetNumber != len(column.SettingIndex.DefaultTargetName) {
		return fmt.Errorf("column %q target arrays do not match targetNumber", column.Name)
	}
	probe := warehouse.Setting{Name: "target", WarehouseName: "target", Path: filepath.Join(column.Path, "target"), Metadata: index.SettingEntry{WarehouseName: "target", TargetDir: repeatedString("", column.SettingIndex.TargetNumber), TargetName: repeatedString("", column.SettingIndex.TargetNumber)}}
	if column.SettingIndex.TargetNumber > 0 {
		probeMappings, err := resolveSettingMappings(column, probe, options)
		if err != nil {
			return err
		}
		if err := validateUniqueMappingTargets(probeMappings); err != nil {
			return err
		}
	}
	for name, setting := range column.Settings {
		if len(setting.Metadata.TargetDir) != column.SettingIndex.TargetNumber || len(setting.Metadata.TargetName) != column.SettingIndex.TargetNumber {
			return fmt.Errorf("setting %q in column %q target arrays do not match targetNumber", name, column.Name)
		}
		if column.SettingIndex.TargetNumber == 0 {
			continue
		}
		mappings, err := resolveSettingMappings(column, setting, options)
		if err != nil {
			return err
		}
		if err := validateUniqueMappingTargets(mappings); err != nil {
			return err
		}
	}
	return nil
}

// ValidateProjectTargets verifies expanded targets stay unique across all Settings and Columns.
func ValidateProjectTargets(project warehouse.Project, options PlanOptions) error {
	mappings := []linker.Mapping{}
	columnNames := make([]string, 0, len(project.Columns))
	for name := range project.Columns {
		columnNames = append(columnNames, name)
	}
	sort.Strings(columnNames)
	for _, columnName := range columnNames {
		column := project.Columns[columnName]
		if err := ValidateColumnTargets(column, options); err != nil {
			return err
		}
		settingNames := make([]string, 0, len(column.Settings))
		for name := range column.Settings {
			settingNames = append(settingNames, name)
		}
		sort.Strings(settingNames)
		for _, settingName := range settingNames {
			if column.SettingIndex.TargetNumber == 0 {
				continue
			}
			resolved, err := resolveSettingMappings(column, column.Settings[settingName], options)
			if err != nil {
				return err
			}
			mappings, err = appendUniqueMappings(mappings, resolved)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// PlanModeColumns plans the mapping set for one named Mode selection.
func PlanModeColumns(project warehouse.Project, modeReference string, current []linker.Mapping, options PlanOptions) ([]linker.Mapping, error) {
	if err := requirePresentProject(project); err != nil {
		return nil, err
	}
	mode, err := project.ResolveMode(modeReference)
	if err != nil {
		return nil, err
	}
	if mode.Missing {
		return nil, MissingResourceError{Kind: "mode", Project: project.Name, Name: mode.Name}
	}
	return PlanColumns(project, mode.Metadata.Columns, current, options)
}

// PlanColumns plans the mapping set for one columns selection map.
func PlanColumns(project warehouse.Project, columns map[string]index.ModeColumnSelection, current []linker.Mapping, options PlanOptions) ([]linker.Mapping, error) {
	if err := requirePresentProject(project); err != nil {
		return nil, err
	}
	byColumn := groupCurrentMappingsByColumn(project, current)
	result := []linker.Mapping{}
	names := make([]string, 0, len(columns))
	for reference := range columns {
		names = append(names, reference)
	}
	sort.Strings(names)
	for _, reference := range names {
		selection := columns[reference]
		column, err := project.ResolveColumn(reference)
		if err != nil {
			return nil, err
		}
		if err := requirePresentColumn(project, column); err != nil {
			return nil, err
		}
		columnMappings, err := planColumnSelection(column, selection, byColumn[column.Name], options)
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
	if err := requirePresentColumn(warehouse.Project{Name: "", Missing: false}, column); err != nil {
		return nil, err
	}
	if err := requirePresentSetting(column, setting); err != nil {
		return nil, err
	}
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
	if project.Missing {
		return currentMappingMatch{}, MissingResourceError{Kind: "project", Project: project.Name}
	}
	for _, column := range project.Columns {
		for _, setting := range column.Settings {
			if setting.Path == mapping.Source {
				return currentMappingMatch{column: column, setting: setting}, nil
			}
		}
	}
	return currentMappingMatch{}, fmt.Errorf("current mapping source %q no longer matches project %q metadata", mapping.Source, project.Name)
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

func planColumnSelection(column warehouse.Column, selection index.ModeColumnSelection, current []linker.Mapping, options PlanOptions) ([]linker.Mapping, error) {
	if err := requirePresentColumn(warehouse.Project{}, column); err != nil {
		return nil, err
	}
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

// requirePresentProject rejects plans that need an absent Project source tree.
func requirePresentProject(project warehouse.Project) error {
	if project.Missing {
		return MissingResourceError{Kind: "project", Project: project.Name}
	}
	return nil
}

// requirePresentColumn rejects plans that need an absent Column source tree.
func requirePresentColumn(project warehouse.Project, column warehouse.Column) error {
	if column.Missing {
		return MissingResourceError{Kind: "column", Project: project.Name, Column: column.Name}
	}
	return nil
}

// requirePresentSetting rejects plans that need unreadable Setting source content.
func requirePresentSetting(column warehouse.Column, setting warehouse.Setting) error {
	if setting.Missing {
		return MissingResourceError{Kind: "setting", Column: column.Name, Name: setting.Name}
	}
	return nil
}
