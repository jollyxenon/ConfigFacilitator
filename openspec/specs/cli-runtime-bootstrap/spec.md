## MODIFIED Requirements

### Requirement: The root help surface is available
The system SHALL print a usable help surface from `cfgfc --help`, and the registered CLI commands SHALL also expose usable help surfaces from `cfgfc <command> --help` and `cfgfc <command> -h` without falling through into normal command parsing. The help surface SHALL also mention the `--version` flag.

#### Scenario: Inspecting the root command
- **WHEN** a user runs `cfgfc --help`
- **THEN** the command prints the CLI usage information, including the `--version` flag, instead of failing at startup

#### Scenario: Inspecting a registered command
- **WHEN** a user runs `cfgfc <command> --help` for a registered command family
- **THEN** the CLI prints that command's help surface instead of a parser error or normal command execution path