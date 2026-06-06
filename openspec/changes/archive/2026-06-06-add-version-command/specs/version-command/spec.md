## ADDED Requirements

### Requirement: Version display via --version flag
The system SHALL display the current version of the cfgfc CLI when the `--version` or `-v` flag is provided.

#### Scenario: Display version with --version flag
- **WHEN** a user runs `cfgfc --version`
- **THEN** the command prints the current version number (e.g., "1.0.0") to standard output

#### Scenario: Display version with -v flag
- **WHEN** a user runs `cfgfc -v`
- **THEN** the command prints the current version number (e.g., "1.0.0") to standard output

#### Scenario: Version flag takes precedence over other arguments
- **WHEN** a user runs `cfgfc --version --help` or `cfgfc --version new`
- **THEN** the command prints the version number and exits, ignoring other arguments

### Requirement: Version injection at build time
The system SHALL support version injection at build time via Go linker flags.

#### Scenario: Build with version injection
- **WHEN** the developer builds the CLI with `-ldflags "-X ...version=X.Y.Z"`
- **THEN** the resulting binary displays "X.Y.Z" when invoked with `--version`

#### Scenario: Default version for development builds
- **WHEN** the developer builds the CLI without version injection
- **THEN** the resulting binary displays "dev" when invoked with `--version`