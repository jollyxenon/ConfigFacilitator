## MODIFIED Requirements

### Requirement: CLI exposes the required command families
The system SHALL expose the command families `new`, `sync`, `switch`, `list`, `apply`, `update`, `reset`, `revert`, and `root` through the `cfgfc` CLI.

#### Scenario: Inspecting available commands
- **WHEN** a user inspects the CLI help surface
- **THEN** the required command families are present as part of the supported interface

### Requirement: Registered commands expose standardized help sections
Each registered `cfgfc` command SHALL expose a standardized help surface that documents the command purpose, accepted usage forms, supported flags or argument forms, and at least one command-specific example.

#### Scenario: Inspecting an operational command help surface
- **WHEN** a user runs `cfgfc sync --help`
- **THEN** the CLI shows a structured help response that includes the command summary, usage, supported sync forms, and at least one sync example

#### Scenario: Inspecting a project-scoped command help surface
- **WHEN** a user runs `cfgfc apply --help`
- **THEN** the CLI explains the accepted apply forms, the role of `-p` versus switched-project context, representative apply examples, and the destructive semantics of `-f` / `--force`

#### Scenario: Inspecting update help surface
- **WHEN** a user runs `cfgfc update --help`
- **THEN** the CLI explains that update refreshes the last applied intent when intent metadata is available, falls back to mapping refresh for legacy state, supports `-p` / `--project`, switched-project context, `-a` / `--all`, `-c` / `--column`, `-f` / `--force`, and representative update examples

#### Scenario: Inspecting revert help surface
- **WHEN** a user runs `cfgfc revert --help`
- **THEN** the CLI documents that `-f` / `--force` restores only the last confirmed managed snapshot and does not recover overwritten unmanaged contents

#### Scenario: Inspecting root help surface
- **WHEN** a user runs `cfgfc root --help`
- **THEN** the CLI explains the read-current-root form, the single-path set form, and that changing the root does not migrate existing warehouse contents

### Requirement: Root help explains command discovery and project-context behavior
The root `cfgfc --help` surface SHALL summarize the registered command families and explain the project-context rules and warehouse-root selection behavior that materially affect later command behavior.

#### Scenario: Inspecting the root help overview
- **WHEN** a user runs `cfgfc --help`
- **THEN** the CLI lists the registered command families and explains the switched-project, `sync --all`, `update --all`, and `root <path>` behaviors that affect command resolution

## ADDED Requirements

### Requirement: Root command manages persistent warehouse root selection
The `root` workflow SHALL report the current effective warehouse root when invoked without a path argument and SHALL persist a new effective warehouse root when invoked with exactly one path argument. Changing the configured root SHALL affect later command resolution for the same user and SHALL NOT migrate or copy any warehouse content from the previously effective root.

#### Scenario: Inspecting the current effective warehouse root
- **WHEN** a user runs `cfgfc root`
- **THEN** the CLI prints the effective warehouse root that later commands will use

#### Scenario: Persisting a new effective warehouse root
- **WHEN** a user runs `cfgfc root <WarehousePath>`
- **THEN** the CLI stores the normalized path as the effective warehouse root for later invocations
- **AND** a later warehouse command resolves projects from `<WarehousePath>` instead of the previous root

#### Scenario: Changing roots does not migrate warehouse contents
- **WHEN** a user runs `cfgfc root <WarehousePath>` and the previous effective root already contains warehouse data
- **THEN** the CLI leaves the previous root untouched
- **AND** it does not copy or move those files into `<WarehousePath>`
