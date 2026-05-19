package cli

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/xenon/ConfigFacilitator/internal/linker"
	"github.com/xenon/ConfigFacilitator/internal/planner"
	"github.com/xenon/ConfigFacilitator/internal/scaffold"
	"github.com/xenon/ConfigFacilitator/internal/session"
	"github.com/xenon/ConfigFacilitator/internal/syncer"
	"github.com/xenon/ConfigFacilitator/internal/warehouse"
)

var commandDescriptions = []struct {
	name        string
	description string
}{
	{name: "new", description: "Scaffold project, column, and mode templates"},
	{name: "sync", description: "Reconcile warehouse indexes with filesystem state"},
	{name: "switch", description: "Select the active project context for this session"},
	{name: "list", description: "Inspect projects, columns, modes, and settings"},
	{name: "apply", description: "Activate a mode or explicit settings selection"},
	{name: "reset", description: "Remove the current project's managed links"},
	{name: "revert", description: "Restore the previous apply state for a project"},
}

// Run executes the cfgfc CLI and returns a process exit code.
func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	executablePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "resolve executable path: %v\n", err)
		return 1
	}
	return RunWithExecutable(args, stdout, stderr, executablePath)
}

// RunWithExecutable executes the cfgfc CLI against an injected executable path.
func RunWithExecutable(args []string, stdout io.Writer, stderr io.Writer, executablePath string) int {
	if len(args) == 0 || isHelpArg(args[0]) {
		writeRootHelp(stdout)
		return 0
	}

	commandName := args[0]
	commandArgs := args[1:]
	warehouseRoot := scaffold.WarehouseRootForExecutable(executablePath)
	switch commandName {
	case "new":
		return runNew(commandArgs, stdout, stderr, warehouseRoot)
	case "sync":
		return runSync(commandArgs, stdout, stderr, warehouseRoot)
	case "switch":
		return runSwitch(commandArgs, stdout, stderr, warehouseRoot)
	case "list":
		return runList(commandArgs, stdout, stderr, warehouseRoot)
	case "apply":
		return runApply(commandArgs, stdout, stderr, warehouseRoot)
	case "reset":
		return runReset(commandArgs, stdout, stderr, warehouseRoot)
	case "revert":
		return runRevert(commandArgs, stdout, stderr, warehouseRoot)
	}

	for _, command := range commandDescriptions {
		if command.name == commandName {
			writePlaceholder(commandName, stdout)
			return 0
		}
	}

	fmt.Fprintf(stderr, "unknown command %q\n\n", commandName)
	writeRootHelp(stderr)
	return 1
}

func isHelpArg(arg string) bool {
	return arg == "--help" || arg == "-h" || arg == "help"
}

func writeRootHelp(writer io.Writer) {
	fmt.Fprintln(writer, "cfgfc manages portable configuration warehouses.")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Usage:")
	fmt.Fprintln(writer, "  cfgfc <command> [flags]")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Available Commands:")
	for _, command := range commandDescriptions {
		fmt.Fprintf(writer, "  %-8s %s\n", command.name, command.description)
	}
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Use \"cfgfc <command> --help\" once a command family is implemented.")
}

func writePlaceholder(commandName string, writer io.Writer) {
	fmt.Fprintf(
		writer,
		"The %s command family is registered, but its behavior will be implemented in a later OpenSpec slice.\n",
		strings.TrimSpace(commandName),
	)
}

func runNew(args []string, stdout io.Writer, stderr io.Writer, warehouseRoot string) int {
	projectName, columnName, modeName, err := parseNewArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	switch {
	case projectName != "" && columnName == "" && modeName == "":
		err = scaffold.CreateProject(warehouseRoot, projectName)
		if err == nil {
			fmt.Fprintf(stdout, "Created project scaffold %q in %s\n", projectName, warehouseRoot)
		}
	case projectName != "" && columnName != "" && modeName == "":
		err = scaffold.CreateColumn(warehouseRoot, projectName, columnName)
		if err == nil {
			fmt.Fprintf(stdout, "Created column scaffold %q in project %q\n", columnName, projectName)
		}
	case projectName != "" && columnName == "" && modeName != "":
		err = scaffold.CreateMode(warehouseRoot, projectName, modeName)
		if err == nil {
			fmt.Fprintf(stdout, "Created mode scaffold %q in project %q\n", modeName, projectName)
		}
	default:
		fmt.Fprintln(stderr, "new requires one of: -p <project>, -p <project> -c <column>, or -p <project> -m <mode>")
		return 1
	}
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	return 0
}

func runSync(args []string, stdout io.Writer, stderr io.Writer, warehouseRoot string) int {
	projectName, err := parseSyncArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if projectName == "" {
		err = syncer.SyncAll(warehouseRoot)
		if err == nil {
			fmt.Fprintf(stdout, "Synchronized all projects in %s\n", warehouseRoot)
		}
	} else {
		err = syncer.SyncProject(warehouseRoot, projectName)
		if err == nil {
			fmt.Fprintf(stdout, "Synchronized project %q\n", projectName)
		}
	}
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	return 0
}

