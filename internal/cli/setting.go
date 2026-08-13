package cli

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/content"
	"github.com/xenon/ConfigFacilitator/internal/mutate"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// settingView is the stable inspection shape for one Setting.
type settingView struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Aliases     []string `json:"aliases"`
	Kind        string   `json:"kind"`
	Missing     bool     `json:"missing"`
	TargetDir   []string `json:"targetDir"`
	TargetName  []string `json:"targetName"`
}

// newSettingCommand constructs the Setting command family with shared Project and Column scope.
func newSettingCommand(context *commandContext) *cobra.Command {
	var scope projectScope
	var column string
	command := &cobra.Command{Use: "setting", Short: "Manage Settings and their content", Args: usageArgs(cobra.NoArgs)}
	addProjectFlag(command, &scope)
	command.PersistentFlags().StringVarP(&column, "column", "c", "", "Select the containing Column")
	command.AddCommand(
		newSettingListCommand(context, &scope, &column),
		newSettingShowCommand(context, &scope, &column),
		newSettingCreateCommand(context, &scope, &column),
		newSettingSetCommand(context, &scope, &column),
		newSettingRenameCommand(context, &scope, &column),
		newSettingDeleteCommand(context, &scope, &column),
		newSettingTargetCommand(context, &scope, &column),
	)
	contentCommand := newSettingContentCommand(context)
	contentCommand.Use = "content"
	command.AddCommand(contentCommand)
	return command
}

// newSettingListCommand constructs Setting listing in one explicit or selected Project and Column.
func newSettingListCommand(context *commandContext, scope *projectScope, columnReference *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Settings",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(command *cobra.Command, args []string) error {
			if err := requireColumnScope(*columnReference); err != nil {
				return err
			}
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			column, err := resolveColumnForInspection(project, *columnReference)
			if err != nil {
				return err
			}
			views := make([]settingView, 0, len(column.Settings))
			names := make([]string, 0, len(column.Settings))
			for _, setting := range sortedSettingResources(column.Settings) {
				views = append(views, viewSetting(setting))
				names = append(names, displayLabel(setting.Metadata.DisplayName, setting.Name))
			}
			return context.renderResult(HumanResult{Message: namesMessage(names), Data: map[string]any{"project": project.Name, "column": column.Name, "settings": views}})
		},
	}
}

// newSettingShowCommand constructs canonical-or-alias Setting inspection.
func newSettingShowCommand(context *commandContext, scope *projectScope, columnReference *string) *cobra.Command {
	return &cobra.Command{
		Use:   "show <Setting>",
		Short: "Show one Setting",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(command *cobra.Command, args []string) error {
			if err := requireColumnScope(*columnReference); err != nil {
				return err
			}
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			column, err := resolveColumnForInspection(project, *columnReference)
			if err != nil {
				return err
			}
			setting, err := resolveSettingForInspection(column, args[0])
			if err != nil {
				return err
			}
			view := viewSetting(setting)
			return context.renderResult(HumanResult{
				Message: fmt.Sprintf("Setting: %s\nDescription: %s\nAliases: %s\nKind: %s\nMissing: %t\nTarget directories: %v\nTarget names: %v", displayLabel(view.DisplayName, view.Name), view.Description, stringsOrNone(view.Aliases), view.Kind, view.Missing, view.TargetDir, view.TargetName),
				Data:    map[string]any{"project": project.Name, "column": column.Name, "setting": view},
			})
		},
	}
}

// newSettingCreateCommand constructs file- or directory-backed Setting creation with optional content.
func newSettingCreateCommand(context *commandContext, scope *projectScope, columnReference *string) *cobra.Command {
	var metadataFlags metadataFlags
	var sourceFlags contentSourceFlags
	var kind string
	command := &cobra.Command{
		Use:   "create <Setting>",
		Short: "Create one Setting",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(command *cobra.Command, args []string) error {
			if err := requireColumnScope(*columnReference); err != nil {
				return err
			}
			if !command.Flags().Changed("kind") {
				return NewUsageError("setting_kind_required", "provide --kind file or --kind directory", nil)
			}
			source, err := parseContentSource(context, command, sourceFlags, false)
			if err != nil {
				return err
			}
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			column, err := resolveColumnForInspection(project, *columnReference)
			if err != nil {
				return err
			}
			metadata, err := createMetadata(context, command, mutate.SettingKind, args[0], metadataFlags)
			if err != nil {
				return err
			}
			rootPath, err := effectiveWarehouseRoot(context.dependencies)
			if err != nil {
				return err
			}
			if err := mutate.CreateSetting(repository.New(rootPath), project.Name, column.Name, args[0], kind, metadata, source); err != nil {
				return classifyMutateError(err)
			}
			return context.renderResult(HumanResult{Message: fmt.Sprintf("Created %s setting %q", kind, args[0]), Data: map[string]any{"project": project.Name, "column": column.Name, "setting": args[0], "kind": kind}})
		},
	}
	addMetadataFlags(command, &metadataFlags)
	addContentSourceFlags(command, &sourceFlags)
	command.Flags().StringVar(&kind, "kind", "", "Create a file or directory Setting")
	return command
}

