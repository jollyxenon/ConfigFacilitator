package cli

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xenon/ConfigFacilitator/internal/linker"
	"github.com/xenon/ConfigFacilitator/internal/planner"
	"github.com/xenon/ConfigFacilitator/internal/repository"
	"github.com/xenon/ConfigFacilitator/internal/session"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

const (
	statusGreen  = "\x1b[32m"
	statusYellow = "\x1b[33m"
	statusRed    = "\x1b[31m"
	statusReset  = "\x1b[0m"
)

// statusMissingResource identifies one indexed resource whose expected source is absent.
type statusMissingResource struct {
	Kind    string `json:"kind"`
	Project string `json:"project"`
	Column  string `json:"column,omitempty"`
	Name    string `json:"name"`
	Path    string `json:"path,omitempty"`
}

// statusColumnSummary describes one Column's current managed coverage and missing count.
type statusColumnSummary struct {
	Column   string `json:"column"`
	Coverage string `json:"coverage"`
	Missing  int    `json:"missing"`
}

// statusProjectSummary describes one Project in warehouse-wide status output.
type statusProjectSummary struct {
	Project    string `json:"project"`
	Status     string `json:"status"`
	ActiveMode string `json:"activeMode,omitempty"`
	Mappings   int    `json:"mappings"`
	Missing    int    `json:"missing"`
}

// newStatusCommand constructs the read-only warehouse and Project status command.
func newStatusCommand(context *commandContext) *cobra.Command {
	var scope projectScope
	command := &cobra.Command{
		Use:     "status",
		Short:   "Inspect warehouse or Project runtime status",
		Args:    usageArgs(cobra.NoArgs),
		Example: "  cfgfc status\n  cfgfc status -p OpenCode\n  cfgfc status --json",
		RunE: func(command *cobra.Command, args []string) error {
			return runStatus(context, scope.project)
		},
	}
	addProjectFlag(command, &scope)
	return command
}

// runStatus renders global summaries or one effective Project status without mutation.
func runStatus(context *commandContext, explicitProject string) error {
	warehouseRoot, err := effectiveWarehouseRoot(context.dependencies)
	if err != nil {
		return err
	}
	effectiveProject, _, resolveErr := session.ResolveProject(explicitProject, context.dependencies.PPID, session.NewStore(warehouseRoot))
	if resolveErr != nil {
		return NewPersistenceError("read_context", "read selected Project context", resolveErr)
	}
	loaded, loadErr := warehouse.LoadWarehouse(warehouseRoot)
	if loadErr != nil {
		return NewInvalidDataError("warehouse_data", loadErr.Error(), nil, loadErr)
	}
	if effectiveProject == "" {
		return renderWarehouseStatus(context, loaded)
	}
	if effectiveProject == globalProjectName {
		return NewResourceError("reserved_project", fmt.Sprintf("project name %q is reserved", globalProjectName), nil, nil)
	}
	project, projectErr := loaded.ResolveProject(effectiveProject)
	if projectErr != nil {
		return NewResourceError("project_not_found", projectErr.Error(), nil, projectErr)
	}
	return renderProjectStatus(context, project)
}

// renderWarehouseStatus renders sorted Project summaries through the selected output mode.
func renderWarehouseStatus(context *commandContext, loaded warehouse.Warehouse) error {
	engine := linker.New()
	planContext := newStatusPlanContext(context.dependencies)
	summaries := make([]statusProjectSummary, 0, len(loaded.Projects))
	lines := make([]string, 0, len(loaded.Projects)+1)
	missing := []statusMissingResource{}
	for _, projectName := range sortedKeys(loaded.Projects) {
		project := loaded.Projects[projectName]
		state, err := engine.LoadCurrentState(project)
		if err != nil {
			return NewInvalidDataError("current_state", err.Error(), map[string]any{"project": project.Name}, err)
		}
		activeMode := ""
		status := "none"
		humanStatus := "None"
		color := statusRed
		if mode, ok := matchedModeIntent(project, state, planContext); ok {
			activeMode = mode.Name
			status = "matched"
			humanStatus = displayLabel(mode.Metadata.DisplayName, mode.Name)
			color = statusGreen
		} else if len(state.Mappings) > 0 || state.Intent != nil {
			status = "unmatched"
			humanStatus = "Unmatched"
		}
		projectMissing := collectProjectMissingResources(project)
		missing = append(missing, projectMissing...)
		lines = append(lines, fmt.Sprintf("%s %s", displayLabel(project.Metadata.DisplayName, project.Name), emphasizeStatusText(context, color, "("+humanStatus+")")))
		summaries = append(summaries, statusProjectSummary{Project: project.Name, Status: status, ActiveMode: activeMode, Mappings: len(state.Mappings), Missing: len(projectMissing)})
	}
	diagnostics, diagnosticsErr := transactionDiagnostics(context.dependencies)
	if diagnosticsErr != nil {
		return NewPersistenceError("transaction_diagnostics", "inspect incomplete repository transactions", diagnosticsErr)
	}
	appendMissingResources(&lines, missing)
	appendStatusDiagnostics(&lines, diagnostics)
	return context.renderResult(HumanResult{
		Message: strings.Join(lines, "\n"),
		Data: map[string]any{
			"scope":        "warehouse",
			"projects":     summaries,
			"missing":      missing,
			"transactions": diagnostics,
		},
	})
}

