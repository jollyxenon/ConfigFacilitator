package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/mutate"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// columnView is the stable inspection shape for one Column.
type columnView struct {
	Name              string   `json:"name"`
	DisplayName       string   `json:"displayName"`
	Description       string   `json:"description"`
	Aliases           []string `json:"aliases"`
	Missing           bool     `json:"missing"`
	TargetNumber      int      `json:"targetNumber"`
	DefaultTargetDir  []string `json:"defaultTargetDir"`
	DefaultTargetName []string `json:"defaultTargetName"`
	SettingCount      int      `json:"settingCount"`
}

// newColumnCommand constructs the Column command family with shared Project scope.
func newColumnCommand(context *commandContext) *cobra.Command {
	var scope projectScope
	command := &cobra.Command{Use: "column", Short: "Manage Columns", Args: usageArgs(cobra.NoArgs)}
	addProjectFlag(command, &scope)
	command.AddCommand(
		newColumnListCommand(context, &scope),
		newColumnShowCommand(context, &scope),
		newColumnCreateCommand(context, &scope),
		newColumnSetCommand(context, &scope),
		newColumnRenameCommand(context, &scope),
		newColumnDeleteCommand(context, &scope),
		newColumnTargetCommand(context, &scope),
	)
	return command
}

// newColumnListCommand constructs context-aware Column listing.
func newColumnListCommand(context *commandContext, scope *projectScope) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Columns",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(command *cobra.Command, args []string) error {
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			views := make([]columnView, 0, len(project.Columns))
			names := make([]string, 0, len(project.Columns))
			for _, column := range sortedColumnResources(project.Columns) {
				views = append(views, viewColumn(column))
				names = append(names, displayLabel(column.Metadata.DisplayName, column.Name))
			}
			return context.renderResult(HumanResult{Message: namesMessage(names), Data: map[string]any{"project": project.Name, "columns": views}})
		},
	}
}

// newColumnShowCommand constructs context-aware canonical-or-alias Column inspection.
func newColumnShowCommand(context *commandContext, scope *projectScope) *cobra.Command {
	return &cobra.Command{
		Use:   "show <Column>",
		Short: "Show one Column",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(command *cobra.Command, args []string) error {
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			column, err := resolveColumnForInspection(project, args[0])
			if err != nil {
				return err
			}
			view := viewColumn(column)
			return context.renderResult(HumanResult{
				Message: fmt.Sprintf("Column: %s\nDescription: %s\nAliases: %s\nMissing: %t\nTargets: %d\nSettings: %d", displayLabel(view.DisplayName, view.Name), view.Description, stringsOrNone(view.Aliases), view.Missing, view.TargetNumber, view.SettingCount),
				Data:    map[string]any{"project": project.Name, "column": view},
			})
		},
	}
}

// newColumnCreateCommand constructs complete transactional zero-target Column creation.
func newColumnCreateCommand(context *commandContext, scope *projectScope) *cobra.Command {
	var flags metadataFlags
	command := &cobra.Command{
		Use:   "create <Column>",
		Short: "Create one zero-target Column",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(command *cobra.Command, args []string) error {
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			metadata, err := createMetadata(context, command, mutate.ColumnKind, args[0], flags)
			if err != nil {
				return err
			}
			rootPath, err := effectiveWarehouseRoot(context.dependencies)
			if err != nil {
				return err
			}
			if err := mutate.CreateColumn(repository.New(rootPath), project.Name, args[0], metadata); err != nil {
				return classifyMutateError(err)
			}
			return context.renderResult(HumanResult{Message: fmt.Sprintf("Created column %q in project %q", args[0], project.Name), Data: map[string]any{"project": project.Name, "column": args[0], "targetNumber": 0}})
		},
	}
	addMetadataFlags(command, &flags)
	return command
}

// newColumnSetCommand constructs transactional Column metadata replacement.
func newColumnSetCommand(context *commandContext, scope *projectScope) *cobra.Command {
	var flags metadataFlags
	command := &cobra.Command{
		Use:   "set <Column>",
		Short: "Change Column metadata",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(command *cobra.Command, args []string) error {
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			patch, err := setMetadataPatch(context, command, mutate.ColumnKind, flags)
			if err != nil {
				return err
			}
			rootPath, err := effectiveWarehouseRoot(context.dependencies)
			if err != nil {
				return err
			}
			canonicalName, err := mutate.SetColumn(repository.New(rootPath), project.Name, args[0], patch)
			if err != nil {
				return classifyMutateError(err)
			}
			return context.renderResult(HumanResult{Message: fmt.Sprintf("Updated column %q", canonicalName), Data: map[string]any{"project": project.Name, "column": canonicalName}})
		},
	}
	addMetadataFlags(command, &flags)
	return command
}

