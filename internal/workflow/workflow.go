// Package workflow coordinates multi-artifact operations that mutate the
// persisted Current state together with resource indexes and managed links.
// CLI commands and the Web API share this layer so their behavior stays
// identical: following Modes synchronize Current atomically, detached Current
// selections replan from their own columns, and every change writes through
// the repository transaction machinery.
package workflow

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/linker"
	"github.com/xenon/ConfigFacilitator/internal/mutate"
	"github.com/xenon/ConfigFacilitator/internal/planner"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// PlanOptions mirrors planner.PlanOptions for callers that only need the
// workflow surface.
type PlanOptions = planner.PlanOptions

// Error classes used to classify failures at the CLI and HTTP boundaries.
const (
	ErrNotFound    = "not_found"
	ErrConflict    = "conflict"
	ErrInvalid     = "invalid"
	ErrRefused     = "refused"
	ErrPersistence = "persistence"
)

// Error is one typed workflow failure with a stable class and code.
type Error struct {
	Class   string
	Code    string
	Message string
	Details any
}

func (err *Error) Error() string { return err.Message }

func invalid(code, message string) error {
	return &Error{Class: ErrInvalid, Code: code, Message: message}
}

func conflict(code, message string) error {
	return &Error{Class: ErrConflict, Code: code, Message: message}
}

func notFound(code, message string) error {
	return &Error{Class: ErrNotFound, Code: code, Message: message}
}

func persistence(code, message string, cause error) error {
	return &Error{Class: ErrPersistence, Code: code, Message: message}
}

func refused(code, message string) error {
	return &Error{Class: ErrRefused, Code: code, Message: message}
}

// Classify extracts the workflow error class and code from any error.
func Classify(err error) (class string, code string) {
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Class, typed.Code
	}
	return ErrPersistence, "workflow_failed"
}

// ColumnsOf converts persisted repository selections to planner selections.
func ColumnsOf(columns map[string]repository.ColumnSelection) map[string]index.ModeColumnSelection {
	result := make(map[string]index.ModeColumnSelection, len(columns))
	for name, selection := range columns {
		result[name] = index.ModeColumnSelection{Strategy: selection.Strategy, Settings: append([]string{}, selection.Settings...)}
	}
	return result
}

// PersistColumns converts planner selections to persisted repository selections.
func PersistColumns(columns map[string]index.ModeColumnSelection) map[string]repository.ColumnSelection {
	result := make(map[string]repository.ColumnSelection, len(columns))
	for name, selection := range columns {
		result[name] = repository.ColumnSelection{Strategy: selection.Strategy, Settings: append([]string{}, selection.Settings...)}
	}
	return result
}

