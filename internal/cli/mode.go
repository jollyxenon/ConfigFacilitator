package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/index"
	"github.com/xenon/ConfigFacilitator/internal/mutate"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// modeView is the stable inspection shape for one Mode.
type modeView struct {
	Name        string                    `json:"name"`
	DisplayName string                    `json:"displayName"`
	Description string                    `json:"description"`
	Aliases     []string                  `json:"aliases"`
	Missing     bool                      `json:"missing"`
	Columns     map[string]modeColumnView `json:"columns"`
}

// modeColumnView is the stable inspection shape for one Mode Column selection.
type modeColumnView struct {
	Strategy string   `json:"strategy"`
	Settings []string `json:"settings"`
}

// newModeCommand constructs the Mode command family with shared Project scope.
func newModeCommand(context *commandContext) *cobra.Command {
	var scope projectScope
	command := &cobra.Command{Use: "mode", Short: "Manage Modes", Args: usageArgs(cobra.NoArgs)}
	addProjectFlag(command, &scope)
	command.AddCommand(
		newModeListCommand(context, &scope),
		newModeShowCommand(context, &scope),
		newModeCreateCommand(context, &scope),
		newModeSetCommand(context, &scope),
		newModeRenameCommand(context, &scope),
		newModeDeleteCommand(context, &scope),
		newModeColumnCommand(context, &scope),
	)
	return command
}

// newModeListCommand constructs context-aware Mode listing.
func newModeListCommand(context *commandContext, scope *projectScope) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Modes",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(command *cobra.Command, args []string) error {
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			views := make([]modeView, 0, len(project.Modes))
			names := make([]string, 0, len(project.Modes))
			for _, mode := range sortedModeResources(project.Modes) {
				views = append(views, viewMode(mode))
				names = append(names, displayLabel(mode.Metadata.DisplayName, mode.Name))
			}
			return context.renderResult(HumanResult{Message: namesMessage(names), Data: map[string]any{"project": project.Name, "modes": views}})
		},
	}
}

// newModeShowCommand constructs canonical-or-alias Mode inspection.
func newModeShowCommand(context *commandContext, scope *projectScope) *cobra.Command {
	return &cobra.Command{
		Use:   "show <Mode>",
		Short: "Show one Mode",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(command *cobra.Command, args []string) error {
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			mode, err := resolveModeForInspection(project, args[0])
			if err != nil {
				return err
			}
			view := viewMode(mode)
			return context.renderResult(HumanResult{
				Message: fmt.Sprintf("Mode: %s\nDescription: %s\nAliases: %s\nMissing: %t\nColumns: %d", displayLabel(view.DisplayName, view.Name), view.Description, stringsOrNone(view.Aliases), view.Missing, len(view.Columns)),
				Data:    map[string]any{"project": project.Name, "mode": view},
			})
		},
	}
}

// newModeCreateCommand constructs selection-free transactional Mode creation.
func newModeCreateCommand(context *commandContext, scope *projectScope) *cobra.Command {
	var flags metadataFlags
	command := &cobra.Command{
		Use:   "create <Mode>",
		Short: "Create one empty Mode",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(command *cobra.Command, args []string) error {
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			metadata, err := createMetadata(context, command, mutate.ModeKind, args[0], flags)
			if err != nil {
				return err
			}
			rootPath, err := effectiveWarehouseRoot(context.dependencies)
			if err != nil {
				return err
			}
			if err := mutate.CreateMode(repository.New(rootPath), project.Name, args[0], metadata); err != nil {
				return classifyMutateError(err)
			}
			return context.renderResult(HumanResult{Message: fmt.Sprintf("Created mode %q", args[0]), Data: map[string]any{"project": project.Name, "mode": args[0], "columns": map[string]any{}}})
		},
	}
	addMetadataFlags(command, &flags)
	return command
}

// newModeSetCommand constructs transactional Mode metadata replacement.
func newModeSetCommand(context *commandContext, scope *projectScope) *cobra.Command {
	var flags metadataFlags
	command := &cobra.Command{
		Use:   "set <Mode>",
		Short: "Change Mode metadata",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(command *cobra.Command, args []string) error {
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			patch, err := setMetadataPatch(context, command, mutate.ModeKind, flags)
			if err != nil {
				return err
			}
			rootPath, err := effectiveWarehouseRoot(context.dependencies)
			if err != nil {
				return err
			}
			canonicalName, err := mutate.SetMode(repository.New(rootPath), project.Name, args[0], patch)
			if err != nil {
				return classifyMutateError(err)
			}
			return context.renderResult(HumanResult{Message: fmt.Sprintf("Updated mode %q", canonicalName), Data: map[string]any{"project": project.Name, "mode": canonicalName}})
		},
	}
	addMetadataFlags(command, &flags)
	return command
}

// viewMode builds one normalized Mode inspection record without exposing extension internals.
func viewMode(mode warehouse.Mode) modeView {
	columns := map[string]modeColumnView{}
	columnNames := make([]string, 0, len(mode.Metadata.Columns))
	for columnName := range mode.Metadata.Columns {
		columnNames = append(columnNames, columnName)
	}
	sort.Strings(columnNames)
	for _, columnName := range columnNames {
		selection := mode.Metadata.Columns[columnName]
		columns[columnName] = viewModeSelection(selection)
	}
	return modeView{Name: mode.Name, DisplayName: mode.Metadata.DisplayName, Description: mode.Metadata.Description, Aliases: aliasesOrEmpty(mode.Metadata.Aliases), Missing: mode.Missing, Columns: columns}
}

// viewModeSelection converts one persisted selection to stable output arrays.
func viewModeSelection(selection index.ModeColumnSelection) modeColumnView {
	return modeColumnView{Strategy: selection.Strategy, Settings: stringsOrEmpty(selection.Settings)}
}

