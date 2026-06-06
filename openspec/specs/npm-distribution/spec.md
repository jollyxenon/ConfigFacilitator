## ADDED Requirements

### Requirement: npm package exposes cfgfc command
The npm distribution SHALL expose a `cfgfc` command through npm's `bin` mechanism and SHALL forward command-line arguments to the ConfigFacilitator Go executable without changing command semantics.

#### Scenario: Global npm command forwards help
- **WHEN** a user installs the npm package globally and runs `cfgfc --help`
- **THEN** the command displays the same root help produced by the Go CLI

#### Scenario: Wrapper preserves exit status
- **WHEN** the underlying Go executable exits with a non-zero status
- **THEN** the npm wrapper exits with the same non-zero status

### Requirement: npm install selects a platform binary
The npm distribution SHALL install a prebuilt `cfgfc` executable matching the user's supported operating system and CPU architecture instead of compiling Go source during installation.

#### Scenario: Supported platform installation
- **WHEN** npm installation runs on a supported `process.platform` and `process.arch` combination
- **THEN** the install script downloads and installs the matching release binary into the package's `bin` directory

#### Scenario: Unsupported platform installation
- **WHEN** npm installation runs on an unsupported platform or architecture
- **THEN** the install script fails with a clear message identifying the unsupported platform tuple

### Requirement: Release assets support npm installation
The release process SHALL publish prebuilt `cfgfc` assets for Linux, macOS, and Windows on x64 and arm64 where supported by the Go toolchain, using a documented naming convention consumed by the npm installer.

#### Scenario: npm version matches release tag
- **WHEN** the npm package version is `X.Y.Z`
- **THEN** the install script resolves binaries from the corresponding GitHub release tag `vX.Y.Z`

#### Scenario: Missing release asset is reported
- **WHEN** the expected release asset for the current platform cannot be downloaded
- **THEN** npm installation fails with an error that includes the attempted asset URL

### Requirement: npm distribution is locally verifiable
The project SHALL document and support local verification commands for npm packaging before publication.

#### Scenario: Package contents preview
- **WHEN** a maintainer runs `npm pack --dry-run` from the npm package directory
- **THEN** npm shows only the intended package metadata, wrapper, install script, and packaged files

#### Scenario: Local npm smoke test
- **WHEN** a maintainer runs a local global install from the npm package directory and then runs `cfgfc --help`
- **THEN** the installed command executes successfully using the downloaded or locally staged binary
