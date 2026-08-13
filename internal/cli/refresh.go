package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/linker"
	"github.com/xenon/ConfigFacilitator/internal/planner"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// newRefreshCommand constructs intent-aware Project, Column, and all-Project refresh scopes.
func newRefreshCommand(context *commandContext) *cobra.Command {
	var scope projectScope
	var column string
	var all bool
	var forceTargets bool
	command := &cobra.Command{
		Use:     "refresh",
		Short:   "Re-plan current managed configuration",
		Args:    usageArgs(cobra.NoArgs),
		Example: "  cfgfc refresh -p OpenCode\n  cfgfc refresh --column Skills\n  cfgfc refresh --all",
		PreRunE: func(command *cobra.Command, args []string) error {
			if all && (scope.project != "" || column != "") {
				return NewUsageError("conflicting_scope", "refresh --all cannot be combined with --project or --column", nil)
			}
			return nil
		},
		RunE: func(command *cobra.Command, args []string) error {
			if all {
				return runRefreshAll(context, forceTargets)
			}
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			return refreshProject(context, project, column, forceTargets)
		},
	}
	addProjectFlag(command, &scope)
	command.Flags().StringVar(&column, "column", "", "Refresh only one Column")
	command.Flags().BoolVar(&all, "all", false, "Refresh every Project with active state")
	command.Flags().BoolVar(&forceTargets, "force-targets", false, "Reclaim occupied recorded target paths")
	return command
}

// runRefreshAll refreshes every Project containing current mappings or intent.
func runRefreshAll(context *commandContext, forceTargets bool) error {
	warehouseRoot, err := effectiveWarehouseRoot(context.dependencies)
	if err != nil {
		return err
	}
	loaded, loadErr := warehouse.LoadWarehouse(warehouseRoot)
	if loadErr != nil {
		return NewInvalidDataError("warehouse_data", loadErr.Error(), nil, loadErr)
	}
	updated := make([]string, 0, len(loaded.Projects))
	for _, projectName := range sortedKeys(loaded.Projects) {
		project := loaded.Projects[projectName]
		state, stateErr := linker.New().LoadCurrentState(project)
		if stateErr != nil {
			return NewInvalidDataError("current_state", stateErr.Error(), map[string]any{"project": project.Name}, stateErr)
		}
		if len(state.Mappings) == 0 && state.Intent == nil {
			continue
		}
		if err := refreshProjectMutation(project, "", forceTargets, context.dependencies); err != nil {
			return err
		}
		updated = append(updated, project.Name)
	}
	return context.renderResult(HumanResult{
		Message: fmt.Sprintf("Refreshed %d project(s)", len(updated)),
		Data:    map[string]any{"projects": updated},
	})
}

// refreshProject refreshes one Project and renders its stable command result.
func refreshProject(context *commandContext, project warehouse.Project, columnReference string, forceTargets bool) error {
	if project.Missing {
		return classifyPlanError("refresh_plan", planner.MissingResourceError{Kind: "project", Project: project.Name})
	}
	columnName := ""
	if columnReference != "" {
		column, err := project.ResolveColumn(columnReference)
		if err != nil {
			return NewResourceError("column_not_found", err.Error(), nil, err)
		}
		if column.Missing {
			return classifyPlanError("refresh_plan", planner.MissingResourceError{Kind: "column", Project: project.Name, Column: column.Name})
		}
		columnName = column.Name
	}
	if err := refreshProjectMutation(project, columnName, forceTargets, context.dependencies); err != nil {
		return err
	}
	if columnName != "" {
		return context.renderResult(HumanResult{
			Message: fmt.Sprintf("Refreshed column %q for project %q", columnName, displayStatusName(project.Metadata.DisplayName, project.Name)),
			Data:    map[string]any{"project": project.Name, "column": columnName},
		})
	}
	return context.renderResult(HumanResult{
		Message: fmt.Sprintf("Refreshed project %q", displayStatusName(project.Metadata.DisplayName, project.Name)),
		Data:    map[string]any{"project": project.Name},
	})
}

// refreshProjectMutation reuses the established planner and linker behavior without exposing removed syntax.
func refreshProjectMutation(project warehouse.Project, columnName string, forceTargets bool, dependencies Dependencies) error {
	engine := linker.New()
	state, err := engine.LoadCurrentState(project)
	if err != nil {
		return NewInvalidDataError("current_state", err.Error(), nil, err)
	}
	var mappings []linker.Mapping
	if columnName != "" {
		if state.Intent != nil {
			mappings, err = planner.PlanIntentColumnUpdateMappings(project, *state.Intent, columnName, state.Mappings, planOptions(dependencies))
		} else {
			mappings, err = planner.PlanColumnUpdateMappings(project, columnName, state.Mappings, planOptions(dependencies))
		}
	} else if state.Intent != nil {
		mappings, err = planner.PlanIntentUpdateMappings(project, *state.Intent, state.Mappings, planOptions(dependencies))
	} else {
		mappings, err = planner.PlanUpdateMappings(project, state.Mappings, planOptions(dependencies))
	}
	if err != nil {
		return classifyPlanError("refresh_plan", err)
	}
	if err := engine.ReplaceState(project, linker.CurrentState{Mappings: mappings, Intent: cloneApplyIntent(state.Intent)}, linker.WithForce(forceTargets)); err != nil {
		return classifyMutationError("refresh_failed", err)
	}
	return nil
}

// cloneApplyIntent copies persisted apply intent before replacing refreshed state.
func cloneApplyIntent(intent *linker.ApplyIntent) *linker.ApplyIntent {
	if intent == nil {
		return nil
	}
	clone := *intent
	clone.Settings = append([]string(nil), intent.Settings...)
	return &clone
}