func parseNewArgs(args []string) (string, string, string, error) {
	var projectName string
	var columnName string
	var modeName string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-p":
			if i+1 >= len(args) {
				return "", "", "", fmt.Errorf("-p requires a value")
			}
			projectName = args[i+1]
			i++
		case "-c":
			if i+1 >= len(args) {
				return "", "", "", fmt.Errorf("-c requires a value")
			}
			columnName = args[i+1]
			i++
		case "-m":
			if i+1 >= len(args) {
				return "", "", "", fmt.Errorf("-m requires a value")
			}
			modeName = args[i+1]
			i++
		default:
			return "", "", "", fmt.Errorf("unexpected arg %q", args[i])
		}
	}
	return projectName, columnName, modeName, nil
}

func parseSyncArgs(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	if len(args) == 2 && args[0] == "-p" {
		return args[1], nil
	}
	if len(args) > 0 && args[0] == "-p" && len(args) < 2 {
		return "", fmt.Errorf("-p requires a value")
	}
	return "", fmt.Errorf("sync accepts either no args or -p <project>")
}

func runSwitch(args []string, stdout io.Writer, stderr io.Writer, warehouseRoot string) int {
	projectName, err := parseSwitchArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	loaded, err := warehouse.LoadWarehouse(warehouseRoot)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if _, ok := loaded.Projects[projectName]; !ok {
		fmt.Fprintf(stderr, "project %q does not exist\n", projectName)
		return 1
	}
	store := session.NewStore(warehouseRoot)
	if err := store.Set(os.Getppid(), projectName); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Switched active project to %q\n", projectName)
	return 0
}

func runList(args []string, stdout io.Writer, stderr io.Writer, warehouseRoot string) int {
	projectName, columnName, modeName, err := parseListArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	store := session.NewStore(warehouseRoot)
	effectiveProject, _, err := session.ResolveProject(projectName, os.Getppid(), store)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	loaded, err := warehouse.LoadWarehouse(warehouseRoot)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if effectiveProject == "" {
		return renderProjectList(loaded, stdout)
	}
	project, ok := loaded.Projects[effectiveProject]
	if !ok {
		fmt.Fprintf(stderr, "project %q does not exist\n", effectiveProject)
		return 1
	}
	if columnName != "" {
		return renderColumn(project, columnName, stdout, stderr)
	}
	if modeName != "" {
		return renderMode(project, modeName, stdout, stderr)
	}
	return renderProject(project, stdout)
}

func parseSwitchArgs(args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("switch requires exactly one project name")
	}
	return args[0], nil
}

func parseListArgs(args []string) (string, string, string, error) {
	var projectName string
	var columnName string
	var modeName string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-p":
			if i+1 >= len(args) {
				return "", "", "", fmt.Errorf("-p requires a value")
			}
			projectName = args[i+1]
			i++
		case "-c":
			if i+1 >= len(args) {
				return "", "", "", fmt.Errorf("-c requires a value")
			}
			columnName = args[i+1]
			i++
		case "-m":
			if i+1 >= len(args) {
				return "", "", "", fmt.Errorf("-m requires a value")
			}
			modeName = args[i+1]
			i++
		default:
			return "", "", "", fmt.Errorf("unexpected arg %q", args[i])
		}
	}
	if columnName != "" && modeName != "" {
		return "", "", "", fmt.Errorf("list accepts only one of -c or -m")
	}
	return projectName, columnName, modeName, nil
}

func renderProjectList(loaded warehouse.Warehouse, stdout io.Writer) int {
	names := make([]string, 0, len(loaded.Projects))
	for name := range loaded.Projects {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintln(stdout, name)
	}
	return 0
}

func renderProject(project warehouse.Project, stdout io.Writer) int {
	fmt.Fprintf(stdout, "Project: %s\n", project.Name)
	fmt.Fprintln(stdout, "Columns:")
	columnNames := sortedKeys(project.Columns)
	for _, name := range columnNames {
		fmt.Fprintf(stdout, "  - %s\n", name)
	}
	fmt.Fprintln(stdout, "Modes:")
	modeNames := sortedKeys(project.Modes)
	for _, name := range modeNames {
		fmt.Fprintf(stdout, "  - %s\n", name)
	}
	return 0
}

func renderColumn(project warehouse.Project, columnName string, stdout io.Writer, stderr io.Writer) int {
	column, ok := project.Columns[columnName]
	if !ok {
		fmt.Fprintf(stderr, "column %q does not exist in project %q\n", columnName, project.Name)
		return 1
	}
	fmt.Fprintf(stdout, "Column: %s\n", column.Name)
	for _, name := range sortedKeys(column.Settings) {
		setting := column.Settings[name]
		status := "present"
		if setting.Missing {
			status = "missing"
		}
		fmt.Fprintf(stdout, "  - %s (%s)\n", name, status)
	}
	return 0
}

