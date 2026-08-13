package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// newCompletionCommand generates a completion script for one supported shell.
func newCompletionCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "completion <bash|zsh|fish|powershell>",
		Short: "Generate a shell completion script",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(command *cobra.Command, args []string) error {
			root := command.Root()
			var err error
			switch args[0] {
			case "bash":
				err = root.GenBashCompletion(command.OutOrStdout())
			case "zsh":
				err = root.GenZshCompletion(command.OutOrStdout())
			case "fish":
				err = root.GenFishCompletion(command.OutOrStdout(), true)
			case "powershell":
				err = root.GenPowerShellCompletionWithDesc(command.OutOrStdout())
			default:
				return NewUsageError("unsupported_shell", fmt.Sprintf("unsupported shell %q; choose bash, zsh, fish, or powershell", args[0]), nil)
			}
			if err != nil {
				return NewPersistenceError("completion_output", "write shell completion script", err)
			}
			return nil
		},
	}
	command.ValidArgsFunction = cobra.FixedCompletions(
		[]string{"bash", "zsh", "fish", "powershell"},
		cobra.ShellCompDirectiveNoFileComp,
	)
	return command
}

// addDynamicCompletions attaches resource and scope completion after the tree is assembled.
func addDynamicCompletions(root *cobra.Command, context *commandContext) {
	walkCommands(root, func(command *cobra.Command) {
		registerProjectFlagCompletion(command, context)
		registerColumnFlagCompletion(command, context)
		registerSettingFlagCompletion(command, context)
	})
	setPositionalCompletion(root, "project show", projectArguments(context, 1))
	setPositionalCompletion(root, "project set", projectArguments(context, 1))
	setPositionalCompletion(root, "project rename", projectArguments(context, 1))
	setPositionalCompletion(root, "project delete", projectArguments(context, 1))
	setPositionalCompletion(root, "use", useArguments(context))

	for _, path := range []string{"column show", "column set", "column rename", "column delete", "column target list", "column target add", "column target set", "column target delete"} {
		setPositionalCompletion(root, path, columnArguments(context, 1))
	}
	for _, path := range []string{"setting show", "setting set", "setting rename", "setting delete", "setting target list", "setting target set", "setting target reset", "setting content list", "setting content read", "setting content write", "setting content mkdir", "setting content move", "setting content delete"} {
		setPositionalCompletion(root, path, settingArguments(context, 1))
	}
	for _, path := range []string{"mode show", "mode set", "mode rename", "mode delete", "mode column list"} {
		setPositionalCompletion(root, path, modeArguments(context, 1))
	}
	setPositionalCompletion(root, "mode column set", modeColumnArguments(context))
	setPositionalCompletion(root, "mode column delete", modeColumnArguments(context))
	setPositionalCompletion(root, "apply mode", modeArguments(context, 1))
	setPositionalCompletion(root, "apply column", applyColumnArguments(context))
}

// setPositionalCompletion installs one completion function on a known command path.
func setPositionalCompletion(root *cobra.Command, path string, completion cobra.CompletionFunc) {
	command, _, err := root.Find(strings.Fields(path))
	if err == nil && command != nil {
		command.ValidArgsFunction = completion
	}
}

// registerProjectFlagCompletion completes inherited -p/--project values when present.
func registerProjectFlagCompletion(command *cobra.Command, context *commandContext) {
	if command.Flag("project") == nil {
		return
	}
	_ = command.RegisterFlagCompletionFunc("project", func(command *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		loaded, err := loadCompletionWarehouse(context.dependencies)
		if err != nil {
			return completionFailure()
		}
		return filterCompletions(projectCompletions(loaded), toComplete), cobra.ShellCompDirectiveNoFileComp
	})
}

// registerColumnFlagCompletion completes -c/--column in the effective Project.
func registerColumnFlagCompletion(command *cobra.Command, context *commandContext) {
	if command.Flag("column") == nil {
		return
	}
	_ = command.RegisterFlagCompletionFunc("column", func(command *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		project, err := completionProject(command, context)
		if err != nil {
			return completionFailure()
		}
		return filterCompletions(columnCompletions(project), toComplete), cobra.ShellCompDirectiveNoFileComp
	})
}

// registerSettingFlagCompletion completes repeated --setting values for a Mode Column selection.
func registerSettingFlagCompletion(command *cobra.Command, context *commandContext) {
	if command.Flag("setting") == nil {
		return
	}
	_ = command.RegisterFlagCompletionFunc("setting", func(command *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) < 2 {
			return completionFailure()
		}
		project, err := completionProject(command, context)
		if err != nil {
			return completionFailure()
		}
		column, err := project.ResolveColumn(args[1])
		if err != nil {
			return completionFailure()
		}
		return filterCompletions(settingCompletions(column), toComplete), cobra.ShellCompDirectiveNoFileComp
	})
}

