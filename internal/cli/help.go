package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// applyStandardHelp adds the same structured sections to every visible command.
func applyStandardHelp(root *cobra.Command) {
	walkCommands(root, func(command *cobra.Command) {
		purpose := strings.TrimSpace(command.Long)
		if purpose == "" {
			purpose = strings.TrimSpace(command.Short)
		}
		if override := helpPurposeOverrides[command.CommandPath()]; override != "" {
			purpose = override
		}
		command.Long = fmt.Sprintf(
			"Purpose:\n  %s\n\nArguments:\n%s\n\nDestructive behavior:\n  %s",
			purpose,
			formatArgumentHelp(command),
			destructiveHelp(command),
		)
		if strings.TrimSpace(command.Example) == "" {
			command.Example = helpExample(command)
		}
	})
}

// walkCommands visits one command tree in stable parent-before-child order.
func walkCommands(command *cobra.Command, visit func(*cobra.Command)) {
	visit(command)
	for _, child := range command.Commands() {
		if child.Hidden {
			continue
		}
		walkCommands(child, visit)
	}
}

// formatArgumentHelp describes every positional form declared by Cobra Use syntax.
func formatArgumentHelp(command *cobra.Command) string {
	fields := strings.Fields(command.Use)
	if len(fields) <= 1 {
		return "  None."
	}
	lines := make([]string, 0, len(fields)-1)
	for _, field := range fields[1:] {
		if !strings.ContainsAny(field, "<[") {
			continue
		}
		name := strings.Trim(field, "<>[]")
		name = strings.TrimSuffix(name, "...")
		lines = append(lines, fmt.Sprintf("  %s  %s", field, argumentDescription(name)))
	}
	if len(lines) == 0 {
		return "  None."
	}
	return strings.Join(lines, "\n")
}

// argumentDescription explains the canonical-or-alias resource argument conventions.
func argumentDescription(name string) string {
	descriptions := map[string]string{
		"Project":                  "Project canonical name or alias.",
		"Project|global":           "Project canonical name or alias; global clears selected context.",
		"Column":                   "Column canonical name or alias in the effective Project.",
		"Setting":                  "Setting canonical name or alias in the selected Column.",
		"Mode":                     "Mode canonical name or alias in the effective Project.",
		"Old":                      "Existing canonical name or alias.",
		"New":                      "New canonical name.",
		"Index":                    "Zero-based target-position index.",
		"Path":                     "Warehouse root path to inspect or persist.",
		"RelativePath":             "Clean path below a directory-backed Setting root.",
		"OldPath":                  "Existing clean path below the Setting root.",
		"NewPath":                  "Absent destination path below the same Setting root.",
		"bash|zsh|fish|powershell": "Shell whose completion script is written to standard output.",
	}
	if description := descriptions[name]; description != "" {
		return description
	}
	return "Positional argument shown in Usage."
}

// destructiveHelp explains independent confirmation, cascade, and target controls.
func destructiveHelp(command *cobra.Command) string {
	path := command.CommandPath()
	switch path {
	case "cfgfc":
		return "--yes confirms deletion; --cascade authorizes dependent-reference cleanup; --force-targets reclaims only affected recorded targets. These controls are independent."
	case "cfgfc sync":
		return "Sync removes index entries for Projects, Columns, and Settings whose source path no longer exists."
	case "cfgfc root":
		return "Supplying Path changes future root resolution only; it never migrates, copies, or deletes existing warehouse contents."
	case "cfgfc revert":
		return "Restores only the previous snapshot. --force-targets may reclaim occupied paths recorded by that restoration."
	case "cfgfc reset":
		return "Removes current managed mappings but preserves warehouse resources. --force-targets may reclaim recorded drifted targets."
	}
	yes := command.Flag("yes") != nil
	cascade := command.Flag("cascade") != nil
	forceTargets := command.Flag("force-targets") != nil
	if yes && cascade && forceTargets {
		return "--yes confirms repository deletion; --cascade separately authorizes dependent-reference repair; --force-targets separately authorizes reclamation of affected recorded targets."
	}
	if yes {
		return "Requires --yes and never prompts interactively. The confirmed operation removes only the addressed target position or content path."
	}
	if forceTargets {
		return "Without --force-targets, occupied or drifted recorded targets block the operation; the flag authorizes reclamation only of affected recorded target paths."
	}
	if command.Runnable() && isHelpMutation(path) {
		return "Commits the described warehouse or context mutation transactionally; validation failures leave durable state unchanged."
	}
	return "None; this command only inspects data, prints help, or generates output."
}