func renderMode(project warehouse.Project, modeName string, stdout io.Writer, stderr io.Writer) int {
	mode, ok := project.Modes[modeName]
	if !ok {
		fmt.Fprintf(stderr, "mode %q does not exist in project %q\n", modeName, project.Name)
		return 1
	}
	fmt.Fprintf(stdout, "Mode: %s\n", mode.Name)
	for _, name := range sortedKeys(mode.Metadata.Columns) {
		selection := mode.Metadata.Columns[name]
		fmt.Fprintf(stdout, "  - %s: strategy=%s settings=%s\n", name, selection.Strategy, strings.Join(selection.Settings, ","))
	}
	return 0
}

func runApply(args []string, stdout io.Writer, stderr io.Writer, warehouseRoot string) int {
	projectName, modeName, columnName, settingsInput, err := parseApplyArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	project, err := resolveProjectForCommand(warehouseRoot, projectName)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	engine := linker.New()
	currentState, err := engine.LoadCurrentState(project)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	planOptions := planner.PlanOptions{HomeDir: os.Getenv("HOME"), Env: envMap(), OS: runtime.GOOS}
	var mappings []linker.Mapping
	switch {
	case modeName != "" && columnName == "" && settingsInput == "":
		mappings, err = planner.PlanModeMappings(project, modeName, currentState.Mappings, planOptions)
		if err == nil {
			err = engine.ReplaceMappings(project, mappings)
		}
		if err == nil {
			fmt.Fprintf(stdout, "Applied mode %q for project %q\n", modeName, project.Name)
		}
	case modeName == "" && columnName != "" && settingsInput != "":
		mappings, err = planner.PlanColumnMappings(project, columnName, planner.ParseSettingList(settingsInput), planOptions)
		if err == nil {
			err = engine.ReplaceMappings(project, mappings)
		}
		if err == nil {
			fmt.Fprintf(stdout, "Applied column %q for project %q\n", columnName, project.Name)
		}
	default:
		fmt.Fprintln(stderr, "apply requires either -m <mode> or -c <column> -s <settings>")
		return 1
	}
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	return 0
}

func runReset(args []string, stdout io.Writer, stderr io.Writer, warehouseRoot string) int {
	projectName, err := parseProjectOptionalArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	project, err := resolveProjectForCommand(warehouseRoot, projectName)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if err := linker.New().Reset(project); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Reset project %q\n", project.Name)
	return 0
}

func runRevert(args []string, stdout io.Writer, stderr io.Writer, warehouseRoot string) int {
	projectName, err := parseProjectOptionalArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	project, err := resolveProjectForCommand(warehouseRoot, projectName)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	engine := linker.New()
	previous, err := engine.LoadPreviousSnapshot(project)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if err := engine.ReplaceMappings(project, previous); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Reverted project %q\n", project.Name)
	return 0
}

func parseApplyArgs(args []string) (string, string, string, string, error) {
	var projectName, modeName, columnName, settingsInput string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-p":
			if i+1 >= len(args) {
				return "", "", "", "", fmt.Errorf("-p requires a value")
			}
			projectName = args[i+1]
			i++
		case "-m":
			if i+1 >= len(args) {
				return "", "", "", "", fmt.Errorf("-m requires a value")
			}
			modeName = args[i+1]
			i++
		case "-c":
			if i+1 >= len(args) {
				return "", "", "", "", fmt.Errorf("-c requires a value")
			}
			columnName = args[i+1]
			i++
		case "-s":
			if i+1 >= len(args) {
				return "", "", "", "", fmt.Errorf("-s requires a value")
			}
			settingsInput = args[i+1]
			i++
		default:
			return "", "", "", "", fmt.Errorf("unexpected arg %q", args[i])
		}
	}
	return projectName, modeName, columnName, settingsInput, nil
}

func parseProjectOptionalArgs(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	if len(args) == 2 && args[0] == "-p" {
		return args[1], nil
	}
	if len(args) > 0 && args[0] == "-p" && len(args) < 2 {
		return "", fmt.Errorf("-p requires a value")
	}
	return "", fmt.Errorf("command accepts either no args or -p <project>")
}

func resolveProjectForCommand(warehouseRoot string, explicitProject string) (warehouse.Project, error) {
	store := session.NewStore(warehouseRoot)
	effectiveProject, _, err := session.ResolveProject(explicitProject, os.Getppid(), store)
	if err != nil {
		return warehouse.Project{}, err
	}
	if effectiveProject == "" {
		return warehouse.Project{}, fmt.Errorf("no active project; provide -p or run switch")
	}
	loaded, err := warehouse.LoadWarehouse(warehouseRoot)
	if err != nil {
		return warehouse.Project{}, err
	}
	project, ok := loaded.Projects[effectiveProject]
	if !ok {
		return warehouse.Project{}, fmt.Errorf("project %q does not exist", effectiveProject)
	}
	return project, nil
}

func envMap() map[string]string {
	result := map[string]string{}
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

func sortedKeys[V any](input map[string]V) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
