package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// newRootPathCommand constructs persistent warehouse-root inspection and update behavior.
func newRootPathCommand(context *commandContext) *cobra.Command {
	return &cobra.Command{
		Use:     "root [Path]",
		Short:   "Inspect or persist the effective warehouse root",
		Long:    "Print the current effective warehouse root or persist a normalized replacement. Changing the root does not migrate, copy, or initialize existing warehouse contents.",
		Args:    usageArgs(cobra.MaximumNArgs(1)),
		Example: "  cfgfc root\n  cfgfc root ~/.configfacilitator-alt",
		RunE: func(command *cobra.Command, args []string) error {
			if len(args) == 0 {
				rootPath, err := effectiveWarehouseRoot(context.dependencies)
				if err != nil {
					return err
				}
				return context.renderResult(HumanResult{Message: rootPath, Data: map[string]any{"root": rootPath}})
			}
			currentRoot, err := effectiveWarehouseRoot(context.dependencies)
			if err != nil {
				return err
			}
			bootstrapPath := filepath.Join(context.dependencies.HomeDir, ".cfgfc-root")
			var rootPath string
			err = repository.New(currentRoot).WithMutation("root-set", []string{bootstrapPath}, func() error {
				var setErr error
				rootPath, setErr = warehouse.SetEffectiveWarehouseRootFor(
					context.dependencies.HomeDir,
					context.dependencies.OperatingSystem,
					args[0],
					context.dependencies.Environment,
				)
				return setErr
			})
			if err != nil {
				return NewPersistenceError("set_warehouse_root", err.Error(), err)
			}
			return context.renderResult(HumanResult{
				Message: fmt.Sprintf("Warehouse root set to %s", rootPath),
				Data:    map[string]any{"root": rootPath},
			})
		},
	}
}