// renderProjectStatus renders one Project's active Mode, current state, missing resources, and Column coverage.
func renderProjectStatus(context *commandContext, project warehouse.Project) error {
	state, err := linker.New().LoadCurrentState(project)
	if err != nil {
		return NewInvalidDataError("current_state", err.Error(), map[string]any{"project": project.Name}, err)
	}
	statusContext := newStatusPlanContext(context.dependencies)
	currentMappings := mappingSet(state.Mappings)
	columns := make([]statusColumnSummary, 0, len(project.Columns))
	lines := []string{fmt.Sprintf("Project: %s", displayLabel(project.Metadata.DisplayName, project.Name))}
	matchedMode := ""
	if mode, ok := matchedModeIntent(project, state, statusContext); ok {
		matchedMode = mode.Name
		lines = append(lines, "Active Mode: "+emphasizeStatusText(context, statusGreen, displayLabel(mode.Metadata.DisplayName, mode.Name)))
	} else {
		lines = append(lines, "Active Mode: "+emphasizeStatusText(context, statusRed, "None"))
	}
	lines = append(lines, "Intent: "+formatStatusIntent(state.Intent), fmt.Sprintf("Mappings: %d", len(state.Mappings)))
	for _, mapping := range sortedStatusMappings(state.Mappings) {
		lines = append(lines, fmt.Sprintf("  - %s -> %s", mapping.Source, mapping.Target))
	}
	lines = append(lines, "Columns:")
	for _, columnName := range sortedKeys(project.Columns) {
		column := project.Columns[columnName]
		coverage := columnCoverage(project, column, currentMappings, statusContext)
		missingCount := countMissingSettings(column)
		lines = append(lines, fmt.Sprintf("  - %s %s", displayLabel(column.Metadata.DisplayName, column.Name), emphasizeStatusText(context, coverageStatusColor(coverage), "("+statusTitle(coverage)+")")))
		columns = append(columns, statusColumnSummary{Column: column.Name, Coverage: coverage, Missing: missingCount})
	}
	missing := collectProjectMissingResources(project)
	appendMissingResources(&lines, missing)
	diagnostics, diagnosticsErr := transactionDiagnostics(context.dependencies)
	if diagnosticsErr != nil {
		return NewPersistenceError("transaction_diagnostics", "inspect incomplete repository transactions", diagnosticsErr)
	}
	appendStatusDiagnostics(&lines, diagnostics)
	return context.renderResult(HumanResult{
		Message: strings.Join(lines, "\n"),
		Data: map[string]any{
			"scope":        "project",
			"project":      project.Name,
			"activeMode":   nullableStatusName(matchedMode),
			"intent":       state.Intent,
			"mappings":     state.Mappings,
			"columns":      columns,
			"missing":      missing,
			"transactions": diagnostics,
		},
	})
}

// formatStatusIntent renders one canonical persisted apply intent for human inspection.
func formatStatusIntent(intent *linker.ApplyIntent) string {
	if intent == nil {
		return "None (mapping-only)"
	}
	switch intent.Kind {
	case "mode":
		return fmt.Sprintf("mode %s", intent.Mode)
	case "column":
		return fmt.Sprintf("column %s [%s]", intent.Column, strings.Join(intent.Settings, ", "))
	default:
		return intent.Kind
	}
}