// viewColumn builds one normalized Column inspection record.
func viewColumn(column warehouse.Column) columnView {
	return columnView{
		Name:              column.Name,
		DisplayName:       column.Metadata.DisplayName,
		Description:       column.Metadata.Description,
		Aliases:           aliasesOrEmpty(column.Metadata.Aliases),
		Missing:           column.Missing,
		TargetNumber:      column.SettingIndex.TargetNumber,
		DefaultTargetDir:  stringsOrEmpty(column.SettingIndex.DefaultTargetDir),
		DefaultTargetName: stringsOrEmpty(column.SettingIndex.DefaultTargetName),
		SettingCount:      len(column.Settings),
	}
}

// stringsOrEmpty stabilizes target arrays in JSON output.
func stringsOrEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// newColumnTargetCommand constructs structural Column target routes.
func newColumnTargetCommand(context *commandContext, scope *projectScope) *cobra.Command {
	command := &cobra.Command{Use: "target", Short: "Manage Column target positions", Args: usageArgs(cobra.NoArgs)}
	command.AddCommand(newColumnTargetListCommand(context, scope), newColumnTargetAddCommand(context, scope), newColumnTargetSetCommand(context, scope), newColumnTargetDeleteCommand(context, scope))
	return command
}

// newColumnTargetListCommand lists zero-based logical target positions.
func newColumnTargetListCommand(context *commandContext, scope *projectScope) *cobra.Command {
	return &cobra.Command{Use: "list <Column>", Short: "List target positions", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(command *cobra.Command, args []string) error {
		project, err := resolveProjectForCommand(context.dependencies, scope.project)
		if err != nil {
			return err
		}
		column, err := resolveColumnForInspection(project, args[0])
		if err != nil {
			return err
		}
		positions, err := mutate.ColumnTargetPositions(column.SettingIndex)
		if err != nil {
			return NewInvalidDataError("target_arrays", err.Error(), nil, err)
		}
		return context.renderResult(HumanResult{Message: fmt.Sprintf("%d target positions", len(positions)), Data: map[string]any{"project": project.Name, "column": column.Name, "targets": positions}})
	}}
}

// newColumnTargetAddCommand appends one Column target position.
func newColumnTargetAddCommand(context *commandContext, scope *projectScope) *cobra.Command {
	var dir, name string
	var nameFromSetting bool
	command := &cobra.Command{Use: "add <Column>", Short: "Add a target position", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(command *cobra.Command, args []string) error {
		if !command.Flags().Changed("dir") {
			return NewUsageError("target_dir_required", "provide --dir", nil)
		}
		if command.Flags().Changed("name") == nameFromSetting {
			return NewUsageError("target_name_required", "provide exactly one of --name or --name-from-setting", nil)
		}
		project, err := resolveProjectForCommand(context.dependencies, scope.project)
		if err != nil {
			return err
		}
		root, err := effectiveWarehouseRoot(context.dependencies)
		if err != nil {
			return err
		}
		nameMode := "fixed"
		if nameFromSetting {
			nameMode = "setting"
		}
		column, position, err := mutate.AddColumnTarget(repository.New(root), project.Name, args[0], mutate.TargetPosition{Dir: dir, Name: name, DirMode: "fixed", NameMode: nameMode}, planOptions(context.dependencies))
		if err != nil {
			return classifyMutateError(err)
		}
		return context.renderResult(HumanResult{Message: fmt.Sprintf("Added target %d", position), Data: map[string]any{"project": project.Name, "column": column, "index": position}})
	}}
	command.Flags().StringVar(&dir, "dir", "", "Target directory")
	command.Flags().StringVar(&name, "name", "", "Fixed target name")
	command.Flags().BoolVar(&nameFromSetting, "name-from-setting", false, "Derive target name from Setting")
	return command
}

