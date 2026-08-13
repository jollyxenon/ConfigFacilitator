package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/mutate"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// metadataFlags binds common metadata replacements for create and set commands.
type metadataFlags struct {
	displayName     string
	description     string
	descriptionFile string
	aliases         string
	clearAliases    bool
}

// addMetadataFlags adds common display, description, and alias replacement flags.
func addMetadataFlags(command *cobra.Command, flags *metadataFlags) {
	command.Flags().StringVar(&flags.displayName, "display-name", "", "Replace the display name")
	command.Flags().StringVar(&flags.description, "description", "", "Replace the description")
	command.Flags().StringVar(&flags.descriptionFile, "description-file", "", "Read the replacement description from a path or - for stdin")
	command.Flags().StringVar(&flags.aliases, "aliases", "", "Replace aliases with comma-separated values")
	command.Flags().BoolVar(&flags.clearAliases, "clear-aliases", false, "Replace aliases with an empty list")
}

// createMetadata reads and validates common metadata supplied during creation.
func createMetadata(context *commandContext, command *cobra.Command, kind mutate.ResourceKind, canonicalName string, flags metadataFlags) (mutate.Metadata, error) {
	patch, changed, err := metadataPatch(context, command, kind, flags)
	if err != nil {
		return mutate.Metadata{}, err
	}
	metadata, err := mutate.NewMetadata(kind, canonicalName, canonicalName, "", []string{})
	if err != nil {
		return mutate.Metadata{}, classifyMutateError(err)
	}
	if !changed {
		return metadata, nil
	}
	metadata, err = mutate.ApplyPatch(kind, canonicalName, metadata, patch)
	if err != nil {
		return mutate.Metadata{}, classifyMutateError(err)
	}
	return metadata, nil
}

// setMetadataPatch reads common metadata replacements and requires at least one supplied field.
func setMetadataPatch(context *commandContext, command *cobra.Command, kind mutate.ResourceKind, flags metadataFlags) (mutate.MetadataPatch, error) {
	patch, changed, err := metadataPatch(context, command, kind, flags)
	if err != nil {
		return mutate.MetadataPatch{}, err
	}
	if !changed {
		return mutate.MetadataPatch{}, NewUsageError("metadata_required", "set requires at least one metadata flag", nil)
	}
	return patch, nil
}

// metadataPatch validates mutually exclusive sources and builds one common metadata replacement.
func metadataPatch(context *commandContext, command *cobra.Command, kind mutate.ResourceKind, flags metadataFlags) (mutate.MetadataPatch, bool, error) {
	descriptionChanged := command.Flags().Changed("description")
	descriptionFileChanged := command.Flags().Changed("description-file")
	aliasesChanged := command.Flags().Changed("aliases")
	clearAliasesChanged := command.Flags().Changed("clear-aliases") && flags.clearAliases
	if descriptionChanged && descriptionFileChanged {
		return mutate.MetadataPatch{}, false, NewUsageError("conflicting_description_sources", "--description and --description-file are mutually exclusive", nil)
	}
	if aliasesChanged && clearAliasesChanged {
		return mutate.MetadataPatch{}, false, NewUsageError("conflicting_alias_sources", "--aliases and --clear-aliases are mutually exclusive", nil)
	}
	patch := mutate.MetadataPatch{}
	changed := false
	if command.Flags().Changed("display-name") {
		value := flags.displayName
		patch.DisplayName = &value
		changed = true
	}
	if descriptionChanged {
		value := flags.description
		patch.Description = &value
		changed = true
	}
	if descriptionFileChanged {
		value, err := readDescription(context.dependencies.Stdin, flags.descriptionFile)
		if err != nil {
			return mutate.MetadataPatch{}, false, err
		}
		patch.Description = &value
		changed = true
	}
	if aliasesChanged {
		aliases := strings.Split(flags.aliases, ",")
		normalized, err := mutate.NormalizeAliases(kind, aliases)
		if err != nil {
			return mutate.MetadataPatch{}, false, classifyMutateError(err)
		}
		patch.Aliases = &normalized
		changed = true
	}
	if clearAliasesChanged {
		aliases := []string{}
		patch.Aliases = &aliases
		changed = true
	}
	return patch, changed, nil
}

// readDescription reads the complete selected file or stdin stream without trimming bytes.
func readDescription(stdin io.Reader, path string) (string, error) {
	if path == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", NewPersistenceError("read_description", "read description from stdin", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", NewPersistenceError("read_description", fmt.Sprintf("read description file %q", path), err)
	}
	return string(data), nil
}