// sortedStatusMappings returns a deterministic copy of current mappings.
func sortedStatusMappings(mappings []linker.Mapping) []linker.Mapping {
	result := append([]linker.Mapping(nil), mappings...)
	sort.Slice(result, func(left int, right int) bool {
		if result[left].Target == result[right].Target {
			return result[left].Source < result[right].Source
		}
		return result[left].Target < result[right].Target
	})
	return result
}

// collectProjectMissingResources returns every missing resource below one Project.
func collectProjectMissingResources(project warehouse.Project) []statusMissingResource {
	missing := []statusMissingResource{}
	if project.Missing {
		missing = append(missing, statusMissingResource{Kind: "project", Project: project.Name, Name: project.Name, Path: project.Path})
	}
	for _, columnName := range sortedKeys(project.Columns) {
		column := project.Columns[columnName]
		if column.Missing {
			missing = append(missing, statusMissingResource{Kind: "column", Project: project.Name, Column: column.Name, Name: column.Name, Path: column.Path})
		}
		for _, settingName := range sortedKeys(column.Settings) {
			setting := column.Settings[settingName]
			if setting.Missing {
				missing = append(missing, statusMissingResource{Kind: "setting", Project: project.Name, Column: column.Name, Name: setting.Name, Path: setting.Path})
			}
		}
	}
	for _, modeName := range sortedKeys(project.Modes) {
		mode := project.Modes[modeName]
		if mode.Missing {
			missing = append(missing, statusMissingResource{Kind: "mode", Project: project.Name, Name: mode.Name})
		}
	}
	return missing
}

// countMissingSettings counts absent Setting sources in one Column.
func countMissingSettings(column warehouse.Column) int {
	count := 0
	for _, setting := range column.Settings {
		if setting.Missing {
			count++
		}
	}
	return count
}

// appendMissingResources adds a concise human missing-resource section when needed.
func appendMissingResources(lines *[]string, missing []statusMissingResource) {
	if len(missing) == 0 {
		return
	}
	*lines = append(*lines, "Missing resources:")
	for _, resource := range missing {
		label := resource.Name
		if resource.Column != "" && resource.Kind == "setting" {
			label = resource.Column + "/" + resource.Name
		}
		*lines = append(*lines, fmt.Sprintf("  - %s: %s", statusTitle(resource.Kind), label))
	}
}

// appendStatusDiagnostics adds detailed prepared-transaction diagnostics to human output.
func appendStatusDiagnostics(lines *[]string, diagnostics []repository.TransactionInfo) {
	if len(diagnostics) == 0 {
		return
	}
	*lines = append(*lines, "Incomplete transactions:")
	for _, diagnostic := range diagnostics {
		*lines = append(*lines, fmt.Sprintf("  - %s (%s): %s", diagnostic.Operation, diagnostic.Status, diagnostic.Directory))
	}
}

// nullableStatusName emits JSON null instead of an empty active Mode identifier.
func nullableStatusName(name string) any {
	if name == "" {
		return nil
	}
	return name
}