// isHelpMutation reports commands whose successful execution writes durable state.
func isHelpMutation(path string) bool {
	for _, prefix := range []string{
		"cfgfc project create", "cfgfc project set", "cfgfc project rename", "cfgfc project delete",
		"cfgfc column create", "cfgfc column set", "cfgfc column rename", "cfgfc column delete", "cfgfc column target add", "cfgfc column target set", "cfgfc column target delete",
		"cfgfc setting create", "cfgfc setting set", "cfgfc setting rename", "cfgfc setting delete", "cfgfc setting target set", "cfgfc setting target reset", "cfgfc setting content write", "cfgfc setting content mkdir", "cfgfc setting content move", "cfgfc setting content delete",
		"cfgfc mode create", "cfgfc mode set", "cfgfc mode rename", "cfgfc mode delete", "cfgfc mode column set", "cfgfc mode column delete",
		"cfgfc use", "cfgfc apply", "cfgfc refresh", "cfgfc sync", "cfgfc reset", "cfgfc revert", "cfgfc root",
	} {
		if path == prefix || strings.HasPrefix(path, prefix+" ") {
			return true
		}
	}
	return false
}

// helpExample returns a runnable example or a discovery example for one command.
func helpExample(command *cobra.Command) string {
	if example := helpExamples[command.CommandPath()]; example != "" {
		return example
	}
	return "  " + command.CommandPath() + " --help"
}

var helpPurposeOverrides = map[string]string{
	"cfgfc":                       rootLongDescription,
	"cfgfc setting create":        "Create one file- or directory-backed Setting atomically. Choose at most one of --from, --stdin, or --text; --stdin and --text are file-only.",
	"cfgfc setting content write": "Create or atomically replace one regular file using exactly one of --from, --stdin, or --text.",
	"cfgfc apply":                 "Apply either a Mode or explicit Settings from one Column. Project scope comes from -p/--project or the PPID context selected by cfgfc use.",
	"cfgfc refresh":               "Re-plan persisted intent, legacy mapping-only state, one --column, or every active Project with --all.",
	"cfgfc sync":                  "Reconcile external filesystem changes, removing metadata for disappeared sources. Scope comes from --project, selected context, or --all.",
	"cfgfc root":                  "Print the effective warehouse root or persist a normalized replacement. Changing it does not migrate existing warehouse contents.",
}

