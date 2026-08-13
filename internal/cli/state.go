package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/planner"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/session"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
	"github.com/xenon/ConfigFacilitator/internal/workflow"
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
			rootPath, err := effectiveWarehouseRoot(context.dependencies)
			if err != nil {
				return err
			}
			if err := workflow.ResetCurrent(repository.New(rootPath), project.Name, forceTargets); err != nil {
				return classifyWorkflowError(err)
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
			rootPath, err := effectiveWarehouseRoot(context.dependencies)
			if err != nil {
				return err
			}
			if err := workflow.RevertCurrent(repository.New(rootPath), project.Name, forceTargets); err != nil {
				return classifyWorkflowError(err)
			}
			return context.renderResult(HumanResult{
				Message: fmt.Sprintf("Reverted project %q", displayStatusName(project.Metadata.DisplayName, project.Name)),
				Data:    map[string]any{"project": project.Name},
			})
		},
	}
	addProjectFlag(command, &scope)
	command.Flags().BoolVar(&forceTargets, "force-targets", false, "Reclaim occupied recorded target paths")
	return command
}

// EffectiveWarehouseRoot resolves the bootstrap file entirely from injected dependencies.
func EffectiveWarehouseRoot(dependencies Dependencies) (string, error) {
	return effectiveWarehouseRoot(dependencies)
}

// effectiveWarehouseRoot resolves the bootstrap file entirely from injected dependencies.
func effectiveWarehouseRoot(dependencies Dependencies) (string, error) {
	rootPath, err := warehouse.EffectiveWarehouseRootFor(dependencies.HomeDir)
	if err != nil {
		return "", NewPersistenceError("warehouse_root", "resolve warehouse root", err)
	}
	return rootPath, nil
}

// classifyWorkflowError maps shared workflow failures to CLI error classes.
func classifyWorkflowError(err error) *CommandError {
	var missing planner.MissingResourceError
	if errors.As(err, &missing) {
		return NewResourceError("resource_missing", err.Error(), nil, err)
	}
	class, code := workflow.Classify(err)
	switch class {
	case workflow.ErrNotFound:
		return NewResourceError(code, err.Error(), nil, err)
	case workflow.ErrConflict:
		return NewResourceError(code, err.Error(), nil, err)
	case workflow.ErrInvalid:
		return NewInvalidDataError(code, err.Error(), nil, err)
	case workflow.ErrRefused:
		return NewRefusalError(code, err.Error(), nil, err)
	default:
		return NewPersistenceError(code, err.Error(), err)
	}
}

// unsafeTargetError reports whether a workflow error is an unsafe-target refusal.
func unsafeTargetError(err error) bool {
	message := strings.ToLower(err.Error())
	for _, indicator := range []string{"unmanaged", "drift", "occupied", "owned", "ownership", "force"} {
		if strings.Contains(message, indicator) {
			return true
		}
	}
	return false
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