// newColumnTargetSetCommand changes only supplied target components.
func newColumnTargetSetCommand(context *commandContext, scope *projectScope) *cobra.Command {
	var dir, name string
	var clearDir, nameFromSetting bool
	command := &cobra.Command{Use: "set <Column> <Index>", Short: "Change a target position", Args: usageArgs(cobra.ExactArgs(2)), RunE: func(command *cobra.Command, args []string) error {
		position, err := mutate.ParseTargetIndex(args[1])
		if err != nil {
			return classifyMutateError(err)
		}
		var dirValue, nameValue *string
		if command.Flags().Changed("dir") {
			dirValue = &dir
		}
		if command.Flags().Changed("name") {
			nameValue = &name
		}
		project, err := resolveProjectForCommand(context.dependencies, scope.project)
		if err != nil {
			return err
		}
		root, err := effectiveWarehouseRoot(context.dependencies)
		if err != nil {
			return err
		}
		column, err := mutate.SetColumnTarget(repository.New(root), project.Name, args[0], position, dirValue, clearDir, nameValue, nameFromSetting, planOptions(context.dependencies))
		if err != nil {
			return classifyMutateError(err)
		}
		return context.renderResult(HumanResult{Message: fmt.Sprintf("Updated target %d", position), Data: map[string]any{"project": project.Name, "column": column, "index": position}})
	}}
	command.Flags().StringVar(&dir, "dir", "", "Target directory")
	command.Flags().StringVar(&name, "name", "", "Fixed target name")
	command.Flags().BoolVar(&clearDir, "clear-dir", false, "Clear target directory")
	command.Flags().BoolVar(&nameFromSetting, "name-from-setting", false, "Derive target name from Setting")
	return command
}

// newColumnTargetDeleteCommand deletes one confirmed target position.
func newColumnTargetDeleteCommand(context *commandContext, scope *projectScope) *cobra.Command {
	var yes bool
	command := &cobra.Command{Use: "delete <Column> <Index>", Short: "Delete a target position", Args: usageArgs(cobra.ExactArgs(2)), RunE: func(command *cobra.Command, args []string) error {
		position, err := mutate.ParseTargetIndex(args[1])
		if err != nil {
			return classifyMutateError(err)
		}
		project, err := resolveProjectForCommand(context.dependencies, scope.project)
		if err != nil {
			return err
		}
		root, err := effectiveWarehouseRoot(context.dependencies)
		if err != nil {
			return err
		}
		if err := mutate.DeleteColumnTarget(repository.New(root), project.Name, args[0], position, yes, planOptions(context.dependencies)); err != nil {
			return classifyMutateError(err)
		}
		return context.renderResult(HumanResult{Message: fmt.Sprintf("Deleted target %d", position), Data: map[string]any{"project": project.Name, "column": args[0], "index": position}})
	}}
	command.Flags().BoolVar(&yes, "yes", false, "Confirm deletion")
	return command
}

// newColumnRenameCommand constructs transactional canonical Column rename.
func newColumnRenameCommand(context *commandContext, scope *projectScope) *cobra.Command {
	var forceTargets bool
	command := &cobra.Command{Use: "rename <Old> <New>", Short: "Rename one Column", Args: usageArgs(cobra.ExactArgs(2)), RunE: func(command *cobra.Command, args []string) error {
		project, err := resolveProjectForCommand(context.dependencies, scope.project)
		if err != nil {
			return err
		}
		rootPath, err := effectiveWarehouseRoot(context.dependencies)
		if err != nil {
			return err
		}
		if err := mutate.RenameColumn(repository.New(rootPath), project.Name, args[0], args[1], forceTargets, planOptions(context.dependencies)); err != nil {
			return classifyMutateError(err)
		}
		return context.renderResult(HumanResult{Message: fmt.Sprintf("Renamed column %q to %q", args[0], args[1]), Data: map[string]any{"project": project.Name, "column": args[1], "previousName": args[0]}})
	}}
	command.Flags().BoolVar(&forceTargets, "force-targets", false, "Reclaim affected recorded target paths")
	return command
}

// newColumnDeleteCommand constructs confirmed transactional Column deletion.
func newColumnDeleteCommand(context *commandContext, scope *projectScope) *cobra.Command {
	var yes, cascade, forceTargets bool
	command := &cobra.Command{Use: "delete <Column>", Short: "Delete one Column", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(command *cobra.Command, args []string) error {
		project, err := resolveProjectForCommand(context.dependencies, scope.project)
		if err != nil {
			return err
		}
		rootPath, err := effectiveWarehouseRoot(context.dependencies)
		if err != nil {
			return err
		}
		report, err := mutate.DeleteColumn(repository.New(rootPath), project.Name, args[0], yes, cascade, forceTargets)
		if err != nil {
			return classifyMutateError(err)
		}
		return context.renderResult(HumanResult{Message: fmt.Sprintf("Deleted column %q", report.Name), Data: map[string]any{"project": project.Name, "column": report.Name, "dependencies": report}})
	}}
	addDeleteFlags(command, &yes, &cascade, &forceTargets)
	return command
}