// newSettingSetCommand constructs transactional Setting metadata replacement.
func newSettingSetCommand(context *commandContext, scope *projectScope, columnReference *string) *cobra.Command {
	var flags metadataFlags
	command := &cobra.Command{
		Use:   "set <Setting>",
		Short: "Change Setting metadata",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(command *cobra.Command, args []string) error {
			if err := requireColumnScope(*columnReference); err != nil {
				return err
			}
			project, err := resolveProjectForCommand(context.dependencies, scope.project)
			if err != nil {
				return err
			}
			column, err := resolveColumnForInspection(project, *columnReference)
			if err != nil {
				return err
			}
			patch, err := setMetadataPatch(context, command, mutate.SettingKind, flags)
			if err != nil {
				return err
			}
			rootPath, err := effectiveWarehouseRoot(context.dependencies)
			if err != nil {
				return err
			}
			canonicalName, err := mutate.SetSetting(repository.New(rootPath), project.Name, column.Name, args[0], patch)
			if err != nil {
				return classifyMutateError(err)
			}
			return context.renderResult(HumanResult{Message: fmt.Sprintf("Updated setting %q", canonicalName), Data: map[string]any{"project": project.Name, "column": column.Name, "setting": canonicalName}})
		},
	}
	addMetadataFlags(command, &flags)
	return command
}

// viewSetting builds one normalized Setting inspection record.
func viewSetting(setting warehouse.Setting) settingView {
	return settingView{Name: setting.Name, DisplayName: setting.Metadata.DisplayName, Description: setting.Metadata.Description, Aliases: aliasesOrEmpty(setting.Metadata.Aliases), Kind: settingKind(setting), Missing: setting.Missing, TargetDir: stringsOrEmpty(setting.Metadata.TargetDir), TargetName: stringsOrEmpty(setting.Metadata.TargetName)}
}

// newSettingTargetCommand constructs Setting target override routes.
func newSettingTargetCommand(context *commandContext, scope *projectScope, columnReference *string) *cobra.Command {
	command := &cobra.Command{Use: "target", Short: "Manage Setting target overrides", Args: usageArgs(cobra.NoArgs)}
	command.AddCommand(newSettingTargetListCommand(context, scope, columnReference), newSettingTargetSetCommand(context, scope, columnReference), newSettingTargetResetCommand(context, scope, columnReference))
	return command
}

// resolveSettingTargetResources resolves shared Project, Column, and Setting target scope.
func resolveSettingTargetResources(context *commandContext, scope *projectScope, columnReference *string, settingReference string) (warehouse.Project, warehouse.Column, warehouse.Setting, error) {
	if err := requireColumnScope(*columnReference); err != nil {
		return warehouse.Project{}, warehouse.Column{}, warehouse.Setting{}, err
	}
	project, err := resolveProjectForCommand(context.dependencies, scope.project)
	if err != nil {
		return warehouse.Project{}, warehouse.Column{}, warehouse.Setting{}, err
	}
	column, err := resolveColumnForInspection(project, *columnReference)
	if err != nil {
		return warehouse.Project{}, warehouse.Column{}, warehouse.Setting{}, err
	}
	setting, err := resolveSettingForInspection(column, settingReference)
	if err != nil {
		return warehouse.Project{}, warehouse.Column{}, warehouse.Setting{}, err
	}
	return project, column, setting, nil
}

