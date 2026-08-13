package cli

import (
	"io"
	"os"
	"runtime"
	"strings"
)

// version is the current cfgfc version and may be replaced through build-time ldflags.
var version = "dev"

// Dependencies contains every process dependency used while constructing one command tree.
type Dependencies struct {
	Stdin           io.Reader
	Stdout          io.Writer
	Stderr          io.Writer
	ExecutablePath  string
	HomeDir         string
	Environment     map[string]string
	PPID            int
	OperatingSystem string
}

// Run executes cfgfc with process-provided dependencies and returns its exit code.
func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	executablePath, err := os.Executable()
	if err != nil {
		commandErr := NewPersistenceError("executable_path", "resolve executable path", err)
		renderCommandError(stderr, commandErr, requestedJSON(args))
		return commandErr.ExitCode()
	}
	return RunWithExecutable(args, stdout, stderr, executablePath)
}

// RunWithExecutable executes cfgfc with an injected executable path and process-provided runtime dependencies.
func RunWithExecutable(args []string, stdout io.Writer, stderr io.Writer, executablePath string) int {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		commandErr := NewPersistenceError("home_directory", "resolve home directory", err)
		renderCommandError(stderr, commandErr, requestedJSON(args))
		return commandErr.ExitCode()
	}
	dependencies := Dependencies{
		Stdin:           os.Stdin,
		Stdout:          stdout,
		Stderr:          stderr,
		ExecutablePath:  executablePath,
		HomeDir:         homeDir,
		Environment:     environmentMap(os.Environ()),
		PPID:            os.Getppid(),
		OperatingSystem: runtime.GOOS,
	}
	return RunWithDependencies(args, dependencies)
}

// RunWithDependencies executes cfgfc with a fully injected and reusable process boundary.
func RunWithDependencies(args []string, dependencies Dependencies) int {
	root := NewRootCommand(dependencies)
	root.SetArgs(args)
	_, err := root.ExecuteC()
	if err == nil {
		return ExitSuccess
	}
	commandErr := AsCommandError(err)
	jsonMode := requestedJSON(args)
	if flag := root.PersistentFlags().Lookup("json"); flag != nil && flag.Value.String() == "true" {
		jsonMode = true
	}
	renderCommandError(dependencies.Stderr, commandErr, jsonMode)
	return commandErr.ExitCode()
}

// environmentMap converts process environment entries into the injected lookup map.
func environmentMap(entries []string) map[string]string {
	environment := make(map[string]string, len(entries))
	for _, entry := range entries {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			environment[parts[0]] = parts[1]
		}
	}
	return environment
}
