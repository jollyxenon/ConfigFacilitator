package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/linker"
	"github.com/xenon/ConfigFacilitator/internal/planner"
	"github.com/xenon/ConfigFacilitator/internal/session"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// newResetCommand constructs the normalized reset syntax.
func newResetCommand(context *commandContext) *cobra.Command {
	var scope projectScope
	var forceTargets bool
	command := &cobra.Command{
		Use:     "reset",
		Short:   "Remove the effective Project's managed mappings",
		Args:    usageArgs(cobra.NoArgs),
		Example: "  cfgfc reset -p OpenCode\n  cfgfc reset --force-targets",
		RunE: func(command *cobra.Command, args []string) error {
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			if err := linker.New().Reset(project, linker.WithForce(forceTargets)); err != nil {
				return classifyMutationError("reset_failed", err)
			}
			return context.renderResult(HumanResult{
				Message: fmt.Sprintf("Reset project %q", displayStatusName(project.Metadata.DisplayName, project.Name)),
				Data:    map[string]any{"project": project.Name},
			})
		},
	}
	addProjectFlag(command, &scope)
	command.Flags().BoolVar(&forceTargets, "force-targets", false, "Reclaim occupied recorded target paths")
	return command
}

// newRevertCommand constructs the normalized one-step revert syntax.
func newRevertCommand(context *commandContext) *cobra.Command {
	var scope projectScope
	var forceTargets bool
	command := &cobra.Command{
		Use:     "revert",
		Short:   "Restore the effective Project's previous snapshot",
		Args:    usageArgs(cobra.NoArgs),
		Example: "  cfgfc revert -p OpenCode\n  cfgfc revert --force-targets",
		RunE: func(command *cobra.Command, args []string) error {
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			engine := linker.New()
			previous, loadErr := engine.LoadPreviousState(project)
			if loadErr != nil {
				return NewResourceError("previous_state_not_found", loadErr.Error(), nil, loadErr)
			}
			if err := engine.ReplaceState(project, previous, linker.WithForce(forceTargets)); err != nil {
				return classifyMutationError("revert_failed", err)
			}
			return context.renderResult(HumanResult{
				Message: fmt.Sprintf("Reverted project %q", displayStatusName(project.Metadata.DisplayName, project.Name)),
				Data:    map[string]any{"project": project.Name, "state": previous},
			})
		},
	}
	addProjectFlag(command, &scope)
	command.Flags().BoolVar(&forceTargets, "force-targets", false, "Reclaim occupied recorded target paths")
	return command
}

// effectiveWarehouseRoot resolves the bootstrap file entirely from injected dependencies.
func effectiveWarehouseRoot(dependencies Dependencies) (string, error) {
	rootPath, err := warehouse.EffectiveWarehouseRootFor(dependencies.HomeDir)
	if err != nil {
		return "", NewPersistenceError("warehouse_root", "resolve warehouse root", err)
	}
	return rootPath, nil
}

// resolveProjectForCommand resolves explicit or injected PPID context into one canonical Project.
func resolveProjectForCommand(dependencies Dependencies, explicitProject string) (warehouse.Project, error) {
	if explicitProject == globalProjectName {
		return warehouse.Project{}, NewResourceError("reserved_project", fmt.Sprintf("project name %q is reserved", globalProjectName), nil, nil)
	}
	rootPath, err := effectiveWarehouseRoot(dependencies)
	if err != nil {
		return warehouse.Project{}, err
	}
	effectiveProject, _, contextErr := session.ResolveProject(explicitProject, dependencies.PPID, session.NewStore(rootPath))
	if contextErr != nil {
		return warehouse.Project{}, NewPersistenceError("read_context", "read selected Project context", contextErr)
	}
	if effectiveProject == "" {
		return warehouse.Project{}, NewResourceError("project_scope_required", "no selected Project; provide -p/--project or run cfgfc use", nil, nil)
	}
	loaded, loadErr := warehouse.LoadWarehouse(rootPath)
	if loadErr != nil {
		return warehouse.Project{}, NewInvalidDataError("warehouse_data", loadErr.Error(), nil, loadErr)
	}
	project, resolveErr := loaded.ResolveProject(effectiveProject)
	if resolveErr != nil {
		return warehouse.Project{}, NewResourceError("project_not_found", resolveErr.Error(), nil, resolveErr)
	}
	return project, nil
}

// planOptions builds planner options from injected home, environment, and OS values.
func planOptions(dependencies Dependencies) planner.PlanOptions {
	return planner.PlanOptions{HomeDir: dependencies.HomeDir, Env: dependencies.Environment, OS: dependencies.OperatingSystem}
}

// classifyMutationError distinguishes unsafe target refusal from persistence failures.
func classifyMutationError(code string, err error) *CommandError {
	message := strings.ToLower(err.Error())
	for _, indicator := range []string{"unmanaged", "drift", "occupied", "owned", "ownership", "force"} {
		if strings.Contains(message, indicator) {
			return NewRefusalError("unsafe_target", err.Error(), nil, err)
		}
	}
	return NewPersistenceError(code, err.Error(), err)
}