// newSettingTargetListCommand lists logical inherited Setting target components.
func newSettingTargetListCommand(context *commandContext, scope *projectScope, columnReference *string) *cobra.Command {
	return &cobra.Command{Use: "list <Setting>", Short: "List Setting target overrides", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(command *cobra.Command, args []string) error {
		project, column, setting, err := resolveSettingTargetResources(context, scope, columnReference, args[0])
		if err != nil {
			return err
		}
		positions, err := mutate.SettingTargetPositions(column.SettingIndex, setting.Metadata)
		if err != nil {
			return NewInvalidDataError("target_arrays", err.Error(), nil, err)
		}
		return context.renderResult(HumanResult{Message: fmt.Sprintf("%d target overrides", len(positions)), Data: map[string]any{"project": project.Name, "column": column.Name, "setting": setting.Name, "targets": positions}})
	}}
}

// newSettingTargetSetCommand changes independently supplied Setting target components.
func newSettingTargetSetCommand(context *commandContext, scope *projectScope, columnReference *string) *cobra.Command {
	var dir, name string
	var inheritDir, inheritName bool
	command := &cobra.Command{Use: "set <Setting> <Index>", Short: "Change a Setting target override", Args: usageArgs(cobra.ExactArgs(2)), RunE: func(command *cobra.Command, args []string) error {
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
		project, column, _, err := resolveSettingTargetResources(context, scope, columnReference, args[0])
		if err != nil {
			return err
		}
		root, err := effectiveWarehouseRoot(context.dependencies)
		if err != nil {
			return err
		}
		setting, err := mutate.SetSettingTarget(repository.New(root), project.Name, column.Name, args[0], position, dirValue, inheritDir, nameValue, inheritName, planOptions(context.dependencies))
		if err != nil {
			return classifyMutateError(err)
		}
		return context.renderResult(HumanResult{Message: fmt.Sprintf("Updated target %d", position), Data: map[string]any{"project": project.Name, "column": column.Name, "setting": setting, "index": position}})
	}}
	command.Flags().StringVar(&dir, "dir", "", "Override target directory")
	command.Flags().StringVar(&name, "name", "", "Override target name")
	command.Flags().BoolVar(&inheritDir, "inherit-dir", false, "Inherit target directory")
	command.Flags().BoolVar(&inheritName, "inherit-name", false, "Inherit target name")
	return command
}

// newSettingTargetResetCommand restores both Setting components to inheritance.
func newSettingTargetResetCommand(context *commandContext, scope *projectScope, columnReference *string) *cobra.Command {
	return &cobra.Command{Use: "reset <Setting> <Index>", Short: "Reset a Setting target override", Args: usageArgs(cobra.ExactArgs(2)), RunE: func(command *cobra.Command, args []string) error {
		position, err := mutate.ParseTargetIndex(args[1])
		if err != nil {
			return classifyMutateError(err)
		}
		project, column, _, err := resolveSettingTargetResources(context, scope, columnReference, args[0])
		if err != nil {
			return err
		}
		root, err := effectiveWarehouseRoot(context.dependencies)
		if err != nil {
			return err
		}
		setting, err := mutate.ResetSettingTarget(repository.New(root), project.Name, column.Name, args[0], position, planOptions(context.dependencies))
		if err != nil {
			return classifyMutateError(err)
		}
		return context.renderResult(HumanResult{Message: fmt.Sprintf("Reset target %d", position), Data: map[string]any{"project": project.Name, "column": column.Name, "setting": setting, "index": position}})
	}}
}

// newSettingContentCommand constructs bounded Setting content routes with their own shared scope.
func newSettingContentCommand(context *commandContext) *cobra.Command {
	var scope projectScope
	var column string
	command := &cobra.Command{Use: "setting content", Short: "Inspect and change Setting content", Args: usageArgs(cobra.NoArgs)}
	addProjectFlag(command, &scope)
	command.PersistentFlags().StringVarP(&column, "column", "c", "", "Select the containing Column")
	command.AddCommand(
		newSettingContentListCommand(context, &scope, &column),
		newSettingContentReadCommand(context, &scope, &column),
		newSettingContentWriteCommand(context, &scope, &column),
		newSettingContentMkdirCommand(context, &scope, &column),
		newSettingContentMoveCommand(context, &scope, &column),
		newSettingContentDeleteCommand(context, &scope, &column),
	)
	return command
}

// contentSourceFlags binds mutually exclusive exact-byte content inputs.
type contentSourceFlags struct {
	from  string
	stdin bool
	text  string
}

