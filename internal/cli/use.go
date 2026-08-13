package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/session"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

const globalProjectName = "global"

// newUseCommand constructs the PPID-scoped Project selection command.
func newUseCommand(context *commandContext) *cobra.Command {
	return &cobra.Command{
		Use:     "use <Project|global>",
		Short:   "Select a Project for the current PPID scope",
		Args:    usageArgs(cobra.ExactArgs(1)),
		Example: "  cfgfc use OpenCode\n  cfgfc use global",
		RunE: func(command *cobra.Command, args []string) error {
			return runUse(context, args[0])
		},
	}
}

// runUse selects a canonical Project or clears the injected PPID context.
func runUse(context *commandContext, projectReference string) error {
	warehouseRoot, err := effectiveWarehouseRoot(context.dependencies)
	if err != nil {
		return err
	}
	store := session.NewStore(warehouseRoot)
	contextPath := repository.New(warehouseRoot).SessionPath(context.dependencies.PPID)
	if projectReference == globalProjectName {
		if err := repository.New(warehouseRoot).WithMutation("use-global", []string{contextPath}, func() error {
			return store.Clear(context.dependencies.PPID)
		}); err != nil {
			return NewPersistenceError("clear_context", "clear selected Project context", err)
		}
		return context.renderResult(HumanResult{
			Message: "Using global Project context",
			Data:    map[string]any{"project": nil, "ppid": context.dependencies.PPID},
		})
	}
	loaded, loadErr := warehouse.LoadWarehouse(warehouseRoot)
	if loadErr != nil {
		return NewPersistenceError("load_warehouse", "load warehouse", loadErr)
	}
	project, resolveErr := loaded.ResolveProject(projectReference)
	if resolveErr != nil {
		return NewResourceError("project_not_found", resolveErr.Error(), nil, resolveErr)
	}
	if err := repository.New(warehouseRoot).WithMutation("use-project", []string{contextPath}, func() error {
		return store.Set(context.dependencies.PPID, project.Name)
	}); err != nil {
		return NewPersistenceError("store_context", "store selected Project context", err)
	}
	return context.renderResult(HumanResult{
		Message: fmt.Sprintf("Using project %q", displayStatusName(project.Metadata.DisplayName, project.Name)),
		Data: map[string]any{
			"project": project.Name,
			"ppid":    context.dependencies.PPID,
		},
	})
}