// projectArguments completes one or more initial Project references.
func projectArguments(context *commandContext, positions int) cobra.CompletionFunc {
	return func(command *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) >= positions {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		loaded, err := loadCompletionWarehouse(context.dependencies)
		if err != nil {
			return completionFailure()
		}
		return filterCompletions(projectCompletions(loaded), toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

// useArguments completes Projects plus the reserved context-clearing global value.
func useArguments(context *commandContext) cobra.CompletionFunc {
	return func(command *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		loaded, err := loadCompletionWarehouse(context.dependencies)
		if err != nil {
			return completionFailure()
		}
		values := append(projectCompletions(loaded), cobra.CompletionWithDesc(globalProjectName, "clear selected Project context"))
		return filterCompletions(values, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

// columnArguments completes initial Column references in the effective Project.
func columnArguments(context *commandContext, positions int) cobra.CompletionFunc {
	return func(command *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) >= positions {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		project, err := completionProject(command, context)
		if err != nil {
			return completionFailure()
		}
		return filterCompletions(columnCompletions(project), toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

// settingArguments completes an initial Setting in the selected Column.
func settingArguments(context *commandContext, positions int) cobra.CompletionFunc {
	return func(command *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) >= positions {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		column, err := completionColumn(command, context)
		if err != nil {
			return completionFailure()
		}
		return filterCompletions(settingCompletions(column), toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

// modeArguments completes initial Mode references in the effective Project.
func modeArguments(context *commandContext, positions int) cobra.CompletionFunc {
	return func(command *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) >= positions {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		project, err := completionProject(command, context)
		if err != nil {
			return completionFailure()
		}
		return filterCompletions(modeCompletions(project), toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

// modeColumnArguments completes a Mode first and a Column second.
func modeColumnArguments(context *commandContext) cobra.CompletionFunc {
	return func(command *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		project, err := completionProject(command, context)
		if err != nil {
			return completionFailure()
		}
		switch len(args) {
		case 0:
			return filterCompletions(modeCompletions(project), toComplete), cobra.ShellCompDirectiveNoFileComp
		case 1:
			return filterCompletions(columnCompletions(project), toComplete), cobra.ShellCompDirectiveNoFileComp
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
}

// applyColumnArguments completes one Column followed by Settings from that Column.
func applyColumnArguments(context *commandContext) cobra.CompletionFunc {
	return func(command *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		project, err := completionProject(command, context)
		if err != nil {
			return completionFailure()
		}
		if len(args) == 0 {
			return filterCompletions(columnCompletions(project), toComplete), cobra.ShellCompDirectiveNoFileComp
		}
		column, err := project.ResolveColumn(args[0])
		if err != nil {
			return completionFailure()
		}
		return filterCompletions(settingCompletions(column), toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

// loadCompletionWarehouse performs the read-only load shared by all dynamic completion.
func loadCompletionWarehouse(dependencies Dependencies) (warehouse.Warehouse, error) {
	rootPath, err := warehouse.EffectiveWarehouseRootFor(dependencies.HomeDir)
	if err != nil {
		return warehouse.Warehouse{}, err
	}
	return warehouse.LoadWarehouse(rootPath)
}

// completionProject resolves explicit --project or selected PPID context without writing state.
func completionProject(command *cobra.Command, context *commandContext) (warehouse.Project, error) {
	explicitProject := ""
	if flag := command.Flag("project"); flag != nil {
		explicitProject = flag.Value.String()
	}
	return resolveProjectForCommand(context.dependencies, explicitProject)
}

// completionColumn resolves the --column canonical name or alias in the effective Project.
func completionColumn(command *cobra.Command, context *commandContext) (warehouse.Column, error) {
	project, err := completionProject(command, context)
	if err != nil {
		return warehouse.Column{}, err
	}
	flag := command.Flag("column")
	if flag == nil || strings.TrimSpace(flag.Value.String()) == "" {
		return warehouse.Column{}, fmt.Errorf("column scope is required")
	}
	return project.ResolveColumn(flag.Value.String())
}

// projectCompletions returns sorted canonical Project names and aliases.
func projectCompletions(loaded warehouse.Warehouse) []string {
	values := []string{}
	for _, name := range sortedKeys(loaded.Projects) {
		project := loaded.Projects[name]
		values = appendResourceCompletions(values, project.Name, project.Metadata.Aliases, "Project")
	}
	return values
}

// columnCompletions returns sorted canonical Column names and aliases.
func columnCompletions(project warehouse.Project) []string {
	values := []string{}
	for _, name := range sortedKeys(project.Columns) {
		column := project.Columns[name]
		values = appendResourceCompletions(values, column.Name, column.Metadata.Aliases, "Column")
	}
	return values
}

// settingCompletions returns sorted canonical Setting names and aliases.
func settingCompletions(column warehouse.Column) []string {
	values := []string{}
	for _, name := range sortedKeys(column.Settings) {
		setting := column.Settings[name]
		values = appendResourceCompletions(values, setting.Name, setting.Metadata.Aliases, "Setting")
	}
	return values
}

// modeCompletions returns sorted canonical Mode names and aliases.
func modeCompletions(project warehouse.Project) []string {
	values := []string{}
	for _, name := range sortedKeys(project.Modes) {
		mode := project.Modes[name]
		values = appendResourceCompletions(values, mode.Name, mode.Metadata.Aliases, "Mode")
	}
	return values
}

// appendResourceCompletions appends one canonical value followed by sorted aliases.
func appendResourceCompletions(values []string, canonical string, aliases []string, kind string) []string {
	values = append(values, cobra.CompletionWithDesc(canonical, kind+" canonical name"))
	aliases = append([]string(nil), aliases...)
	sort.Strings(aliases)
	for _, alias := range aliases {
		values = append(values, cobra.CompletionWithDesc(alias, kind+" alias for "+canonical))
	}
	return values
}

// filterCompletions applies Cobra's current prefix before returning candidates.
func filterCompletions(values []string, prefix string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		candidate := strings.SplitN(value, "\t", 2)[0]
		if strings.HasPrefix(candidate, prefix) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

// completionFailure suppresses load diagnostics and disables accidental file completion.
func completionFailure() ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveNoFileComp
}
