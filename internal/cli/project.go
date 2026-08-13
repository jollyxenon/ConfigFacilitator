package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/mutate"
	"github.com/xenon/ConfigFacilitator/internal/repository"
)

// projectView is the stable inspection shape for one Project.
type projectView struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Aliases     []string `json:"aliases"`
	Missing     bool     `json:"missing"`
}

// newProjectCommand constructs the Project command family.
func newProjectCommand(context *commandContext) *cobra.Command {
	command := &cobra.Command{
		Use:   "project",
		Short: "Manage Projects",
		Args:  usageArgs(cobra.NoArgs),
	}
	command.AddCommand(
		newProjectListCommand(context),
		newProjectShowCommand(context),
		newProjectCreateCommand(context),
		newProjectSetCommand(context),
		newProjectRenameCommand(context),
		newProjectDeleteCommand(context),
	)
	return command
}

// newProjectListCommand constructs Project listing.
func newProjectListCommand(context *commandContext) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Projects",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(command *cobra.Command, args []string) error {
			loaded, _, err := loadWarehouseForInspection(context.dependencies)
			if err != nil {
				return err
			}
			views := make([]projectView, 0, len(loaded.Projects))
			names := make([]string, 0, len(loaded.Projects))
			for _, project := range sortedProjectResources(loaded.Projects) {
				views = append(views, viewProject(project.Name, project.Metadata.DisplayName, project.Metadata.Description, project.Metadata.Aliases, project.Missing))
				names = append(names, displayLabel(project.Metadata.DisplayName, project.Name))
			}
			return context.renderResult(HumanResult{Message: namesMessage(names), Data: map[string]any{"projects": views}})
		},
	}
}

// newProjectShowCommand constructs canonical-or-alias Project inspection.
func newProjectShowCommand(context *commandContext) *cobra.Command {
	return &cobra.Command{
		Use:   "show <Project>",
		Short: "Show one Project",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(command *cobra.Command, args []string) error {
			loaded, _, err := loadWarehouseForInspection(context.dependencies)
			if err != nil {
				return err
			}
			project, err := loaded.ResolveProject(args[0])
			if err != nil {
				return NewResourceError("project_not_found", err.Error(), nil, err)
			}
			view := viewProject(project.Name, project.Metadata.DisplayName, project.Metadata.Description, project.Metadata.Aliases, project.Missing)
			return context.renderResult(HumanResult{
				Message: fmt.Sprintf("Project: %s\nDescription: %s\nAliases: %s\nMissing: %t", displayLabel(view.DisplayName, view.Name), view.Description, stringsOrNone(view.Aliases), view.Missing),
				Data:    map[string]any{"project": view},
			})
		},
	}
}

// newProjectCreateCommand constructs complete transactional Project creation.
func newProjectCreateCommand(context *commandContext) *cobra.Command {
	var flags metadataFlags
	command := &cobra.Command{
		Use:   "create <Project>",
		Short: "Create one complete Project",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(command *cobra.Command, args []string) error {
			metadata, err := createMetadata(context, command, mutate.ProjectKind, args[0], flags)
			if err != nil {
				return err
			}
			rootPath, err := effectiveWarehouseRoot(context.dependencies)
			if err != nil {
				return err
			}
			if err := mutate.CreateProject(repository.New(rootPath), args[0], metadata); err != nil {
				return classifyMutateError(err)
			}
			return context.renderResult(HumanResult{Message: fmt.Sprintf("Created project %q", args[0]), Data: map[string]any{"project": viewProject(args[0], metadata.DisplayName, metadata.Description, metadata.Aliases, false)}})
		},
	}
	addMetadataFlags(command, &flags)
	return command
}

// newProjectSetCommand constructs transactional Project metadata replacement.
func newProjectSetCommand(context *commandContext) *cobra.Command {
	var flags metadataFlags
	command := &cobra.Command{
		Use:   "set <Project>",
		Short: "Change Project metadata",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(command *cobra.Command, args []string) error {
			patch, err := setMetadataPatch(context, command, mutate.ProjectKind, flags)
			if err != nil {
				return err
			}
			rootPath, err := effectiveWarehouseRoot(context.dependencies)
			if err != nil {
				return err
			}
			canonicalName, err := mutate.SetProject(repository.New(rootPath), args[0], patch)
			if err != nil {
				return classifyMutateError(err)
			}
			return context.renderResult(HumanResult{Message: fmt.Sprintf("Updated project %q", canonicalName), Data: map[string]any{"project": canonicalName}})
		},
	}
	addMetadataFlags(command, &flags)
	return command
}

// viewProject builds one normalized Project inspection record.
func viewProject(name string, displayName string, description string, aliases []string, missing bool) projectView {
	return projectView{Name: name, DisplayName: displayName, Description: description, Aliases: aliasesOrEmpty(aliases), Missing: missing}
}

// stringsOrNone renders aliases concisely for human inspection.
func stringsOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return fmt.Sprintf("%v", values)
}

// aliasesOrEmpty stabilizes JSON arrays for resource output.
func aliasesOrEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// newPendingCommand constructs one validated controlled-error command for a later implementation slice.
func newPendingCommand(use string, short string, validator cobra.PositionalArgs, path string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  usageArgs(validator),
		RunE:  unimplementedHandler(path),
	}
}

// newProjectRenameCommand constructs transactional canonical Project rename.
func newProjectRenameCommand(context *commandContext) *cobra.Command {
	var forceTargets bool
	command := &cobra.Command{Use: "rename <Old> <New>", Short: "Rename one Project", Args: usageArgs(cobra.ExactArgs(2)), RunE: func(command *cobra.Command, args []string) error {
		rootPath, err := effectiveWarehouseRoot(context.dependencies)
		if err != nil {
			return err
		}
		if err := mutate.RenameProject(repository.New(rootPath), args[0], args[1], forceTargets, planOptions(context.dependencies)); err != nil {
			return classifyMutateError(err)
		}
		return context.renderResult(HumanResult{Message: fmt.Sprintf("Renamed project %q to %q", args[0], args[1]), Data: map[string]any{"project": args[1], "previousName": args[0]}})
	}}
	command.Flags().BoolVar(&forceTargets, "force-targets", false, "Reclaim affected recorded target paths")
	return command
}

// newProjectDeleteCommand constructs confirmed transactional Project deletion.
func newProjectDeleteCommand(context *commandContext) *cobra.Command {
	var yes, cascade, forceTargets bool
	command := &cobra.Command{Use: "delete <Project>", Short: "Delete one Project", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(command *cobra.Command, args []string) error {
		rootPath, err := effectiveWarehouseRoot(context.dependencies)
		if err != nil {
			return err
		}
		report, err := mutate.DeleteProject(repository.New(rootPath), args[0], yes, cascade, forceTargets)
		if err != nil {
			return classifyMutateError(err)
		}
		return context.renderResult(HumanResult{Message: fmt.Sprintf("Deleted project %q", report.Name), Data: map[string]any{"project": report.Name, "dependencies": report}})
	}}
	addDeleteFlags(command, &yes, &cascade, &forceTargets)
	return command
}

// addDeleteFlags binds the independent confirmation, cascade, and target-reclamation controls.
func addDeleteFlags(command *cobra.Command, yes, cascade, forceTargets *bool) {
	command.Flags().BoolVar(yes, "yes", false, "Confirm resource deletion")
	command.Flags().BoolVar(cascade, "cascade", false, "Remove or repair dependent references")
	command.Flags().BoolVar(forceTargets, "force-targets", false, "Reclaim affected recorded target paths")
}