var helpExamples = map[string]string{
	"cfgfc":                        "  cfgfc project --help\n  cfgfc use OpenCode\n  cfgfc status --json",
	"cfgfc project":                "  cfgfc project list\n  cfgfc project create OpenCode",
	"cfgfc project list":           "  cfgfc project list",
	"cfgfc project show":           "  cfgfc project show OpenCode",
	"cfgfc project create":         "  cfgfc project create OpenCode --aliases oc --description \"OpenCode configuration\"",
	"cfgfc project set":            "  cfgfc project set OpenCode --display-name \"OpenCode Config\" --clear-aliases",
	"cfgfc project rename":         "  cfgfc project rename OpenCode OpenCodeNext",
	"cfgfc project delete":         "  cfgfc project delete OpenCode --yes --cascade --force-targets",
	"cfgfc column":                 "  cfgfc column list -p OpenCode\n  cfgfc column target --help",
	"cfgfc column list":            "  cfgfc column list -p OpenCode",
	"cfgfc column show":            "  cfgfc column show Skills -p OpenCode",
	"cfgfc column create":          "  cfgfc column create Skills -p OpenCode",
	"cfgfc column set":             "  cfgfc column set Skills -p OpenCode --description \"Agent skills\"",
	"cfgfc column rename":          "  cfgfc column rename Skills AgentSkills -p OpenCode",
	"cfgfc column delete":          "  cfgfc column delete Skills -p OpenCode --yes --cascade --force-targets",
	"cfgfc column target":          "  cfgfc column target list Skills -p OpenCode",
	"cfgfc column target list":     "  cfgfc column target list Skills -p OpenCode",
	"cfgfc column target add":      "  cfgfc column target add Skills -p OpenCode --dir ~/.config/opencode/skills --name-from-setting",
	"cfgfc column target set":      "  cfgfc column target set Skills 0 -p OpenCode --dir ~/.config/opencode/skills --name-from-setting",
	"cfgfc column target delete":   "  cfgfc column target delete Skills 0 -p OpenCode --yes",
	"cfgfc setting":                "  cfgfc setting list -p OpenCode -c Skills\n  cfgfc setting content --help",
	"cfgfc setting list":           "  cfgfc setting list -p OpenCode -c Skills",
	"cfgfc setting show":           "  cfgfc setting show GPT.json -p OpenCode -c Models",
	"cfgfc setting create":         "  cfgfc setting create GPT.json -p OpenCode -c Models --kind file --stdin\n  cfgfc setting create prompts -p OpenCode -c Skills --kind directory --from ./prompts",
	"cfgfc setting set":            "  cfgfc setting set GPT.json -p OpenCode -c Models --aliases gpt",
	"cfgfc setting rename":         "  cfgfc setting rename GPT.json Primary.json -p OpenCode -c Models",
	"cfgfc setting delete":         "  cfgfc setting delete GPT.json -p OpenCode -c Models --yes --cascade --force-targets",
	"cfgfc setting target":         "  cfgfc setting target list GPT.json -p OpenCode -c Models",
	"cfgfc setting target list":    "  cfgfc setting target list GPT.json -p OpenCode -c Models",
	"cfgfc setting target set":     "  cfgfc setting target set GPT.json 0 -p OpenCode -c Models --inherit-dir --name model.json",
	"cfgfc setting target reset":   "  cfgfc setting target reset GPT.json 0 -p OpenCode -c Models",
	"cfgfc setting content":        "  cfgfc setting content list prompts -p OpenCode -c Skills",
	"cfgfc setting content list":   "  cfgfc setting content list prompts -p OpenCode -c Skills",
	"cfgfc setting content read":   "  cfgfc setting content read prompts system.md -p OpenCode -c Skills",
	"cfgfc setting content write":  "  cfgfc setting content write prompts system.md -p OpenCode -c Skills --stdin",
	"cfgfc setting content mkdir":  "  cfgfc setting content mkdir prompts system -p OpenCode -c Skills",
	"cfgfc setting content move":   "  cfgfc setting content move prompts old.md archive/old.md -p OpenCode -c Skills",
	"cfgfc setting content delete": "  cfgfc setting content delete prompts archive -p OpenCode -c Skills --yes",
	"cfgfc mode":                   "  cfgfc mode list -p OpenCode\n  cfgfc mode column --help",
	"cfgfc mode list":              "  cfgfc mode list -p OpenCode",
	"cfgfc mode show":              "  cfgfc mode show Max -p OpenCode",
	"cfgfc mode create":            "  cfgfc mode create Max -p OpenCode",
	"cfgfc mode set":               "  cfgfc mode set Max -p OpenCode --description \"Maximum configuration\"",
	"cfgfc mode rename":            "  cfgfc mode rename Max Full -p OpenCode",
	"cfgfc mode delete":            "  cfgfc mode delete Max -p OpenCode --yes --cascade --force-targets",
	"cfgfc mode column":            "  cfgfc mode column list Max -p OpenCode",
	"cfgfc mode column list":       "  cfgfc mode column list Max -p OpenCode",
	"cfgfc mode column set":        "  cfgfc mode column set Max Models -p OpenCode --strategy cover --setting GPT.json --setting Tools.json",
	"cfgfc mode column delete":     "  cfgfc mode column delete Max Models -p OpenCode",
	"cfgfc use":                    "  cfgfc use OpenCode\n  cfgfc use global",
	"cfgfc status":                 "  cfgfc status\n  cfgfc status -p OpenCode --json",
	"cfgfc apply":                  "  cfgfc apply mode Max -p OpenCode\n  cfgfc apply column Skills Skill-A Skill-B -p OpenCode",
	"cfgfc apply mode":             "  cfgfc apply mode Max -p OpenCode",
	"cfgfc apply column":           "  cfgfc apply column Skills Skill-A Skill-B -p OpenCode",
	"cfgfc refresh":                "  cfgfc refresh -p OpenCode\n  cfgfc refresh --column Skills -p OpenCode\n  cfgfc refresh --all",
	"cfgfc sync":                   "  cfgfc sync\n  cfgfc sync -p OpenCode\n  cfgfc sync --all",
	"cfgfc root":                   "  cfgfc root\n  cfgfc root ~/.configfacilitator-alt",
	"cfgfc reset":                  "  cfgfc reset -p OpenCode\n  cfgfc reset -p OpenCode --force-targets",
	"cfgfc revert":                 "  cfgfc revert -p OpenCode\n  cfgfc revert -p OpenCode --force-targets",
	"cfgfc completion":             "  cfgfc completion bash\n  cfgfc completion zsh\n  cfgfc completion fish\n  cfgfc completion powershell",
}
