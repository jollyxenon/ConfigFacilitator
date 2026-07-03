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

type cliEmphasis string

const (
	cliEmphasisGreen  cliEmphasis = "\033[32m"
	cliEmphasisYellow cliEmphasis = "\033[33m"
	cliEmphasisRed    cliEmphasis = "\033[31m"
	cliEmphasisReset  cliEmphasis = "\033[0m"
)

// version is the current version of the cfgfc CLI, injected at build time via ldflags.
var version = "dev"

var commandDescriptions = []struct {
	name        string
	description string
}{
	{name: "new", description: "Scaffold project, column, and mode templates"},
	{name: "sync", description: "Reconcile warehouse indexes with filesystem state"},
	{name: "switch", description: "Select the active project context for this session"},
	{name: "list", description: "Inspect projects, columns, modes, and current usage status"},
	{name: "apply", description: "Activate a mode or explicit settings selection"},
	{name: "update", description: "Refresh the last applied intent from current warehouse metadata"},
	{name: "reset", description: "Remove the current project's managed links"},
	{name: "revert", description: "Restore the previous apply state for a project"},
	{name: "root", description: "Inspect or change the persistent warehouse root"},
}

type helpFlag struct {
	usage       string
	description string
}

type commandHelp struct {
	description string
	usage       []string
	notes       []string
	flags       []helpFlag
	examples    []string
}

// applyArgs stores parsed flags for the apply workflow.
type applyArgs struct {
	projectName   string
	modeName      string
	columnName    string
	settingsInput string
	force         bool
}

// projectOptionalArgs stores optional project-scoped command flags.
type projectOptionalArgs struct {
	projectName string
	force       bool
}

