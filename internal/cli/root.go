package cli

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/repository"
)

const rootLongDescription = `cfgfc manages portable configuration warehouses through resource-oriented commands.

Project-scoped commands use -p/--project when supplied, otherwise they use the Project selected for the current PPID by cfgfc use. Use cfgfc use global to clear that context. sync --all and refresh --all ignore selected context.

--json emits one stable object without ANSI or extra prose. Destructive operations keep --yes confirmation, --cascade reference cleanup, and --force-targets target reclamation as independent controls. cfgfc root <Path> changes later warehouse resolution without migrating existing contents.`

const rootCommandName = "cfgfc"

// commandContext holds per-execution flags and injected dependencies.
type commandContext struct {
	dependencies Dependencies
	json         bool
}

// NewRootCommand creates a fresh fully injectable Cobra command tree.
func NewRootCommand(dependencies Dependencies) *cobra.Command {
	context := &commandContext{dependencies: normalizeDependencies(dependencies)}
	root := &cobra.Command{
		Use:           rootCommandName,
		Short:         "Manage portable configuration warehouses",
		Long:          rootLongDescription,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          usageArgs(cobra.NoArgs),
		PersistentPreRunE: func(command *cobra.Command, args []string) error {
			if !isMutatingCommand(command, args) {
				return nil
			}
			rootPath, err := effectiveWarehouseRoot(context.dependencies)
			if err != nil {
				return err
			}
			if err := repository.New(rootPath).Recover(); err != nil {
				return NewPersistenceError("transaction_recovery", "recover incomplete repository transaction", err)
			}
			return nil
		},
		RunE: func(command *cobra.Command, args []string) error {
			return command.Help()
		},
	}
	root.SetIn(context.dependencies.Stdin)
	root.SetOut(context.dependencies.Stdout)
	root.SetErr(context.dependencies.Stderr)
	root.PersistentFlags().BoolVar(&context.json, "json", false, "Emit one stable JSON object")
	root.SetFlagErrorFunc(func(command *cobra.Command, err error) error {
		return NewUsageError("invalid_usage", err.Error(), err)
	})
	root.SetHelpCommand(&cobra.Command{Hidden: true})
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetHelpTemplate(strings.ReplaceAll(root.HelpTemplate(), "{{with (or .CommandPath .UseLine)}}{{.}}", rootCommandName))
	root.SetUsageTemplate(strings.ReplaceAll(root.UsageTemplate(), "{{.CommandPath}}", rootCommandName))
	root.AddCommand(
		newProjectCommand(context),
		newColumnCommand(context),
		newSettingCommand(context),
		newModeCommand(context),
		newCurrentCommand(context),
		newUseCommand(context),
		newStatusCommand(context),
		newApplyCommand(context),
		newRefreshCommand(context),
		newSyncCommand(context),
		newRootPathCommand(context),
		newResetCommand(context),
		newRevertCommand(context),
		newWebCommand(context),
		newCompletionCommand(),
	)
	addDynamicCompletions(root, context)
	applyStandardHelp(root)
	return root
}

// isMutatingCommand reports whether a parsed command must recover prepared work first.
func isMutatingCommand(command *cobra.Command, args []string) bool {
	path := command.CommandPath()
	if path == "cfgfc completion" || strings.HasPrefix(path, "cfgfc __complete") {
		return false
	}
	if path == rootCommandName {
		return false
	}
	readOnly := map[string]bool{
		"cfgfc project list": true, "cfgfc project show": true,
		"cfgfc column list": true, "cfgfc column show": true, "cfgfc column target list": true,
		"cfgfc setting list": true, "cfgfc setting show": true, "cfgfc setting target list": true,
		"cfgfc setting content list": true, "cfgfc setting content read": true,
		"cfgfc mode list": true, "cfgfc mode show": true, "cfgfc mode column list": true,
		"cfgfc status": true,
	}
	if readOnly[path] {
		return false
	}
	if path == "cfgfc root" && len(args) == 0 {
		return false
	}
	return true
}

// normalizeDependencies fills safe stream and process defaults for direct command construction.
func normalizeDependencies(dependencies Dependencies) Dependencies {
	if dependencies.Stdin == nil {
		dependencies.Stdin = strings.NewReader("")
	}
	if dependencies.Stdout == nil {
		dependencies.Stdout = ioDiscard{}
	}
	if dependencies.Stderr == nil {
		dependencies.Stderr = ioDiscard{}
	}
	if dependencies.Environment == nil {
		dependencies.Environment = map[string]string{}
	}
	if dependencies.OperatingSystem == "" {
		dependencies.OperatingSystem = runtime.GOOS
	}
	return dependencies
}

// ioDiscard is a tiny writer used only when a direct caller omits output dependencies.
type ioDiscard struct{}

// Write accepts and discards output bytes.
func (ioDiscard) Write(data []byte) (int, error) {
	return len(data), nil
}

// usageArgs converts Cobra positional-validation failures into stable usage errors.
func usageArgs(validator cobra.PositionalArgs) cobra.PositionalArgs {
	return func(command *cobra.Command, args []string) error {
		if err := validator(command, args); err != nil {
			return NewUsageError("invalid_arguments", err.Error(), err)
		}
		return nil
	}
}

// unimplementedHandler returns a controlled error for a registered later-slice command.
func unimplementedHandler(commandPath string) func(*cobra.Command, []string) error {
	return func(command *cobra.Command, args []string) error {
		return NewInvalidDataError(
			"not_implemented",
			fmt.Sprintf("%s is registered but not implemented in this OpenSpec slice", commandPath),
			nil,
			nil,
		)
	}
}

// renderResult writes a successful human or JSON command response.
func (context *commandContext) renderResult(result HumanResult) error {
	if err := (output{stdout: context.dependencies.Stdout, json: context.json}).renderSuccess(result); err != nil {
		return NewPersistenceError("write_output", "write command output", err)
	}
	return nil
}

// projectScope binds the shared explicit Project selector.
type projectScope struct {
	project string
}

// addProjectFlag adds the shared Project selector to one command.
func addProjectFlag(command *cobra.Command, scope *projectScope) {
	command.PersistentFlags().StringVarP(&scope.project, "project", "p", "", "Select a Project explicitly")
}
