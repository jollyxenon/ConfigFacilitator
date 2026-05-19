package pathvars

import (
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var bracePattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
var percentPattern = regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_]*)%`)

// Options controls environment-aware path expansion.
type Options struct {
	HomeDir string
	Env     map[string]string
	OS      string
}

// Expand resolves the supported path-variable forms into a concrete target path.
func Expand(input string, options Options) (string, error) {
	resolved := input
	operatingSystem := options.OS
	if operatingSystem == "" {
		operatingSystem = runtime.GOOS
	}

	if strings.HasPrefix(resolved, "~/") || resolved == "~" {
		if options.HomeDir == "" {
			return "", fmt.Errorf("cannot expand ~ without a home directory")
		}
		resolved = filepath.Join(options.HomeDir, strings.TrimPrefix(resolved, "~/"))
	}

	var err error
	resolved, err = replacePattern(resolved, bracePattern, options.Env)
	if err != nil {
		return "", err
	}

	if operatingSystem == "windows" {
		resolved, err = replacePattern(resolved, percentPattern, options.Env)
		if err != nil {
			return "", err
		}
	}

	if strings.Contains(resolved, "${") || (operatingSystem == "windows" && strings.Contains(resolved, "%")) {
		return "", fmt.Errorf("path contains unresolved variables: %s", resolved)
	}

	return resolved, nil
}

// replacePattern expands one variable pattern against the provided environment map.
func replacePattern(input string, pattern *regexp.Regexp, env map[string]string) (string, error) {
	result := input
	matches := pattern.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		variableName := match[1]
		value, ok := env[variableName]
		if !ok {
			return "", fmt.Errorf("unresolved variable %q", variableName)
		}
		result = strings.ReplaceAll(result, match[0], value)
	}
	return result, nil
}