// addContentSourceFlags adds local-file, stdin, and literal-text source selectors.
func addContentSourceFlags(command *cobra.Command, flags *contentSourceFlags) {
	command.Flags().StringVar(&flags.from, "from", "", "Read exact content from a regular file or directory")
	command.Flags().BoolVar(&flags.stdin, "stdin", false, "Read exact file bytes from standard input")
	command.Flags().StringVar(&flags.text, "text", "", "Use exact literal file bytes without adding a newline")
}

// parseContentSource enforces exclusivity and loads exact stdin bytes when selected.
func parseContentSource(context *commandContext, command *cobra.Command, flags contentSourceFlags, required bool) (content.Source, error) {
	selected := 0
	if command.Flags().Changed("from") {
		selected++
	}
	if command.Flags().Changed("stdin") {
		selected++
	}
	if command.Flags().Changed("text") {
		selected++
	}
	if selected > 1 {
		return content.Source{}, NewUsageError("conflicting_content_sources", "--from, --stdin, and --text are mutually exclusive", nil)
	}
	if selected == 0 {
		if required {
			return content.Source{}, NewUsageError("content_source_required", "provide exactly one of --from, --stdin, or --text", nil)
		}
		return content.Source{Mode: content.SourceEmpty}, nil
	}
	if command.Flags().Changed("from") {
		return content.Source{Mode: content.SourcePath, Path: flags.from}, nil
	}
	if command.Flags().Changed("stdin") {
		data, err := ioReadAll(context.dependencies.Stdin)
		if err != nil {
			return content.Source{}, NewPersistenceError("read_content_stdin", "read Setting content from stdin", err)
		}
		return content.Source{Mode: content.SourceBytes, Bytes: data}, nil
	}
	return content.Source{Mode: content.SourceBytes, Bytes: []byte(flags.text)}, nil
}

// resolveSettingContent resolves one present Setting and maps its observed source kind.
func resolveSettingContent(context *commandContext, scope *projectScope, columnReference *string, reference string) (warehouse.Project, warehouse.Column, warehouse.Setting, content.Kind, repository.Repository, error) {
	project, column, setting, err := resolveSettingTargetResources(context, scope, columnReference, reference)
	if err != nil {
		return warehouse.Project{}, warehouse.Column{}, warehouse.Setting{}, "", repository.Repository{}, err
	}
	if setting.Missing || !setting.Exists {
		return warehouse.Project{}, warehouse.Column{}, warehouse.Setting{}, "", repository.Repository{}, NewResourceError("setting_missing", fmt.Sprintf("setting %q is missing", setting.Name), nil, nil)
	}
	kind := content.KindFile
	if setting.IsDir {
		kind = content.KindDirectory
	}
	root, err := effectiveWarehouseRoot(context.dependencies)
	if err != nil {
		return warehouse.Project{}, warehouse.Column{}, warehouse.Setting{}, "", repository.Repository{}, err
	}
	return project, column, setting, kind, repository.New(root), nil
}

// newSettingContentListCommand lists one Setting source in lexical order.
func newSettingContentListCommand(context *commandContext, scope *projectScope, columnReference *string) *cobra.Command {
	return &cobra.Command{Use: "list <Setting>", Short: "List Setting content", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(command *cobra.Command, args []string) error {
		project, column, setting, kind, _, err := resolveSettingContent(context, scope, columnReference, args[0])
		if err != nil {
			return err
		}
		entries, err := content.List(setting.Path, kind)
		if err != nil {
			return classifyContentError(err)
		}
		lines := make([]string, 0, len(entries))
		for _, entry := range entries {
			lines = append(lines, fmt.Sprintf("%s\t%s\t%d", entry.Path, entry.Kind, entry.Size))
		}
		return context.renderResult(HumanResult{Message: strings.Join(lines, "\n"), Data: map[string]any{"project": project.Name, "column": column.Name, "setting": setting.Name, "entries": entries}})
	}}
}