// newModeColumnCommand constructs Mode Column-selection routes.
func newModeColumnCommand(context *commandContext, scope *projectScope) *cobra.Command {
	command := &cobra.Command{Use: "column", Short: "Manage Mode Column selections", Args: usageArgs(cobra.NoArgs)}
	command.AddCommand(
		newModeColumnListCommand(context, scope),
		newModeColumnSetCommand(context, scope),
		newModeColumnDeleteCommand(context, scope),
	)
	return command
}

// newModeColumnListCommand lists canonical persisted selections for one Mode.
func newModeColumnListCommand(context *commandContext, scope *projectScope) *cobra.Command {
	return &cobra.Command{Use: "list <Mode>", Short: "List Mode Column selections", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(command *cobra.Command, args []string) error {
		project, err := resolveProjectForCommand(context.dependencies, scope.project)
		if err != nil {
			return err
		}
		mode, err := resolveModeForInspection(project, args[0])
		if err != nil {
			return err
		}
		columns := viewMode(mode).Columns
		return context.renderResult(HumanResult{Message: fmt.Sprintf("%d Mode Column selections", len(columns)), Data: map[string]any{"project": project.Name, "mode": mode.Name, "columns": columns}})
	}}
}

// newModeColumnSetCommand validates strategy flags and transactionally persists canonical references.
func newModeColumnSetCommand(context *commandContext, scope *projectScope) *cobra.Command {
	var strategy string
	var settings []string
	command := &cobra.Command{Use: "set <Mode> <Column>", Short: "Set a Mode Column selection", Args: usageArgs(cobra.ExactArgs(2)), RunE: func(command *cobra.Command, args []string) error {
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
		mode, column, canonicalSettings, err := mutate.SetModeColumnSelection(repository.New(rootPath), project.Name, args[0], args[1], strategy, settings)
		if err != nil {
			return classifyMutateError(err)
		}
		selection := modeColumnView{Strategy: strategy, Settings: stringsOrEmpty(canonicalSettings)}
		return context.renderResult(HumanResult{Message: fmt.Sprintf("Set column %q in mode %q", column, mode), Data: map[string]any{"project": project.Name, "mode": mode, "column": column, "selection": selection}})
	}}
	command.Flags().StringVar(&strategy, "strategy", "", "Selection strategy: cover, increment, none, or full")
	command.Flags().StringArrayVar(&settings, "setting", nil, "Select one Setting; repeat for multiple Settings")
	return command
}

// newModeColumnDeleteCommand transactionally removes one selection without resource deletion confirmation.
func newModeColumnDeleteCommand(context *commandContext, scope *projectScope) *cobra.Command {
	return &cobra.Command{Use: "delete <Mode> <Column>", Short: "Delete a Mode Column selection", Args: usageArgs(cobra.ExactArgs(2)), RunE: func(command *cobra.Command, args []string) error {
		project, err := resolveProjectForCommand(context.dependencies, scope.project)
		if err != nil {
			return err
		}
		rootPath, err := effectiveWarehouseRoot(context.dependencies)
		if err != nil {
			return err
		}
		mode, column, err := mutate.DeleteModeColumnSelection(repository.New(rootPath), project.Name, args[0], args[1])
		if err != nil {
			return classifyMutateError(err)
		}
		return context.renderResult(HumanResult{Message: fmt.Sprintf("Deleted column %q from mode %q", column, mode), Data: map[string]any{"project": project.Name, "mode": mode, "column": column}})
	}}
}

// newModeRenameCommand constructs transactional canonical Mode rename.
func newModeRenameCommand(context *commandContext, scope *projectScope) *cobra.Command {
	var forceTargets bool
	command := &cobra.Command{Use: "rename <Old> <New>", Short: "Rename one Mode", Args: usageArgs(cobra.ExactArgs(2)), RunE: func(command *cobra.Command, args []string) error {
		project, err := resolveProjectForCommand(context.dependencies, scope.project)
		if err != nil {
			return err
		}
		rootPath, err := effectiveWarehouseRoot(context.dependencies)
		if err != nil {
			return err
		}
		if err := mutate.RenameMode(repository.New(rootPath), project.Name, args[0], args[1], forceTargets, planOptions(context.dependencies)); err != nil {
			return classifyMutateError(err)
		}
		return context.renderResult(HumanResult{Message: fmt.Sprintf("Renamed mode %q to %q", args[0], args[1]), Data: map[string]any{"project": project.Name, "mode": args[1], "previousName": args[0]}})
	}}
	command.Flags().BoolVar(&forceTargets, "force-targets", false, "Reclaim affected recorded target paths")
	return command
}

// newModeDeleteCommand constructs confirmed transactional Mode deletion.
func newModeDeleteCommand(context *commandContext, scope *projectScope) *cobra.Command {
	var yes, cascade, forceTargets bool
	command := &cobra.Command{Use: "delete <Mode>", Short: "Delete one Mode", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(command *cobra.Command, args []string) error {
		project, err := resolveProjectForCommand(context.dependencies, scope.project)
		if err != nil { return err }
		rootPath, err := effectiveWarehouseRoot(context.dependencies)
		if err != nil { return err }
		report, err := mutate.DeleteMode(repository.New(rootPath), project.Name, args[0], yes, cascade, forceTargets)
		if err != nil { return classifyMutateError(err) }
		return context.renderResult(HumanResult{Message: fmt.Sprintf("Deleted mode %q", report.Name), Data: map[string]any{"project": project.Name, "mode": report.Name, "dependencies": report}})
	}}
	addDeleteFlags(command, &yes, &cascade, &forceTargets)
	return command
}
