package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
	"github.com/xenon/ConfigFacilitator/internal/workflow"
)

// newRefreshCommand constructs Project refresh scopes backed by the shared workflow.
func newRefreshCommand(context *commandContext) *cobra.Command {
	var scope projectScope
	var all bool
	var forceTargets bool
	command := &cobra.Command{
		Use:     "refresh",
		Short:   "Re-plan current managed configuration",
		Args:    usageArgs(cobra.NoArgs),
		Example: "  cfgfc refresh -p OpenCode\n  cfgfc refresh --all",
		PreRunE: func(command *cobra.Command, args []string) error {
			if all && scope.project != "" {
				return NewUsageError("conflicting_scope", "refresh --all cannot be combined with --project", nil)
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
			return refreshProject(context, project, forceTargets)
		},
	}
	addProjectFlag(command, &scope)
	command.Flags().BoolVar(&all, "all", false, "Refresh every Project with active state")
	command.Flags().BoolVar(&forceTargets, "force-targets", false, "Reclaim occupied recorded target paths")
	return command
}

// runRefreshAll refreshes every Project containing a Current state.
func runRefreshAll(context *commandContext, forceTargets bool) error {
	rootPath, err := effectiveWarehouseRoot(context.dependencies)
	if err != nil {
		return err
	}
	loaded, _, err := loadWarehouseForInspection(context.dependencies)
	if err != nil {
		return err
	}
	updated := make([]string, 0, len(loaded.Projects))
	for _, projectName := range sortedKeys(loaded.Projects) {
		project := loaded.Projects[projectName]
		state, stateErr := repository.New(rootPath).LoadCurrentState(project.Name)
		if stateErr != nil {
			return NewInvalidDataError("current_state", stateErr.Error(), map[string]any{"project": project.Name}, stateErr)
		}
		if len(state.Mappings) == 0 && len(state.Columns) == 0 {
			continue
		}
		if err := refreshProjectMutation(project, forceTargets, context.dependencies); err != nil {
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
func refreshProject(context *commandContext, project warehouse.Project, forceTargets bool) error {
	if project.Missing {
		return NewResourceError("project_missing", fmt.Sprintf("project %q source is missing", project.Name), nil, nil)
	}
	if err := refreshProjectMutation(project, forceTargets, context.dependencies); err != nil {
		return err
	}
	return context.renderResult(HumanResult{
		Message: fmt.Sprintf("Refreshed project %q", displayStatusName(project.Metadata.DisplayName, project.Name)),
		Data:    map[string]any{"project": project.Name},
	})
}

// refreshProjectMutation replans the Current state through the shared workflow.
func refreshProjectMutation(project warehouse.Project, forceTargets bool, dependencies Dependencies) error {
	rootPath, err := effectiveWarehouseRoot(dependencies)
	if err != nil {
		return err
	}
	if err := workflow.RefreshCurrent(repository.New(rootPath), project.Name, forceTargets, planOptions(dependencies)); err != nil {
		return classifyWorkflowError(err)
	}
	return nil
}