// statusTitle converts a stable lowercase status value to its human label.
func statusTitle(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

// coverageStatusColor maps Column coverage to the retained ANSI emphasis rule.
func coverageStatusColor(coverage string) string {
	switch coverage {
	case "full":
		return statusGreen
	case "partial":
		return statusYellow
	default:
		return statusRed
	}
}

// emphasizeStatusText applies ANSI emphasis only to color-capable human terminal output.
func emphasizeStatusText(context *commandContext, color string, text string) string {
	if text == "" || context.json || !shouldUseStatusColor(context) {
		return text
	}
	return color + text + statusReset
}

// shouldUseStatusColor preserves terminal, NO_COLOR, and TERM=dumb ANSI behavior.
func shouldUseStatusColor(context *commandContext) bool {
	if _, disabled := context.dependencies.Environment["NO_COLOR"]; disabled {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(context.dependencies.Environment["TERM"]), "dumb") {
		return false
	}
	file, ok := context.dependencies.Stdout.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// statusPlanContext contains injected planner dependencies for conservative status matching.
type statusPlanContext struct {
	options planner.PlanOptions
}

// newStatusPlanContext builds planner dependencies without reading process globals.
func newStatusPlanContext(dependencies Dependencies) statusPlanContext {
	return statusPlanContext{options: planner.PlanOptions{
		HomeDir: dependencies.HomeDir,
		Env:     dependencies.Environment,
		OS:      dependencies.OperatingSystem,
	}}
}

// matchedModeIntent identifies a persisted Mode only while its replanned mappings still match.
func matchedModeIntent(project warehouse.Project, state linker.CurrentState, context statusPlanContext) (warehouse.Mode, bool) {
	if state.Intent == nil || state.Intent.Kind != "mode" || strings.TrimSpace(state.Intent.Mode) == "" {
		return warehouse.Mode{}, false
	}
	mode, err := project.ResolveMode(state.Intent.Mode)
	if err != nil {
		return warehouse.Mode{}, false
	}
	plannedMappings, err := planner.PlanModeMappings(project, mode.Name, state.Mappings, context.options)
	if err != nil {
		var missing planner.MissingResourceError
		if errors.As(err, &missing) {
			return mode, true
		}
		return warehouse.Mode{}, false
	}
	if !sameMappingSet(plannedMappings, state.Mappings) {
		return warehouse.Mode{}, false
	}
	return mode, true
}

// columnCoverage reports none, partial, or full current mapping coverage for one Column.
func columnCoverage(project warehouse.Project, column warehouse.Column, currentMappings map[mappingPair]struct{}, context statusPlanContext) string {
	total := 0
	full := 0
	for _, setting := range column.Settings {
		if setting.Missing {
			continue
		}
		total++
		planned, err := planner.PlanColumnMappings(project, column.Name, []string{setting.Name}, context.options)
		if err != nil || len(planned) == 0 {
			continue
		}
		matched := 0
		for _, mapping := range planned {
			if _, ok := currentMappings[mappingPair{Source: mapping.Source, Target: mapping.Target}]; ok {
				matched++
			}
		}
		if matched > 0 && matched < len(planned) {
			return "partial"
		}
		if matched == len(planned) {
			full++
		}
	}
	switch {
	case full == 0:
		return "none"
	case full == total:
		return "full"
	default:
		return "partial"
	}
}

// mappingPair is the comparable identity of one managed source-target relationship.
type mappingPair struct {
	Source string
	Target string
}

// mappingSet converts mappings into an unordered source-target lookup set.
func mappingSet(mappings []linker.Mapping) map[mappingPair]struct{} {
	result := make(map[mappingPair]struct{}, len(mappings))
	for _, mapping := range mappings {
		result[mappingPair{Source: mapping.Source, Target: mapping.Target}] = struct{}{}
	}
	return result
}

// sameMappingSet compares mapping slices as unordered source-target sets.
func sameMappingSet(left []linker.Mapping, right []linker.Mapping) bool {
	if len(left) != len(right) {
		return false
	}
	leftSet := mappingSet(left)
	rightSet := mappingSet(right)
	if len(leftSet) != len(left) || len(rightSet) != len(right) {
		return false
	}
	for pair := range leftSet {
		if _, ok := rightSet[pair]; !ok {
			return false
		}
	}
	return true
}

// sortedKeys returns deterministic map keys for human and JSON output.
func sortedKeys[V any](input map[string]V) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// displayLabel preserves a canonical identifier when a distinct display name is used.
func displayLabel(displayName string, canonicalID string) string {
	displayName = strings.TrimSpace(displayName)
	canonicalID = strings.TrimSpace(canonicalID)
	if displayName == "" || displayName == canonicalID {
		return canonicalID
	}
	return fmt.Sprintf("%s [%s]", displayName, canonicalID)
}

// displayStatusName returns the preferred concise display-facing name.
func displayStatusName(displayName string, canonicalID string) string {
	if strings.TrimSpace(displayName) == "" {
		return strings.TrimSpace(canonicalID)
	}
	return strings.TrimSpace(displayName)
}

// transactionDiagnostics reports prepared repository work without recovering it.
func transactionDiagnostics(dependencies Dependencies) ([]repository.TransactionInfo, error) {
	rootPath, err := effectiveWarehouseRoot(dependencies)
	if err != nil {
		return nil, err
	}
	return repository.New(rootPath).Diagnostics()
}
