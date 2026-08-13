package mutate

import (
	"fmt"
	"strconv"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/planner"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// TargetPosition is the logical representation of one persisted target-array position.
type TargetPosition struct {
	Dir      string `json:"dir"`
	Name     string `json:"name"`
	DirMode  string `json:"dirMode"`
	NameMode string `json:"nameMode"`
}

// ColumnTargetPositions converts Column defaults into logical target positions.
func ColumnTargetPositions(settingIndex index.SettingIndex) ([]TargetPosition, error) {
	if err := ValidateTargetArrays(settingIndex); err != nil {
		return nil, err
	}
	positions := make([]TargetPosition, settingIndex.TargetNumber)
	for position := range positions {
		positions[position] = TargetPosition{Dir: settingIndex.DefaultTargetDir[position], Name: settingIndex.DefaultTargetName[position], DirMode: "fixed", NameMode: "fixed"}
		if settingIndex.DefaultTargetName[position] == "" {
			positions[position].NameMode = "setting"
		}
	}
	return positions, nil
}

// SettingTargetPositions converts one Setting override into logical positions.
func SettingTargetPositions(settingIndex index.SettingIndex, entry index.SettingEntry) ([]TargetPosition, error) {
	if err := ValidateTargetArrays(settingIndex); err != nil {
		return nil, err
	}
	if err := validateSettingOverrideLengths(settingIndex.TargetNumber, entry); err != nil {
		return nil, err
	}
	positions := make([]TargetPosition, settingIndex.TargetNumber)
	for position := range positions {
		positions[position] = TargetPosition{Dir: entry.TargetDir[position], Name: entry.TargetName[position], DirMode: "explicit", NameMode: "explicit"}
		if entry.TargetDir[position] == "" {
			positions[position].DirMode = "inherit"
		}
		if entry.TargetName[position] == "" {
			positions[position].NameMode = "inherit"
		}
	}
	return positions, nil
}

// ValidateTargetArrays checks every persisted parallel target array before mutation.
func ValidateTargetArrays(settingIndex index.SettingIndex) error {
	if settingIndex.TargetNumber < 0 {
		return fmt.Errorf("targetNumber must be non-negative")
	}
	if len(settingIndex.DefaultTargetDir) != settingIndex.TargetNumber || len(settingIndex.DefaultTargetName) != settingIndex.TargetNumber {
		return fmt.Errorf("default target arrays must match targetNumber")
	}
	for name, entry := range settingIndex.Settings {
		if err := validateSettingOverrideLengths(settingIndex.TargetNumber, entry); err != nil {
			return fmt.Errorf("setting %q: %w", name, err)
		}
	}
	return nil
}

// validateSettingOverrideLengths enforces lockstep Setting override arrays.
func validateSettingOverrideLengths(targetNumber int, entry index.SettingEntry) error {
	if len(entry.TargetDir) != targetNumber || len(entry.TargetName) != targetNumber {
		return fmt.Errorf("target override arrays must match targetNumber")
	}
	return nil
}

// AddColumnTarget appends one logical target and extends every Setting override.
func AddColumnTarget(repo repository.Repository, projectReference string, columnReference string, position TargetPosition, options planner.PlanOptions) (string, int, error) {
	_, project, column, err := loadTargetColumn(repo.RootPath, projectReference, columnReference)
	if err != nil {
		return "", 0, err
	}
	if err := validateColumnPosition(position); err != nil {
		return "", 0, err
	}
	positionIndex := column.SettingIndex.TargetNumber
	updated := column.SettingIndex
	updated.TargetNumber++
	updated.DefaultTargetDir = append(append([]string{}, updated.DefaultTargetDir...), position.Dir)
	updated.DefaultTargetName = append(append([]string{}, updated.DefaultTargetName...), position.Name)
	for name, entry := range updated.Settings {
		entry.TargetDir = append(append([]string{}, entry.TargetDir...), "")
		entry.TargetName = append(append([]string{}, entry.TargetName...), "")
		updated.Settings[name] = entry
	}
	if err := validatePlannedProjectTargets(project, column, updated, options); err != nil {
		return "", 0, err
	}
	if err := repo.WithMutation("column-target-add", []string{repo.SettingIndexPath(project.Name, column.Name)}, func() error {
		return repo.SaveSettingIndex(project.Name, column.Name, updated)
	}); err != nil {
		return "", 0, persistence("column_target_add", fmt.Sprintf("add target to column %q", column.Name), err)
	}
	return column.Name, positionIndex, nil
}

// SetColumnTarget replaces only supplied components at one Column target index.
func SetColumnTarget(repo repository.Repository, projectReference string, columnReference string, positionIndex int, dir *string, clearDir bool, name *string, nameFromSetting bool, options planner.PlanOptions) (string, error) {
	_, project, column, err := loadTargetColumn(repo.RootPath, projectReference, columnReference)
	if err != nil {
		return "", err
	}
	if err := validateTargetIndex(column.SettingIndex.TargetNumber, positionIndex); err != nil {
		return "", err
	}
	if dir != nil && clearDir {
		return "", invalid("conflicting_target_dir", "--dir and --clear-dir are mutually exclusive", nil)
	}
	if name != nil && nameFromSetting {
		return "", invalid("conflicting_target_name", "--name and --name-from-setting are mutually exclusive", nil)
	}
	if dir == nil && !clearDir && name == nil && !nameFromSetting {
		return "", invalid("target_component_required", "set requires a target directory or name component", nil)
	}
	if dir != nil && *dir == "" {
		return "", invalid("invalid_target_dir", "target directory cannot be empty; use --clear-dir", nil)
	}
	if name != nil && *name == "" {
		return "", invalid("invalid_target_name", "target name cannot be empty; use --name-from-setting", nil)
	}
	updated := column.SettingIndex
	updated.DefaultTargetDir = append([]string{}, updated.DefaultTargetDir...)
	updated.DefaultTargetName = append([]string{}, updated.DefaultTargetName...)
	if dir != nil {
		updated.DefaultTargetDir[positionIndex] = *dir
	}
	if clearDir {
		updated.DefaultTargetDir[positionIndex] = ""
	}
	if name != nil {
		updated.DefaultTargetName[positionIndex] = *name
	}
	if nameFromSetting {
		updated.DefaultTargetName[positionIndex] = ""
	}
	if err := validatePlannedProjectTargets(project, column, updated, options); err != nil {
		return "", err
	}
	if err := repo.WithMutation("column-target-set", []string{repo.SettingIndexPath(project.Name, column.Name)}, func() error {
		return repo.SaveSettingIndex(project.Name, column.Name, updated)
	}); err != nil {
		return "", persistence("column_target_set", fmt.Sprintf("set target %d in column %q", positionIndex, column.Name), err)
	}
	return column.Name, nil
}

// DeleteColumnTarget removes one target position from defaults and every override.
func DeleteColumnTarget(repo repository.Repository, projectReference string, columnReference string, positionIndex int, confirmed bool, options planner.PlanOptions) error {
	_, project, column, err := loadTargetColumn(repo.RootPath, projectReference, columnReference)
	if err != nil {
		return err
	}
	if !confirmed {
		return refusal("confirmation_required", "target deletion requires --yes", nil)
	}
	if err := validateTargetIndex(column.SettingIndex.TargetNumber, positionIndex); err != nil {
		return err
	}
	updated := column.SettingIndex
	updated.TargetNumber--
	updated.DefaultTargetDir = removeStringAt(updated.DefaultTargetDir, positionIndex)
	updated.DefaultTargetName = removeStringAt(updated.DefaultTargetName, positionIndex)
	for name, entry := range updated.Settings {
		entry.TargetDir = removeStringAt(entry.TargetDir, positionIndex)
		entry.TargetName = removeStringAt(entry.TargetName, positionIndex)
		updated.Settings[name] = entry
	}
	if err := validatePlannedProjectTargets(project, column, updated, options); err != nil {
		return err
	}
	if err := repo.WithMutation("column-target-delete", []string{repo.SettingIndexPath(project.Name, column.Name)}, func() error {
		return repo.SaveSettingIndex(project.Name, column.Name, updated)
	}); err != nil {
		return persistence("column_target_delete", fmt.Sprintf("delete target %d in column %q", positionIndex, column.Name), err)
	}
	return nil
}

// SetSettingTarget changes independently inherited or explicit Setting components.
func SetSettingTarget(repo repository.Repository, projectReference string, columnReference string, settingReference string, positionIndex int, dir *string, inheritDir bool, name *string, inheritName bool, options planner.PlanOptions) (string, error) {
	_, project, column, setting, err := loadTargetSetting(repo.RootPath, projectReference, columnReference, settingReference)
	if err != nil {
		return "", err
	}
	if err := validateTargetIndex(column.SettingIndex.TargetNumber, positionIndex); err != nil {
		return "", err
	}
	if dir != nil && inheritDir {
		return "", invalid("conflicting_target_dir", "--dir and --inherit-dir are mutually exclusive", nil)
	}
	if name != nil && inheritName {
		return "", invalid("conflicting_target_name", "--name and --inherit-name are mutually exclusive", nil)
	}
	if dir == nil && !inheritDir && name == nil && !inheritName {
		return "", invalid("target_component_required", "set requires a target directory or name component", nil)
	}
	if dir != nil && *dir == "" {
		return "", invalid("invalid_target_dir", "target directory cannot be empty; use --inherit-dir", nil)
	}
	if name != nil && *name == "" {
		return "", invalid("invalid_target_name", "target name cannot be empty; use --inherit-name", nil)
	}
	updated := column.SettingIndex
	entry := updated.Settings[setting.Name]
	entry.TargetDir = append([]string{}, entry.TargetDir...)
	entry.TargetName = append([]string{}, entry.TargetName...)
	if dir != nil {
		entry.TargetDir[positionIndex] = *dir
	}
	if inheritDir {
		entry.TargetDir[positionIndex] = ""
	}
	if name != nil {
		entry.TargetName[positionIndex] = *name
	}
	if inheritName {
		entry.TargetName[positionIndex] = ""
	}
	updated.Settings[setting.Name] = entry
	if err := validatePlannedProjectTargets(project, column, updated, options); err != nil {
		return "", err
	}
	if err := repo.WithMutation("setting-target-set", []string{repo.SettingIndexPath(project.Name, column.Name)}, func() error {
		return repo.SaveSettingIndex(project.Name, column.Name, updated)
	}); err != nil {
		return "", persistence("setting_target_set", fmt.Sprintf("set target %d for setting %q", positionIndex, setting.Name), err)
	}
	return setting.Name, nil
}

// ResetSettingTarget restores both Setting target components to Column inheritance.
func ResetSettingTarget(repo repository.Repository, projectReference string, columnReference string, settingReference string, positionIndex int, options planner.PlanOptions) (string, error) {
	return SetSettingTarget(repo, projectReference, columnReference, settingReference, positionIndex, nil, true, nil, true, options)
}

// loadTargetColumn resolves a present Column with lockstep arrays for target mutation.
func loadTargetColumn(rootPath string, projectReference string, columnReference string) (warehouse.Warehouse, warehouse.Project, warehouse.Column, error) {
	loaded, project, err := loadProject(rootPath, projectReference)
	if err != nil {
		return warehouse.Warehouse{}, warehouse.Project{}, warehouse.Column{}, err
	}
	if project.Missing {
		return warehouse.Warehouse{}, warehouse.Project{}, warehouse.Column{}, missing("project_missing", fmt.Sprintf("project %q is missing", project.Name), nil)
	}
	column, err := project.ResolveColumn(columnReference)
	if err != nil {
		return warehouse.Warehouse{}, warehouse.Project{}, warehouse.Column{}, missing("column_not_found", err.Error(), err)
	}
	if column.Missing {
		return warehouse.Warehouse{}, warehouse.Project{}, warehouse.Column{}, missing("column_missing", fmt.Sprintf("column %q is missing", column.Name), nil)
	}
	if err := ValidateTargetArrays(column.SettingIndex); err != nil {
		return warehouse.Warehouse{}, warehouse.Project{}, warehouse.Column{}, invalid("target_arrays", err.Error(), err)
	}
	return loaded, project, column, nil
}

// loadTargetSetting resolves one Setting in a target-mutable Column.
func loadTargetSetting(rootPath string, projectReference string, columnReference string, settingReference string) (warehouse.Warehouse, warehouse.Project, warehouse.Column, warehouse.Setting, error) {
	loaded, project, column, err := loadTargetColumn(rootPath, projectReference, columnReference)
	if err != nil {
		return warehouse.Warehouse{}, warehouse.Project{}, warehouse.Column{}, warehouse.Setting{}, err
	}
	setting, err := column.ResolveSetting(settingReference)
	if err != nil {
		return warehouse.Warehouse{}, warehouse.Project{}, warehouse.Column{}, warehouse.Setting{}, missing("setting_not_found", err.Error(), err)
	}
	return loaded, project, column, setting, nil
}

// validatePlannedProjectTargets reuses planner validation against tentative Column target data.
func validatePlannedProjectTargets(project warehouse.Project, column warehouse.Column, settingIndex index.SettingIndex, options planner.PlanOptions) error {
	column.SettingIndex = settingIndex
	for name, setting := range column.Settings {
		setting.Metadata = settingIndex.Settings[name]
		column.Settings[name] = setting
	}
	project.Columns[column.Name] = column
	if err := planner.ValidateProjectTargets(project, options); err != nil {
		return invalid("target_plan", err.Error(), err)
	}
	return nil
}

// removeStringAt removes one known-valid zero-based array position.
func removeStringAt(values []string, position int) []string {
	result := append([]string{}, values...)
	return append(result[:position], result[position+1:]...)
}

// validateColumnPosition validates components used by one appended Column default.
func validateColumnPosition(position TargetPosition) error {
	if position.Dir == "" {
		return invalid("invalid_target_dir", "target directory cannot be empty", nil)
	}
	if position.NameMode == "setting" && position.Name != "" {
		return invalid("conflicting_target_name", "setting-derived name cannot include a fixed name", nil)
	}
	if position.NameMode != "setting" && position.Name == "" {
		return invalid("invalid_target_name", "target name cannot be empty", nil)
	}
	return nil
}

// validateTargetIndex enforces zero-based target indexing.
func validateTargetIndex(targetNumber int, position int) error {
	if position < 0 || position >= targetNumber {
		return invalid("invalid_target_index", fmt.Sprintf("target index %d is outside the valid range", position), nil)
	}
	return nil
}

// ParseTargetIndex converts one CLI index into a strict zero-based integer.
func ParseTargetIndex(value string) (int, error) {
	position, err := strconv.Atoi(value)
	if err != nil {
		return 0, invalid("invalid_target_index", fmt.Sprintf("target index %q must be a zero-based integer", value), err)
	}
	return position, nil
}

// refusal constructs a destructive-operation refusal mutation error.
func refusal(code string, message string, cause error) *Error {
	return &Error{Kind: RefusalError, Code: code, Message: message, Cause: cause}
}