// newSettingContentReadCommand emits exact human bytes or one UTF-8/base64 JSON envelope.
func newSettingContentReadCommand(context *commandContext, scope *projectScope, columnReference *string) *cobra.Command {
	return &cobra.Command{Use: "read <Setting> [RelativePath]", Short: "Read Setting content", Args: usageArgs(cobra.RangeArgs(1, 2)), RunE: func(command *cobra.Command, args []string) error {
		project, column, setting, kind, _, err := resolveSettingContent(context, scope, columnReference, args[0])
		if err != nil {
			return err
		}
		var relative *string
		if len(args) == 2 {
			relative = &args[1]
		}
		data, err := content.Read(setting.Path, kind, relative)
		if err != nil {
			return classifyContentError(err)
		}
		if !context.json {
			if _, err := context.dependencies.Stdout.Write(data); err != nil {
				return NewPersistenceError("write_output", "write Setting content", err)
			}
			return nil
		}
		payload := map[string]any{"project": project.Name, "column": column.Name, "setting": setting.Name}
		if relative != nil {
			payload["path"] = *relative
		}
		if utf8.Valid(data) {
			payload["content"] = string(data)
			payload["encoding"] = "utf-8"
		} else {
			payload["content"] = encodeBase64(data)
			payload["encoding"] = "base64"
		}
		return context.renderResult(HumanResult{Data: payload})
	}}
}

// newSettingContentWriteCommand atomically creates or replaces one regular file.
func newSettingContentWriteCommand(context *commandContext, scope *projectScope, columnReference *string) *cobra.Command {
	var flags contentSourceFlags
	command := &cobra.Command{Use: "write <Setting> [RelativePath]", Short: "Write Setting content", Args: usageArgs(cobra.RangeArgs(1, 2)), RunE: func(command *cobra.Command, args []string) error {
		source, err := parseContentSource(context, command, flags, true)
		if err != nil {
			return err
		}
		project, column, setting, kind, repo, err := resolveSettingContent(context, scope, columnReference, args[0])
		if err != nil {
			return err
		}
		var relative *string
		if len(args) == 2 {
			relative = &args[1]
		}
		if err := content.Write(repo, setting.Path, kind, relative, source); err != nil {
			return classifyContentError(err)
		}
		return context.renderResult(HumanResult{Message: "Updated Setting content", Data: map[string]any{"project": project.Name, "column": column.Name, "setting": setting.Name}})
	}}
	addContentSourceFlags(command, &flags)
	return command
}

// newSettingContentMkdirCommand creates one nested directory in a directory-backed Setting.
func newSettingContentMkdirCommand(context *commandContext, scope *projectScope, columnReference *string) *cobra.Command {
	return &cobra.Command{Use: "mkdir <Setting> <RelativePath>", Short: "Create a content directory", Args: usageArgs(cobra.ExactArgs(2)), RunE: func(command *cobra.Command, args []string) error {
		project, column, setting, kind, repo, err := resolveSettingContent(context, scope, columnReference, args[0])
		if err != nil {
			return err
		}
		if err := content.Mkdir(repo, setting.Path, kind, args[1]); err != nil {
			return classifyContentError(err)
		}
		return context.renderResult(HumanResult{Message: fmt.Sprintf("Created content directory %q", args[1]), Data: map[string]any{"project": project.Name, "column": column.Name, "setting": setting.Name, "path": args[1]}})
	}}
}

// newSettingContentMoveCommand moves one regular file or directory without overwriting.
func newSettingContentMoveCommand(context *commandContext, scope *projectScope, columnReference *string) *cobra.Command {
	return &cobra.Command{Use: "move <Setting> <OldPath> <NewPath>", Short: "Move Setting content", Args: usageArgs(cobra.ExactArgs(3)), RunE: func(command *cobra.Command, args []string) error {
		project, column, setting, kind, repo, err := resolveSettingContent(context, scope, columnReference, args[0])
		if err != nil {
			return err
		}
		if err := content.Move(repo, setting.Path, kind, args[1], args[2]); err != nil {
			return classifyContentError(err)
		}
		return context.renderResult(HumanResult{Message: fmt.Sprintf("Moved content %q to %q", args[1], args[2]), Data: map[string]any{"project": project.Name, "column": column.Name, "setting": setting.Name, "from": args[1], "to": args[2]}})
	}}
}

