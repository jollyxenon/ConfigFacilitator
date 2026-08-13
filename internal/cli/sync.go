package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/session"
	"github.com/xenon/ConfigFacilitator/internal/syncer"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// newSyncCommand constructs the normalized sync scope surface.
func newSyncCommand(context *commandContext) *cobra.Command {
	var scope projectScope
	var all bool
	command := &cobra.Command{
		Use:     "sync",
		Short:   "Reconcile warehouse indexes with filesystem state",
		Args:    usageArgs(cobra.NoArgs),
		Example: "  cfgfc sync\n  cfgfc sync -p OpenCode\n  cfgfc sync --all",
		PreRunE: func(command *cobra.Command, args []string) error {
			if all && scope.project != "" {
				return NewUsageError("conflicting_scope", "sync --all cannot be combined with --project", nil)
			}
			return nil
		},
		RunE: func(command *cobra.Command, args []string) error {
			return runSync(context, scope.project, all)
		},
	}
	addProjectFlag(command, &scope)
	command.Flags().BoolVar(&all, "all", false, "Synchronize every Project")
	return command
}

// runSync resolves scope and executes one transactional reconciliation.
func runSync(context *commandContext, explicitProject string, all bool) error {
	if explicitProject == globalProjectName {
		return NewResourceError("reserved_project", fmt.Sprintf("project name %q is reserved", globalProjectName), nil, nil)
	}
	warehouseRoot, err := effectiveWarehouseRoot(context.dependencies)
	if err != nil {
		return err
	}
	projectReference := explicitProject
	if projectReference == "" && !all {
		projectReference, _, err = session.ResolveProject("", context.dependencies.PPID, session.NewStore(warehouseRoot))
		if err != nil {
			return NewPersistenceError("read_context", "read selected Project context", err)
		}
	}
	if projectReference == "" || all {
		if err := syncer.SyncAll(warehouseRoot, planOptions(context.dependencies)); err != nil {
			return classifySyncError(err)
		}
		return context.renderResult(HumanResult{
			Message: fmt.Sprintf("Synchronized all projects in %s", warehouseRoot),
			Data:    map[string]any{"scope": "all", "root": warehouseRoot},
		})
	}
	loaded, loadErr := warehouse.LoadWarehouse(warehouseRoot)
	if loadErr != nil {
		return NewInvalidDataError("warehouse_data", loadErr.Error(), nil, loadErr)
	}
	project, resolveErr := loaded.ResolveProject(projectReference)
	if resolveErr != nil {
		return NewResourceError("project_not_found", resolveErr.Error(), nil, resolveErr)
	}
	if err := syncer.SyncProject(warehouseRoot, project.Name, planOptions(context.dependencies)); err != nil {
		return classifySyncError(err)
	}
	return context.renderResult(HumanResult{
		Message: fmt.Sprintf("Synchronized project %q", displayStatusName(project.Metadata.DisplayName, project.Name)),
		Data:    map[string]any{"scope": "project", "project": project.Name},
	})
}

// classifySyncError separates malformed external indexes from persistence failures.
func classifySyncError(err error) error {
	if strings.Contains(strings.ToLower(err.Error()), "parse") {
		return NewInvalidDataError("warehouse_data", err.Error(), nil, err)
	}
	return NewPersistenceError("sync_failed", "synchronize warehouse", err)
}
