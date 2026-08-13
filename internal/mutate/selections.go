package mutate

import (
	"encoding/json"
	"fmt"

	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

const (
	modeStrategyCover     = "cover"
	modeStrategyIncrement = "increment"
	modeStrategyNone      = "none"
	modeStrategyFull      = "full"
)

// SetModeColumnSelection validates and transactionally replaces one Mode Column selection.
func SetModeColumnSelection(repo repository.Repository, projectReference string, modeReference string, columnReference string, strategy string, settingReferences []string) (string, string, []string, error) {
	_, project, err := loadProject(repo.RootPath, projectReference)
	if err != nil {
		return "", "", nil, err
	}
	if project.Missing {
		return "", "", nil, missing("project_missing", fmt.Sprintf("project %q is missing", project.Name), nil)
	}
	mode, err := project.ResolveMode(modeReference)
	if err != nil {
		return "", "", nil, missing("mode_not_found", err.Error(), err)
	}
	if mode.Missing {
		return "", "", nil, missing("mode_missing", fmt.Sprintf("mode %q is missing", mode.Name), nil)
	}
	column, err := project.ResolveColumn(columnReference)
	if err != nil {
		return "", "", nil, missing("column_not_found", err.Error(), err)
	}
	if column.Missing {
		return "", "", nil, missing("column_missing", fmt.Sprintf("column %q is missing", column.Name), nil)
	}
	canonicalSettings, err := canonicalModeSettings(column, strategy, settingReferences)
	if err != nil {
		return "", "", nil, err
	}

	entry := project.ModeIndex.Modes[mode.Name]
	existingKey, existing, found, err := findModeColumnSelection(project, entry, column.Name)
	if err != nil {
		return "", "", nil, err
	}
	extra := map[string]json.RawMessage{}
	if found {
		extra = existing.Extra
		delete(entry.Columns, existingKey)
	} else {
		for persistedColumn := range entry.Columns {
			if persistedColumn == column.Name {
				return "", "", nil, conflict("mode_column_ambiguous", fmt.Sprintf("mode %q contains an unresolved selection for column %q", mode.Name, column.Name), nil)
			}
		}
	}
	entry.Columns[column.Name] = index.ModeColumnSelection{Strategy: strategy, Settings: canonicalSettings, Extra: extra}
	project.ModeIndex.Modes[mode.Name] = entry
	if err := repo.WithMutation("mode-column-set", []string{repo.ModeIndexPath(project.Name)}, func() error {
		return repo.SaveModeIndex(project.Name, project.ModeIndex)
	}); err != nil {
		return "", "", nil, persistence("mode_column_set", fmt.Sprintf("set column %q in mode %q", column.Name, mode.Name), err)
	}
	return mode.Name, column.Name, canonicalSettings, nil
}

// DeleteModeColumnSelection transactionally removes one existing Mode Column selection without resource-deletion confirmation.
func DeleteModeColumnSelection(repo repository.Repository, projectReference string, modeReference string, columnReference string) (string, string, error) {
	_, project, err := loadProject(repo.RootPath, projectReference)
	if err != nil {
		return "", "", err
	}
	if project.Missing {
		return "", "", missing("project_missing", fmt.Sprintf("project %q is missing", project.Name), nil)
	}
	mode, err := project.ResolveMode(modeReference)
	if err != nil {
		return "", "", missing("mode_not_found", err.Error(), err)
	}
	if mode.Missing {
		return "", "", missing("mode_missing", fmt.Sprintf("mode %q is missing", mode.Name), nil)
	}
	column, err := project.ResolveColumn(columnReference)
	if err != nil {
		return "", "", missing("column_not_found", err.Error(), err)
	}
	if column.Missing {
		return "", "", missing("column_missing", fmt.Sprintf("column %q is missing", column.Name), nil)
	}

	entry := project.ModeIndex.Modes[mode.Name]
	existingKey, _, found, err := findModeColumnSelection(project, entry, column.Name)
	if err != nil {
		return "", "", err
	}
	if !found {
		if _, unresolved := entry.Columns[column.Name]; unresolved {
			return "", "", conflict("mode_column_ambiguous", fmt.Sprintf("mode %q contains an unresolved selection for column %q", mode.Name, column.Name), nil)
		}
		return "", "", missing("mode_column_not_found", fmt.Sprintf("mode %q has no selection for column %q", mode.Name, column.Name), nil)
	}
	delete(entry.Columns, existingKey)
	project.ModeIndex.Modes[mode.Name] = entry
	if err := repo.WithMutation("mode-column-delete", []string{repo.ModeIndexPath(project.Name)}, func() error {
		return repo.SaveModeIndex(project.Name, project.ModeIndex)
	}); err != nil {
		return "", "", persistence("mode_column_delete", fmt.Sprintf("delete column %q from mode %q", column.Name, mode.Name), err)
	}
	return mode.Name, column.Name, nil
}

// canonicalModeSettings validates strategy-specific Setting input and resolves canonical identities before writing.
func canonicalModeSettings(column warehouse.Column, strategy string, references []string) ([]string, error) {
	switch strategy {
	case modeStrategyCover, modeStrategyIncrement:
		if len(references) == 0 {
			return nil, invalid("mode_settings_required", fmt.Sprintf("strategy %q requires at least one --setting", strategy), nil)
		}
	case modeStrategyNone, modeStrategyFull:
		if len(references) != 0 {
			return nil, invalid("mode_settings_forbidden", fmt.Sprintf("strategy %q does not accept --setting", strategy), nil)
		}
		return []string{}, nil
	default:
		return nil, invalid("invalid_mode_strategy", fmt.Sprintf("unsupported mode strategy %q", strategy), nil)
	}

	canonical := make([]string, 0, len(references))
	for _, reference := range references {
		setting, err := column.ResolveSetting(reference)
		if err != nil {
			return nil, missing("setting_not_found", err.Error(), err)
		}
		if setting.Missing {
			return nil, missing("setting_missing", fmt.Sprintf("setting %q in column %q is missing", setting.Name, column.Name), nil)
		}
		canonical = append(canonical, setting.Name)
	}
	return canonical, nil
}

// findModeColumnSelection resolves persisted Column keys and rejects duplicate references to one canonical Column.
func findModeColumnSelection(project warehouse.Project, entry index.ModeEntry, canonicalColumn string) (string, index.ModeColumnSelection, bool, error) {
	matchedKey := ""
	matched := index.ModeColumnSelection{}
	found := false
	for reference, selection := range entry.Columns {
		column, err := project.ResolveColumn(reference)
		if err != nil || column.Name != canonicalColumn {
			continue
		}
		if found {
			return "", index.ModeColumnSelection{}, false, conflict("mode_column_ambiguous", fmt.Sprintf("mode %q contains multiple selections for column %q", entry.WarehouseName, canonicalColumn), nil)
		}
		matchedKey = reference
		matched = selection
		found = true
	}
	return matchedKey, matched, found, nil
}