// newSettingContentDeleteCommand deletes one regular file or directory tree after confirmation.
func newSettingContentDeleteCommand(context *commandContext, scope *projectScope, columnReference *string) *cobra.Command {
	var yes bool
	command := &cobra.Command{Use: "delete <Setting> <RelativePath>", Short: "Delete Setting content", Args: usageArgs(cobra.ExactArgs(2)), RunE: func(command *cobra.Command, args []string) error {
		project, column, setting, kind, repo, err := resolveSettingContent(context, scope, columnReference, args[0])
		if err != nil {
			return err
		}
		if err := content.Delete(repo, setting.Path, kind, args[1], yes); err != nil {
			return classifyContentError(err)
		}
		return context.renderResult(HumanResult{Message: fmt.Sprintf("Deleted content %q", args[1]), Data: map[string]any{"project": project.Name, "column": column.Name, "setting": setting.Name, "path": args[1]}})
	}}
	command.Flags().BoolVar(&yes, "yes", false, "Confirm content deletion")
	return command
}

// classifyContentError maps content-domain failures to documented CLI classes.
func classifyContentError(err error) *CommandError {
	var contentErr *content.Error
	if !errors.As(err, &contentErr) {
		return NewPersistenceError("content_failed", err.Error(), err)
	}
	switch contentErr.Kind {
	case content.InvalidError:
		return NewInvalidDataError(contentErr.Code, contentErr.Message, nil, contentErr)
	case content.ConflictError, content.MissingError:
		return NewResourceError(contentErr.Code, contentErr.Message, nil, contentErr)
	case content.RefusalError:
		return NewRefusalError(contentErr.Code, contentErr.Message, nil, contentErr)
	default:
		return NewPersistenceError(contentErr.Code, contentErr.Message, contentErr)
	}
}

// ioReadAll reads exact source bytes from one injected stream.
func ioReadAll(reader io.Reader) ([]byte, error) { return io.ReadAll(reader) }

// encodeBase64 returns RFC 4648 base64 for invalid UTF-8 JSON content.
func encodeBase64(data []byte) string { return base64.StdEncoding.EncodeToString(data) }

// newSettingRenameCommand constructs transactional canonical Setting rename.
func newSettingRenameCommand(context *commandContext, scope *projectScope, columnReference *string) *cobra.Command {
	var forceTargets bool
	command := &cobra.Command{Use: "rename <Old> <New>", Short: "Rename one Setting", Args: usageArgs(cobra.ExactArgs(2)), RunE: func(command *cobra.Command, args []string) error {
		if err := requireColumnScope(*columnReference); err != nil {
			return err
		}
		project, err := resolveProjectForCommand(context.dependencies, scope.project)
		if err != nil {
			return err
		}
		column, err := resolveColumnForInspection(project, *columnReference)
		if err != nil {
			return err
		}
		rootPath, err := effectiveWarehouseRoot(context.dependencies)
		if err != nil {
			return err
		}
		if err := mutate.RenameSetting(repository.New(rootPath), project.Name, column.Name, args[0], args[1], forceTargets, planOptions(context.dependencies)); err != nil {
			return classifyMutateError(err)
		}
		return context.renderResult(HumanResult{Message: fmt.Sprintf("Renamed setting %q to %q", args[0], args[1]), Data: map[string]any{"project": project.Name, "column": column.Name, "setting": args[1], "previousName": args[0]}})
	}}
	command.Flags().BoolVar(&forceTargets, "force-targets", false, "Reclaim affected recorded target paths")
	return command
}

// newSettingDeleteCommand constructs confirmed transactional Setting deletion.
func newSettingDeleteCommand(context *commandContext, scope *projectScope, columnReference *string) *cobra.Command {
	var yes, cascade, forceTargets bool
	command := &cobra.Command{Use: "delete <Setting>", Short: "Delete one Setting", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(command *cobra.Command, args []string) error {
		if err := requireColumnScope(*columnReference); err != nil { return err }
		project, err := resolveProjectForCommand(context.dependencies, scope.project)
		if err != nil { return err }
		column, err := resolveColumnForInspection(project, *columnReference)
		if err != nil { return err }
		rootPath, err := effectiveWarehouseRoot(context.dependencies)
		if err != nil { return err }
		report, err := mutate.DeleteSetting(repository.New(rootPath), project.Name, column.Name, args[0], yes, cascade, forceTargets)
		if err != nil { return classifyMutateError(err) }
		return context.renderResult(HumanResult{Message: fmt.Sprintf("Deleted setting %q", report.Name), Data: map[string]any{"project": project.Name, "column": column.Name, "setting": report.Name, "dependencies": report}})
	}}
	addDeleteFlags(command, &yes, &cascade, &forceTargets)
	return command
}
