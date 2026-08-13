package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
	"github.com/xenon/ConfigFacilitator/internal/workflow"
)

// newApplyCommand constructs the nested apply mode and apply column syntax.
func newApplyCommand(context *commandContext) *cobra.Command {
	var scope projectScope
	var forceTargets bool
	command := &cobra.Command{
		Use:   "apply",
		Short: "Apply a Mode or explicit Column Settings",
		Args:  usageArgs(cobra.NoArgs),
	}
	addProjectFlag(command, &scope)
	command.PersistentFlags().BoolVar(&forceTargets, "force-targets", false, "Reclaim occupied recorded target paths")
	command.AddCommand(
		&cobra.Command{
			Use:     "mode <Mode>",
			Short:   "Apply one Mode",
			Args:    usageArgs(cobra.ExactArgs(1)),
			Example: "  cfgfc apply mode Max -p OpenCode",
			RunE: func(command *cobra.Command, args []string) error {
				return runApplyMode(context, scope.project, args[0], forceTargets)
			},
		},
		&cobra.Command{
			Use:     "column <Column> <Setting>...",
			Short:   "Apply one Column with explicit Settings",
			Args:    usageArgs(cobra.MinimumNArgs(2)),
			Example: "  cfgfc apply column Skills Skill-A Skill-B -p OpenCode",
			RunE: func(command *cobra.Command, args []string) error {
				return runApplyColumn(context, scope.project, args[0], args[1:], forceTargets)
			},
		},
	)
	return command
}

// runApplyMode applies one Mode as the new Current state through the shared workflow.
func runApplyMode(context *commandContext, explicitProject string, modeReference string, forceTargets bool) error {
	project, err := resolveProjectForCommand(context.dependencies, explicitProject)
	if err != nil {
		return err
	}
	rootPath, err := effectiveWarehouseRoot(context.dependencies)
	if err != nil {
		return err
	}
	if err := workflow.ApplyMode(repository.New(rootPath), project.Name, modeReference, forceTargets, planOptions(context.dependencies)); err != nil {
		return classifyWorkflowError(err)
	}
	return context.renderResult(HumanResult{
		Message: fmt.Sprintf("Applied mode %q for project %q", displayStatusName(displayNameOfMode(project, modeReference), modeReference), displayStatusName(project.Metadata.DisplayName, project.Name)),
		Data:    map[string]any{"project": project.Name, "kind": "mode", "mode": modeReference},
	})
}

// runApplyColumn applies one explicit Column cover selection as the new Current state.
func runApplyColumn(context *commandContext, explicitProject string, columnReference string, settingReferences []string, forceTargets bool) error {
	project, err := resolveProjectForCommand(context.dependencies, explicitProject)
	if err != nil {
		return err
	}
	rootPath, err := effectiveWarehouseRoot(context.dependencies)
	if err != nil {
		return err
	}
	column, err := project.ResolveColumn(columnReference)
	if err != nil {
		return NewResourceError("column_not_found", err.Error(), nil, err)
	}
	if err := workflow.ApplyColumn(repository.New(rootPath), project.Name, column.Name, settingReferences, forceTargets, planOptions(context.dependencies)); err != nil {
		return classifyWorkflowError(err)
	}
	return context.renderResult(HumanResult{
		Message: fmt.Sprintf("Applied column %q for project %q", displayStatusName(column.Metadata.DisplayName, column.Name), displayStatusName(project.Metadata.DisplayName, project.Name)),
		Data:    map[string]any{"project": project.Name, "kind": "column", "column": column.Name, "settings": settingReferences},
	})
}

// displayNameOfMode resolves a Mode display label when possible.
func displayNameOfMode(project warehouse.Project, reference string) string {
	if mode, err := project.ResolveMode(reference); err == nil {
		return mode.Metadata.DisplayName
	}
	return reference
}
