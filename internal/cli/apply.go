package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/linker"
	"github.com/xenon/ConfigFacilitator/internal/planner"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

// newApplyCommand constructs the nested apply mode and apply column syntax.
func newApplyCommand(context *commandContext) *cobra.Command {
	var scope projectScope
	var forceTargets bool
	command := &cobra.Command{
		Use:   "apply",
		Short: "Apply a Mode or explicit Column Settings",
		Args:  usageArgs(cobra.NoArgs),
	}
	addProjectFlag(command, &scope)
	command.PersistentFlags().BoolVar(&forceTargets, "force-targets", false, "Reclaim occupied recorded target paths")
	command.AddCommand(
		&cobra.Command{
			Use:     "mode <Mode>",
			Short:   "Apply one Mode",
			Args:    usageArgs(cobra.ExactArgs(1)),
			Example: "  cfgfc apply mode Max -p OpenCode",
			RunE: func(command *cobra.Command, args []string) error {
				return runApplyMode(context, scope.project, args[0], forceTargets)
			},
		},
		&cobra.Command{
			Use:     "column <Column> <Setting>...",
			Short:   "Apply one Column with explicit Settings",
			Args:    usageArgs(cobra.MinimumNArgs(2)),
			Example: "  cfgfc apply column Skills Skill-A Skill-B -p OpenCode",
			RunE: func(command *cobra.Command, args []string) error {
				return runApplyColumn(context, scope.project, args[0], args[1:], forceTargets)
			},
		},
	)
	return command
}

// runApplyMode plans and commits one canonical Mode intent through the existing engine.
func runApplyMode(context *commandContext, explicitProject string, modeReference string, forceTargets bool) error {
	project, err := resolveProjectForCommand(context.dependencies, explicitProject)
	if err != nil {
		return err
	}
	if project.Missing {
		return classifyPlanError("apply_plan", planner.MissingResourceError{Kind: "project", Project: project.Name})
	}
	mode, resolveErr := project.ResolveMode(modeReference)
	if resolveErr != nil {
		return NewResourceError("mode_not_found", resolveErr.Error(), nil, resolveErr)
	}
	engine := linker.New()
	currentState, loadErr := engine.LoadCurrentState(project)
	if loadErr != nil {
		return NewInvalidDataError("current_state", loadErr.Error(), nil, loadErr)
	}
	mappings, planErr := planner.PlanModeMappings(project, mode.Name, currentState.Mappings, planOptions(context.dependencies))
	if planErr != nil {
		return classifyPlanError("apply_plan", planErr)
	}
	if replaceErr := engine.ReplaceState(project, linker.CurrentState{
		Mappings: mappings,
		Intent:   &linker.ApplyIntent{Kind: "mode", Mode: mode.Name},
	}, linker.WithForce(forceTargets)); replaceErr != nil {
		return classifyMutationError("apply_failed", replaceErr)
	}
	return context.renderResult(HumanResult{
		Message: fmt.Sprintf("Applied mode %q for project %q", displayStatusName(mode.Metadata.DisplayName, mode.Name), displayStatusName(project.Metadata.DisplayName, project.Name)),
		Data: map[string]any{
			"project":  project.Name,
			"kind":     "mode",
			"mode":     mode.Name,
			"mappings": mappings,
		},
	})
}

// runApplyColumn plans and commits one canonical direct-Column intent through the existing engine.
func runApplyColumn(context *commandContext, explicitProject string, columnReference string, settingReferences []string, forceTargets bool) error {
	project, err := resolveProjectForCommand(context.dependencies, explicitProject)
	if err != nil {
		return err
	}
	column, resolveErr := project.ResolveColumn(columnReference)
	if resolveErr != nil {
		return NewResourceError("column_not_found", resolveErr.Error(), nil, resolveErr)
	}
	if project.Missing {
		return classifyPlanError("apply_plan", planner.MissingResourceError{Kind: "project", Project: project.Name})
	}
	if column.Missing {
		return classifyPlanError("apply_plan", planner.MissingResourceError{Kind: "column", Project: project.Name, Column: column.Name})
	}
	settingNames, settingErr := canonicalSettingNames(column, settingReferences)
	if settingErr != nil {
		return NewResourceError("setting_not_found", settingErr.Error(), nil, settingErr)
	}
	mappings, planErr := planner.PlanColumnMappings(project, column.Name, settingNames, planOptions(context.dependencies))
	if planErr != nil {
		return classifyPlanError("apply_plan", planErr)
	}
	if replaceErr := linker.New().ReplaceState(project, linker.CurrentState{
		Mappings: mappings,
		Intent:   &linker.ApplyIntent{Kind: "column", Column: column.Name, Settings: settingNames},
	}, linker.WithForce(forceTargets)); replaceErr != nil {
		return classifyMutationError("apply_failed", replaceErr)
	}
	return context.renderResult(HumanResult{
		Message: fmt.Sprintf("Applied column %q for project %q", displayStatusName(column.Metadata.DisplayName, column.Name), displayStatusName(project.Metadata.DisplayName, project.Name)),
		Data: map[string]any{
			"project":  project.Name,
			"kind":     "column",
			"column":   column.Name,
			"settings": settingNames,
			"mappings": mappings,
		},
	})
}

// canonicalSettingNames resolves Setting aliases into canonical persisted identities.
func canonicalSettingNames(column warehouse.Column, references []string) ([]string, error) {
	settings := make([]string, 0, len(references))
	for _, reference := range references {
		setting, err := column.ResolveSetting(reference)
		if err != nil {
			return nil, err
		}
		if setting.Missing {
			return nil, planner.MissingResourceError{Kind: "setting", Column: column.Name, Name: setting.Name}
		}
		settings = append(settings, setting.Name)
	}
	return settings, nil
}

// classifyPlanError reports missing filesystem-backed resources as resource failures.
func classifyPlanError(code string, err error) error {
	var missing planner.MissingResourceError
	if errors.As(err, &missing) {
		return NewResourceError("resource_missing", err.Error(), missing, err)
	}
	return NewInvalidDataError(code, err.Error(), nil, err)
}