var commandHelpByName = map[string]commandHelp{
	"new": {
		description: "Scaffold project, column, and mode templates.",
		usage: []string{
			"cfgfc new -p <project>",
			"cfgfc new -p <project> -c <column>",
			"cfgfc new -p <project> -m <mode>",
			"cfgfc new -c <column>",
			"cfgfc new -m <mode>",
		},
		notes: []string{
			"`global` is reserved and cannot be used as a project name.",
			"After `cfgfc switch <project>`, project-scoped `new` forms can omit `-p`.",
			"Existing projects can be referenced by canonical name or alias.",
			"`new` accepts one scaffold target at a time: project, column, or mode.",
		},
		flags: []helpFlag{
			{usage: "-p <project>", description: "Create within the named project, or create the project itself when used alone."},
			{usage: "-c <column>", description: "Create a column scaffold inside the effective project."},
			{usage: "-m <mode>", description: "Create a mode scaffold inside the effective project."},
		},
		examples: []string{
			"cfgfc new -p OpenCode",
			"cfgfc new -p OpenCode -c Skills",
			"cfgfc new -p OpenCode -m Max",
			"cfgfc switch OpenCode",
			"cfgfc new -c Skills",
			"cfgfc new -m Max",
		},
	},
	"sync": {
		description: "Reconcile warehouse indexes with filesystem state.",
		usage: []string{
			"cfgfc sync",
			"cfgfc sync -p <project>",
			"cfgfc sync --all",
			"cfgfc sync -a",
		},
		notes: []string{
			"When a switched project is active, `cfgfc sync` targets that project by default.",
			"Without an active project, `cfgfc sync` falls back to a full-warehouse sync.",
			"Project references accept canonical names and aliases.",
			"`--all` and `-a` ignore switched-project context and force a warehouse-wide sync.",
			"`global` is reserved and cannot be used with `-p`.",
		},
		flags: []helpFlag{
			{usage: "-p <project>", description: "Synchronize only the named project."},
			{usage: "--all", description: "Synchronize every project in the warehouse and ignore active context."},
			{usage: "-a", description: "Alias for `--all`."},
		},
		examples: []string{
			"cfgfc sync",
			"cfgfc sync -p OpenCode",
			"cfgfc sync --all",
			"cfgfc sync -a",
		},
	},
	"switch": {
		description: "Select the active project context for this session.",
		usage: []string{
			"cfgfc switch <project>",
			"cfgfc switch global",
		},
		notes: []string{
			"The active project is stored for the current PPID-scoped session.",
			"`switch` resolves projects by canonical name or alias and stores the normalized project identifier.",
			"`cfgfc switch global` clears the active project context and returns later commands to global resolution.",
		},
		flags: []helpFlag{
			{usage: "<project>", description: "Existing project name to store as the active project context."},
			{usage: "global", description: "Clear the active project context for the current session."},
		},
		examples: []string{
			"cfgfc switch OpenCode",
			"cfgfc switch global",
		},
	},
	"list": {
		description: "Inspect projects, columns, modes, and current usage status.",
		usage: []string{
			"cfgfc list",
			"cfgfc list -p <project>",
			"cfgfc list -p <project> -c <column>",
			"cfgfc list -p <project> -m <mode>",
		},
		notes: []string{
			"Without an effective project, `cfgfc list` shows each project's persisted mode name when available, or `(Unmatched)` / `(None)` when no mode summary can be resolved.",
			"After `cfgfc switch <project>`, project-scoped `list` forms can omit `-p`.",
			"Project-scoped `cfgfc list` adds `(Full)` / `(Partial)` / `(None)` to each column and highlights the active mode when terminal output supports color.",
			"`cfgfc list -c <column>` highlights enabled settings when terminal output supports color and still prints missing index entries as `(missing)`.",
			"Project, column, and mode references accept canonical names and aliases.",
			"`list` accepts only one detailed target at a time: `-c` or `-m`.",
		},
		flags: []helpFlag{
			{usage: "-p <project>", description: "Inspect the named project instead of using switched-project context."},
			{usage: "-c <column>", description: "Show the named column's settings inside the effective project."},
			{usage: "-m <mode>", description: "Show the named mode's column selections inside the effective project."},
		},
		examples: []string{
			"cfgfc list",
			"cfgfc list -p OpenCode",
			"cfgfc switch OpenCode",
			"cfgfc list -c Skills",
			"cfgfc list -p OpenCode -m Max",
		},
	},
	"apply": {
		description: "Activate a mode or explicit settings selection.",
		usage: []string{
			"cfgfc apply -p <project> -m <mode>",
			"cfgfc apply -p <project> -m <mode> --force",
			"cfgfc apply -m <mode>",
			"cfgfc apply -p <project> -c <column> -s <settings>",
			"cfgfc apply -p <project> -c <column> -s <settings> -f",
			"cfgfc apply -c <column> -s <settings>",
		},
		notes: []string{
			"`apply` accepts either mode apply (`-m`) or single-column apply (`-c` with `-s`).",
			"After `cfgfc switch <project>`, project-scoped apply forms can omit `-p`.",
			"Project, column, mode, and setting references accept canonical names and aliases.",
			"`-s` accepts one or more comma-separated setting names for single-column apply.",
			"`-f` and `--force` reclaim occupied targets by deleting files, symlinks, or directories recursively, and only guarantee restoration of the last confirmed managed state.",
		},
		flags: []helpFlag{
			{usage: "-p <project>", description: "Apply within the named project instead of using switched-project context."},
			{usage: "-m <mode>", description: "Apply the named mode."},
			{usage: "-c <column>", description: "Apply one column directly."},
			{usage: "-s <settings>", description: "Comma-separated settings to apply for the named column."},
			{usage: "-f, --force", description: "Delete occupied target paths recursively and continue even when targets are unmanaged or drifted."},
		},
		examples: []string{
			"cfgfc apply -p OpenCode -m Max",
			"cfgfc switch OpenCode",
			"cfgfc apply -m Max",
			"cfgfc apply -p OpenCode -c opencode.json -s GPT.json",
			"cfgfc apply -c opencode.json -s GPT.json,CLAUDE.json",
		},
	},
	"update": {
		description: "Refresh the last applied intent from current warehouse metadata.",
		usage: []string{
			"cfgfc update",
			"cfgfc update -p <project>",
			"cfgfc update -p <project> --force",
			"cfgfc update --project <project>",
			"cfgfc update -p <project> -c <column>",
			"cfgfc update -p <project> -c <column> -f",
			"cfgfc update --all",
			"cfgfc update -a",
		},
		notes: []string{
			"`update` replans the persisted mode or column apply intent when available, so mode `full` columns can include newly synced settings.",
			"When no apply intent is recorded, `update` refreshes current mappings by matching active sources back to current metadata.",
			"Run `cfgfc sync` before `cfgfc update` when newly added files or directories need to be reflected in indexes.",
			"After `cfgfc switch <project>`, project-scoped update forms can omit `-p`.",
			"Project and column references accept canonical names and aliases.",
			"`--all` and `-a` ignore switched-project context and skip projects with no active mappings or intent.",
			"Use `-c` or `--column` to refresh only one active column while preserving other current mappings.",
			"`-f` and `--force` reclaim occupied targets by deleting files, symlinks, or directories recursively, and only guarantee restoration of the last confirmed managed state.",
		},
		flags: []helpFlag{
			{usage: "-p <project>", description: "Update the named project instead of using switched-project context."},
			{usage: "--project <project>", description: "Long form of `-p`."},
			{usage: "-c <column>", description: "Refresh only the named column's active mappings."},
			{usage: "--column <column>", description: "Long form of `-c`."},
			{usage: "--all", description: "Update every project with active mappings and ignore active context."},
			{usage: "-a", description: "Alias for `--all`."},
			{usage: "-f, --force", description: "Delete occupied target paths recursively and continue even when targets are unmanaged or drifted."},
		},
		examples: []string{
			"cfgfc update -p OpenCode",
			"cfgfc switch OpenCode",
			"cfgfc update",
			"cfgfc update -c Skills",
			"cfgfc update --all",
		},
	},
	"reset": {
		description: "Remove the current project's managed links.",
		usage: []string{
			"cfgfc reset",
			"cfgfc reset -p <project>",
			"cfgfc reset -p <project> --force",
		},
		notes: []string{
			"After `cfgfc switch <project>`, `reset` can omit `-p` and use the active project context.",
			"Project references accept canonical names and aliases.",
			"`reset` removes the resolved project's current managed mappings.",
			"`-f` and `--force` delete every recorded target path for the project, even when ownership has drifted, and do not restore overwritten unmanaged contents.",
		},
		flags: []helpFlag{
			{usage: "-p <project>", description: "Reset the named project instead of using switched-project context."},
			{usage: "-f, --force", description: "Delete every recorded target path recursively, even when it is no longer owned by the recorded source."},
		},
		examples: []string{
			"cfgfc reset -p OpenCode",
			"cfgfc switch OpenCode",
			"cfgfc reset",
		},
	},
	"revert": {
		description: "Restore the previous apply state for a project.",
		usage: []string{
			"cfgfc revert",
			"cfgfc revert -p <project>",
			"cfgfc revert -p <project> --force",
		},
		notes: []string{
			"After `cfgfc switch <project>`, `revert` can omit `-p` and use the active project context.",
			"Project references accept canonical names and aliases.",
			"`revert` restores the most recent previous snapshot recorded for the resolved project.",
			"`-f` and `--force` reclaim occupied targets recursively and restore only the last confirmed managed snapshot, not overwritten unmanaged contents.",
		},
		flags: []helpFlag{
			{usage: "-p <project>", description: "Revert the named project instead of using switched-project context."},
			{usage: "-f, --force", description: "Delete occupied target paths recursively and restore the previous managed snapshot despite unmanaged conflicts or drift."},
		},
		examples: []string{
			"cfgfc revert -p OpenCode",
			"cfgfc switch OpenCode",
			"cfgfc revert",
		},
	},
	"root": {
		description: "Inspect or change the persistent warehouse root.",
		usage: []string{
			"cfgfc root",
			"cfgfc root <path>",
		},
		notes: []string{
			"Without a path argument, `root` prints the current effective warehouse root.",
			"With one path argument, `root` normalizes and persists the effective warehouse root for later commands.",
			"Changing the warehouse root only changes where later commands look for warehouse data; it does not migrate, copy, or initialize warehouse contents.",
		},
		examples: []string{
			"cfgfc root",
			"cfgfc root ~/.configfacilitator-alt",
		},
	},
}