// sameColumns reports whether two persisted column maps are semantically equal.
func sameColumns(left, right map[string]repository.ColumnSelection) bool {
	if len(left) != len(right) {
		return false
	}
	for name, selection := range left {
		other, ok := right[name]
		if !ok || other.Strategy != selection.Strategy || !sameStrings(other.Settings, selection.Settings) {
			return false
		}
	}
	return true
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// loadProject resolves one Project and rejects absent source trees.
func loadProject(repo repository.Repository, projectReference string) (warehouse.Warehouse, warehouse.Project, error) {
	loaded, err := warehouse.LoadWarehouse(repo.RootPath)
	if err != nil {
		return warehouse.Warehouse{}, warehouse.Project{}, err
	}
	project, err := loaded.ResolveProject(projectReference)
	if err != nil {
		return warehouse.Warehouse{}, warehouse.Project{}, notFound("project_not_found", err.Error())
	}
	return loaded, project, nil
}

// ValidateRelation checks persisted relation shape and origin availability.
func ValidateRelation(project warehouse.Project, relation *repository.CurrentRelation) error {
	if relation == nil {
		return nil
	}
	if relation.Kind != "following" && relation.Kind != "detached" {
		return invalid("relation_kind", fmt.Sprintf("unsupported relation kind %q", relation.Kind))
	}
	if relation.OriginMode == "" {
		return invalid("relation_origin", "relation originMode cannot be empty")
	}
	if _, err := project.ResolveMode(relation.OriginMode); err != nil {
		return invalid("relation_origin", fmt.Sprintf("relation originMode %q does not resolve", relation.OriginMode))
	}
	return nil
}

// currentColumnsOf returns the columns that drive Current planning: the
// origin Mode when following, otherwise the Current's own columns.
func currentColumnsOf(project warehouse.Project, state repository.CurrentState) (map[string]index.ModeColumnSelection, error) {
	if state.Relation != nil && state.Relation.Kind == "following" {
		mode, err := project.ResolveMode(state.Relation.OriginMode)
		if err != nil {
			return nil, notFound("relation_origin_missing", fmt.Sprintf("following mode %q no longer resolves", state.Relation.OriginMode))
		}
		if mode.Missing {
			return nil, notFound("relation_origin_missing", fmt.Sprintf("following mode %q source is missing", state.Relation.OriginMode))
		}
		return mode.Metadata.Columns, nil
	}
	return ColumnsOf(state.Columns), nil
}

// planCurrentMappings plans the mapping set for one persisted Current state.
// Missing-resource errors are preserved so the CLI can classify them.
func planCurrentMappings(project warehouse.Project, state repository.CurrentState, options PlanOptions) ([]linker.Mapping, error) {
	columns, err := currentColumnsOf(project, state)
	if err != nil {
		return nil, err
	}
	return planner.PlanColumns(project, columns, state.Mappings, options)
}

// ApplyMode applies one named Mode selection as the new Current state.
func ApplyMode(repo repository.Repository, projectReference, modeReference string, forceTargets bool, options PlanOptions) error {
	_, project, err := loadProject(repo, projectReference)
	if err != nil {
		return err
	}
	mode, err := project.ResolveMode(modeReference)
	if err != nil {
		return notFound("mode_not_found", err.Error())
	}
	if mode.Missing {
		return notFound("mode_missing", fmt.Sprintf("mode %q source is missing", mode.Name))
	}
	state, err := repo.LoadCurrentState(project.Name)
	if err != nil {
		return invalid("current_state", err.Error())
	}
	next := repository.CurrentState{
		Columns:  PersistColumns(mode.Metadata.Columns),
		Relation: &repository.CurrentRelation{Kind: "following", OriginMode: mode.Name},
		Mappings: state.Mappings,
	}
	mappings, err := planCurrentMappings(project, next, options)
	if err != nil {
		return err
	}
	next.Mappings = mappings
	return linker.New().ReplaceState(project, next, linker.WithForce(forceTargets))
}

// ApplyColumn applies one explicit Column cover selection as the new Current state.
func ApplyColumn(repo repository.Repository, projectReference, columnReference string, settingReferences []string, forceTargets bool, options PlanOptions) error {
	_, project, err := loadProject(repo, projectReference)
	if err != nil {
		return err
	}
	column, err := project.ResolveColumn(columnReference)
	if err != nil {
		return notFound("column_not_found", err.Error())
	}
	if column.Missing {
		return notFound("column_missing", fmt.Sprintf("column %q source is missing", column.Name))
	}
	canonical, err := CanonicalSettings(column, "cover", settingReferences)
	if err != nil {
		return err
	}
	state, err := repo.LoadCurrentState(project.Name)
	if err != nil {
		return invalid("current_state", err.Error())
	}
	next := repository.CurrentState{
		Columns:  map[string]repository.ColumnSelection{column.Name: {Strategy: "cover", Settings: canonical}},
		Mappings: state.Mappings,
	}
	mappings, err := planner.PlanColumns(project, ColumnsOf(next.Columns), state.Mappings, options)
	if err != nil {
		return err
	}
	next.Mappings = mappings
	return linker.New().ReplaceState(project, next, linker.WithForce(forceTargets))
}

// SetCurrentColumn atomically replaces one Current Column selection. The
// first direct modification detaches a following Current.
func SetCurrentColumn(repo repository.Repository, projectReference, columnReference, strategy string, settingReferences []string, forceTargets bool, options PlanOptions) error {
	_, project, err := loadProject(repo, projectReference)
	if err != nil {
		return err
	}
	column, err := project.ResolveColumn(columnReference)
	if err != nil {
		return notFound("column_not_found", err.Error())
	}
	if column.Missing {
		return notFound("column_missing", fmt.Sprintf("column %q source is missing", column.Name))
	}
	canonical, err := CanonicalSettings(column, strategy, settingReferences)
	if err != nil {
		return err
	}
	state, err := repo.LoadCurrentState(project.Name)
	if err != nil {
		return invalid("current_state", err.Error())
	}
	if err := ValidateRelation(project, state.Relation); err != nil {
		return err
	}
	nextColumns := make(map[string]repository.ColumnSelection, len(state.Columns)+1)
	for name, selection := range state.Columns {
		nextColumns[name] = selection
	}
	nextColumns[column.Name] = repository.ColumnSelection{Strategy: strategy, Settings: canonical}
	if sameColumns(nextColumns, state.Columns) {
		return nil
	}
	next := repository.CurrentState{
		Columns:  nextColumns,
		Relation: detachRelation(state.Relation),
		Mappings: state.Mappings,
	}
	mappings, err := planner.PlanColumns(project, ColumnsOf(nextColumns), state.Mappings, options)
	if err != nil {
		return invalid("current_plan", err.Error())
	}
	next.Mappings = mappings
	return linker.New().ReplaceState(project, next, linker.WithForce(forceTargets))
}

// DeleteCurrentColumn atomically removes one Current Column selection.
func DeleteCurrentColumn(repo repository.Repository, projectReference, columnReference string, forceTargets bool, options PlanOptions) error {
	_, project, err := loadProject(repo, projectReference)
	if err != nil {
		return err
	}
	column, err := project.ResolveColumn(columnReference)
	if err != nil {
		return notFound("column_not_found", err.Error())
	}
	if column.Missing {
		return notFound("column_missing", fmt.Sprintf("column %q source is missing", column.Name))
	}
	state, err := repo.LoadCurrentState(project.Name)
	if err != nil {
		return invalid("current_state", err.Error())
	}
	if err := ValidateRelation(project, state.Relation); err != nil {
		return err
	}
	if _, exists := state.Columns[column.Name]; !exists {
		return notFound("current_column_not_found", fmt.Sprintf("current has no selection for column %q", column.Name))
	}
	nextColumns := make(map[string]repository.ColumnSelection, len(state.Columns)-1)
	for name, selection := range state.Columns {
		if name != column.Name {
			nextColumns[name] = selection
		}
	}
	next := repository.CurrentState{
		Columns:  nextColumns,
		Relation: detachRelation(state.Relation),
		Mappings: state.Mappings,
	}
	mappings, err := planner.PlanColumns(project, ColumnsOf(nextColumns), state.Mappings, options)
	if err != nil {
		return invalid("current_plan", err.Error())
	}
	next.Mappings = mappings
	return linker.New().ReplaceState(project, next, linker.WithForce(forceTargets))
}

// ReplaceCurrent atomically replaces the complete Current Column selection.
// Clients submit only columns; relation and mappings are derived by the backend.
func ReplaceCurrent(repo repository.Repository, projectReference string, columns map[string]repository.ColumnSelection, forceTargets bool, options PlanOptions) error {
	_, project, err := loadProject(repo, projectReference)
	if err != nil {
		return err
	}
	normalized, err := validateColumnSelections(project, columns)
	if err != nil {
		return err
	}
	state, err := repo.LoadCurrentState(project.Name)
	if err != nil {
		return invalid("current_state", err.Error())
	}
	if err := ValidateRelation(project, state.Relation); err != nil {
		return err
	}
	if sameColumns(normalized, state.Columns) {
		return nil
	}
	next := repository.CurrentState{
		Columns:  normalized,
		Relation: detachRelation(state.Relation),
		Mappings: state.Mappings,
	}
	mappings, err := planner.PlanColumns(project, ColumnsOf(normalized), state.Mappings, options)
	if err != nil {
		return invalid("current_plan", err.Error())
	}
	next.Mappings = mappings
	return linker.New().ReplaceState(project, next, linker.WithForce(forceTargets))
}

// detachRelation keeps the origin Mode but stops following it.
func detachRelation(relation *repository.CurrentRelation) *repository.CurrentRelation {
	if relation == nil {
		return nil
	}
	return &repository.CurrentRelation{Kind: "detached", OriginMode: relation.OriginMode}
}

// validateColumnSelections resolves every Column and Setting reference.
func validateColumnSelections(project warehouse.Project, columns map[string]repository.ColumnSelection) (map[string]repository.ColumnSelection, error) {
	normalized := make(map[string]repository.ColumnSelection, len(columns))
	for reference, selection := range columns {
		column, err := project.ResolveColumn(reference)
		if err != nil {
			return nil, notFound("column_not_found", err.Error())
		}
		if column.Missing {
			return nil, notFound("column_missing", fmt.Sprintf("column %q source is missing", column.Name))
		}
		canonical, err := CanonicalSettings(column, selection.Strategy, selection.Settings)
		if err != nil {
			return nil, err
		}
		normalized[column.Name] = repository.ColumnSelection{Strategy: selection.Strategy, Settings: canonical}
	}
	return normalized, nil
}

// CanonicalSettings validates strategy-specific Setting input and resolves
// canonical identities before writing.
func CanonicalSettings(column warehouse.Column, strategy string, references []string) ([]string, error) {
	switch strategy {
	case "cover", "increment":
		if len(references) == 0 {
			return nil, invalid("mode_settings_required", fmt.Sprintf("strategy %q requires at least one setting", strategy))
		}
	case "none", "full":
		if len(references) != 0 {
			return nil, invalid("mode_settings_forbidden", fmt.Sprintf("strategy %q does not accept settings", strategy))
		}
		return []string{}, nil
	default:
		return nil, invalid("invalid_mode_strategy", fmt.Sprintf("unsupported mode strategy %q", strategy))
	}
	canonical := make([]string, 0, len(references))
	for _, reference := range references {
		setting, err := column.ResolveSetting(reference)
		if err != nil {
			return nil, notFound("setting_not_found", err.Error())
		}
		if setting.Missing {
			return nil, notFound("setting_missing", fmt.Sprintf("setting %q in column %q is missing", setting.Name, column.Name))
		}
		canonical = append(canonical, setting.Name)
	}
	return canonical, nil
}

// RefreshCurrent replans the Current mapping set from its authoritative
// columns (origin Mode when following, own columns otherwise) and updates
// links. The relation is preserved.
func RefreshCurrent(repo repository.Repository, projectReference string, forceTargets bool, options PlanOptions) error {
	_, project, err := loadProject(repo, projectReference)
	if err != nil {
		return err
	}
	state, err := repo.LoadCurrentState(project.Name)
	if err != nil {
		return invalid("current_state", err.Error())
	}
	if err := ValidateRelation(project, state.Relation); err != nil {
		return err
	}
	next := repository.CurrentState{
		Columns:  state.Columns,
		Relation: state.Relation,
		Mappings: state.Mappings,
	}
	mappings, err := planCurrentMappings(project, next, options)
	if err != nil {
		return err
	}
	next.Mappings = mappings
	return linker.New().ReplaceState(project, next, linker.WithForce(forceTargets))
}

// ResetCurrent clears the Current selection, relation, and mappings.
func ResetCurrent(repo repository.Repository, projectReference string, forceTargets bool) error {
	_, project, err := loadProject(repo, projectReference)
	if err != nil {
		return err
	}
	return linker.New().Reset(project, linker.WithForce(forceTargets))
}

// RevertCurrent restores the previous persisted Current state snapshot.
func RevertCurrent(repo repository.Repository, projectReference string, forceTargets bool) error {
	_, project, err := loadProject(repo, projectReference)
	if err != nil {
		return err
	}
	engine := linker.New()
	previous, err := engine.LoadPreviousState(project)
	if err != nil {
		return invalid("previous_state", err.Error())
	}
	return engine.ReplaceState(project, previous, linker.WithForce(forceTargets))
}

// SetModeColumn atomically replaces one Mode Column selection and, when the
// Current follows that Mode, replans and updates the Current in the same
// transaction.
func SetModeColumn(repo repository.Repository, projectReference, modeReference, columnReference, strategy string, settingReferences []string, forceTargets bool, options PlanOptions) error {
	return mutateModeColumn(repo, projectReference, modeReference, columnReference, strategy, settingReferences, false, forceTargets, options)
}

// DeleteModeColumn atomically removes one Mode Column selection and, when the
// Current follows that Mode, replans and updates the Current in the same
// transaction.
func DeleteModeColumn(repo repository.Repository, projectReference, modeReference, columnReference string, forceTargets bool, options PlanOptions) error {
	return mutateModeColumn(repo, projectReference, modeReference, columnReference, "", nil, true, forceTargets, options)
}

func mutateModeColumn(repo repository.Repository, projectReference, modeReference, columnReference, strategy string, settingReferences []string, remove bool, forceTargets bool, options PlanOptions) error {
	_, project, err := loadProject(repo, projectReference)
	if err != nil {
		return err
	}
	mode, err := project.ResolveMode(modeReference)
	if err != nil {
		return notFound("mode_not_found", err.Error())
	}
	if mode.Missing {
		return notFound("mode_missing", fmt.Sprintf("mode %q source is missing", mode.Name))
	}
	column, err := project.ResolveColumn(columnReference)
	if err != nil {
		return notFound("column_not_found", err.Error())
	}
	if column.Missing {
		return notFound("column_missing", fmt.Sprintf("column %q source is missing", column.Name))
	}
	var canonical []string
	if !remove {
		canonical, err = CanonicalSettings(column, strategy, settingReferences)
		if err != nil {
			return err
		}
	}
	state, err := repo.LoadCurrentState(project.Name)
	if err != nil {
		return invalid("current_state", err.Error())
	}
	if err := ValidateRelation(project, state.Relation); err != nil {
		return err
	}

	modeIndex := project.ModeIndex
	entry := modeIndex.Modes[mode.Name]
	if entry.Columns == nil {
		entry.Columns = map[string]index.ModeColumnSelection{}
	}
	if remove {
		if _, exists := entry.Columns[column.Name]; !exists {
			return notFound("mode_column_not_found", fmt.Sprintf("mode %q has no selection for column %q", mode.Name, column.Name))
		}
		delete(entry.Columns, column.Name)
	} else {
		entry.Columns[column.Name] = index.ModeColumnSelection{Strategy: strategy, Settings: canonical}
	}
	modeIndex.Modes[mode.Name] = entry

	paths := []string{repo.ModeIndexPath(project.Name)}
	follows := state.Relation != nil && state.Relation.Kind == "following" && state.Relation.OriginMode == mode.Name
	if follows {
		paths = append(paths, repo.CurrentStatePath(project.Name), repo.HistoryPath(project.Name))
	}
	return repo.WithMutation("mode-column", paths, func() error {
		if err := repo.SaveModeIndex(project.Name, modeIndex); err != nil {
			return err
		}
		if !follows {
			return nil
		}
		next := repository.CurrentState{
			Columns:  PersistColumns(modeIndex.Modes[mode.Name].Columns),
			Relation: state.Relation,
			Mappings: state.Mappings,
		}
		mappings, err := planner.PlanColumns(project, modeIndex.Modes[mode.Name].Columns, state.Mappings, options)
		if err != nil {
			return invalid("mode_column_plan", err.Error())
		}
		next.Mappings = mappings
		return linker.New().ReplaceStateLocked(project, next, linker.WithForce(forceTargets))
	})
}

// ReplaceMode atomically replaces a Mode's complete Column selection map and
// syncs a following Current in the same transaction.
func ReplaceMode(repo repository.Repository, projectReference, modeReference string, columns map[string]repository.ColumnSelection, forceTargets bool, options PlanOptions) error {
	_, project, err := loadProject(repo, projectReference)
	if err != nil {
		return err
	}
	mode, err := project.ResolveMode(modeReference)
	if err != nil {
		return notFound("mode_not_found", err.Error())
	}
	if mode.Missing {
		return notFound("mode_missing", fmt.Sprintf("mode %q source is missing", mode.Name))
	}
	normalized, err := validateColumnSelections(project, columns)
	if err != nil {
		return err
	}
	state, err := repo.LoadCurrentState(project.Name)
	if err != nil {
		return invalid("current_state", err.Error())
	}
	if err := ValidateRelation(project, state.Relation); err != nil {
		return err
	}
	modeIndex := project.ModeIndex
	modeIndex.Modes[mode.Name] = index.ModeEntry{
		WarehouseName: mode.Metadata.WarehouseName,
		DisplayName:   mode.Metadata.DisplayName,
		Description:   mode.Metadata.Description,
		Aliases:       mode.Metadata.Aliases,
		Columns:       ColumnsOf(normalized),
		Extra:         mode.Metadata.Extra,
	}
	paths := []string{repo.ModeIndexPath(project.Name)}
	follows := state.Relation != nil && state.Relation.Kind == "following" && state.Relation.OriginMode == mode.Name
	if follows {
		paths = append(paths, repo.CurrentStatePath(project.Name), repo.HistoryPath(project.Name))
	}
	return repo.WithMutation("mode-replace", paths, func() error {
		if err := repo.SaveModeIndex(project.Name, modeIndex); err != nil {
			return err
		}
		if !follows {
			return nil
		}
		next := repository.CurrentState{
			Columns:  normalized,
			Relation: state.Relation,
			Mappings: state.Mappings,
		}
		mappings, err := planner.PlanColumns(project, ColumnsOf(normalized), state.Mappings, options)
		if err != nil {
			return invalid("mode_replace_plan", err.Error())
		}
		next.Mappings = mappings
		return linker.New().ReplaceStateLocked(project, next, linker.WithForce(forceTargets))
	})
}

// DeleteModeWorkflow deletes one Mode and clears any Current relation that
// points to it, preserving Current selections and mappings.
func DeleteModeWorkflow(repo repository.Repository, projectReference, modeReference string, yes, forceTargets bool) error {
	if !yes {
		return refused("confirmation_required", "mode deletion requires --yes")
	}
	_, project, err := loadProject(repo, projectReference)
	if err != nil {
		return err
	}
	mode, err := project.ResolveMode(modeReference)
	if err != nil {
		return notFound("mode_not_found", err.Error())
	}
	state, err := repo.LoadCurrentState(project.Name)
	if err != nil {
		return invalid("current_state", err.Error())
	}
	modeIndex := project.ModeIndex
	delete(modeIndex.Modes, mode.Name)
	paths := []string{repo.ModeIndexPath(project.Name)}
	next := repository.CurrentState{Columns: state.Columns, Mappings: state.Mappings}
	if state.Relation != nil && state.Relation.OriginMode == mode.Name {
		next.Relation = nil
		paths = append(paths, repo.CurrentStatePath(project.Name), repo.HistoryPath(project.Name))
	} else {
		next.Relation = state.Relation
	}
	return repo.WithMutation("mode-delete", paths, func() error {
		if err := repo.SaveModeIndex(project.Name, modeIndex); err != nil {
			return err
		}
		if state.Relation != nil && state.Relation.OriginMode == mode.Name {
			return linker.New().ReplaceStateLocked(project, next, linker.WithForce(forceTargets))
		}
		return nil
	})
}

// CurrentDirectory resolves the Setting content root for one Setting.
func CurrentDirectory(projectName, columnName, settingName string) string {
	return filepath.Join(projectName, "Column", columnName, settingName)
}

// SortedColumnNames returns deterministic Current Column names.
func SortedColumnNames(columns map[string]repository.ColumnSelection) []string {
	names := make([]string, 0, len(columns))
	for name := range columns {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ProjectCreate creates one empty Project through the mutation layer.
func ProjectCreate(repo repository.Repository, name, displayName, description string, aliases []string) error {
	metadata, err := mutate.NewMetadata(mutate.ProjectKind, name, displayName, description, aliases)
	if err != nil {
		return err
	}
	return mutate.CreateProject(repo, name, metadata)
}

// ProjectDelete deletes one Project after confirmation.
func ProjectDelete(repo repository.Repository, reference string, yes, forceTargets bool) error {
	_, err := mutate.DeleteProject(repo, reference, yes, false, forceTargets)
	return err
}

// ColumnDelete deletes one Column after confirmation and optional cascade.
func ColumnDelete(repo repository.Repository, projectReference, reference string, yes, cascade, forceTargets bool) error {
	_, err := mutate.DeleteColumn(repo, projectReference, reference, yes, cascade, forceTargets)
	return err
}

// SettingDelete deletes one Setting after confirmation and optional cascade.
func SettingDelete(repo repository.Repository, projectReference, columnReference, reference string, yes, cascade, forceTargets bool) error {
	_, err := mutate.DeleteSetting(repo, projectReference, columnReference, reference, yes, cascade, forceTargets)
	return err
}