// classifyMutateError maps domain mutation failures to documented CLI error classes.
func classifyMutateError(err error) *CommandError {
	var mutationErr *mutate.Error
	if !errors.As(err, &mutationErr) {
		return NewPersistenceError("mutation_failed", err.Error(), err)
	}
	switch mutationErr.Kind {
	case mutate.InvalidError:
		return NewInvalidDataError(mutationErr.Code, mutationErr.Message, mutationErr.Details, mutationErr)
	case mutate.ConflictError, mutate.MissingError:
		return NewResourceError(mutationErr.Code, mutationErr.Message, mutationErr.Details, mutationErr)
	case mutate.RefusalError:
		return NewRefusalError(mutationErr.Code, mutationErr.Message, mutationErr.Details, mutationErr)
	case mutate.PersistenceError:
		return NewPersistenceError(mutationErr.Code, mutationErr.Message, mutationErr)
	default:
		return NewPersistenceError("mutation_failed", mutationErr.Message, mutationErr)
	}
}

// loadWarehouseForInspection loads durable resource data without changing it.
func loadWarehouseForInspection(dependencies Dependencies) (warehouse.Warehouse, string, error) {
	rootPath, err := effectiveWarehouseRoot(dependencies)
	if err != nil {
		return warehouse.Warehouse{}, "", err
	}
	loaded, err := warehouse.LoadWarehouse(rootPath)
	if err != nil {
		return warehouse.Warehouse{}, "", NewInvalidDataError("warehouse_data", err.Error(), nil, err)
	}
	return loaded, rootPath, nil
}

// resolveColumnForInspection resolves a canonical or aliased Column in one Project.
func resolveColumnForInspection(project warehouse.Project, reference string) (warehouse.Column, error) {
	column, err := project.ResolveColumn(reference)
	if err != nil {
		return warehouse.Column{}, NewResourceError("column_not_found", err.Error(), nil, err)
	}
	return column, nil
}

// resolveSettingForInspection resolves a canonical or aliased Setting in one Column.
func resolveSettingForInspection(column warehouse.Column, reference string) (warehouse.Setting, error) {
	setting, err := column.ResolveSetting(reference)
	if err != nil {
		return warehouse.Setting{}, NewResourceError("setting_not_found", err.Error(), nil, err)
	}
	return setting, nil
}

// resolveModeForInspection resolves a canonical or aliased Mode in one Project.
func resolveModeForInspection(project warehouse.Project, reference string) (warehouse.Mode, error) {
	mode, err := project.ResolveMode(reference)
	if err != nil {
		return warehouse.Mode{}, NewResourceError("mode_not_found", err.Error(), nil, err)
	}
	return mode, nil
}

// requireColumnScope validates the mandatory Setting-containing Column selector.
func requireColumnScope(column string) error {
	if strings.TrimSpace(column) == "" {
		return NewResourceError("column_scope_required", "provide -c/--column", nil, nil)
	}
	return nil
}

// sortedProjectResources returns Projects in canonical-name order.
func sortedProjectResources(projects map[string]warehouse.Project) []warehouse.Project {
	result := make([]warehouse.Project, 0, len(projects))
	for _, project := range projects {
		result = append(result, project)
	}
	sort.Slice(result, func(left int, right int) bool { return result[left].Name < result[right].Name })
	return result
}

// sortedColumnResources returns Columns in canonical-name order.
func sortedColumnResources(columns map[string]warehouse.Column) []warehouse.Column {
	result := make([]warehouse.Column, 0, len(columns))
	for _, column := range columns {
		result = append(result, column)
	}
	sort.Slice(result, func(left int, right int) bool { return result[left].Name < result[right].Name })
	return result
}

// sortedSettingResources returns Settings in canonical-name order.
func sortedSettingResources(settings map[string]warehouse.Setting) []warehouse.Setting {
	result := make([]warehouse.Setting, 0, len(settings))
	for _, setting := range settings {
		result = append(result, setting)
	}
	sort.Slice(result, func(left int, right int) bool { return result[left].Name < result[right].Name })
	return result
}

// sortedModeResources returns Modes in canonical-name order.
func sortedModeResources(modes map[string]warehouse.Mode) []warehouse.Mode {
	result := make([]warehouse.Mode, 0, len(modes))
	for _, mode := range modes {
		result = append(result, mode)
	}
	sort.Slice(result, func(left int, right int) bool { return result[left].Name < result[right].Name })
	return result
}

// namesMessage renders one concise line-oriented human list.
func namesMessage(names []string) string {
	return strings.Join(names, "\n")
}

// settingKind reports the observed source kind or missing state.
func settingKind(setting warehouse.Setting) string {
	if setting.Missing || !setting.Exists {
		return "missing"
	}
	if setting.IsDir {
		return "directory"
	}
	return "file"
}
