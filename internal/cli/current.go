package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/workflow"
)

// currentView is the stable inspection shape for one Project Current state.
type currentView struct {
	Project  string                                `json:"project"`
	Columns  map[string]repository.ColumnSelection `json:"columns"`
	Relation *repository.CurrentRelation           `json:"relation,omitempty"`
	Mappings []repository.Mapping                  `json:"mappings"`
}

// newCurrentCommand constructs the Current state command family with shared Project scope.
func newCurrentCommand(context *commandContext) *cobra.Command {
	var scope projectScope
	command := &cobra.Command{Use: "current", Short: "Manage the Current (temporary Mode) state", Args: usageArgs(cobra.NoArgs)}
	addProjectFlag(command, &scope)
	command.AddCommand(
		newCurrentShowCommand(context, &scope),
		newCurrentColumnCommand(context, &scope),
	)
	return command
}

// newCurrentShowCommand constructs Current state inspection.
func newCurrentShowCommand(context *commandContext, scope *projectScope) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the Current state",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(command *cobra.Command, args []string) error {
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			rootPath, err := effectiveWarehouseRoot(context.dependencies)
			if err != nil {
				return err
			}
			state, err := repository.New(rootPath).LoadCurrentState(project.Name)
			if err != nil {
				return NewInvalidDataError("current_state", err.Error(), nil, err)
			}
			view := currentView{Project: project.Name, Columns: state.Columns, Relation: state.Relation, Mappings: state.Mappings}
			lines := []string{fmt.Sprintf("Project: %s", project.Name)}
			if state.Relation != nil {
				lines = append(lines, fmt.Sprintf("Relation: %s (origin %s)", state.Relation.Kind, state.Relation.OriginMode))
			} else {
				lines = append(lines, "Relation: none")
			}
			lines = append(lines, fmt.Sprintf("Columns: %d", len(state.Columns)))
			for _, columnName := range sortedColumnNames(state.Columns) {
				selection := state.Columns[columnName]
				lines = append(lines, fmt.Sprintf("  - %s %s [%s]", columnName, selection.Strategy, strings.Join(selection.Settings, ", ")))
			}
			lines = append(lines, fmt.Sprintf("Mappings: %d", len(state.Mappings)))
			for _, mapping := range state.Mappings {
				lines = append(lines, fmt.Sprintf("  - %s -> %s", mapping.Source, mapping.Target))
			}
			return context.renderResult(HumanResult{Message: strings.Join(lines, "\n"), Data: map[string]any{"project": view.Project, "current": view}})
		},
	}
}

// sortedColumnNames returns deterministic Current Column names.
func sortedColumnNames(columns map[string]repository.ColumnSelection) []string {
	names := make([]string, 0, len(columns))
	for name := range columns {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// newCurrentColumnCommand constructs Current Column selection routes.
func newCurrentColumnCommand(context *commandContext, scope *projectScope) *cobra.Command {
	command := &cobra.Command{Use: "column", Short: "Manage Current Column selections", Args: usageArgs(cobra.NoArgs)}
	command.AddCommand(
		newCurrentColumnListCommand(context, scope),
		newCurrentColumnSetCommand(context, scope),
		newCurrentColumnDeleteCommand(context, scope),
	)
	return command
}

// newCurrentColumnListCommand lists persisted Current Column selections.
func newCurrentColumnListCommand(context *commandContext, scope *projectScope) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Current Column selections",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(command *cobra.Command, args []string) error {
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			rootPath, err := effectiveWarehouseRoot(context.dependencies)
			if err != nil {
				return err
			}
			state, err := repository.New(rootPath).LoadCurrentState(project.Name)
			if err != nil {
				return NewInvalidDataError("current_state", err.Error(), nil, err)
			}
			lines := make([]string, 0, len(state.Columns))
			for _, columnName := range sortedColumnNames(state.Columns) {
				selection := state.Columns[columnName]
				lines = append(lines, fmt.Sprintf("%s %s [%s]", columnName, selection.Strategy, strings.Join(selection.Settings, ", ")))
			}
			return context.renderResult(HumanResult{
				Message: strings.Join(lines, "\n"),
				Data:    map[string]any{"project": project.Name, "columns": state.Columns, "relation": state.Relation},
			})
		},
	}
}

// newCurrentColumnSetCommand atomically replaces one Current Column selection.
func newCurrentColumnSetCommand(context *commandContext, scope *projectScope) *cobra.Command {
	var strategy string
	var settings []string
	var forceTargets bool
	command := &cobra.Command{
		Use:   "set <Column>",
		Short: "Set a Current Column selection",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(command *cobra.Command, args []string) error {
			if !command.Flags().Changed("strategy") {
				return NewUsageError("mode_strategy_required", "provide --strategy cover, increment, none, or full", nil)
			}
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			rootPath, err := effectiveWarehouseRoot(context.dependencies)
			if err != nil {
				return err
			}
			if err := workflow.SetCurrentColumn(repository.New(rootPath), project.Name, args[0], strategy, settings, forceTargets, planOptions(context.dependencies)); err != nil {
				return classifyWorkflowError(err)
			}
			return context.renderResult(HumanResult{
				Message: fmt.Sprintf("Set column %q in Current for project %q", args[0], project.Name),
				Data:    map[string]any{"project": project.Name, "column": args[0], "strategy": strategy, "settings": settings},
			})
		},
	}
	command.Flags().StringVar(&strategy, "strategy", "", "Selection strategy: cover, increment, none, or full")
	command.Flags().StringArrayVar(&settings, "setting", nil, "Select one Setting; repeat for multiple Settings")
	command.Flags().BoolVar(&forceTargets, "force-targets", false, "Reclaim occupied recorded target paths")
	return command
}

// newCurrentColumnDeleteCommand atomically removes one Current Column selection.
func newCurrentColumnDeleteCommand(context *commandContext, scope *projectScope) *cobra.Command {
	var forceTargets bool
	command := &cobra.Command{
		Use:   "delete <Column>",
		Short: "Delete a Current Column selection",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(command *cobra.Command, args []string) error {
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			rootPath, err := effectiveWarehouseRoot(context.dependencies)
			if err != nil {
				return err
			}
			if err := workflow.DeleteCurrentColumn(repository.New(rootPath), project.Name, args[0], forceTargets, planOptions(context.dependencies)); err != nil {
				return classifyWorkflowError(err)
			}
			return context.renderResult(HumanResult{
				Message: fmt.Sprintf("Deleted column %q from Current for project %q", args[0], project.Name),
				Data:    map[string]any{"project": project.Name, "column": args[0]},
			})
		},
	}
	command.Flags().BoolVar(&forceTargets, "force-targets", false, "Reclaim occupied recorded target paths")
	return command
}