const globalProjectName = "global"

type syncArgs struct {
	projectName string
	forceAll    bool
}

type updateArgs struct {
	projectName string
	columnName  string
	forceAll    bool
	force       bool
}

// Run executes the cfgfc CLI and returns a process exit code.
func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	return run(args, stdout, stderr)
}

// RunWithExecutable executes the cfgfc CLI against an injected executable path.
func RunWithExecutable(args []string, stdout io.Writer, stderr io.Writer, executablePath string) int {
	_ = executablePath
	return run(args, stdout, stderr)
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	// Check for version flag before any other processing.
	if len(args) > 0 && isVersionArg(args[0]) {
		fmt.Fprintln(stdout, version)
		return 0
	}

	if len(args) >= 2 && args[0] == "help" {
		return writeNamedHelp(args[1], stdout, stderr)
	}

	if len(args) == 0 || isHelpArg(args[0]) {
		writeRootHelp(stdout)
		return 0
	}

	commandName := args[0]
	commandArgs := args[1:]
	if hasHelpArg(commandArgs) {
		return writeNamedHelp(commandName, stdout, stderr)
	}
	if commandName == "root" {
		return runRoot(commandArgs, stdout, stderr)
	}

	warehouseRoot, err := scaffold.WarehouseRoot()
	if err != nil {
		fmt.Fprintf(stderr, "resolve warehouse root: %v\n", err)
		return 1
	}
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
	case "update":
		return runUpdate(commandArgs, stdout, stderr, warehouseRoot)
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

// runRoot inspects or updates the persisted warehouse root override.
func runRoot(args []string, stdout io.Writer, stderr io.Writer) int {
	switch len(args) {
	case 0:
		warehouseRoot, err := warehouse.EffectiveWarehouseRoot()
		if err != nil {
			fmt.Fprintf(stderr, "resolve warehouse root: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, warehouseRoot)
		return 0
	case 1:
		warehouseRoot, err := warehouse.SetEffectiveWarehouseRoot(args[0])
		if err != nil {
			fmt.Fprintf(stderr, "set warehouse root: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Warehouse root set to %s\n", warehouseRoot)
		return 0
	default:
		fmt.Fprintln(stderr, "root accepts zero arguments to print the current root or one path argument to set it")
		return 1
	}
}

func isHelpArg(arg string) bool {
	return arg == "--help" || arg == "-h" || arg == "help"
}

// isVersionArg reports whether the argument is a version flag.
func isVersionArg(arg string) bool {
	return arg == "--version" || arg == "-v"
}

// hasHelpArg reports whether the provided argument list contains an explicit help request.
func hasHelpArg(args []string) bool {
	for _, arg := range args {
		if isHelpArg(arg) {
			return true
		}
	}
	return false
}

// writeRootHelp renders the root command help surface.
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
	fmt.Fprintln(writer, "Project resolution:")
	fmt.Fprintln(writer, "  Project-scoped references accept canonical names and aliases.")
	fmt.Fprintln(writer, "  After `cfgfc switch <project>`, project-scoped commands can omit `-p`.")
	fmt.Fprintln(writer, "  `cfgfc switch global` clears the active project context for the current session.")
	fmt.Fprintln(writer, "  `cfgfc sync --all` or `cfgfc sync -a` forces a full-warehouse sync and ignores any active project context.")
	fmt.Fprintln(writer, "  `cfgfc update --all` or `cfgfc update -a` refreshes all projects with active state and ignores context.")
	fmt.Fprintln(writer, "  `cfgfc sync` targets the active project when one is set; otherwise it syncs all projects.")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Warehouse root:")
	fmt.Fprintln(writer, "  `cfgfc root` prints the effective warehouse root for later commands.")
	fmt.Fprintln(writer, "  `cfgfc root <path>` changes later warehouse resolution without moving existing warehouse contents.")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Flags:")
	fmt.Fprintln(writer, "  --version, -v   Print the version and exit.")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Development:")
	fmt.Fprintln(writer, "  Use `pixi run compile` to verify all Go packages compile.")
	fmt.Fprintln(writer, "  Use `pixi run build` to create the local CLI binary at `dist/cfgfc`.")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Examples:")
	fmt.Fprintln(writer, "  cfgfc new -p OpenCode")
	fmt.Fprintln(writer, "  cfgfc switch OpenCode")
	fmt.Fprintln(writer, "  cfgfc sync --all")
	fmt.Fprintln(writer, "  cfgfc update --all")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Use \"cfgfc <command> --help\" for more information about a command.")
}

// writeNamedHelp renders help for one registered command and reports whether it succeeded.
func writeNamedHelp(commandName string, stdout io.Writer, stderr io.Writer) int {
	if _, ok := commandHelpByName[commandName]; ok {
		writeCommandHelp(commandName, stdout)
		return 0
	}

	fmt.Fprintf(stderr, "unknown command %q\n\n", commandName)
	writeRootHelp(stderr)
	return 1
}

// writeCommandHelp renders the help surface for one command family.
func writeCommandHelp(commandName string, writer io.Writer) {
	help, ok := commandHelpByName[commandName]
	if !ok {
		writePlaceholder(commandName, writer)
		return
	}

	fmt.Fprintln(writer, help.description)
	writeHelpSection(writer, "Usage:", help.usage)
	if len(help.notes) > 0 {
		writeHelpSection(writer, "Notes:", help.notes)
	}
	if len(help.flags) > 0 {
		fmt.Fprintln(writer)
		fmt.Fprintln(writer, "Flags:")
		for _, flag := range help.flags {
			fmt.Fprintf(writer, "  %-18s %s\n", flag.usage, flag.description)
		}
	}
	writeHelpSection(writer, "Examples:", help.examples)
}

// writeHelpSection prints one titled multi-line help section.
func writeHelpSection(writer io.Writer, title string, lines []string) {
	if len(lines) == 0 {
		return
	}

	fmt.Fprintln(writer)
	fmt.Fprintln(writer, title)
	for _, line := range lines {
		fmt.Fprintf(writer, "  %s\n", line)
	}
}

// writePlaceholder explains that a registered command has no detailed behavior yet.
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
	// Reuse PPID-scoped project context for project-scoped new operations when -p is omitted.
	if projectName == "" && (columnName != "" || modeName != "") {
		projectName, _, err = session.ResolveProject("", os.Getppid(), session.NewStore(warehouseRoot))
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
	}
	if err := validateProjectName(projectName); err != nil {
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
		project, resolveErr := resolveProjectReference(warehouseRoot, projectName)
		if resolveErr != nil {
			fmt.Fprintf(stderr, "%v\n", resolveErr)
			return 1
		}
		err = scaffold.CreateColumn(warehouseRoot, project.Name, columnName)
		if err == nil {
			fmt.Fprintf(stdout, "Created column scaffold %q in project %q\n", columnName, displayStatusName(project.Metadata.DisplayName, project.Name))
		}
	case projectName != "" && columnName == "" && modeName != "":
		project, resolveErr := resolveProjectReference(warehouseRoot, projectName)
		if resolveErr != nil {
			fmt.Fprintf(stderr, "%v\n", resolveErr)
			return 1
		}
		err = scaffold.CreateMode(warehouseRoot, project.Name, modeName)
		if err == nil {
			fmt.Fprintf(stdout, "Created mode scaffold %q in project %q\n", modeName, displayStatusName(project.Metadata.DisplayName, project.Name))
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
	parsed, err := parseSyncArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	projectName := parsed.projectName
	if err := validateProjectName(projectName); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	// Prefer the active switched project, but preserve full-warehouse sync when no project resolves.
	if projectName == "" && !parsed.forceAll {
		projectName, _, err = session.ResolveProject("", os.Getppid(), session.NewStore(warehouseRoot))
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
	}
	if projectName == "" {
		err = syncer.SyncAll(warehouseRoot)
		if err == nil {
			fmt.Fprintf(stdout, "Synchronized all projects in %s\n", warehouseRoot)
		}
	} else {
		err = syncer.SyncProject(warehouseRoot, projectName)
		if err == nil {
			project, resolveErr := resolveProjectReference(warehouseRoot, projectName)
			if resolveErr != nil {
				fmt.Fprintf(stderr, "%v\n", resolveErr)
				return 1
			}
			fmt.Fprintf(stdout, "Synchronized project %q\n", displayStatusName(project.Metadata.DisplayName, project.Name))
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

func parseSyncArgs(args []string) (syncArgs, error) {
	var parsed syncArgs
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-p":
			if i+1 >= len(args) {
				return syncArgs{}, fmt.Errorf("-p requires a value")
			}
			if parsed.forceAll {
				return syncArgs{}, fmt.Errorf("sync does not accept -p together with --all or -a")
			}
			parsed.projectName = args[i+1]
			i++
		case "--all", "-a":
			if parsed.projectName != "" {
				return syncArgs{}, fmt.Errorf("sync does not accept -p together with --all or -a")
			}
			parsed.forceAll = true
		default:
			return syncArgs{}, fmt.Errorf("unexpected arg %q", args[i])
		}
	}
	return parsed, nil
}

func runSwitch(args []string, stdout io.Writer, stderr io.Writer, warehouseRoot string) int {
	projectName, err := parseSwitchArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	store := session.NewStore(warehouseRoot)
	if projectName == globalProjectName {
		if err := store.Clear(os.Getppid()); err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Switched to global context")
		return 0
	}
	loaded, err := warehouse.LoadWarehouse(warehouseRoot)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	project, err := loaded.ResolveProject(projectName)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if err := store.Set(os.Getppid(), project.Name); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Switched active project to %q\n", displayStatusName(project.Metadata.DisplayName, project.Name))
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
	statusContext := newListStatusContext()
	if effectiveProject == "" {
		return renderProjectList(loaded, statusContext, stdout, stderr)
	}
	project, err := loaded.ResolveProject(effectiveProject)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	currentState, err := linker.New().LoadCurrentState(project)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if columnName != "" {
		return renderColumn(project, currentState, columnName, statusContext, stdout, stderr)
	}
	if modeName != "" {
		return renderMode(project, modeName, stdout, stderr)
	}
	return renderProject(project, currentState, statusContext, stdout)
}

func parseSwitchArgs(args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("switch requires exactly one project name")
	}
	return args[0], nil
}

func validateProjectName(projectName string) error {
	if projectName == "" {
		return nil
	}
	if projectName == globalProjectName {
		return fmt.Errorf("project name %q is reserved", projectName)
	}
	return nil
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

func renderProjectList(loaded warehouse.Warehouse, statusContext listStatusContext, stdout io.Writer, stderr io.Writer) int {
	engine := linker.New()
	names := make([]string, 0, len(loaded.Projects))
	for _, project := range loaded.Projects {
		currentState, err := engine.LoadCurrentState(project)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		name := displayLabel(project.Metadata.DisplayName, project.Name)
		names = append(names, fmt.Sprintf("%s %s", name, projectUsageSummary(project, currentState, statusContext, stdout)))
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintln(stdout, name)
	}
	return 0
}

func renderProject(project warehouse.Project, currentState linker.CurrentState, statusContext listStatusContext, stdout io.Writer) int {
	currentMappings := mappingSet(currentState.Mappings)
	activeMode, hasActiveMode := matchedModeIntent(project, currentState, statusContext)
	fmt.Fprintf(stdout, "Project: %s\n", displayLabel(project.Metadata.DisplayName, project.Name))
	fmt.Fprintln(stdout, "Columns:")
	columnNames := sortedKeys(project.Columns)
	for _, name := range columnNames {
		column := project.Columns[name]
		fmt.Fprintf(stdout, "  - %s %s\n", displayLabel(column.Metadata.DisplayName, column.Name), columnUsageSummary(project, column, currentMappings, statusContext, stdout))
	}
	fmt.Fprintln(stdout, "Modes:")
	modeNames := sortedKeys(project.Modes)
	for _, name := range modeNames {
		mode := project.Modes[name]
		line := fmt.Sprintf("  - %s", displayLabel(mode.Metadata.DisplayName, mode.Name))
		if hasActiveMode && mode.Name == activeMode.Name {
			line = emphasizeText(stdout, cliEmphasisGreen, line)
		}
		fmt.Fprintln(stdout, line)
	}
	return 0
}

func renderColumn(project warehouse.Project, currentState linker.CurrentState, columnName string, statusContext listStatusContext, stdout io.Writer, stderr io.Writer) int {
	column, err := project.ResolveColumn(columnName)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	currentMappings := mappingSet(currentState.Mappings)
	fmt.Fprintf(stdout, "Column: %s\n", displayLabel(column.Metadata.DisplayName, column.Name))
	for _, name := range sortedKeys(column.Settings) {
		setting := column.Settings[name]
		status := "present"
		if setting.Missing {
			status = "missing"
		}
		line := fmt.Sprintf("  - %s (%s)", displayLabel(setting.Metadata.DisplayName, setting.Name), status)
		if settingCoverage(project, column, setting, currentMappings, statusContext) == coverageFull {
			line = emphasizeText(stdout, cliEmphasisGreen, line)
		}
		fmt.Fprintln(stdout, line)
	}
	return 0
}

func renderMode(project warehouse.Project, modeName string, stdout io.Writer, stderr io.Writer) int {
	mode, err := project.ResolveMode(modeName)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Mode: %s\n", displayLabel(mode.Metadata.DisplayName, mode.Name))
	for _, name := range sortedKeys(mode.Metadata.Columns) {
		selection := mode.Metadata.Columns[name]
		fmt.Fprintf(stdout, "  - %s: strategy=%s settings=%s\n", displayModeColumn(project, name), selection.Strategy, strings.Join(displayModeSettings(project, name, selection.Settings), ","))
	}
	return 0
}

type listStatusContext struct {
	planOptions planner.PlanOptions
	canPlan     bool
}

type coverageStatus int

const (
	coverageNone coverageStatus = iota
	coveragePartial
	coverageFull
)

type mappingPair struct {
	Source string
	Target string
}

// newListStatusContext captures the environment required for conservative list status replanning.
func newListStatusContext() listStatusContext {
	statusContext := listStatusContext{
		planOptions: planner.PlanOptions{Env: envMap(), OS: runtime.GOOS},
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return statusContext
	}
	statusContext.planOptions.HomeDir = homeDir
	statusContext.canPlan = true
	return statusContext
}

// projectUsageSummary renders one project's persisted usage summary for the global list view.
func projectUsageSummary(project warehouse.Project, currentState linker.CurrentState, statusContext listStatusContext, stdout io.Writer) string {
	mode, ok := matchedModeIntent(project, currentState, statusContext)
	if ok {
		return emphasizeParenthesized(stdout, cliEmphasisGreen, displayLabel(mode.Metadata.DisplayName, mode.Name))
	}
	if len(currentState.Mappings) > 0 || currentState.Intent != nil {
		return emphasizeParenthesized(stdout, cliEmphasisRed, "Unmatched")
	}
	return emphasizeParenthesized(stdout, cliEmphasisRed, "None")
}

// columnUsageSummary renders one column's active-setting coverage label for the project view.
func columnUsageSummary(project warehouse.Project, column warehouse.Column, currentMappings map[mappingPair]struct{}, statusContext listStatusContext, stdout io.Writer) string {
	totalSettings := 0
	fullyCoveredSettings := 0
	for _, setting := range column.Settings {
		if setting.Missing {
			continue
		}
		totalSettings++
		switch settingCoverage(project, column, setting, currentMappings, statusContext) {
		case coverageFull:
			fullyCoveredSettings++
		case coveragePartial:
			return emphasizeParenthesized(stdout, cliEmphasisYellow, "Partial")
		}
	}
	switch {
	case fullyCoveredSettings == 0:
		return emphasizeParenthesized(stdout, cliEmphasisRed, "None")
	case fullyCoveredSettings == totalSettings:
		return emphasizeParenthesized(stdout, cliEmphasisGreen, "Full")
	default:
		return emphasizeParenthesized(stdout, cliEmphasisYellow, "Partial")
	}
}

// matchedModeIntent resolves one persisted mode intent only when its current mappings still match replanned metadata.
func matchedModeIntent(project warehouse.Project, currentState linker.CurrentState, statusContext listStatusContext) (warehouse.Mode, bool) {
	if currentState.Intent == nil || currentState.Intent.Kind != "mode" || strings.TrimSpace(currentState.Intent.Mode) == "" {
		return warehouse.Mode{}, false
	}
	mode, err := project.ResolveMode(currentState.Intent.Mode)
	if err != nil {
		return warehouse.Mode{}, false
	}
	if !statusContext.canPlan {
		return warehouse.Mode{}, false
	}
	plannedMappings, err := planner.PlanModeMappings(project, mode.Name, currentState.Mappings, statusContext.planOptions)
	if err != nil {
		return warehouse.Mode{}, false
	}
	if !sameMappingSet(plannedMappings, currentState.Mappings) {
		return warehouse.Mode{}, false
	}
	return mode, true
}

// settingCoverage reports whether one setting's full current metadata mapping set is present in current state.
func settingCoverage(project warehouse.Project, column warehouse.Column, setting warehouse.Setting, currentMappings map[mappingPair]struct{}, statusContext listStatusContext) coverageStatus {
	if setting.Missing || !statusContext.canPlan {
		return coverageNone
	}
	expectedMappings, ok := plannedSettingMappings(project, column, setting, statusContext)
	if !ok || len(expectedMappings) == 0 {
		return coverageNone
	}
	matchedMappings := 0
	for _, mapping := range expectedMappings {
		if _, exists := currentMappings[mappingPair{Source: mapping.Source, Target: mapping.Target}]; exists {
			matchedMappings++
		}
	}
	switch {
	case matchedMappings == 0:
		return coverageNone
	case matchedMappings == len(expectedMappings):
		return coverageFull
	default:
		return coveragePartial
	}
}

// plannedSettingMappings replans one setting's complete source-target mapping set from current metadata.
func plannedSettingMappings(project warehouse.Project, column warehouse.Column, setting warehouse.Setting, statusContext listStatusContext) ([]linker.Mapping, bool) {
	if !statusContext.canPlan {
		return nil, false
	}
	mappings, err := planner.PlanColumnMappings(project, column.Name, []string{setting.Name}, statusContext.planOptions)
	if err != nil {
		return nil, false
	}
	return mappings, true
}

// sameMappingSet compares two mapping slices as unordered source-target sets.
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
		if _, exists := rightSet[pair]; !exists {
			return false
		}
	}
	return true
}

// mappingSet converts one mapping slice into a source-target lookup set.
func mappingSet(mappings []linker.Mapping) map[mappingPair]struct{} {
	currentMappings := make(map[mappingPair]struct{}, len(mappings))
	for _, mapping := range mappings {
		currentMappings[mappingPair{Source: mapping.Source, Target: mapping.Target}] = struct{}{}
	}
	return currentMappings
}

// emphasizeParenthesized colorizes one parenthesized label when the output target supports ANSI.
func emphasizeParenthesized(stdout io.Writer, emphasis cliEmphasis, label string) string {
	return emphasizeText(stdout, emphasis, fmt.Sprintf("(%s)", label))
}

// emphasizeText wraps one text fragment in ANSI color only for color-capable terminal outputs.
func emphasizeText(stdout io.Writer, emphasis cliEmphasis, text string) string {
	if text == "" || !shouldUseColor(stdout) {
		return text
	}
	return string(emphasis) + text + string(cliEmphasisReset)
}

// shouldUseColor reports whether this CLI output should emit ANSI color codes.
func shouldUseColor(stdout io.Writer) bool {
	if strings.TrimSpace(os.Getenv("NO_COLOR")) != "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") {
		return false
	}
	file, ok := stdout.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// displayLabel formats one display-facing entity label while preserving the normalized callable identifier when it differs.
func displayLabel(displayName string, canonicalID string) string {
	displayName = strings.TrimSpace(displayName)
	canonicalID = strings.TrimSpace(canonicalID)
	if displayName == "" || displayName == canonicalID {
		return canonicalID
	}
	return fmt.Sprintf("%s [%s]", displayName, canonicalID)
}

func displayStatusName(displayName string, canonicalID string) string {
	displayName = strings.TrimSpace(displayName)
	canonicalID = strings.TrimSpace(canonicalID)
	if displayName == "" {
		return canonicalID
	}
	return displayName
}

// displayModeColumn renders one mode column reference using column display metadata when available.
func displayModeColumn(project warehouse.Project, reference string) string {
	column, err := project.ResolveColumn(reference)
	if err != nil {
		return reference
	}
	return displayLabel(column.Metadata.DisplayName, column.Name)
}

// displayModeSettings renders mode setting references using setting display metadata when available.
func displayModeSettings(project warehouse.Project, columnReference string, settingReferences []string) []string {
	column, err := project.ResolveColumn(columnReference)
	if err != nil {
		return settingReferences
	}
	labels := make([]string, 0, len(settingReferences))
	for _, reference := range settingReferences {
		setting, resolveErr := column.ResolveSetting(reference)
		if resolveErr != nil {
			labels = append(labels, reference)
			continue
		}
		labels = append(labels, displayLabel(setting.Metadata.DisplayName, setting.Name))
	}
	return labels
}

func runApply(args []string, stdout io.Writer, stderr io.Writer, warehouseRoot string) int {
	parsed, err := parseApplyArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	project, err := resolveProjectForCommand(warehouseRoot, parsed.projectName)
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
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(stderr, "resolve home directory: %v\n", err)
		return 1
	}
	planOptions := planner.PlanOptions{HomeDir: homeDir, Env: envMap(), OS: runtime.GOOS}
	var mappings []linker.Mapping
	switch {
	case parsed.modeName != "" && parsed.columnName == "" && parsed.settingsInput == "":
		mode, resolveErr := project.ResolveMode(parsed.modeName)
		if resolveErr != nil {
			fmt.Fprintf(stderr, "%v\n", resolveErr)
			return 1
		}
		mappings, err = planner.PlanModeMappings(project, mode.Name, currentState.Mappings, planOptions)
		if err == nil {
			err = engine.ReplaceState(project, linker.CurrentState{Mappings: mappings, Intent: &linker.ApplyIntent{Kind: "mode", Mode: mode.Name}}, linker.WithForce(parsed.force))
		}
		if err == nil {
			fmt.Fprintf(stdout, "Applied mode %q for project %q\n", displayStatusName(mode.Metadata.DisplayName, mode.Name), displayStatusName(project.Metadata.DisplayName, project.Name))
		}
	case parsed.modeName == "" && parsed.columnName != "" && parsed.settingsInput != "":
		column, resolveErr := project.ResolveColumn(parsed.columnName)
		if resolveErr != nil {
			fmt.Fprintf(stderr, "%v\n", resolveErr)
			return 1
		}
		settingNames, resolveErr := canonicalSettingNames(column, planner.ParseSettingList(parsed.settingsInput))
		if resolveErr != nil {
			fmt.Fprintf(stderr, "%v\n", resolveErr)
			return 1
		}
		mappings, err = planner.PlanColumnMappings(project, column.Name, settingNames, planOptions)
		if err == nil {
			err = engine.ReplaceState(project, linker.CurrentState{Mappings: mappings, Intent: &linker.ApplyIntent{Kind: "column", Column: column.Name, Settings: settingNames}}, linker.WithForce(parsed.force))
		}
		if err == nil {
			fmt.Fprintf(stdout, "Applied column %q for project %q\n", displayStatusName(column.Metadata.DisplayName, column.Name), displayStatusName(project.Metadata.DisplayName, project.Name))
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

func runUpdate(args []string, stdout io.Writer, stderr io.Writer, warehouseRoot string) int {
	parsed, err := parseUpdateArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if err := validateProjectName(parsed.projectName); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if parsed.forceAll {
		return runUpdateAll(stdout, stderr, warehouseRoot, parsed.force)
	}
	project, err := resolveProjectForCommand(warehouseRoot, parsed.projectName)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	return updateProject(project, parsed.columnName, parsed.force, stdout, stderr)
}

// runUpdateAll refreshes all projects that currently have active managed mappings.
func runUpdateAll(stdout io.Writer, stderr io.Writer, warehouseRoot string, force bool) int {
	loaded, err := warehouse.LoadWarehouse(warehouseRoot)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	projectNames := sortedKeys(loaded.Projects)
	updated := 0
	for _, projectName := range projectNames {
		project := loaded.Projects[projectName]
		currentState, loadErr := linker.New().LoadCurrentState(project)
		if loadErr != nil {
			fmt.Fprintf(stderr, "%v\n", loadErr)
			return 1
		}
		if len(currentState.Mappings) == 0 && currentState.Intent == nil {
			continue
		}
		if exitCode := updateProject(project, "", force, stdout, stderr); exitCode != 0 {
			return exitCode
		}
		updated++
	}
	fmt.Fprintf(stdout, "Updated %d project(s) in %s\n", updated, warehouseRoot)
	return 0
}

// updateProject recomputes the current project mapping set and commits it through the linker engine.
func updateProject(project warehouse.Project, columnName string, force bool, stdout io.Writer, stderr io.Writer) int {
	engine := linker.New()
	currentState, err := engine.LoadCurrentState(project)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(stderr, "resolve home directory: %v\n", err)
		return 1
	}
	planOptions := planner.PlanOptions{HomeDir: homeDir, Env: envMap(), OS: runtime.GOOS}
	if columnName != "" {
		column, resolveErr := project.ResolveColumn(columnName)
		if resolveErr != nil {
			fmt.Fprintf(stderr, "%v\n", resolveErr)
			return 1
		}
		mappings, planErr := planUpdateColumnMappings(project, column.Name, currentState, planOptions)
		if planErr != nil {
			fmt.Fprintf(stderr, "%v\n", planErr)
			return 1
		}
		if replaceErr := engine.ReplaceState(project, linker.CurrentState{Mappings: mappings, Intent: cloneApplyIntent(currentState.Intent)}, linker.WithForce(force)); replaceErr != nil {
			fmt.Fprintf(stderr, "%v\n", replaceErr)
			return 1
		}
		fmt.Fprintf(stdout, "Updated column %q for project %q\n", displayStatusName(column.Metadata.DisplayName, column.Name), displayStatusName(project.Metadata.DisplayName, project.Name))
		return 0
	}
	mappings, err := planUpdateMappings(project, currentState, planOptions)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if err := engine.ReplaceState(project, linker.CurrentState{Mappings: mappings, Intent: cloneApplyIntent(currentState.Intent)}, linker.WithForce(force)); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Updated project %q\n", displayStatusName(project.Metadata.DisplayName, project.Name))
	return 0
}

// planUpdateMappings chooses intent-aware update planning when current state has persisted intent.
func planUpdateMappings(project warehouse.Project, currentState linker.CurrentState, options planner.PlanOptions) ([]linker.Mapping, error) {
	if currentState.Intent != nil {
		return planner.PlanIntentUpdateMappings(project, *currentState.Intent, currentState.Mappings, options)
	}
	return planner.PlanUpdateMappings(project, currentState.Mappings, options)
}

// planUpdateColumnMappings refreshes one column using intent metadata when available.
func planUpdateColumnMappings(project warehouse.Project, columnName string, currentState linker.CurrentState, options planner.PlanOptions) ([]linker.Mapping, error) {
	if currentState.Intent != nil {
		return planner.PlanIntentColumnUpdateMappings(project, *currentState.Intent, columnName, currentState.Mappings, options)
	}
	return planner.PlanColumnUpdateMappings(project, columnName, currentState.Mappings, options)
}

// cloneApplyIntent copies persisted apply intent before writing refreshed state.
func cloneApplyIntent(intent *linker.ApplyIntent) *linker.ApplyIntent {
	if intent == nil {
		return nil
	}
	clone := *intent
	if intent.Settings != nil {
		clone.Settings = append([]string{}, intent.Settings...)
	}
	return &clone
}

// canonicalSettingNames resolves setting references to canonical names for persisted column intent.
func canonicalSettingNames(column warehouse.Column, settingReferences []string) ([]string, error) {
	settings := make([]string, 0, len(settingReferences))
	for _, reference := range settingReferences {
		setting, err := column.ResolveSetting(reference)
		if err != nil {
			return nil, err
		}
		settings = append(settings, setting.Name)
	}
	return settings, nil
}

func runReset(args []string, stdout io.Writer, stderr io.Writer, warehouseRoot string) int {
	parsed, err := parseProjectOptionalArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	project, err := resolveProjectForCommand(warehouseRoot, parsed.projectName)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if err := linker.New().Reset(project, linker.WithForce(parsed.force)); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Reset project %q\n", displayStatusName(project.Metadata.DisplayName, project.Name))
	return 0
}

func runRevert(args []string, stdout io.Writer, stderr io.Writer, warehouseRoot string) int {
	parsed, err := parseProjectOptionalArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	project, err := resolveProjectForCommand(warehouseRoot, parsed.projectName)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	engine := linker.New()
	previous, err := engine.LoadPreviousState(project)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if err := engine.ReplaceState(project, previous, linker.WithForce(parsed.force)); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Reverted project %q\n", displayStatusName(project.Metadata.DisplayName, project.Name))
	return 0
}

// parseApplyArgs parses flags for the apply workflow.
func parseApplyArgs(args []string) (applyArgs, error) {
	var parsed applyArgs
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-p":
			if i+1 >= len(args) {
				return applyArgs{}, fmt.Errorf("-p requires a value")
			}
			parsed.projectName = args[i+1]
			i++
		case "-m":
			if i+1 >= len(args) {
				return applyArgs{}, fmt.Errorf("-m requires a value")
			}
			parsed.modeName = args[i+1]
			i++
		case "-c":
			if i+1 >= len(args) {
				return applyArgs{}, fmt.Errorf("-c requires a value")
			}
			parsed.columnName = args[i+1]
			i++
		case "-s":
			if i+1 >= len(args) {
				return applyArgs{}, fmt.Errorf("-s requires a value")
			}
			parsed.settingsInput = args[i+1]
			i++
		case "-f", "--force":
			parsed.force = true
		default:
			return applyArgs{}, fmt.Errorf("unexpected arg %q", args[i])
		}
	}
	return parsed, nil
}

func parseUpdateArgs(args []string) (updateArgs, error) {
	var parsed updateArgs
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-p", "--project":
			if i+1 >= len(args) {
				return updateArgs{}, fmt.Errorf("%s requires a value", args[i])
			}
			if parsed.forceAll {
				return updateArgs{}, fmt.Errorf("update does not accept --all or -a together with -p/--project or -c/--column")
			}
			parsed.projectName = args[i+1]
			i++
		case "-c", "--column":
			if i+1 >= len(args) {
				return updateArgs{}, fmt.Errorf("%s requires a value", args[i])
			}
			if parsed.forceAll {
				return updateArgs{}, fmt.Errorf("update does not accept --all or -a together with -p/--project or -c/--column")
			}
			parsed.columnName = args[i+1]
			i++
		case "--all", "-a":
			if parsed.projectName != "" || parsed.columnName != "" {
				return updateArgs{}, fmt.Errorf("update does not accept --all or -a together with -p/--project or -c/--column")
			}
			parsed.forceAll = true
		case "-f", "--force":
			parsed.force = true
		default:
			return updateArgs{}, fmt.Errorf("unexpected arg %q", args[i])
		}
	}
	return parsed, nil
}

// parseProjectOptionalArgs parses project-scoped command flags that may omit -p.
func parseProjectOptionalArgs(args []string) (projectOptionalArgs, error) {
	var parsed projectOptionalArgs
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-p":
			if i+1 >= len(args) {
				return projectOptionalArgs{}, fmt.Errorf("-p requires a value")
			}
			parsed.projectName = args[i+1]
			i++
		case "-f", "--force":
			parsed.force = true
		default:
			return projectOptionalArgs{}, fmt.Errorf("command accepts only -p <project> and optional -f/--force")
		}
	}
	return parsed, nil
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
	return loaded.ResolveProject(effectiveProject)
}

func resolveProjectReference(warehouseRoot string, reference string) (warehouse.Project, error) {
	loaded, err := warehouse.LoadWarehouse(warehouseRoot)
	if err != nil {
		return warehouse.Project{}, err
	}
	return loaded.ResolveProject(reference)
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
